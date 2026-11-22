package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// TestSmokeFlows is the end-to-end smoke test of the documented flow:
// add source -> scan (recorded Workable fixture through the real adapter)
// -> classify -> track the application (state + timeline event).
func TestSmokeFlows(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("..", "sources", "testdata", "workable_jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RequestURI() {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/api/v1/widget/accounts/acme?details=true":
			w.Header().Set("Content-Type", "application/json")
			w.Write(fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	ts := newTestServer(t)
	client := sources.NewClient()
	client.DefaultInterval = 0
	ts.Server.HTTPClient = client
	ts.Server.Fetcher = sources.Registry{
		model.SourceWorkable: &sources.Workable{Client: client, BaseURL: srv.URL},
	}

	// 1. Add a source from its public careers URL.
	rec := ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Hugging Face", "url": "https://apply.workable.com/acme",
		"source_type": "workable", "config": map[string]any{"account": "acme"},
	})
	wantCode(t, rec, 201)
	src := decodeBody[sourceResp](t, rec).Source
	if src.SourceType != "workable" {
		t.Fatalf("source = %+v", src)
	}

	// 2. Scan it: the adapter parses the fixture, the scan pipeline
	// resolves companies/offers and the classifier fills the scores.
	rec = ts.do(t, "POST", "/api/sources/"+strconv.FormatInt(src.ID, 10)+"/scan", nil)
	wantCode(t, rec, 200)
	scan := decodeBody[scanResp](t, rec)
	if scan.Result.Status != "ok" || scan.Result.New != 3 || scan.Result.Fetched != 3 {
		t.Fatalf("scan result = %+v", scan.Result)
	}
	if scan.Source.LastScanStatus == nil || *scan.Source.LastScanStatus != "ok" {
		t.Errorf("source status = %+v", scan.Source)
	}

	rec = ts.do(t, "GET", "/api/offers?sort=title", nil)
	wantCode(t, rec, 200)
	list := decodeBody[offerListResp](t, rec)
	if list.Total != 3 || len(list.Offers) != 3 {
		t.Fatalf("offers = %+v", list)
	}
	var target offerJSON
	for _, o := range list.Offers {
		if o.CompanyName != "Hugging Face" {
			t.Errorf("company = %q", o.CompanyName)
		}
		if o.RemotePolicy == "" {
			t.Errorf("offer %d has no remote policy fact: %+v", o.ID, o)
		}
		if o.ApplicationState != "todo" {
			t.Errorf("new offer %d state = %q", o.ID, o.ApplicationState)
		}
		if o.ID > target.ID {
			target = o
		}
	}

	// 3. Track the application: set the state and add a backdated event.
	rec = ts.do(t, "PATCH", "/api/offers/"+strconv.FormatInt(target.ID, 10), map[string]any{
		"application_state": "applied",
	})
	wantCode(t, rec, 200)
	occurred := time.Date(2026, 11, 14, 10, 30, 0, 0, time.UTC)
	rec = ts.do(t, "POST", "/api/offers/"+strconv.FormatInt(target.ID, 10)+"/events", map[string]any{
		"type": "cv_sent", "description": "sent through the board",
		"occurred_at": occurred.Format(time.RFC3339),
	})
	wantCode(t, rec, 201)

	rec = ts.do(t, "GET", "/api/offers/"+strconv.FormatInt(target.ID, 10), nil)
	wantCode(t, rec, 200)
	detail := decodeBody[offerDetailResp](t, rec)
	if detail.Offer.ApplicationState != "applied" {
		t.Errorf("application state = %q", detail.Offer.ApplicationState)
	}
	if len(detail.Events) != 1 || detail.Events[0].Type != "cv_sent" {
		t.Errorf("events = %+v", detail.Events)
	}
	if len(detail.Provenance) != 1 || detail.Provenance[0].SourceName == nil {
		t.Errorf("provenance = %+v", detail.Provenance)
	}
	if detail.LatestSnapshot == nil || detail.LatestSnapshot.RawText == "" {
		t.Errorf("snapshot missing: %+v", detail.LatestSnapshot)
	}

	// The Applications view (has_application + recent_event) sees it.
	rec = ts.do(t, "GET", "/api/offers?has_application=true&sort=recent_event", nil)
	wantCode(t, rec, 200)
	apps := decodeBody[offerListResp](t, rec)
	if apps.Total != 1 || apps.Offers[0].ID != target.ID || apps.Offers[0].LastEventAt == nil {
		t.Errorf("applications = %+v", apps)
	}
}
