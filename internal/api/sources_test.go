package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

type sourceResp struct {
	Source sourceJSON `json:"source"`
}

type sourceListResp struct {
	Sources []sourceJSON `json:"sources"`
}

type scanResp struct {
	Result scanResultJSON `json:"result"`
	Source sourceJSON     `json:"source"`
}

func TestSourcesCRUD(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Acme", "url": "https://boards.greenhouse.io/acme",
	})
	wantCode(t, rec, 201)
	created := decodeBody[sourceResp](t, rec).Source
	if created.SourceType != "greenhouse" || !created.Enabled {
		t.Errorf("source = %+v", created)
	}
	if string(created.Config) != "{}" {
		t.Errorf("config = %s", created.Config)
	}

	wantErrorCode(t, ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Dup", "url": "https://boards.greenhouse.io/acme",
	}), 409, "duplicate_source")

	wantErrorCode(t, ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Bad", "url": "https://example.com/x", "source_type": "nope",
	}), 400, "validation_failed")

	wantErrorCode(t, ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Bad", "url": "not a url", "source_type": "greenhouse",
	}), 400, "validation_failed")

	rec = ts.do(t, "GET", "/api/sources", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[sourceListResp](t, rec).Sources; len(got) != 72 {
		t.Fatalf("sources = %d, want 72 (71 seeded + Acme)", len(got))
	}

	id := fmt.Sprintf("%d", created.ID)
	disabled := false
	rec = ts.do(t, "PATCH", "/api/sources/"+id, map[string]any{"enabled": disabled, "config": map[string]any{"company": "Acme SAS"}})
	wantCode(t, rec, 200)
	patched := decodeBody[sourceResp](t, rec).Source
	if patched.Enabled {
		t.Error("source still enabled")
	}
	if string(patched.Config) != `{"company":"Acme SAS"}` {
		t.Errorf("config = %s", patched.Config)
	}

	wantErrorCode(t, ts.do(t, "PATCH", "/api/sources/"+id, map[string]any{"config": []string{"x"}}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET", "/api/sources/99", nil), 404, "not_found")

	rec = ts.do(t, "DELETE", "/api/sources/"+id, nil)
	wantCode(t, rec, 204)
	wantErrorCode(t, ts.do(t, "GET", "/api/sources/"+id, nil), 404, "not_found")
}

func TestSourceScan(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Acme", "url": "https://boards.greenhouse.io/acme",
		"config": map[string]any{"company": "Acme"},
	})
	wantCode(t, rec, 201)
	src := decodeBody[sourceResp](t, rec).Source

	ts.fetcher.offers[src.ID] = []sources.RawOffer{
		{Title: "Site Reliability Engineer", Location: "Paris, France", URL: "https://ex.com/jobs/1", Description: "Kubernetes"},
		{Title: "Platform Engineer", Location: "Remote - France", URL: "https://ex.com/jobs/2"},
	}

	srcID := fmt.Sprintf("%d", src.ID)
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)
	out := decodeBody[scanResp](t, rec)
	if out.Result.Status != "ok" || out.Result.Fetched != 2 || out.Result.New != 2 {
		t.Errorf("scan result = %+v", out.Result)
	}
	if out.Source.LastScanStatus == nil || *out.Source.LastScanStatus != "ok" || out.Source.LastScanAt == nil {
		t.Errorf("source scan status = %+v", out.Source)
	}

	// Scanning again is a plain sighting.
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)
	if got := decodeBody[scanResp](t, rec).Result; got.Seen != 2 || got.New != 0 {
		t.Errorf("rescan = %+v", got)
	}

	// A fetch error marks the source error but still answers 200.
	ts.fetcher.errs[src.ID] = errors.New("boom")
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)
	out = decodeBody[scanResp](t, rec)
	if out.Result.Status != "error" || out.Result.Error == "" {
		t.Errorf("failed scan = %+v", out.Result)
	}
	if out.Source.LastScanStatus == nil || *out.Source.LastScanStatus != "error" {
		t.Errorf("source status after failure = %+v", out.Source)
	}

	wantErrorCode(t, ts.do(t, "POST", "/api/sources/99/scan", nil), 404, "not_found")
}

func TestSourceScanGhostedCount(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Acme", "url": "https://boards.greenhouse.io/acme",
		"config": map[string]any{"company": "Acme"},
	})
	wantCode(t, rec, 201)
	src := decodeBody[sourceResp](t, rec).Source
	srcID := fmt.Sprintf("%d", src.ID)

	ts.fetcher.offers[src.ID] = []sources.RawOffer{
		{Title: "Site Reliability Engineer", Location: "Paris", URL: "https://ex.com/jobs/1"},
		{Title: "Keepalive Role", Location: "Paris", URL: "https://ex.com/jobs/keep"},
	}
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)

	var offerID int64
	if err := ts.db.QueryRowContext(context.Background(),
		`SELECT id FROM job_offers WHERE title = ?`, "Site Reliability Engineer").Scan(&offerID); err != nil {
		t.Fatal(err)
	}
	applied := model.AppApplied
	if err := db.UpdateOfferFields(context.Background(), ts.db, offerID,
		db.OfferUpdate{ApplicationState: &applied}, ts.clock.now()); err != nil {
		t.Fatal(err)
	}

	ts.clock.advance(time.Hour)
	ts.fetcher.offers[src.ID] = []sources.RawOffer{
		{Title: "Keepalive Role", Location: "Paris", URL: "https://ex.com/jobs/keep"},
	}
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)
	out := decodeBody[scanResp](t, rec)
	if out.Result.Disappeared != 1 || out.Result.Ghosted != 1 {
		t.Errorf("scan result = %+v, want 1 disappeared and 1 ghosted", out.Result)
	}
	var events int
	if err := ts.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM events WHERE offer_id = ? AND type = 'ghosted'`, offerID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Errorf("ghosted events = %d, want 1", events)
	}

	// A repeat scan reports no plain disappearance and no ghost.
	ts.clock.advance(time.Hour)
	rec = ts.do(t, "POST", "/api/sources/"+srcID+"/scan", nil)
	wantCode(t, rec, 200)
	out = decodeBody[scanResp](t, rec)
	if out.Result.Disappeared != 0 || out.Result.Ghosted != 0 {
		t.Errorf("repeat scan = %+v, want 0 disappeared and 0 ghosted", out.Result)
	}
}

func TestSourceScanNotImplemented(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, "POST", "/api/sources", map[string]any{
		"name": "Feed", "url": "https://example.com/feed.xml", "source_type": string(model.SourceRSS),
	})
	wantCode(t, rec, 201)
	created := decodeBody[sourceResp](t, rec).Source
	ts.Server.Fetcher = sources.Registry{}
	rec = ts.do(t, "POST", "/api/sources/"+fmt.Sprintf("%d", created.ID)+"/scan", nil)
	wantCode(t, rec, 200)
	out := decodeBody[scanResp](t, rec)
	if out.Result.Status != "skipped" || out.Result.Error == "" {
		t.Errorf("skipped scan = %+v", out.Result)
	}
	if out.Source.LastScanStatus != nil {
		t.Errorf("skipped source got status %v", *out.Source.LastScanStatus)
	}
}
