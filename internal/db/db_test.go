package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/migrations"
)

func openTest(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return sqlDB
}

func TestOpenPragmasAndSchema(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()

	var mode string
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
	var fk int
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatal(err)
	}
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}

	for _, table := range []string{
		"schema_migrations", "companies", "sources", "job_offers", "job_offer_provenance",
		"events", "notes", "snapshots", "job_offers_fts", "notes_fts",
	} {
		var n int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing", table)
		}
	}
}

func TestSeededKnownSources(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	srcs, err := ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"Datadog (Workday)": true, "Cisco (Workday)": true, "SAP (SuccessFactors)": true, "Welcome to the Jungle (search)": true}
	for _, src := range srcs {
		if !want[src.Name] {
			continue
		}
		delete(want, src.Name)
		if src.Enabled {
			t.Errorf("seeded source %q is enabled", src.Name)
		}
		if !strings.Contains(src.ConfigJSON, "note") {
			t.Errorf("seeded source %q has no note: %s", src.Name, src.ConfigJSON)
		}
	}
	if len(want) > 0 {
		t.Errorf("missing seeded sources: %v", want)
	}

	// Re-applying the migration file is harmless.
	before := len(srcs)
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM sources WHERE name = 'SAP (SuccessFactors)'`); err != nil {
		t.Fatal(err)
	}
	body, err := migrations.FS.ReadFile("0002_known_hard_sources.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("re-apply seed migration: %v", err)
	}
	srcs, err = ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != before {
		t.Errorf("sources after re-apply = %d, want %d", len(srcs), before)
	}
	found := false
	for _, src := range srcs {
		if src.Name == "SAP (SuccessFactors)" {
			found = true
		}
	}
	if !found {
		t.Error("SAP (SuccessFactors) was not restored by re-applying 0002")
	}
}

func TestSeededRemoteSources(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	srcs, err := ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 69 {
		t.Fatalf("seeded sources = %d, want 69 (4 known-hard + 65 from 5a/5b)", len(srcs))
	}
	byURL := make(map[string]model.Source, len(srcs))
	enabled := 0
	for _, src := range srcs {
		if _, dup := byURL[src.URL]; dup {
			t.Errorf("duplicate seeded source URL %q", src.URL)
		}
		byURL[src.URL] = src
		if src.Enabled {
			enabled++
		}
	}
	if enabled != 37 {
		t.Errorf("enabled seeded sources = %d, want 37", enabled)
	}

	check := func(url, typ, configPart string, wantEnabled bool) {
		t.Helper()
		src, ok := byURL[url]
		if !ok {
			t.Fatalf("seeded source %q missing", url)
		}
		if string(src.Type) != typ {
			t.Errorf("%s type = %q, want %q", url, src.Type, typ)
		}
		if src.Enabled != wantEnabled {
			t.Errorf("%s enabled = %v, want %v", url, src.Enabled, wantEnabled)
		}
		if !strings.Contains(src.ConfigJSON, configPart) {
			t.Errorf("%s config = %s, want it to contain %s", url, src.ConfigJSON, configPart)
		}
	}
	check("https://job-boards.greenhouse.io/grafanalabs/", "greenhouse", `"board_token":"grafanalabs"`, true)
	check("https://careers.epicompany.eu/jobs.rss", "rss", `"location_field":"locations/location/name"`, true)
	check("https://murena.com/jobs/", "html", `"item_selector":"table#jobs_table`, true)
	check("https://careers.achievetestprep.com/jobs/Careers", "html", `"note"`, false)
	check("https://www.welcometothejungle.com/fr/jobs-matches", "wttj", `"note"`, false)

	// Re-applying the data migration must neither duplicate rows nor
	// overwrite a user edit; a deleted seed row comes back.
	body, err := migrations.FS.ReadFile("0003_seed_remote_sources.sql")
	if err != nil {
		t.Fatal(err)
	}
	editedURL := "https://job-boards.greenhouse.io/grafanalabs/"
	deletedURL := "https://jobs.lever.co/malt"
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE sources SET name = 'Edited Grafana', config_json = '{"edited":true}', enabled = 0 WHERE url = ?`, editedURL); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM sources WHERE url = ?`, deletedURL); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("re-apply remote seed migration: %v", err)
	}
	srcs, err = ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 69 {
		t.Errorf("sources after re-apply = %d, want 69", len(srcs))
	}
	for _, src := range srcs {
		switch src.URL {
		case editedURL:
			if src.Name != "Edited Grafana" || src.ConfigJSON != `{"edited":true}` || src.Enabled {
				t.Errorf("user-edited source was overwritten: %+v", src)
			}
		case deletedURL:
			if !src.Enabled {
				t.Errorf("deleted seed source not restored enabled: %+v", src)
			}
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	var n int
	if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("migration 1 recorded %d times", n)
	}
}

func TestMigrateRunner(t *testing.T) {
	tests := []struct {
		name    string
		files   fstest.MapFS
		wantErr string
		tables  []string
	}{
		{
			name: "ordered apply ignores other files",
			files: fstest.MapFS{
				"0002_b.sql":   {Data: []byte(`CREATE TABLE b (id INTEGER REFERENCES a(id));`)},
				"0001_a.sql":   {Data: []byte(`CREATE TABLE a (id INTEGER PRIMARY KEY);`)},
				"README.md":    {Data: []byte(`not a migration`)},
				"bad_name.sql": {Data: []byte(`garbage`)},
			},
			tables: []string{"a", "b"},
		},
		{
			name: "duplicate version",
			files: fstest.MapFS{
				"0001_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER);`)},
				"0001_b.sql": {Data: []byte(`CREATE TABLE b (id INTEGER);`)},
			},
			wantErr: "duplicate migration version",
		},
		{
			name: "failing migration rolls back",
			files: fstest.MapFS{
				"0001_a.sql": {Data: []byte(`CREATE TABLE a (id INTEGER); THIS IS NOT SQL;`)},
			},
			wantErr: "apply migration 0001_a.sql",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			sqlDB, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "m.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer sqlDB.Close()
			err = Migrate(ctx, sqlDB, tt.files)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Migrate error = %v, want containing %q", err, tt.wantErr)
				}
				var n int
				_ = sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'a'`).Scan(&n)
				if n != 0 && tt.name == "failing migration rolls back" {
					t.Error("table a exists after failed migration")
				}
				return
			}
			if err != nil {
				t.Fatalf("Migrate: %v", err)
			}
			for _, tbl := range tt.tables {
				var n int
				if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = ?`, tbl).Scan(&n); err != nil || n != 1 {
					t.Errorf("table %s missing (err=%v)", tbl, err)
				}
			}
		})
	}
}

func TestMigrateRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO schema_migrations VALUES (9999, 'future', '2030-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	err := Migrate(ctx, sqlDB, migrations.FS)
	if err == nil || !strings.Contains(err.Error(), "unknown to this binary") {
		t.Fatalf("Migrate error = %v, want unknown-migration error", err)
	}
}

func TestFTSTriggers(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	c, _, err := ResolveCompany(ctx, sqlDB, "Acme SAS", now)
	if err != nil {
		t.Fatal(err)
	}
	raw := "Kubernetes clusters and Terraform"
	o, err := InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: c.ID, Title: "Ingénieur Plateforme", TitleNormalized: "ingenieur plateforme",
		Fingerprint: "fp1", RawText: &raw,
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	match := func(table, q string) int {
		t.Helper()
		var n int
		if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE `+table+` MATCH ?`, q).Scan(&n); err != nil {
			t.Fatalf("fts query %q: %v", q, err)
		}
		return n
	}

	if got := match("job_offers_fts", "kubernetes"); got != 1 {
		t.Errorf("after insert: kubernetes matches = %d, want 1", got)
	}
	if got := match("job_offers_fts", "ingenieur"); got != 1 {
		t.Errorf("diacritics folding: ingenieur matches = %d, want 1", got)
	}

	newRaw := "Nomad and Consul"
	if err := UpdateOfferSighting(ctx, sqlDB, o.ID, model.OfferChanged, &newRaw, nil, now); err != nil {
		t.Fatal(err)
	}
	if got := match("job_offers_fts", "kubernetes"); got != 0 {
		t.Errorf("after update: kubernetes matches = %d, want 0", got)
	}
	if got := match("job_offers_fts", "consul"); got != 1 {
		t.Errorf("after update: consul matches = %d, want 1", got)
	}

	res, err := sqlDB.ExecContext(ctx, `INSERT INTO notes(entity_type, entity_id, body) VALUES ('company', ?, 'recruiter likes Go')`, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	noteID, _ := res.LastInsertId()
	if got := match("notes_fts", "recruiter"); got != 1 {
		t.Errorf("notes insert: matches = %d, want 1", got)
	}
	if _, err := sqlDB.ExecContext(ctx, `UPDATE notes SET body = 'hiring manager' WHERE id = ?`, noteID); err != nil {
		t.Fatal(err)
	}
	if got := match("notes_fts", "recruiter"); got != 0 {
		t.Errorf("notes update: old term matches = %d, want 0", got)
	}
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, noteID); err != nil {
		t.Fatal(err)
	}
	if got := match("notes_fts", "hiring"); got != 0 {
		t.Errorf("notes delete: matches = %d, want 0", got)
	}
}

func TestResolveCompany(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	now := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		in          string
		wantCreated bool
		wantNorm    string
	}{
		{"first insert", "Datadog, Inc.", true, "datadog"},
		{"exact normalized match", "DATADOG", false, "datadog"},
		{"different company", "Qonto SAS", true, "qonto"},
		{"suffix variant", "Qonto", false, "qonto"},
	}
	ids := map[string]int64{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, created, err := ResolveCompany(ctx, sqlDB, tt.in, now)
			if err != nil {
				t.Fatal(err)
			}
			if created != tt.wantCreated || c.NameNormalized != tt.wantNorm {
				t.Errorf("got created=%v norm=%q, want %v %q", created, c.NameNormalized, tt.wantCreated, tt.wantNorm)
			}
			if prev, ok := ids[c.NameNormalized]; ok && prev != c.ID {
				t.Errorf("same normalized name resolved to different IDs %d and %d", prev, c.ID)
			}
			ids[c.NameNormalized] = c.ID
		})
	}
	if _, _, err := ResolveCompany(ctx, sqlDB, "  ", now); err == nil {
		t.Error("empty company name accepted")
	}
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)
	if _, _, err := ResolveCompany(ctx, sqlDB, "Acme", time.Now()); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := Backup(ctx, sqlDB, dest); err != nil {
		t.Fatalf("Backup: %v", err)
	}
	copyDB, err := Open(ctx, dest)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	if _, err := GetCompanyByNormalized(ctx, copyDB, "acme"); err != nil {
		t.Errorf("company missing from backup: %v", err)
	}
	if err := Backup(ctx, sqlDB, dest); err == nil {
		t.Error("Backup overwrote an existing file")
	}
	if err := ValidateBackup(ctx, dest); err != nil {
		t.Errorf("ValidateBackup: %v", err)
	}
	garbage := filepath.Join(t.TempDir(), "garbage.db")
	if err := os.WriteFile(garbage, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateBackup(ctx, garbage); err == nil {
		t.Error("ValidateBackup accepted a non-database file")
	}
	if _, err := os.Stat(filepath.Join(t.TempDir(), "missing.db")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stat missing file: %v", err)
	}
	if _, err := GetSource(ctx, sqlDB, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSource missing = %v, want ErrNotFound", err)
	}
}
