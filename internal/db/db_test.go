package db

import (
	"context"
	"database/sql"
	"errors"
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
	if _, err := GetSource(ctx, sqlDB, 42); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSource missing = %v, want ErrNotFound", err)
	}
}
