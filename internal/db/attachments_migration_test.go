package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/migrations"
)

// TestMigrationAddsOfferDocuments upgrades a database shaped like the live
// one before 0018: every existing row count must stay identical, the two
// document tables must appear empty, and an upgrade database must accept
// uploads, downloads and detach afterwards.
func TestMigrationAddsOfferDocuments(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "documents.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || e.Name() >= "0018_" {
			continue
		}
		body, err := migrations.FS.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		base[e.Name()] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply pre-0018 migrations: %v", err)
	}

	now := time.Date(2026, 11, 26, 9, 0, 0, 0, time.UTC)
	company, err := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	offer, err := InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: &company.ID, Title: "SRE", TitleNormalized: "sre", Fingerprint: "pre-0018",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertProvenance(ctx, sqlDB, offer.ID, nil, nil, "https://ex.com/1", now); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertEvent(ctx, sqlDB, model.Event{
		OfferID: offer.ID, Type: model.EventCVSent, OccurredAt: now,
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityOffer, EntityID: offer.ID, Body: "keep me",
	}, now); err != nil {
		t.Fatal(err)
	}
	if err := InsertSnapshot(ctx, sqlDB, offer.ID, "child description", now); err != nil {
		t.Fatal(err)
	}

	counts := func() map[string]int {
		out := map[string]int{}
		for _, q := range []struct{ name, query string }{
			{"offers", `SELECT COUNT(*) FROM job_offers`},
			{"events", `SELECT COUNT(*) FROM events`},
			{"notes", `SELECT COUNT(*) FROM notes`},
			{"snapshots", `SELECT COUNT(*) FROM snapshots`},
			{"provenance", `SELECT COUNT(*) FROM job_offer_provenance`},
			{"companies", `SELECT COUNT(*) FROM companies`},
		} {
			out[q.name] = countRows(t, sqlDB, q.query)
		}
		return out
	}
	before := counts()

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}

	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 18 AND name = 'offer_documents'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0018 recorded %d times, want 1", recorded)
	}

	after := counts()
	for name, want := range before {
		if after[name] != want {
			t.Errorf("%s rows = %d, want %d", name, after[name], want)
		}
	}
	for _, table := range []string{"attachment_blobs", "offer_attachments"} {
		if n := countRows(t, sqlDB,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table); n != 1 {
			t.Errorf("table %s missing after 0018", table)
		} else if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM `+table); n != 0 {
			t.Errorf("table %s starts with %d rows, want 0", table, n)
		}
	}

	// The upgraded database accepts the full document lifecycle.
	content := []byte("%PDF-1.7\n1 0 obj\n<< /Title (post-upgrade) >>\nendobj\n%%EOF\n")
	att, err := AttachOfferDocument(ctx, sqlDB, offer.ID, model.AttachmentCV, "cv.pdf",
		content, model.PDFMetadata{}, now)
	if err != nil {
		t.Fatalf("attach on upgraded database: %v", err)
	}
	file, err := GetOfferAttachmentFile(ctx, sqlDB, offer.ID, att.ID)
	if err != nil {
		t.Fatalf("download on upgraded database: %v", err)
	}
	if string(file.Bytes) != string(content) {
		t.Error("download bytes changed on upgraded database")
	}
	if err := DetachOfferAttachment(ctx, sqlDB, offer.ID, att.ID); err != nil {
		t.Fatalf("detach on upgraded database: %v", err)
	}
	if n := countRows(t, sqlDB, `SELECT COUNT(*) FROM attachment_blobs`); n != 0 {
		t.Errorf("blobs after detach = %d, want 0", n)
	}
	if got := countRows(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); got != before["offers"] {
		t.Errorf("offers after document lifecycle = %d, want %d", got, before["offers"])
	}
}
