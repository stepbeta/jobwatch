package scan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
)

func TestParseImport(t *testing.T) {
	tests := []struct {
		name    string
		arg     string
		want    []ImportInput
		wantErr string
	}{
		{"url", "https://www.linkedin.com/jobs/view/123", []ImportInput{{URL: "https://www.linkedin.com/jobs/view/123"}}, ""},
		{"object", `{"company":"Acme","title":"SRE","url":"https://x/1"}`, []ImportInput{{Company: "Acme", Title: "SRE", URL: "https://x/1"}}, ""},
		{"array", `[{"company":"A","title":"T1"},{"company":"B","title":"T2"}]`, []ImportInput{{Company: "A", Title: "T1"}, {Company: "B", Title: "T2"}}, ""},
		{"unknown field", `{"company":"A","titel":"typo"}`, nil, "invalid JSON"},
		{"broken json", `{"company":`, nil, "invalid JSON"},
		{"not a url", "acme sre", nil, "neither JSON nor"},
		{"ftp url", "ftp://x/y", nil, "neither JSON nor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImport(tt.arg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestImport(t *testing.T) {
	sqlDB, _, _, _ := setup(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		in          ImportInput
		wantErr     string
		wantCreated bool
		wantSameAs  int // index of an earlier step expected to share the offer ID, -1 for none
		wantProv    int // provenance rows of the resulting offer
	}{
		{"missing fields", ImportInput{URL: "https://x/1"}, "missing company, title", false, -1, 0},
		{"bad url", ImportInput{Company: "A", Title: "T", URL: "nope"}, "invalid url", false, -1, 0},
		{"new offer", ImportInput{Company: "Qonto SAS", Title: "SRE (H/F)", Location: "Paris", URL: "https://www.linkedin.com/jobs/view/1", Description: "desc"}, "", true, -1, 1},
		{"same offer elsewhere adds provenance", ImportInput{Company: "Qonto", Title: "SRE", URL: "https://www.welcometothejungle.com/qonto/sre"}, "", false, 2, 2},
		{"same offer same url is idempotent", ImportInput{Company: "qonto", Title: "sre", URL: "https://www.linkedin.com/jobs/view/1"}, "", false, 2, 2},
		{"offer without url", ImportInput{Company: "Other", Title: "Platform Engineer"}, "", true, -1, 0},
	}
	ids := make([]int64, len(tests))
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Import(ctx, sqlDB, tt.in, now)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ids[i] = res.OfferID
			if res.Created != tt.wantCreated {
				t.Errorf("created = %v, want %v", res.Created, tt.wantCreated)
			}
			if tt.wantSameAs >= 0 && ids[tt.wantSameAs] != res.OfferID {
				t.Errorf("offer id = %d, want same as step %d (%d)", res.OfferID, tt.wantSameAs, ids[tt.wantSameAs])
			}
			prov, err := db.ListProvenance(ctx, sqlDB, res.OfferID)
			if err != nil {
				t.Fatal(err)
			}
			if len(prov) != tt.wantProv {
				t.Errorf("provenance rows = %d, want %d", len(prov), tt.wantProv)
			}
			for _, p := range prov {
				if p.SourceID != nil {
					t.Errorf("manual provenance has source_id %d", *p.SourceID)
				}
			}
		})
	}
}
