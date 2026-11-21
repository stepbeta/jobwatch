package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// fakeFetcher returns, per source ID, the configured offers or error.
type fakeFetcher struct {
	offers map[int64][]sources.RawOffer
	errs   map[int64]error
}

func (f *fakeFetcher) Fetch(_ context.Context, src model.Source) ([]sources.RawOffer, error) {
	if err := f.errs[src.ID]; err != nil {
		return nil, err
	}
	return f.offers[src.ID], nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }
func (c *clock) advance()       { c.t = c.t.Add(time.Hour) }

func setup(t *testing.T) (*sql.DB, *Scanner, *fakeFetcher, *clock) {
	t.Helper()
	sqlDB, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	// Isolate each test from the sources seeded by migrations 0002/0003.
	if _, err := sqlDB.ExecContext(context.Background(), `DELETE FROM sources`); err != nil {
		t.Fatalf("clear sources: %v", err)
	}
	ff := &fakeFetcher{offers: map[int64][]sources.RawOffer{}, errs: map[int64]error{}}
	c := &clock{t: time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)}
	return sqlDB, &Scanner{DB: sqlDB, Fetcher: ff, Now: c.now}, ff, c
}

func addSource(t *testing.T, sqlDB *sql.DB, name, cfg string) model.Source {
	t.Helper()
	src, err := db.InsertSource(context.Background(), sqlDB, model.Source{
		Name: name, URL: "https://example.com/" + name, Type: model.SourceGreenhouse, ConfigJSON: cfg, Enabled: true,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func offerByTitle(t *testing.T, sqlDB *sql.DB, title string) model.JobOffer {
	t.Helper()
	var id int64
	if err := sqlDB.QueryRow(`SELECT id FROM job_offers WHERE title = ?`, title).Scan(&id); err != nil {
		t.Fatalf("offer %q: %v", title, err)
	}
	o, err := db.GetOffer(context.Background(), sqlDB, id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func count(t *testing.T, sqlDB *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

var (
	rawSRE = sources.RawOffer{ExternalUID: "1", Title: "Site Reliability Engineer", Location: "Paris, France",
		URL: "https://ex.com/jobs/1", Description: "Run Kubernetes."}
	rawSREChanged = sources.RawOffer{ExternalUID: "1", Title: "Site Reliability Engineer", Location: "Paris, France",
		URL: "https://ex.com/jobs/1", Description: "Run Kubernetes and Terraform."}
	rawPlatform = sources.RawOffer{ExternalUID: "2", Title: "Platform Engineer (H/F)", Location: "Remote - France",
		URL: "https://ex.com/jobs/2", Description: "Build the platform."}
)

func TestScanLifecycle(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	src := addSource(t, sqlDB, "Acme", `{"company":"Acme SAS"}`)
	ctx := context.Background()

	type wantOffer struct {
		title     string
		state     model.OfferState
		snapshots int
	}
	steps := []struct {
		name       string
		offers     []sources.RawOffer
		fetchErr   error
		wantStatus model.ScanStatus
		want       Result
		offers2    []wantOffer
	}{
		{
			name:       "first scan creates offers and snapshots",
			offers:     []sources.RawOffer{rawSRE, rawPlatform},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 2, New: 2},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 1},
				{"Platform Engineer (H/F)", model.OfferActive, 1},
			},
		},
		{
			name:       "rescan with same content is a plain sighting",
			offers:     []sources.RawOffer{rawSRE, rawPlatform},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 2, Seen: 2},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 1},
				{"Platform Engineer (H/F)", model.OfferActive, 1},
			},
		},
		{
			name:       "description edit and missing offer",
			offers:     []sources.RawOffer{rawSREChanged},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 1, Seen: 1, Disappeared: 1},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "fetch error leaves offers untouched",
			fetchErr:   errors.New("connection refused"),
			wantStatus: model.ScanError,
			want:       Result{},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "blocked source",
			fetchErr:   fmt.Errorf("robots: %w", sources.ErrBlocked),
			wantStatus: model.ScanBlocked,
			want:       Result{},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "disappeared offer comes back, description edit stays active",
			offers:     []sources.RawOffer{rawSREChanged, rawPlatform},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 2, Seen: 2, Reappeared: 1},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferActive, 2},
				{"Platform Engineer (H/F)", model.OfferActive, 1},
			},
		},
		{
			name:       "empty board marks everything disappeared",
			offers:     nil,
			wantStatus: model.ScanOK,
			want:       Result{Disappeared: 2},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferDisappeared, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			clk.advance()
			ff.offers[src.ID] = st.offers
			ff.errs[src.ID] = st.fetchErr

			fresh, err := db.GetSource(ctx, sqlDB, src.ID)
			if err != nil {
				t.Fatal(err)
			}
			got := sc.ScanSource(ctx, fresh)
			if got.Status != st.wantStatus {
				t.Fatalf("status = %q (err %v), want %q", got.Status, got.Err, st.wantStatus)
			}
			if (got.Err != nil) != (st.fetchErr != nil) {
				t.Errorf("err = %v", got.Err)
			}
			st.want.SourceID, st.want.SourceName, st.want.Status, st.want.Err = got.SourceID, got.SourceName, got.Status, got.Err
			if got != st.want {
				t.Errorf("result = %+v, want %+v", got, st.want)
			}

			after, _ := db.GetSource(ctx, sqlDB, src.ID)
			if after.LastScanStatus == nil || *after.LastScanStatus != st.wantStatus {
				t.Errorf("last_scan_status = %v, want %q", after.LastScanStatus, st.wantStatus)
			}
			if after.LastScanAt == nil || !after.LastScanAt.Equal(clk.t) {
				t.Errorf("last_scan_at = %v, want %v", after.LastScanAt, clk.t)
			}
			for _, w := range st.offers2 {
				o := offerByTitle(t, sqlDB, w.title)
				if o.OfferState != w.state {
					t.Errorf("%s: state = %q, want %q", w.title, o.OfferState, w.state)
				}
				if n, _ := db.CountSnapshots(ctx, sqlDB, o.ID); n != w.snapshots {
					t.Errorf("%s: snapshots = %d, want %d", w.title, n, w.snapshots)
				}
			}
		})
	}

	// Offers are never deleted and use the configured company.
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 2 {
		t.Errorf("offers = %d, want 2", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM companies WHERE name_normalized = 'acme'`); n != 1 {
		t.Errorf("acme company rows = %d, want 1", n)
	}
	sre := offerByTitle(t, sqlDB, "Site Reliability Engineer")
	if sre.RawText == nil || *sre.RawText != rawSREChanged.Description {
		t.Errorf("raw_text = %v, want latest description", sre.RawText)
	}
	if !sre.FirstSeenAt.Before(sre.LastSeenAt) {
		t.Errorf("first_seen %v not before last_seen %v", sre.FirstSeenAt, sre.LastSeenAt)
	}
}

func offerByProvenance(t *testing.T, sqlDB *sql.DB, sourceID int64, uid string) model.JobOffer {
	t.Helper()
	var id int64
	if err := sqlDB.QueryRow(
		`SELECT offer_id FROM job_offer_provenance WHERE source_id = ? AND external_uid = ?`,
		sourceID, uid).Scan(&id); err != nil {
		t.Fatalf("offer for uid %q: %v", uid, err)
	}
	o, err := db.GetOffer(context.Background(), sqlDB, id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func offerByURL(t *testing.T, sqlDB *sql.DB, sourceID int64, url string) model.JobOffer {
	t.Helper()
	var id int64
	if err := sqlDB.QueryRow(
		`SELECT offer_id FROM job_offer_provenance WHERE source_id = ? AND url = ?`,
		sourceID, url).Scan(&id); err != nil {
		t.Fatalf("offer for url %q: %v", url, err)
	}
	o, err := db.GetOffer(context.Background(), sqlDB, id)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// TestScanIdentity covers the provenance identity contract: the same
// (source, uid) keeps one offer through title, location, company-name and
// description changes; different uids or a missing uid fall back to the
// source URL; and a disappeared offer reopens under the same identity.
func TestScanIdentity(t *testing.T) {
	type want struct{ New, Seen, Reappeared, Disappeared int }
	type step struct {
		offers []sources.RawOffer
		want   want
	}
	raw := func(uid, title, location, url, desc string) sources.RawOffer {
		return sources.RawOffer{ExternalUID: uid, Title: title, Location: location, URL: url, Description: desc}
	}
	tests := []struct {
		name  string
		steps []step
		check func(t *testing.T, sqlDB *sql.DB, src model.Source)
	}{
		{
			name: "title edit under the same uid updates the offer",
			steps: []step{
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "run it")}, want{New: 1}},
				{[]sources.RawOffer{raw("1", "Senior SRE", "Paris", "https://x/1", "run it")}, want{Seen: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
					t.Errorf("offers = %d, want 1", n)
				}
				o := offerByProvenance(t, sqlDB, src.ID, "1")
				if o.Title != "Senior SRE" || o.OfferState != model.OfferActive {
					t.Errorf("offer = %+v, want Senior SRE/active", o)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 1 {
					t.Errorf("snapshots = %d, want 1", n)
				}
			},
		},
		{
			name: "same-title siblings with different uids are two offers",
			steps: []step{
				{[]sources.RawOffer{
					raw("1", "SRE", "Paris", "https://x/1", "one"),
					raw("2", "SRE", "Paris", "https://x/2", "two"),
				}, want{New: 2}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 2 {
					t.Errorf("offers = %d, want 2", n)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE source_id = ?`, src.ID); n != 2 {
					t.Errorf("provenance = %d, want 2", n)
				}
			},
		},
		{
			name: "closed then reopened keeps the same offer",
			steps: []step{
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "d")}, want{New: 1}},
				{nil, want{Disappeared: 1}},
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "d")}, want{Seen: 1, Reappeared: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
					t.Errorf("offers = %d, want 1", n)
				}
				o := offerByProvenance(t, sqlDB, src.ID, "1")
				if o.OfferState != model.OfferActive {
					t.Errorf("state = %q, want active", o.OfferState)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 1 {
					t.Errorf("snapshots = %d, want 1", n)
				}
			},
		},
		{
			name: "a different company name under the same uid does not move or duplicate the offer",
			steps: []step{
				{[]sources.RawOffer{{ExternalUID: "1", Company: "Acme", Title: "SRE", Location: "Paris", URL: "https://x/1"}}, want{New: 1}},
				{[]sources.RawOffer{{ExternalUID: "1", Company: "Acme Careers", Title: "SRE", Location: "Paris", URL: "https://x/1"}}, want{Seen: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
					t.Errorf("offers = %d, want 1", n)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM companies`); n != 1 {
					t.Errorf("companies = %d, want 1 (the renamed company was not invented)", n)
				}
				var company string
				if err := sqlDB.QueryRow(`SELECT c.name FROM job_offers o JOIN companies c ON c.id = o.company_id`).Scan(&company); err != nil {
					t.Fatal(err)
				}
				if company != "Acme" {
					t.Errorf("company = %q, want Acme", company)
				}
			},
		},
		{
			name: "no uid falls back to the source url",
			steps: []step{
				{[]sources.RawOffer{raw("", "SRE", "Paris", "https://x/1", "d")}, want{New: 1}},
				{[]sources.RawOffer{raw("", "Senior SRE", "Paris", "https://x/1", "d")}, want{Seen: 1}},
				{[]sources.RawOffer{raw("", "SRE", "Paris", "https://x/2", "d")}, want{New: 1, Disappeared: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 2 {
					t.Errorf("offers = %d, want 2", n)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE source_id = ?`, src.ID); n != 2 {
					t.Errorf("provenance = %d, want 2", n)
				}
				if o := offerByURL(t, sqlDB, src.ID, "https://x/1"); o.Title != "Senior SRE" {
					t.Errorf("title at /1 = %q, want Senior SRE", o.Title)
				}
			},
		},
		{
			name: "description edit keeps the offer active and adds a snapshot",
			steps: []step{
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "one")}, want{New: 1}},
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "two")}, want{Seen: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				o := offerByProvenance(t, sqlDB, src.ID, "1")
				if o.OfferState != model.OfferActive {
					t.Errorf("state = %q, want active", o.OfferState)
				}
				if o.RawText == nil || *o.RawText != "two" {
					t.Errorf("raw_text = %v, want two", o.RawText)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 2 {
					t.Errorf("snapshots = %d, want 2", n)
				}
			},
		},
		{
			name: "a new url under the same uid keeps one provenance row",
			steps: []step{
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/1", "d")}, want{New: 1}},
				{[]sources.RawOffer{raw("1", "SRE", "Paris", "https://x/2", "d")}, want{Seen: 1}},
			},
			check: func(t *testing.T, sqlDB *sql.DB, src model.Source) {
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
					t.Errorf("offers = %d, want 1", n)
				}
				if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE source_id = ?`, src.ID); n != 1 {
					t.Errorf("provenance = %d, want 1", n)
				}
				var url string
				if err := sqlDB.QueryRow(`SELECT url FROM job_offer_provenance WHERE source_id = ? AND external_uid = '1'`, src.ID).Scan(&url); err != nil {
					t.Fatal(err)
				}
				if url != "https://x/1" {
					t.Errorf("provenance url = %q, want the first-seen https://x/1", url)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, sc, ff, clk := setup(t)
			src := addSource(t, sqlDB, "Acme", `{"company":"Acme"}`)
			ctx := context.Background()
			for i, st := range tt.steps {
				clk.advance()
				ff.offers[src.ID] = st.offers
				got := sc.ScanSource(ctx, src)
				if got.Status != model.ScanOK {
					t.Fatalf("step %d: status = %q (err %v)", i, got.Status, got.Err)
				}
				if got.New != st.want.New || got.Seen != st.want.Seen ||
					got.Reappeared != st.want.Reappeared || got.Disappeared != st.want.Disappeared {
					t.Fatalf("step %d: result = %+v, want %+v", i, got, st.want)
				}
			}
			tt.check(t, sqlDB, src)
		})
	}
}

// TestScanIdentityIncludesSource proves that the source is part of the
// identity: the same uid on two sources is two offers until an explicit
// merge links them.
func TestScanIdentityIncludesSource(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()
	s1 := addSource(t, sqlDB, "One", `{"company":"Acme"}`)
	s2 := addSource(t, sqlDB, "Two", `{"company":"Acme"}`)

	clk.advance()
	ff.offers[s1.ID] = []sources.RawOffer{rawSRE}
	if r := sc.ScanSource(ctx, s1); r.New != 1 {
		t.Fatalf("first source = %+v, want 1 new", r)
	}
	clk.advance()
	ff.offers[s2.ID] = []sources.RawOffer{rawSRE}
	if r := sc.ScanSource(ctx, s2); r.New != 1 {
		t.Fatalf("second source = %+v, want 1 new", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 2 {
		t.Errorf("offers = %d, want 2", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance`); n != 2 {
		t.Errorf("provenance = %d, want 2", n)
	}
}

// TestScanDoesNotInventCompanyFromSourceName covers task 3.2: the source
// name is no longer a company fallback. An item with no company evidence
// and no config.company is stored incomplete; a declared company is used.
func TestScanDoesNotInventCompanyFromSourceName(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()

	src := addSource(t, sqlDB, "Acme", "")
	ff.offers[src.ID] = []sources.RawOffer{{ExternalUID: "1", Title: "SRE", URL: "https://x/1"}}
	clk.advance()
	r := sc.ScanSource(ctx, src)
	if r.Status != model.ScanOK || r.Skipped != 0 || r.New != 0 || r.Incomplete != 1 {
		t.Fatalf("scan without company = %+v, want 1 incomplete", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM companies`); n != 0 {
		t.Errorf("companies = %d, want 0 (the source name was not used)", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
		t.Errorf("offers = %d, want 1 incomplete offer", n)
	}
	var companyID sql.NullInt64
	var reason, raw string
	if err := sqlDB.QueryRow(`SELECT company_id, incomplete_reason, incomplete_raw FROM job_offers`).
		Scan(&companyID, &reason, &raw); err != nil {
		t.Fatal(err)
	}
	if companyID.Valid || reason != string(model.IncompleteMissingCompany) {
		t.Errorf("incomplete offer company/reason = %v/%q, want NULL/missing_company", companyID, reason)
	}
	if !strings.Contains(raw, "https://x/1") || !strings.Contains(raw, "SRE") {
		t.Errorf("incomplete_raw = %s, want the reported title and url", raw)
	}

	declared := addSource(t, sqlDB, "Beta", `{"company":"Beta SAS"}`)
	ff.offers[declared.ID] = []sources.RawOffer{{ExternalUID: "1", Title: "SRE", URL: "https://x/2"}}
	clk.advance()
	r = sc.ScanSource(ctx, declared)
	if r.Status != model.ScanOK || r.New != 1 || r.Skipped != 0 || r.Incomplete != 0 {
		t.Fatalf("scan with config.company = %+v, want 1 new", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM companies WHERE name_normalized = 'beta'`); n != 1 {
		t.Errorf("declared company rows = %d, want 1", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 2 {
		t.Errorf("offers = %d, want 2 (one incomplete, one normal)", n)
	}
}

// TestScanIncompleteItems covers task 5: an item missing a title is stored
// with its known company and raw values, a rescan does not duplicate it, and
// a user repair survives later scans (including a later complete sighting).
func TestScanIncompleteItems(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()
	src := addSource(t, sqlDB, "Acme", `{"company":"Acme"}`)

	raw := sources.RawOffer{
		ExternalUID: "1", Title: "", Location: "Paris", URL: "https://x/1", Description: "desc",
	}
	ff.offers[src.ID] = []sources.RawOffer{raw}
	clk.advance()
	r := sc.ScanSource(ctx, src)
	if r.Status != model.ScanOK || r.Incomplete != 1 || r.New != 0 || r.Skipped != 0 {
		t.Fatalf("first scan = %+v, want 1 incomplete", r)
	}
	var id int64
	var companyID sql.NullInt64
	var reason, rawJSON, title string
	if err := sqlDB.QueryRow(
		`SELECT id, company_id, title, incomplete_reason, incomplete_raw FROM job_offers`).
		Scan(&id, &companyID, &title, &reason, &rawJSON); err != nil {
		t.Fatal(err)
	}
	if !companyID.Valid || title != "" || reason != string(model.IncompleteMissingTitle) {
		t.Errorf("incomplete offer company/title/reason = %v/%q/%q", companyID, title, reason)
	}
	if !strings.Contains(rawJSON, "https://x/1") || !strings.Contains(rawJSON, `"company":"Acme"`) {
		t.Errorf("incomplete_raw = %s, want the reported values", rawJSON)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE offer_id = ?`, id); n != 1 {
		t.Errorf("provenance = %d, want 1", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, id); n != 1 {
		t.Errorf("snapshots = %d, want 1", n)
	}

	// A rescan of the same incomplete item only refreshes the sighting.
	clk.advance()
	r = sc.ScanSource(ctx, src)
	if r.Incomplete != 0 || r.Seen != 1 || r.New != 0 {
		t.Fatalf("rescan = %+v, want 1 seen", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
		t.Errorf("offers after rescan = %d, want 1", n)
	}

	// Repair assigns the company and title, clears the reason and keeps the
	// raw values as the durable marker.
	loc := "Paris, France"
	repaired, err := db.RepairOffer(ctx, sqlDB, id, companyID.Int64, "Site Reliability Engineer", &loc, clk.now())
	if err != nil {
		t.Fatalf("RepairOffer: %v", err)
	}
	if repaired.Title != "Site Reliability Engineer" || repaired.IncompleteReason != "" ||
		repaired.IncompleteRaw == nil || repaired.CompanyID == nil {
		t.Errorf("repaired offer = %+v", repaired)
	}

	// A later scan of the same (still unnormalized) item must not overwrite
	// the repair.
	clk.advance()
	ff.offers[src.ID] = []sources.RawOffer{raw}
	r = sc.ScanSource(ctx, src)
	if r.Seen != 1 || r.Incomplete != 0 {
		t.Fatalf("scan after repair = %+v, want 1 seen", r)
	}
	got := offerByProvenance(t, sqlDB, src.ID, "1")
	if got.Title != "Site Reliability Engineer" || got.IncompleteReason != "" {
		t.Errorf("repaired offer after rescan = %+v", got)
	}

	// Even a complete sighting must not replace the user's title.
	clk.advance()
	ff.offers[src.ID] = []sources.RawOffer{{
		ExternalUID: "1", Title: "Source Title", Location: "Lyon", URL: "https://x/1",
	}}
	r = sc.ScanSource(ctx, src)
	if r.Seen != 1 || r.Incomplete != 0 {
		t.Fatalf("complete sighting after repair = %+v, want 1 seen", r)
	}
	got = offerByProvenance(t, sqlDB, src.ID, "1")
	if got.Title != "Site Reliability Engineer" || got.Location == nil || *got.Location != loc {
		t.Errorf("repaired title/location overwritten: %+v", got)
	}
}

func TestScanAgainstManualDescription(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	src := addSource(t, sqlDB, "Acme", `{"company":"Acme"}`)
	ctx := context.Background()

	ff.offers[src.ID] = []sources.RawOffer{rawSRE}
	if got := sc.ScanSource(ctx, src); got.New != 1 {
		t.Fatalf("first scan = %+v, want 1 new", got)
	}
	o := offerByTitle(t, sqlDB, rawSRE.Title)
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 1 {
		t.Fatalf("snapshots after first scan = %d, want 1", n)
	}

	// A manual edit replaces the text, recomputes the hash and versions it.
	manual := "Manually pasted description."
	clk.advance()
	if err := db.UpdateOfferFields(ctx, sqlDB, o.ID, db.OfferUpdate{Description: &manual}, clk.now()); err != nil {
		t.Fatal(err)
	}
	o = offerByTitle(t, sqlDB, rawSRE.Title)
	if o.RawText == nil || *o.RawText != manual || o.TextHash == nil || *o.TextHash != model.TextHash(manual) {
		t.Fatalf("after manual edit raw_text=%v text_hash=%v", o.RawText, o.TextHash)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 2 {
		t.Fatalf("snapshots after manual edit = %d, want 2", n)
	}

	// A sighting without a description keeps the manual text and state.
	clk.advance()
	noDesc := rawSRE
	noDesc.Description = ""
	ff.offers[src.ID] = []sources.RawOffer{noDesc}
	if got := sc.ScanSource(ctx, src); got.Seen != 1 {
		t.Fatalf("empty-description scan = %+v, want 1 seen", got)
	}
	o = offerByTitle(t, sqlDB, rawSRE.Title)
	if o.RawText == nil || *o.RawText != manual {
		t.Errorf("sighting without description cleared manual text: %v", o.RawText)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 2 {
		t.Errorf("snapshots after empty-description scan = %d, want 2", n)
	}

	// A later source description that differs from the manual text is not
	// special-cased: it records a snapshot and keeps the offer active.
	clk.advance()
	ff.offers[src.ID] = []sources.RawOffer{rawSREChanged}
	if got := sc.ScanSource(ctx, src); got.Seen != 1 {
		t.Fatalf("differing-description scan = %+v, want 1 seen", got)
	}
	o = offerByTitle(t, sqlDB, rawSRE.Title)
	if o.OfferState != model.OfferActive {
		t.Errorf("offer_state = %q, want active", o.OfferState)
	}
	if o.RawText == nil || *o.RawText != rawSREChanged.Description {
		t.Errorf("raw_text = %v, want the source description", o.RawText)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o.ID); n != 3 {
		t.Errorf("snapshots after source change = %d, want 3", n)
	}
}

func TestScanDedup(t *testing.T) {
	tests := []struct {
		name          string
		offers        []sources.RawOffer
		wantOffers    int
		wantProv      int
		wantFirstLocs []string // locations of created offers, in ID order
	}{
		{
			name: "same title different location buckets split",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", Location: "New York, NY", URL: "https://x/a"},
				{ExternalUID: "b", Title: "SRE", Location: "London", URL: "https://x/b"},
				{ExternalUID: "c", Title: "SRE", Location: "Toronto", URL: "https://x/c"},
			},
			wantOffers: 3, wantProv: 3,
			wantFirstLocs: []string{"New York, NY", "London", "Toronto"},
		},
		{
			name: "same title different uids are separate offers",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", Location: "Paris", URL: "https://x/a", Description: "one"},
				{ExternalUID: "b", Title: "SRE (F/H)", Location: "Paris, France", URL: "https://x/b", Description: "two"},
			},
			wantOffers: 2, wantProv: 2,
			wantFirstLocs: []string{"Paris", "Paris, France"},
		},
		{
			name: "unknown location with a different uid is a separate offer",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", Location: "Paris", URL: "https://x/a"},
				{ExternalUID: "b", Title: "SRE", Location: "", URL: "https://x/b"},
			},
			wantOffers: 2, wantProv: 2,
			wantFirstLocs: []string{"Paris", ""},
		},
		{
			name: "same url twice is one provenance row",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", URL: "https://x/a"},
				{ExternalUID: "a", Title: "SRE", URL: "https://x/a"},
			},
			wantOffers: 1, wantProv: 1,
			wantFirstLocs: []string{""},
		},
		{
			name: "missing title is stored incomplete, missing url is skipped",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "", URL: "https://x/a"},
				{ExternalUID: "b", Title: "SRE", URL: ""},
			},
			wantOffers: 1, wantProv: 1,
			wantFirstLocs: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, sc, ff, _ := setup(t)
			src := addSource(t, sqlDB, "Acme", `{"company":"Acme"}`)
			ff.offers[src.ID] = tt.offers
			res := sc.ScanSource(context.Background(), src)
			if res.Status != model.ScanOK {
				t.Fatalf("scan: %v", res.Err)
			}
			if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != tt.wantOffers {
				t.Errorf("offers = %d, want %d", n, tt.wantOffers)
			}
			if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE source_id = ?`, src.ID); n != tt.wantProv {
				t.Errorf("provenance = %d, want %d", n, tt.wantProv)
			}
			rows, err := sqlDB.Query(`SELECT COALESCE(location, '') FROM job_offers ORDER BY id`)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var locs []string
			for rows.Next() {
				var l string
				rows.Scan(&l)
				locs = append(locs, l)
			}
			if strings.Join(locs, "|") != strings.Join(tt.wantFirstLocs, "|") {
				t.Errorf("locations = %q, want %q", locs, tt.wantFirstLocs)
			}

			// A second identical scan must be stable: no new offers, no changes.
			res2 := sc.ScanSource(context.Background(), src)
			if res2.New != 0 || res2.Disappeared != 0 {
				t.Errorf("rescan not stable: %+v", res2)
			}
		})
	}
}

func TestScanCrossSourceDisappearance(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()
	s1 := addSource(t, sqlDB, "Acme careers", `{"company":"Acme"}`)
	s2 := addSource(t, sqlDB, "Aggregator", "")

	// The company site creates the offer.
	ff.offers[s1.ID] = []sources.RawOffer{rawSRE}
	clk.advance()
	if r := sc.ScanSource(ctx, s1); r.Status != model.ScanOK {
		t.Fatal(r.Err)
	}
	offer := offerByTitle(t, sqlDB, rawSRE.Title)

	// A cross-source link is explicit (the merge path creates it): the
	// aggregator's own uid points at the same offer.
	onAggregator := sources.RawOffer{ExternalUID: "agg-1", Title: "Site Reliability Engineer",
		Location: "Paris, France", URL: "https://aggregator.example/acme-sre", Company: "ACME Inc."}
	ts := model.FormatTime(clk.now())
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO job_offer_provenance(
		offer_id, source_id, external_uid, url, first_seen_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?)`,
		offer.ID, s2.ID, onAggregator.ExternalUID, onAggregator.URL, ts, ts); err != nil {
		t.Fatal(err)
	}

	ff.offers[s2.ID] = []sources.RawOffer{onAggregator}
	clk.advance()
	if r := sc.ScanSource(ctx, s2); r.Status != model.ScanOK {
		t.Fatal(r.Err)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
		t.Fatalf("offers = %d, want 1 (the aggregator uid resolved to the same offer)", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance`); n != 2 {
		t.Fatalf("provenance = %d, want 2", n)
	}

	// Gone from the company site but still on the aggregator: stays active.
	clk.advance()
	ff.offers[s1.ID] = nil
	if r := sc.ScanSource(ctx, s1); r.Disappeared != 0 {
		t.Errorf("disappeared = %d, want 0 while another source still lists it", r.Disappeared)
	}
	if o := offerByTitle(t, sqlDB, rawSRE.Title); o.OfferState != model.OfferActive {
		t.Errorf("state = %q, want active", o.OfferState)
	}

	// Gone from the aggregator too: disappeared.
	clk.advance()
	ff.offers[s2.ID] = nil
	if r := sc.ScanSource(ctx, s2); r.Disappeared != 1 {
		t.Errorf("disappeared = %d, want 1", r.Disappeared)
	}
	if o := offerByTitle(t, sqlDB, rawSRE.Title); o.OfferState != model.OfferDisappeared {
		t.Errorf("state = %q, want disappeared", o.OfferState)
	}
}

func TestScanAll(t *testing.T) {
	sqlDB, sc, ff, _ := setup(t)
	ctx := context.Background()
	bad := addSource(t, sqlDB, "Broken", "")
	good := addSource(t, sqlDB, "Good", `{"company":"Good"}`)
	disabled, err := db.InsertSource(ctx, sqlDB, model.Source{Name: "Off", URL: "https://x", Type: model.SourceLever}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ff.errs[bad.ID] = errors.New("boom")
	ff.offers[good.ID] = []sources.RawOffer{rawSRE}

	results, err := sc.ScanAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2 (disabled source skipped)", len(results))
	}
	if results[0].Status != model.ScanError || results[1].Status != model.ScanOK || results[1].New != 1 {
		t.Errorf("results = %+v", results)
	}
	if s, _ := db.GetSource(ctx, sqlDB, disabled.ID); s.LastScanAt != nil {
		t.Error("disabled source was scanned")
	}
}

func TestScanNotImplementedIsSkipped(t *testing.T) {
	sqlDB, sc, _, _ := setup(t)
	ctx := context.Background()
	src, err := db.InsertSource(ctx, sqlDB, model.Source{Name: "Feed", URL: "https://x/rss", Type: model.SourceRSS, Enabled: true}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	sc.Fetcher = sources.Registry{}
	r := sc.ScanSource(ctx, src)
	if r.Status != "" || !errors.Is(r.Err, sources.ErrNotImplemented) {
		t.Errorf("result = %+v", r)
	}
	if s, _ := db.GetSource(ctx, sqlDB, src.ID); s.LastScanAt != nil || s.LastScanStatus != nil {
		t.Error("skipped source got a scan status")
	}
}

// End-to-end through the real Lever adapter and recorded fixture: three
// postings with the same title in different cities become three offers.
func TestScanWithLeverFixture(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "sources", "testdata", "lever_postings.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		w.Write(body)
	}))
	defer srv.Close()

	sqlDB, sc, _, _ := setup(t)
	ctx := context.Background()
	client := sources.NewClient()
	client.DefaultInterval = time.Millisecond
	sc.Fetcher = &sources.Lever{Client: client, BaseURL: srv.URL}
	src, err := db.InsertSource(ctx, sqlDB, model.Source{
		Name: "Spotify", URL: "https://jobs.lever.co/spotify", Type: model.SourceLever, ConfigJSON: `{"company":"Spotify"}`, Enabled: true,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := sc.ScanSource(ctx, src)
	if r.Status != model.ScanOK || r.New != 3 {
		t.Fatalf("result = %+v", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 3 {
		t.Errorf("offers = %d, want 3", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE external_uid IS NOT NULL`); n != 3 {
		t.Errorf("provenance with uid = %d, want 3", n)
	}
}

func TestScanClassifiesOffers(t *testing.T) {
	sqlDB, sc, ff, _ := setup(t)
	src := addSource(t, sqlDB, "Acme", `{"company":"Acme"}`)
	ff.offers[src.ID] = []sources.RawOffer{rawSRE, rawPlatform}
	if r := sc.ScanSource(context.Background(), src); r.Status != model.ScanOK {
		t.Fatal(r.Err)
	}
	sre := offerByTitle(t, sqlDB, rawSRE.Title)
	if sre.TitleConfidence == nil || *sre.TitleConfidence != 85 {
		t.Errorf("SRE title_confidence = %v, want 85", sre.TitleConfidence)
	}
	if sre.LocationConfidence == nil || *sre.LocationConfidence != 60 {
		t.Errorf("SRE location_confidence = %v, want 60", sre.LocationConfidence)
	}
	if sre.MatchReasonsJSON == nil || !strings.Contains(*sre.MatchReasonsJSON, "site reliability") {
		t.Errorf("SRE reasons = %v", sre.MatchReasonsJSON)
	}
	platform := offerByTitle(t, sqlDB, rawPlatform.Title)
	if platform.RemotePolicy != model.RemoteFull {
		t.Errorf("platform remote_policy = %q, want remote", platform.RemotePolicy)
	}
	if platform.LocationConfidence == nil || *platform.LocationConfidence != 80 {
		t.Errorf("platform location_confidence = %v, want 80", platform.LocationConfidence)
	}
}
