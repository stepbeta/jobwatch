package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

type offerListResp struct {
	Offers []offerJSON `json:"offers"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type offerDetailResp struct {
	Offer          offerJSON        `json:"offer"`
	Company        companyJSON      `json:"company"`
	Provenance     []provenanceJSON `json:"provenance"`
	Events         []eventJSON      `json:"events"`
	Notes          []noteJSON       `json:"notes"`
	LatestSnapshot *snapshotJSON    `json:"latest_snapshot"`
	SnapshotCount  int              `json:"snapshot_count"`
}

func seedOfferSet(t *testing.T, ts *testServer) (model.JobOffer, model.Company) {
	t.Helper()
	acme := ts.seedCompany(t, "Acme")
	other := ts.seedCompany(t, "Other")
	sre := ts.seedOffer(t, acme, "Site Reliability Engineer", "Paris", model.AppApplied, model.RemoteFull, 95, 90)
	ts.seedOffer(t, acme, "Platform Engineer", "Remote - France", model.AppTodo, model.RemoteFull, 80, 85)
	ts.seedOffer(t, other, "SRE", "Lyon", model.AppInterview, model.RemoteHybrid, 70, 60)
	ts.seedOffer(t, other, "Marketing Manager", "", model.AppTodo, model.RemoteOnsite, 0, 0)
	if _, err := ts.db.ExecContext(context.Background(),
		`UPDATE job_offers SET raw_text = 'Kubernetes and Terraform' WHERE id = ?`, sre.ID); err != nil {
		t.Fatal(err)
	}
	return sre, acme
}

func TestListOffers(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	tests := []struct {
		name  string
		path  string
		total int
		n     int
	}{
		{"all", "/api/offers", 4, 4},
		{"application state", "/api/offers?application_state=applied", 1, 1},
		{"application state list", "/api/offers?application_state=applied,interview", 2, 2},
		{"offer state", "/api/offers?offer_state=active", 4, 4},
		{"remote policy", "/api/offers?remote_policy=remote", 2, 2},
		{"min title confidence", "/api/offers?min_title_confidence=90", 1, 1},
		{"min location confidence", "/api/offers?min_location_confidence=85", 2, 2},
		{"company", "/api/offers?company_id=2", 2, 2},
		{"has application", "/api/offers?has_application=true&sort=recent_event", 2, 2},
		{"full text", "/api/offers?q=kubernetes", 1, 1},
		{"pagination", "/api/offers?limit=2&offset=2", 4, 2},
		{"limit capped response", "/api/offers?limit=200", 4, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := ts.do(t, "GET", tt.path, nil)
			wantCode(t, rec, 200)
			got := decodeBody[offerListResp](t, rec)
			if got.Total != tt.total {
				t.Errorf("total = %d, want %d", got.Total, tt.total)
			}
			if len(got.Offers) != tt.n {
				t.Errorf("offers = %d, want %d", len(got.Offers), tt.n)
			}
		})
	}

	rec := ts.do(t, "GET", "/api/offers?sort=title", nil)
	wantCode(t, rec, 200)
	got := decodeBody[offerListResp](t, rec)
	for i := 1; i < len(got.Offers); i++ {
		if strings.ToLower(got.Offers[i-1].Title) > strings.ToLower(got.Offers[i].Title) {
			t.Errorf("title sort: %q before %q", got.Offers[i-1].Title, got.Offers[i].Title)
		}
	}

	bad := []string{
		"/api/offers?sort=nope",
		"/api/offers?application_state=nope",
		"/api/offers?offer_state=nope",
		"/api/offers?remote_policy=nope",
		"/api/offers?min_title_confidence=101",
		"/api/offers?min_location_confidence=-1",
		"/api/offers?company_id=abc",
		"/api/offers?limit=0",
	}
	for _, path := range bad {
		wantErrorCode(t, ts.do(t, "GET", path, nil), 400, "validation_failed")
	}
	wantErrorCode(t, ts.do(t, "GET", `/api/offers?q="unterminated`, nil), 400, "invalid_query")
}

func TestOfferDetail(t *testing.T) {
	ts := newTestServer(t)
	sre, acme := seedOfferSet(t, ts)
	ctx := context.Background()

	if _, err := db.UpsertProvenance(ctx, ts.db, sre.ID, nil, nil, "https://ex.com/jobs/1", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertSnapshot(ctx, ts.db, sre.ID, "Kubernetes and Terraform", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	e := ts.seedEvent(t, sre.ID, model.EventCVSent, ts.clock.now().AddDate(0, 0, -2), "CV sent")
	if _, err := db.InsertNote(ctx, ts.db, model.Note{
		EntityType: model.EntityOffer, EntityID: sre.ID, Body: "follow up", PinnedEventID: &e.ID,
	}, ts.clock.now()); err != nil {
		t.Fatal(err)
	}

	rec := ts.do(t, "GET", "/api/offers/1", nil)
	wantCode(t, rec, 200)
	got := decodeBody[offerDetailResp](t, rec)
	if got.Offer.ID != sre.ID || got.Offer.CompanyName != "Acme" {
		t.Errorf("offer = %+v", got.Offer)
	}
	if got.Company.ID != acme.ID {
		t.Errorf("company = %+v", got.Company)
	}
	if got.Company.OfferCount != 2 || got.Company.NoteCount != 0 {
		t.Errorf("company counts = %d offers, %d notes", got.Company.OfferCount, got.Company.NoteCount)
	}
	if len(got.Provenance) != 1 || got.Provenance[0].URL != "https://ex.com/jobs/1" || got.Provenance[0].SourceID != nil {
		t.Errorf("provenance = %+v", got.Provenance)
	}
	if len(got.Events) != 1 || got.Events[0].Type != "cv_sent" {
		t.Errorf("events = %+v", got.Events)
	}
	if len(got.Notes) != 1 || got.Notes[0].PinnedEventID == nil || *got.Notes[0].PinnedEventID != e.ID {
		t.Errorf("notes = %+v", got.Notes)
	}
	if got.LatestSnapshot == nil || got.LatestSnapshot.RawText != "Kubernetes and Terraform" {
		t.Errorf("snapshot = %+v", got.LatestSnapshot)
	}
	if got.SnapshotCount != 1 {
		t.Errorf("snapshot_count = %d", got.SnapshotCount)
	}

	wantErrorCode(t, ts.do(t, "GET", "/api/offers/99", nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/abc", nil), 400, "invalid_id")
}

func TestPatchOffer(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	rec := ts.do(t, "PATCH", "/api/offers/1", map[string]any{
		"application_state": "interview",
		"offer_state":       "changed",
		"location":          "Remote - France",
	})
	wantCode(t, rec, 200)
	got := decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	if got.ApplicationState != "interview" || got.OfferState != "changed" || got.Location == nil || *got.Location != "Remote - France" {
		t.Errorf("patched = %+v", got)
	}

	rec = ts.do(t, "PATCH", "/api/offers/1", map[string]any{"location": nil})
	wantCode(t, rec, 200)
	got = decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	if got.Location != nil {
		t.Errorf("location not cleared: %v", *got.Location)
	}

	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"application_state": "nope"}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"offer_state": "nope"}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{}), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"location": 42}), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/99", map[string]any{"application_state": "applied"}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"bogus": 1}), 400, "invalid_json")
}

func TestImportOffer(t *testing.T) {
	ts := newTestServer(t)

	rec := ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Qonto", "title": "SRE (H/F)", "location": "Paris", "url": "https://example.com/job/1",
	})
	wantCode(t, rec, 201)
	first := decodeBody[struct {
		OfferID int64 `json:"offer_id"`
		Created bool  `json:"created"`
	}](t, rec)
	if !first.Created || first.OfferID == 0 {
		t.Errorf("import = %+v", first)
	}

	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Qonto SAS", "title": "SRE", "url": "https://example.com/job/2",
	})
	wantCode(t, rec, 200)
	second := decodeBody[struct {
		OfferID int64 `json:"offer_id"`
		Created bool  `json:"created"`
	}](t, rec)
	if second.Created || second.OfferID != first.OfferID {
		t.Errorf("second import = %+v, want existing offer %d", second, first.OfferID)
	}

	rec = ts.do(t, "GET", "/api/offers/1", nil)
	wantCode(t, rec, 200)
	detail := decodeBody[offerDetailResp](t, rec)
	if len(detail.Provenance) != 2 {
		t.Errorf("provenance = %d, want 2", len(detail.Provenance))
	}

	wantErrorCode(t, ts.do(t, "POST", "/api/offers/import", map[string]any{"company": "Qonto"}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Qonto", "title": "SRE", "url": "ftp://nope",
	}), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Qonto", "title": "SRE", "unknown": true,
	}), 400, "invalid_json")
}
