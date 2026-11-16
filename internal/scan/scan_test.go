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
			name:       "changed text and missing offer",
			offers:     []sources.RawOffer{rawSREChanged},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 1, Seen: 1, Changed: 1, Disappeared: 1},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferChanged, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "fetch error leaves offers untouched",
			fetchErr:   errors.New("connection refused"),
			wantStatus: model.ScanError,
			want:       Result{},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferChanged, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "blocked source",
			fetchErr:   fmt.Errorf("robots: %w", sources.ErrBlocked),
			wantStatus: model.ScanBlocked,
			want:       Result{},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferChanged, 2},
				{"Platform Engineer (H/F)", model.OfferDisappeared, 1},
			},
		},
		{
			name:       "disappeared offer comes back, changed one stays changed",
			offers:     []sources.RawOffer{rawSREChanged, rawPlatform},
			wantStatus: model.ScanOK,
			want:       Result{Fetched: 2, Seen: 2, Reappeared: 1},
			offers2: []wantOffer{
				{"Site Reliability Engineer", model.OfferChanged, 2},
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
			name: "same title same bucket merges with two provenance rows",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", Location: "Paris", URL: "https://x/a", Description: "one"},
				{ExternalUID: "b", Title: "SRE (F/H)", Location: "Paris, France", URL: "https://x/b", Description: "two"},
			},
			wantOffers: 1, wantProv: 2,
			wantFirstLocs: []string{"Paris"},
		},
		{
			name: "unknown location matches base offer",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "SRE", Location: "Paris", URL: "https://x/a"},
				{ExternalUID: "b", Title: "SRE", Location: "", URL: "https://x/b"},
			},
			wantOffers: 1, wantProv: 2,
			wantFirstLocs: []string{"Paris"},
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
			name: "offers without title or url are skipped",
			offers: []sources.RawOffer{
				{ExternalUID: "a", Title: "", URL: "https://x/a"},
				{ExternalUID: "b", Title: "SRE", URL: ""},
			},
			wantOffers: 0, wantProv: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, sc, ff, _ := setup(t)
			src := addSource(t, sqlDB, "Acme", "")
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
			if res2.New != 0 || res2.Changed != 0 || res2.Disappeared != 0 {
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

	onAggregator := rawSRE
	onAggregator.URL = "https://aggregator.example/acme-sre"
	onAggregator.Company = "ACME Inc."

	ff.offers[s1.ID] = []sources.RawOffer{rawSRE}
	ff.offers[s2.ID] = []sources.RawOffer{onAggregator}
	for _, s := range []model.Source{s1, s2} {
		clk.advance()
		if r := sc.ScanSource(ctx, s); r.Status != model.ScanOK {
			t.Fatal(r.Err)
		}
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers`); n != 1 {
		t.Fatalf("offers = %d, want 1 (same company+title across sources)", n)
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
	good := addSource(t, sqlDB, "Good", "")
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
		Name: "Spotify", URL: "https://jobs.lever.co/spotify", Type: model.SourceLever, Enabled: true,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := sc.ScanSource(ctx, src)
	if r.Status != model.ScanOK || r.New != 3 {
		t.Fatalf("result = %+v", r)
	}
	if n := count(t, sqlDB, `SELECT COUNT(DISTINCT fingerprint) FROM job_offers`); n != 3 {
		t.Errorf("distinct fingerprints = %d, want 3", n)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offer_provenance WHERE external_uid IS NOT NULL`); n != 3 {
		t.Errorf("provenance with uid = %d, want 3", n)
	}
}

func TestScanClassifiesOffers(t *testing.T) {
	sqlDB, sc, ff, _ := setup(t)
	src := addSource(t, sqlDB, "Acme", "")
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
