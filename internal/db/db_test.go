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
	if len(srcs) != 71 {
		t.Fatalf("seeded sources = %d, want 71 (4 known-hard + 65 from 5a/5b + BambooHR + WTTJ Fabriq)", len(srcs))
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
	if enabled != 39 {
		t.Errorf("enabled seeded sources = %d, want 39", enabled)
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
	if len(srcs) != 71 {
		t.Errorf("sources after re-apply = %d, want 71", len(srcs))
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

func TestSeed5cAudit(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	var applied int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 4 AND name = 'seed_5c_sources'`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("migration 0004 recorded %d times, want 1", applied)
	}

	srcs, err := ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 71 {
		t.Fatalf("seeded sources = %d, want 71", len(srcs))
	}
	byURL := make(map[string]model.Source, len(srcs))
	for _, src := range srcs {
		byURL[src.URL] = src
	}
	rows := []struct {
		url, name, typ, config string
		enabled                bool
	}{
		{"https://weworkremotely.com/remote-jobs.rss", "We Work Remotely", "rss", `{"location_field":"region"}`, true},
		{"https://jobs.workable.com/search?location=France&workplace=remote", "Workable (search)", "html", `{"note":"robots.txt disallows the Workable job search path."}`, false},
		{"https://www.flexjobs.com/search", "FlexJobs", "html", `{"note":"FlexJobs requires a paid membership/login and its robots.txt was unreachable; not crawled."}`, false},
		{"https://jobgether.com", "Jobgether", "html", `{"note":"Jobgether is an aggregator that requires an account and renders matches client-side."}`, false},
		{"https://www.welcometothejungle.com/fr/jobs-matches", "Welcome to the Jungle (matches)", "wttj", `{"note":"WTTJ uses an internal API with no public contract. Manual import only."}`, false},
		{"https://my.greenhouse.io/dashboard", "Greenhouse (candidate dashboard)", "html", `{"note":"Greenhouse candidate dashboard; requires a candidate login."}`, false},
		{"https://app.theproductcrew.io/", "The Product Crew", "html", `{"note":"The Product Crew is a candidate-only platform; requires login."}`, false},
		{"https://remotemore.com/candidates/", "RemoteMore", "html", `{"note":"RemoteMore is a candidate signup platform; no public job listing."}`, false},
	}
	for _, want := range rows {
		src, ok := byURL[want.url]
		if !ok {
			t.Errorf("5c source %q missing", want.url)
			continue
		}
		if src.Name != want.name || string(src.Type) != want.typ || src.Enabled != want.enabled || src.ConfigJSON != want.config {
			t.Errorf("5c source %q = %+v, want name=%q type=%q enabled=%v config=%s",
				want.url, src, want.name, want.typ, want.enabled, want.config)
		}
	}

	// 0004 is an audit migration: it has no INSERT, so re-applying it
	// (even after a user edit or deletion) must change nothing.
	body, err := migrations.FS.ReadFile("0004_seed_5c_sources.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE sources SET name = 'Edited WWR', enabled = 0 WHERE url = 'https://weworkremotely.com/remote-jobs.rss'`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM sources WHERE url = 'https://www.flexjobs.com/search'`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("re-apply 0004: %v", err)
		}
	}
	srcs, err = ListSources(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 70 {
		t.Errorf("sources after re-applying 0004 = %d, want 70 (edited row kept, deleted row not restored)", len(srcs))
	}
	for _, src := range srcs {
		switch src.URL {
		case "https://weworkremotely.com/remote-jobs.rss":
			if src.Name != "Edited WWR" || src.Enabled {
				t.Errorf("0004 modified a user-edited source: %+v", src)
			}
		case "https://www.flexjobs.com/search":
			t.Error("0004 restored a deleted source; it must contain no INSERT")
		}
	}
}

func TestSeedBambooHRSource(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 5 AND name = 'seed_bamboohr_sources'`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Fatalf("migration 0005 recorded %d times, want 1", recorded)
	}

	const url = "https://semble.bamboohr.com/careers"
	byURL := func() (model.Source, int) {
		t.Helper()
		srcs, err := ListSources(ctx, sqlDB, false)
		if err != nil {
			t.Fatal(err)
		}
		var found model.Source
		n := 0
		for _, src := range srcs {
			if src.URL == url {
				found = src
				n++
			}
		}
		return found, n
	}

	src, n := byURL()
	if n != 1 {
		t.Fatalf("Semble source rows = %d, want 1", n)
	}
	if src.Name != "Semble" || string(src.Type) != "bamboohr" || !src.Enabled {
		t.Errorf("Semble row = %+v, want name/type/enabled Semble/bamboohr/true", src)
	}
	if src.ConfigJSON != `{"subdomain":"semble"}` {
		t.Errorf("Semble config = %s, want {\"subdomain\":\"semble\"}", src.ConfigJSON)
	}

	body, err := migrations.FS.ReadFile("0005_seed_bamboohr_sources.sql")
	if err != nil {
		t.Fatal(err)
	}

	// Re-applying the migration twice neither duplicates the row nor
	// overwrites a user edit.
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE sources SET name = 'Edited Semble', config_json = '{"edited":true}', enabled = 0 WHERE url = ?`, url); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("re-apply seed migration: %v", err)
		}
	}
	src, n = byURL()
	if n != 1 || src.Name != "Edited Semble" || src.ConfigJSON != `{"edited":true}` || src.Enabled {
		t.Errorf("re-applying 0005 changed a user-edited row: %+v (rows=%d)", src, n)
	}

	// A deliberately deleted seed row is restored on the next application
	// (the seed only fills missing URLs).
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM sources WHERE url = ?`, url); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("re-apply 0005 after delete: %v", err)
	}
	src, n = byURL()
	if n != 1 || src.Name != "Semble" || string(src.Type) != "bamboohr" || !src.Enabled ||
		src.ConfigJSON != `{"subdomain":"semble"}` {
		t.Errorf("deleted Semble row not restored: %+v (rows=%d)", src, n)
	}
}

const wttjFabriqURL = "https://www.welcometothejungle.com/en/companies/fabriq/jobs"

func TestSeedWTTJSource(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 11 AND name = 'seed_wttj_sources'`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Fatalf("migration 0011 recorded %d times, want 1", recorded)
	}

	byURL := func() (model.Source, int) {
		t.Helper()
		srcs, err := ListSources(ctx, sqlDB, false)
		if err != nil {
			t.Fatal(err)
		}
		var found model.Source
		n := 0
		for _, src := range srcs {
			if src.URL == wttjFabriqURL {
				found = src
				n++
			}
		}
		return found, n
	}

	src, n := byURL()
	if n != 1 {
		t.Fatalf("Fabriq source rows = %d, want 1", n)
	}
	if src.Name != "Fabriq" || string(src.Type) != "wttj" || !src.Enabled || src.ConfigJSON != `{}` {
		t.Errorf("Fabriq row = %+v, want name/type/enabled/config Fabriq/wttj/true/{}", src)
	}

	// The migration 0002 WTTJ row has a different URL and is never
	// touched by 0011.
	var legacyName, legacyConfig string
	var legacyEnabled bool
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT name, config_json, enabled FROM sources WHERE url = ?`,
		"https://www.welcometothejungle.com/en/jobs?query=platform%20engineer").
		Scan(&legacyName, &legacyConfig, &legacyEnabled); err != nil {
		t.Fatalf("0002 WTTJ row: %v", err)
	}
	if legacyName != "Welcome to the Jungle (search)" || legacyEnabled || !strings.Contains(legacyConfig, `"note"`) {
		t.Errorf("0002 WTTJ row was modified: %q %q %v", legacyName, legacyConfig, legacyEnabled)
	}

	body, err := migrations.FS.ReadFile("0011_seed_wttj_sources.sql")
	if err != nil {
		t.Fatal(err)
	}

	// Re-applying the migration twice neither duplicates the row nor
	// overwrites a user edit.
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE sources SET name = 'Edited Fabriq', config_json = '{"edited":true}', enabled = 0 WHERE url = ?`, wttjFabriqURL); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("re-apply 0011: %v", err)
		}
	}
	src, n = byURL()
	if n != 1 || src.Name != "Edited Fabriq" || src.ConfigJSON != `{"edited":true}` || src.Enabled {
		t.Errorf("re-applying 0011 changed a user-edited row: %+v (rows=%d)", src, n)
	}

	// A deliberately deleted seed row is restored on the next application.
	if _, err := sqlDB.ExecContext(ctx, `DELETE FROM sources WHERE url = ?`, wttjFabriqURL); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
		t.Fatalf("re-apply 0011 after delete: %v", err)
	}
	src, n = byURL()
	if n != 1 || src.Name != "Fabriq" || string(src.Type) != "wttj" || !src.Enabled {
		t.Errorf("deleted Fabriq row not restored: %+v (rows=%d)", src, n)
	}
}

// TestMigrationRenamesReplyReceived applies the first five migrations,
// seeds an event with the old type and then upgrades with the real
// embedded set, proving the data-only 0006 rename fixes existing
// timelines without touching other columns.
func TestMigrationRenamesReplyReceived(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	for _, name := range []string{
		"0001_init.sql",
		"0002_known_hard_sources.sql",
		"0003_seed_remote_sources.sql",
		"0004_seed_5c_sources.sql",
		"0005_seed_bamboohr_sources.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply migrations 0001-0005: %v", err)
	}

	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	c, _, err := ResolveCompany(ctx, sqlDB, "Acme", now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: c.ID, Title: "SRE", TitleNormalized: "sre", Fingerprint: "pre-0006",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	firstContact := "first contact"
	oldEvent, err := InsertEvent(ctx, sqlDB, model.Event{
		OfferID: o.ID, Type: model.EventType("reply_received"),
		Description: &firstContact, OccurredAt: now.AddDate(0, 0, -2),
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}

	var typ, occurred string
	var description sql.NullString
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT type, occurred_at, description FROM events WHERE id = ?`, oldEvent.ID).
		Scan(&typ, &occurred, &description); err != nil {
		t.Fatal(err)
	}
	if typ != string(model.EventMessageReceived) {
		t.Errorf("event type after migration = %q, want %q", typ, model.EventMessageReceived)
	}
	if occurred != model.FormatTime(oldEvent.OccurredAt) || !description.Valid || description.String != "first contact" {
		t.Errorf("migration changed other event fields: occurred=%q description=%v", occurred, description)
	}
	var old int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM events WHERE type = 'reply_received'`).Scan(&old); err != nil {
		t.Fatal(err)
	}
	if old != 0 {
		t.Errorf("reply_received rows after migration = %d, want 0", old)
	}
	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 6 AND name = 'event_message_types'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0006 recorded %d times, want 1", recorded)
	}

	// The rename is data-only and idempotent: re-applying 0006 changes no
	// row and cannot recreate reply_received.
	body, err := migrations.FS.ReadFile("0006_event_message_types.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := sqlDB.ExecContext(ctx, string(body)); err != nil {
			t.Fatalf("re-apply 0006: %v", err)
		}
	}
	events, err := ListEvents(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != model.EventMessageReceived {
		t.Errorf("events after re-applying 0006 = %+v", events)
	}
}

// TestMigrationAddsSnapshotOrigin upgrades a pre-0007 database: existing
// snapshots keep their text/timestamps and become scan-origin rows, and
// new rows can record manual origin.
func TestMigrationAddsSnapshotOrigin(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snapshots.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	for _, name := range []string{
		"0001_init.sql",
		"0002_known_hard_sources.sql",
		"0003_seed_remote_sources.sql",
		"0004_seed_5c_sources.sql",
		"0005_seed_bamboohr_sources.sql",
		"0006_event_message_types.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply migrations 0001-0006: %v", err)
	}

	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	c, _, err := ResolveCompany(ctx, sqlDB, "Acme", now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: c.ID, Title: "SRE", TitleNormalized: "sre", Fingerprint: "pre-0007",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	// Pre-0007 snapshots have no origin column yet.
	oldRows := []struct {
		text string
		at   time.Time
	}{
		{"first version", now},
		{"second version", now.Add(10 * time.Minute)},
	}
	for _, row := range oldRows {
		if _, err := sqlDB.ExecContext(ctx,
			`INSERT INTO snapshots(offer_id, raw_text, captured_at) VALUES (?, ?, ?)`,
			o.ID, row.text, model.FormatTime(row.at)); err != nil {
			t.Fatal(err)
		}
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}

	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 7 AND name = 'snapshot_origin'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0007 recorded %d times, want 1", recorded)
	}

	rows, err := sqlDB.QueryContext(ctx,
		`SELECT raw_text, captured_at, origin FROM snapshots WHERE offer_id = ? ORDER BY id`, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var n int
	for rows.Next() {
		var text, captured, origin string
		if err := rows.Scan(&text, &captured, &origin); err != nil {
			t.Fatal(err)
		}
		want := oldRows[n]
		if text != want.text || captured != model.FormatTime(want.at) || origin != string(model.SnapshotOriginScan) {
			t.Errorf("snapshot %d after migration = (%q, %q, %q), want (%q, %q, scan)",
				n, text, captured, origin, want.text, model.FormatTime(want.at))
		}
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if n != len(oldRows) {
		t.Errorf("snapshots after migration = %d, want %d", n, len(oldRows))
	}

	// The column default keeps old-style inserts working, and an explicit
	// manual insert is stored as such.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO snapshots(offer_id, raw_text, captured_at) VALUES (?, 'defaulted', ?)`,
		o.ID, model.FormatTime(now.Add(20*time.Minute))); err != nil {
		t.Fatal(err)
	}
	if err := InsertSnapshotOrigin(ctx, sqlDB, o.ID, "typed manually", model.SnapshotOriginManual, now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := countSnapshotsByOrigin(t, sqlDB, o.ID); got[model.SnapshotOriginScan] != 3 || got[model.SnapshotOriginManual] != 1 {
		t.Errorf("origins after new inserts = %v, want 3 scan and 1 manual", got)
	}
}

// TestMigrationAddsOfferSavedAt upgrades a pre-0008 database: existing
// offers keep their columns and read as not saved (saved_at NULL), and the
// bookmark can be set and cleared afterwards.
func TestMigrationAddsOfferSavedAt(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "saved.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	for _, name := range []string{
		"0001_init.sql",
		"0002_known_hard_sources.sql",
		"0003_seed_remote_sources.sql",
		"0004_seed_5c_sources.sql",
		"0005_seed_bamboohr_sources.sql",
		"0006_event_message_types.sql",
		"0007_snapshot_origin.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply migrations 0001-0007: %v", err)
	}

	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	c, _, err := ResolveCompany(ctx, sqlDB, "Acme", now)
	if err != nil {
		t.Fatal(err)
	}
	o, err := InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID: c.ID, Title: "SRE", TitleNormalized: "sre", Fingerprint: "pre-0008",
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}

	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 8 AND name = 'offer_saved_at'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0008 recorded %d times, want 1", recorded)
	}

	got, err := GetOffer(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SavedAt != nil {
		t.Errorf("pre-0008 offer saved_at = %v, want NULL", got.SavedAt)
	}
	if got.Title != "SRE" || got.Fingerprint != "pre-0008" {
		t.Errorf("migration changed offer identity: %+v", got)
	}

	savedAt := now.Add(time.Hour)
	if err := UpdateOfferFields(ctx, sqlDB, o.ID, OfferUpdate{Saved: boolPtr(true)}, savedAt); err != nil {
		t.Fatal(err)
	}
	got, err = GetOffer(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SavedAt == nil || !got.SavedAt.Equal(savedAt) {
		t.Errorf("saved_at = %v, want %v", got.SavedAt, savedAt)
	}
	if err := UpdateOfferFields(ctx, sqlDB, o.ID, OfferUpdate{Saved: boolPtr(false)}, savedAt.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err = GetOffer(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SavedAt != nil {
		t.Errorf("saved_at after clearing = %v, want NULL", got.SavedAt)
	}
}

// TestMigrationAddsInterviewEntries upgrades a pre-0009 database: the new
// table appears empty (no existing row is rewritten) and accepts entries
// afterwards.
func TestMigrationAddsInterviewEntries(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "interview.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	for _, name := range []string{
		"0001_init.sql",
		"0002_known_hard_sources.sql",
		"0003_seed_remote_sources.sql",
		"0004_seed_5c_sources.sql",
		"0005_seed_bamboohr_sources.sql",
		"0006_event_message_types.sql",
		"0007_snapshot_origin.sql",
		"0008_offer_saved_at.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply migrations 0001-0008: %v", err)
	}
	var before int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'interview_entries'`).
		Scan(&before); err != nil {
		t.Fatal(err)
	}
	if before != 0 {
		t.Fatalf("interview_entries exists before 0009")
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}
	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 9 AND name = 'interview_entries'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0009 recorded %d times, want 1", recorded)
	}

	entries, err := ListInterviewEntries(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("upgraded database has %d interview entries, want 0", len(entries))
	}

	now := time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)
	created, err := InsertInterviewEntry(ctx, sqlDB, model.InterviewEntry{
		Question: "  What is your on-call experience?  ",
		Answer:   "Two years of weekly rotation.",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if created.Question != "What is your on-call experience?" || created.AnswerFormat != model.AnswerFormatMarkdown {
		t.Errorf("created entry = %+v", created)
	}
	got, err := GetInterviewEntry(ctx, sqlDB, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "Two years of weekly rotation." || got.DisplayOrder != nil {
		t.Errorf("stored entry = %+v", got)
	}
}

// TestMigrationAddsInterviewEntryCategory upgrades a 0009 database that
// already contains an entry: the row survives untouched and reads as
// uncategorized (NULL), and re-applying the migrations is a no-op.
func TestMigrationAddsInterviewEntryCategory(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "category.db")
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	base := fstest.MapFS{}
	for _, name := range []string{
		"0001_init.sql",
		"0002_known_hard_sources.sql",
		"0003_seed_remote_sources.sql",
		"0004_seed_5c_sources.sql",
		"0005_seed_bamboohr_sources.sql",
		"0006_event_message_types.sql",
		"0007_snapshot_origin.sql",
		"0008_offer_saved_at.sql",
		"0009_interview_entries.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply migrations 0001-0009: %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO interview_entries(question, answer, answer_format, display_order, created_at, updated_at)
		 VALUES ('Old question', 'old answer', 'markdown', 7, '2026-11-16T09:00:00Z', '2026-11-16T09:00:00Z')`); err != nil {
		t.Fatalf("insert pre-0010 row: %v", err)
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}
	entries, err := ListInterviewEntries(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("upgraded database has %d entries, want 1", len(entries))
	}
	got := entries[0]
	if got.Question != "Old question" || got.Answer != "old answer" ||
		got.AnswerFormat != model.AnswerFormatMarkdown || got.DisplayOrder == nil || *got.DisplayOrder != 7 {
		t.Errorf("existing row changed by 0010: %+v", got)
	}
	if got.Category != nil {
		t.Errorf("existing row category = %q, want NULL", *got.Category)
	}

	// Re-applying the embedded set must not rerun 0010 or duplicate rows.
	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("re-apply embedded migrations: %v", err)
	}
	var recorded int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schema_migrations WHERE version = 10 AND name = 'interview_entry_category'`).
		Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != 1 {
		t.Errorf("migration 0010 recorded %d times, want 1", recorded)
	}
	entries, err = ListInterviewEntries(ctx, sqlDB)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != got.ID {
		t.Errorf("entries after re-apply = %+v", entries)
	}

	// The upgraded table accepts a category (trimmed on insert).
	category := " Behavioural "
	created, err := InsertInterviewEntry(ctx, sqlDB, model.InterviewEntry{
		Question: "New question",
		Category: &category,
	}, time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if created.Category == nil || *created.Category != "Behavioural" {
		t.Errorf("created category = %v, want Behavioural", created.Category)
	}
}

func boolPtr(v bool) *bool { return &v }

func countSnapshotsByOrigin(t *testing.T, sqlDB *sql.DB, offerID int64) map[model.SnapshotOrigin]int {
	t.Helper()
	rows, err := sqlDB.Query(`SELECT origin, COUNT(*) FROM snapshots WHERE offer_id = ? GROUP BY origin`, offerID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[model.SnapshotOrigin]int{}
	for rows.Next() {
		var origin string
		var n int
		if err := rows.Scan(&origin, &n); err != nil {
			t.Fatal(err)
		}
		out[model.SnapshotOrigin(origin)] = n
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
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
