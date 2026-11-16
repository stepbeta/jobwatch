package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

func runCLI(t *testing.T, dbPath string, args ...string) (string, error) {
	t.Helper()
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(append([]string{"--db", dbPath}, args...))
	err := root.Execute()
	return buf.String(), err
}

func openCLITestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	sqlDB, err := db.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	// Drop the disabled seed rows from migration 0002 so the assertions
	// only see sources created by the test.
	if _, err := sqlDB.ExecContext(context.Background(), `DELETE FROM sources`); err != nil {
		t.Fatalf("clear sources: %v", err)
	}
	return sqlDB, path
}

type wantSource struct {
	name, typ, url string
	enabled        bool
}

func TestParseSourceList(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []sourceListLine
	}{
		{"empty", "", nil},
		{"comments and blanks", "# a\n\n   # b\n\t\n", nil},
		{"plain urls", "https://a.example/jobs\nhttps://b.example/jobs\n", []sourceListLine{
			{1, "https://a.example/jobs"}, {2, "https://b.example/jobs"},
		}},
		{"inline comment", "https://a.example/jobs # note\n", []sourceListLine{
			{1, "https://a.example/jobs"},
		}},
		{"inline comment with tab", "https://a.example/jobs\t# note\n", []sourceListLine{
			{1, "https://a.example/jobs"},
		}},
		{"url fragment kept", "https://a.example/jobs#openings\n", []sourceListLine{
			{1, "https://a.example/jobs#openings"},
		}},
		{"fragment kept, trailing comment stripped", "https://a.example/jobs#openings # note\n", []sourceListLine{
			{1, "https://a.example/jobs#openings"},
		}},
		{"line numbers count skipped lines", "# x\n\nhttps://a.example/jobs\n", []sourceListLine{
			{3, "https://a.example/jobs"},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSourceList(strings.NewReader(tt.in))
			if err != nil {
				t.Fatalf("parseSourceList: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseSourceList = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAddSources(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		seed       []model.Source
		disabled   bool
		wantAdded  int
		wantExist  int
		wantNotDet int
		want       []wantSource
		wantOutput []string
	}{
		{
			name:       "mixed file skips comments, duplicates and unknown urls",
			file:       "mixed.txt",
			wantAdded:  3,
			wantExist:  2,
			wantNotDet: 3,
			want: []wantSource{
				{"acme", "greenhouse", "https://boards.greenhouse.io/acme", true},
				{"acme", "lever", "https://jobs.lever.co/acme#openings", true},
				{"other", "ashby", "https://jobs.ashbyhq.com/other", true},
			},
			wantOutput: []string{
				"read 8 URL(s) from",
				"added 3, already registered 2, not detected 3",
				"duplicate of line 4",
				"duplicate of line 5",
				"duplicate of line 9 (not detected)",
				"no known ATS board URL",
				"not-a-url",
				"https://careers.example.com/jobs",
				"NOT DETECTED URLs",
			},
		},
		{
			name: "url already in the database",
			file: "registered.txt",
			seed: []model.Source{{
				Name: "acme", URL: "https://boards.greenhouse.io/acme",
				Type: model.SourceGreenhouse, ConfigJSON: `{"board_token":"acme"}`, Enabled: true,
			}},
			wantAdded: 1,
			wantExist: 1,
			want: []wantSource{
				{"acme", "greenhouse", "https://boards.greenhouse.io/acme", true},
				{"acme", "lever", "https://jobs.lever.co/acme", true},
			},
			wantOutput: []string{"already registered", "https://boards.greenhouse.io/acme"},
		},
		{
			name:       "comments only",
			file:       "comments-only.txt",
			wantOutput: []string{"read 0 URL(s)", "ADDED (0)", "ALREADY REGISTERED (0)", "NOT DETECTED (0)"},
		},
		{
			name:      "disabled flag",
			file:      "disabled.txt",
			disabled:  true,
			wantAdded: 1,
			want: []wantSource{
				{"other", "ashby", "https://jobs.ashbyhq.com/other", false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			sqlDB, dbPath := openCLITestDB(t)
			for _, s := range tt.seed {
				if _, err := db.InsertSource(ctx, sqlDB, s, time.Now()); err != nil {
					t.Fatalf("seed source: %v", err)
				}
			}

			args := []string{"add-sources", filepath.Join("testdata", "source-lists", tt.file)}
			if tt.disabled {
				args = append(args, "--disabled")
			}
			out, err := runCLI(t, dbPath, args...)
			if err != nil {
				t.Fatalf("add-sources: %v\n%s", err, out)
			}

			summary := fmt.Sprintf("added %d, already registered %d, not detected %d",
				tt.wantAdded, tt.wantExist, tt.wantNotDet)
			for _, want := range append(tt.wantOutput, summary) {
				if !strings.Contains(out, want) {
					t.Errorf("output does not contain %q:\n%s", want, out)
				}
			}

			sources, err := db.ListSources(ctx, sqlDB, false)
			if err != nil {
				t.Fatalf("list sources: %v", err)
			}
			if len(sources) != len(tt.want) {
				t.Fatalf("sources = %d, want %d: %+v", len(sources), len(tt.want), sources)
			}
			for i, want := range tt.want {
				got := sources[i]
				if got.Name != want.name || string(got.Type) != want.typ || got.URL != want.url || got.Enabled != want.enabled {
					t.Errorf("source %d = %s/%s/%s enabled=%v, want %s/%s/%s enabled=%v",
						i, got.Name, got.Type, got.URL, got.Enabled, want.name, want.typ, want.url, want.enabled)
				}
			}
		})
	}
}
