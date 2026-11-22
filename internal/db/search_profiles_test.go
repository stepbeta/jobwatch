package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/migrations"
)

// TestMigrationSearchProfiles checks the additive 0015 migration on a
// fresh database: the three profile tables, the seeded default profile
// with the exact hardcoded term tables, the remote selection, and the
// untouched legacy score columns.
func TestMigrationSearchProfiles(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	for _, table := range []string{"search_profiles", "search_profile_terms", "search_profile_remote"} {
		var n int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("table %s missing", table)
		}
	}

	var name, titleLabel, locationLabel, scale string
	var isDefault int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT name, title_label, location_label, bucket_scale, is_default FROM search_profiles`).
		Scan(&name, &titleLabel, &locationLabel, &scale, &isDefault); err != nil {
		t.Fatalf("read default profile: %v", err)
	}
	if name != "Default" || titleLabel != "Platform Engineer / SRE" ||
		locationLabel != "France / full remote" || scale != "five" || isDefault != 1 {
		t.Errorf("default profile = (%q, %q, %q, %q, %d)", name, titleLabel, locationLabel, scale, isDefault)
	}

	var profiles int
	if err := sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM search_profiles`).Scan(&profiles); err != nil {
		t.Fatal(err)
	}
	if profiles != 1 {
		t.Errorf("seeded %d profiles, want 1", profiles)
	}

	// The seed is a one-to-one copy of the removed classifier tables:
	// 35 title, 79 location and 15 description terms.
	wantCounts := map[string]int{"title": 35, "location": 79, "description": 15}
	wantSums := map[string]int{"title": -835, "location": -870, "description": 120}
	for affinity, want := range wantCounts {
		var n, sum int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT COUNT(*), COALESCE(SUM(weight), 0) FROM search_profile_terms WHERE affinity = ?`,
			affinity).Scan(&n, &sum); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s terms = %d, want %d", affinity, n, want)
		}
		if sum != wantSums[affinity] {
			t.Errorf("%s weight sum = %d, want %d", affinity, sum, wantSums[affinity])
		}
	}

	// Spot-check one term per bucket and affinity, including the
	// custom weights inside a bucket.
	for _, tc := range []struct {
		affinity string
		term     string
		bucket   string
		weight   int
	}{
		{"title", "platform engineer", "strong_positive", 60},
		{"title", "kubernetes", "mild_positive", 25},
		{"title", "linux", "mild_positive", 15},
		{"title", "manager", "mild_negative", -20},
		{"title", "marketing", "strong_negative", -120},
		{"location", "france", "strong_positive", 35},
		{"location", "paris", "mild_positive", 25},
		{"location", "emea", "mild_positive", 20},
		{"location", "utc 1", "mild_positive", 10},
		{"location", "united kingdom", "mild_negative", -20},
		{"location", "spain", "mild_negative", -15},
		{"location", "usa", "strong_negative", -35},
		{"description", "france", "mild_positive", 20},
		{"description", "paris", "mild_positive", 15},
		{"description", "cet", "mild_positive", 5},
	} {
		var bucket string
		var weight int
		err := sqlDB.QueryRowContext(ctx,
			`SELECT bucket, weight FROM search_profile_terms WHERE affinity = ? AND term = ?`,
			tc.affinity, tc.term).Scan(&bucket, &weight)
		if err != nil {
			t.Errorf("%s term %q: %v", tc.affinity, tc.term, err)
			continue
		}
		if bucket != tc.bucket || weight != tc.weight {
			t.Errorf("%s term %q = (%q, %d), want (%q, %d)",
				tc.affinity, tc.term, bucket, weight, tc.bucket, tc.weight)
		}
	}

	rows, err := sqlDB.QueryContext(ctx, `SELECT policy, accepted FROM search_profile_remote ORDER BY policy`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[model.RemotePolicy]bool{}
	for rows.Next() {
		var policy string
		var accepted int
		if err := rows.Scan(&policy, &accepted); err != nil {
			t.Fatal(err)
		}
		got[model.RemotePolicy(policy)] = accepted == 1
	}
	want := map[model.RemotePolicy]bool{
		model.RemoteFull: true, model.RemoteUnknown: true,
		model.RemoteHybrid: false, model.RemoteOnsite: false,
	}
	for policy, accepted := range want {
		value, ok := got[policy]
		if !ok || value != accepted {
			t.Errorf("remote policy %q accepted = %v (present %v), want %v", policy, value, ok, accepted)
		}
	}
	if len(got) != len(want) {
		t.Errorf("remote rows = %d, want %d", len(got), len(want))
	}

	// Legacy score columns stay in place, unused.
	for _, col := range []string{"title_confidence", "location_confidence", "match_reasons_json"} {
		if !hasColumn(t, sqlDB, "job_offers", col) {
			t.Errorf("legacy column job_offers.%s was dropped", col)
		}
	}

	// The partial unique index allows only one default profile.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO search_profiles(name, bucket_scale, is_default, created_at, updated_at)
		 VALUES ('Other', 'five', 1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err == nil {
		t.Error("inserting a second default profile succeeded, want unique-index error")
	}
}

// TestMigrationSearchProfilesFromLegacyDatabase upgrades a database
// shaped like the live one before the change, with stored legacy scores,
// and checks that no offer row is rewritten.
func TestMigrationSearchProfilesFromLegacyDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
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
		"0010_interview_entry_category.sql",
		"0011_seed_wttj_sources.sql",
		"0012_company_identity.sql",
		"0013_company_resolution.sql",
		"0014_wwr_employer_rule.sql",
	} {
		body, err := migrations.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		base[name] = &fstest.MapFile{Data: body}
	}
	if err := Migrate(ctx, sqlDB, base); err != nil {
		t.Fatalf("apply pre-change migrations: %v", err)
	}

	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO companies(id, name, name_normalized) VALUES (1, 'Acme', 'acme')`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO job_offers(
		company_id, title, title_normalized, location, remote_policy, fingerprint,
		first_seen_at, last_seen_at, title_confidence, location_confidence, match_reasons_json)
		VALUES (1, 'SRE', 'sre', 'Paris', 'remote', 'fp-1',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z', 70, 55, '{"title":["sre"],"location":[]}')`); err != nil {
		t.Fatal(err)
	}

	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		t.Fatalf("upgrade with embedded migrations: %v", err)
	}

	var offers int
	var titleConf, locConf int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*), MAX(title_confidence), MAX(location_confidence) FROM job_offers`).
		Scan(&offers, &titleConf, &locConf); err != nil {
		t.Fatal(err)
	}
	if offers != 1 || titleConf != 70 || locConf != 55 {
		t.Errorf("offers=%d title_confidence=%d location_confidence=%d, want 1/70/55", offers, titleConf, locConf)
	}

	var profile int
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM search_profiles WHERE is_default = 1`).Scan(&profile); err != nil {
		t.Fatal(err)
	}
	if profile != 1 {
		t.Errorf("default profiles after upgrade = %d, want 1", profile)
	}
}

func hasColumn(t *testing.T, sqlDB *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := sqlDB.QueryContext(context.Background(), `PRAGMA table_info(`+table+`)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}

func TestSearchProfilesCRUD(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTest(t)

	def, err := DefaultSearchProfile(ctx, sqlDB)
	if err != nil {
		t.Fatalf("DefaultSearchProfile: %v", err)
	}
	if !def.IsDefault || def.Name != "Default" {
		t.Fatalf("default profile = %+v", def)
	}
	if len(def.Terms) != 129 {
		t.Errorf("default profile terms = %d, want 129", len(def.Terms))
	}
	if !def.Accepts(model.RemoteFull) || !def.Accepts(model.RemoteUnknown) {
		t.Error("default profile must accept remote and unknown")
	}
	if def.Accepts(model.RemoteHybrid) || def.Accepts(model.RemoteOnsite) {
		t.Error("default profile must not accept hybrid or onsite")
	}

	if _, err := GetSearchProfile(ctx, sqlDB, def.ID); err != nil {
		t.Fatalf("GetSearchProfile: %v", err)
	}
	if _, err := GetSearchProfile(ctx, sqlDB, 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSearchProfile(missing) = %v, want ErrNotFound", err)
	}

	// A second, non-default profile is stored and listed after the
	// default one.
	if _, err := sqlDB.ExecContext(ctx,
		`INSERT INTO search_profiles(name, bucket_scale, is_default, created_at, updated_at)
		 VALUES ('Contracting', 'three', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	profiles, err := ListSearchProfiles(ctx, sqlDB)
	if err != nil {
		t.Fatalf("ListSearchProfiles: %v", err)
	}
	if len(profiles) != 2 || !profiles[0].IsDefault || profiles[0].ID != def.ID {
		t.Fatalf("profiles = %+v, want default first then the second profile", profiles)
	}

	// Replace the default profile: empty term list is allowed, labels
	// and remote selection change, is_default is preserved.
	updated := model.SearchProfile{
		ID:            def.ID,
		Name:          "Platform / SRE",
		TitleLabel:    "Platform roles",
		LocationLabel: "France",
		BucketScale:   model.BucketScaleThree,
		Terms:         []model.SearchProfileTerm{},
		Remote: []model.SearchProfileRemote{
			{Policy: model.RemoteFull, Accepted: true},
			{Policy: model.RemoteUnknown, Accepted: false},
		},
	}
	got, err := UpdateSearchProfile(ctx, sqlDB, updated, testNow)
	if err != nil {
		t.Fatalf("UpdateSearchProfile: %v", err)
	}
	if !got.IsDefault || got.Name != "Platform / SRE" || got.TitleLabel != "Platform roles" ||
		got.BucketScale != model.BucketScaleThree {
		t.Errorf("updated profile = %+v", got)
	}
	if len(got.Terms) != 0 {
		t.Errorf("terms after empty replace = %d, want 0", len(got.Terms))
	}
	if !got.Accepts(model.RemoteFull) || got.Accepts(model.RemoteUnknown) {
		t.Errorf("remote selection after replace = %+v", got.Remote)
	}

	// Invalid input is rejected and leaves the stored profile unchanged.
	for _, tc := range []struct {
		name   string
		change func(*model.SearchProfile)
	}{
		{"empty term", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{{Affinity: model.AffinityTitle, Bucket: model.BucketPositive, Term: "  ", Weight: 10}}
		}},
		{"neutral with weight", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{{Affinity: model.AffinityTitle, Bucket: model.BucketNeutral, Term: "sre", Weight: 10}}
		}},
		{"weighted bucket without weight", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{{Affinity: model.AffinityTitle, Bucket: model.BucketPositive, Term: "sre", Weight: 0}}
		}},
		{"bucket outside scale", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{{Affinity: model.AffinityTitle, Bucket: model.BucketStrongPositive, Term: "sre", Weight: 60}}
		}},
		{"duplicate term", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{
				{Affinity: model.AffinityTitle, Bucket: model.BucketPositive, Term: "SRE", Weight: 40},
				{Affinity: model.AffinityTitle, Bucket: model.BucketPositive, Term: "sre", Weight: 30},
			}
		}},
		{"invalid affinity", func(p *model.SearchProfile) {
			p.Terms = []model.SearchProfileTerm{{Affinity: "company", Bucket: model.BucketPositive, Term: "acme", Weight: 5}}
		}},
		{"invalid policy", func(p *model.SearchProfile) {
			p.Remote = []model.SearchProfileRemote{{Policy: "sometimes", Accepted: true}}
		}},
		{"empty name", func(p *model.SearchProfile) { p.Name = "   " }},
		{"invalid scale", func(p *model.SearchProfile) { p.BucketScale = "seven" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := got
			bad.Terms = nil
			tc.change(&bad)
			if _, err := UpdateSearchProfile(ctx, sqlDB, bad, testNow); !errors.Is(err, ErrInvalidProfile) {
				t.Fatalf("UpdateSearchProfile(%s) = %v, want ErrInvalidProfile", tc.name, err)
			}
			stored, err := GetSearchProfile(ctx, sqlDB, def.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.Name != "Platform / SRE" || stored.BucketScale != model.BucketScaleThree ||
				len(stored.Terms) != 0 || !stored.Accepts(model.RemoteFull) {
				t.Errorf("profile changed after rejected update: %+v", stored)
			}
		})
	}

	if _, err := UpdateSearchProfile(ctx, sqlDB, model.SearchProfile{ID: 9999, Name: "x", BucketScale: model.BucketScaleFive}, testNow); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateSearchProfile(missing) = %v, want ErrNotFound", err)
	}
}
