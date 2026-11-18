package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

type offerListResp struct {
	Offers []offerJSON `json:"offers"`
	Total  int         `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

type MatchReasons struct {
	Title    []string `json:"title"`
	Location []string `json:"location"`
}

type offerDetailResp struct {
	Offer          offerJSON             `json:"offer"`
	Company        companyJSON           `json:"company"`
	Provenance     []provenanceJSON      `json:"provenance"`
	Events         []eventJSON           `json:"events"`
	Notes          []noteJSON            `json:"notes"`
	Snapshots      []snapshotSummaryJSON `json:"snapshots"`
	LatestSnapshot *snapshotJSON         `json:"latest_snapshot"`
	SnapshotDiff   *snapshotDiffJSON     `json:"snapshot_diff"`
	SnapshotCount  int                   `json:"snapshot_count"`
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
		{"title confidence sort", "/api/offers?sort=title_confidence&order=asc", 4, 4},
		{"location sort", "/api/offers?sort=location&order=desc", 4, 4},
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

	rec = ts.do(t, "GET", "/api/offers?sort=title&order=desc", nil)
	wantCode(t, rec, 200)
	got = decodeBody[offerListResp](t, rec)
	for i := 1; i < len(got.Offers); i++ {
		if strings.ToLower(got.Offers[i-1].Title) < strings.ToLower(got.Offers[i].Title) {
			t.Errorf("title desc sort: %q before %q", got.Offers[i-1].Title, got.Offers[i].Title)
		}
	}

	bad := []string{
		"/api/offers?sort=nope",
		"/api/offers?order=nope",
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

func TestOfferSaved(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	rec := ts.do(t, "PATCH", "/api/offers/1", map[string]any{"saved": true})
	wantCode(t, rec, 200)
	got := decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	want := model.FormatTime(ts.clock.now())
	if got.SavedAt == nil || *got.SavedAt != want {
		t.Errorf("saved_at = %v, want %q", got.SavedAt, want)
	}

	ts.clock.advance(time.Hour)
	rec = ts.do(t, "PATCH", "/api/offers/2", map[string]any{"saved": true})
	wantCode(t, rec, 200)

	rec = ts.do(t, "GET", "/api/offers?saved=true&sort=saved", nil)
	wantCode(t, rec, 200)
	list := decodeBody[offerListResp](t, rec)
	if list.Total != 2 || len(list.Offers) != 2 {
		t.Fatalf("saved list = %d offers, total %d, want 2", len(list.Offers), list.Total)
	}
	if list.Offers[0].ID != 2 || list.Offers[1].ID != 1 {
		t.Errorf("saved order = [%d %d], want [2 1] (newest bookmark first)",
			list.Offers[0].ID, list.Offers[1].ID)
	}

	rec = ts.do(t, "GET", "/api/offers?saved=false", nil)
	wantCode(t, rec, 200)
	list = decodeBody[offerListResp](t, rec)
	if list.Total != 2 {
		t.Errorf("unsaved total = %d, want 2", list.Total)
	}

	rec = ts.do(t, "PATCH", "/api/offers/1", map[string]any{"saved": false})
	wantCode(t, rec, 200)
	got = decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	if got.SavedAt != nil {
		t.Errorf("saved_at after unbookmark = %v, want null", got.SavedAt)
	}

	rec = ts.do(t, "GET", "/api/offers?saved=true", nil)
	list = decodeBody[offerListResp](t, rec)
	if list.Total != 1 || list.Offers[0].ID != 2 {
		t.Errorf("saved list after unbookmark = %+v", list)
	}

	wantErrorCode(t, ts.do(t, "GET", "/api/offers?saved=maybe", nil), 400, "validation_failed")
}

func TestPatchOfferDescription(t *testing.T) {
	ts := newTestServer(t)
	sre, _ := seedOfferSet(t, ts)
	ctx := context.Background()

	const oldText = "Kubernetes and Terraform"
	oldHash := model.TextHash(oldText)
	if _, err := ts.db.ExecContext(ctx,
		`UPDATE job_offers SET raw_text = ?, text_hash = ? WHERE id = ?`, oldText, oldHash, sre.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertSnapshot(ctx, ts.db, sre.ID, oldText, ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	before, err := db.GetOffer(ctx, ts.db, sre.ID)
	if err != nil {
		t.Fatal(err)
	}
	ts.clock.advance(5 * time.Minute)

	const newText = "Kubernetes, Terraform and Nomad\nRemote-first team."
	rec := ts.do(t, "PATCH", "/api/offers/1", map[string]any{"description": newText})
	wantCode(t, rec, 200)
	patched := decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	if patched.RawText == nil || *patched.RawText != newText {
		t.Errorf("patched offer raw_text = %v, want %q", patched.RawText, newText)
	}

	// The stored text and hash are replaced, updated_at is bumped and the
	// manual edit becomes the latest versioned snapshot.
	after, err := db.GetOffer(ctx, ts.db, sre.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.RawText == nil || *after.RawText != newText {
		t.Errorf("raw_text = %v, want %q", after.RawText, newText)
	}
	if after.TextHash == nil || *after.TextHash != model.TextHash(newText) {
		t.Errorf("text_hash = %v, want %q", after.TextHash, model.TextHash(newText))
	}
	if !after.UpdatedAt.Equal(ts.clock.now()) || after.UpdatedAt.Equal(before.UpdatedAt) {
		t.Errorf("updated_at = %v, want %v (was %v)", after.UpdatedAt, ts.clock.now(), before.UpdatedAt)
	}
	if after.OfferState != before.OfferState {
		t.Errorf("description edit changed offer_state to %q", after.OfferState)
	}

	rec = ts.do(t, "GET", "/api/offers/1", nil)
	wantCode(t, rec, 200)
	detail := decodeBody[offerDetailResp](t, rec)
	if detail.SnapshotCount != 2 || detail.LatestSnapshot == nil || detail.LatestSnapshot.RawText != newText {
		t.Fatalf("snapshot_count = %d latest = %+v, want 2 and the new text", detail.SnapshotCount, detail.LatestSnapshot)
	}
	if detail.SnapshotDiff == nil || detail.SnapshotDiff.From.ID == detail.SnapshotDiff.To.ID {
		t.Errorf("snapshot_diff = %+v, want old -> new", detail.SnapshotDiff)
	}
	if detail.Offer.RawText == nil || *detail.Offer.RawText != newText {
		t.Errorf("detail raw_text = %v, want %q", detail.Offer.RawText, newText)
	}

	// A second edit appends another snapshot rather than replacing one.
	rec = ts.do(t, "PATCH", "/api/offers/1", map[string]any{"description": "Third version"})
	wantCode(t, rec, 200)
	if n, err := db.CountSnapshots(ctx, ts.db, sre.ID); err != nil || n != 3 {
		t.Errorf("snapshots after second edit = %d (err=%v), want 3", n, err)
	}

	// null is rejected: the snapshot invariant has no "cleared" revision.
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"description": nil}),
		400, "validation_failed")
	// Wrong JSON type and empty patch are rejected too.
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{"description": 42}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/1", map[string]any{}), 400, "validation_failed")
	if n, err := db.CountSnapshots(ctx, ts.db, sre.ID); err != nil || n != 3 {
		t.Errorf("rejected patches changed snapshots: %d (err=%v)", n, err)
	}

	// Description can be combined with the other editable fields in one
	// atomic update.
	rec = ts.do(t, "PATCH", "/api/offers/1", map[string]any{
		"application_state": "interview", "description": "Fourth version",
	})
	wantCode(t, rec, 200)
	got := decodeBody[struct {
		Offer offerJSON `json:"offer"`
	}](t, rec).Offer
	if got.ApplicationState != "interview" {
		t.Errorf("application_state = %q, want interview", got.ApplicationState)
	}

	wantErrorCode(t, ts.do(t, "PATCH", "/api/offers/99", map[string]any{"description": "x"}),
		404, "not_found")
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

func TestOfferDetailSnapshotDiff(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	o := ts.seedOffer(t, acme, "SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	ctx := context.Background()
	if err := db.InsertSnapshot(ctx, ts.db, o.ID, "one\ntwo\nthree", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	ts.clock.advance(10 * time.Minute)
	if err := db.InsertSnapshot(ctx, ts.db, o.ID, "one\nTWO\nthree\nfour", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	rec := ts.do(t, "GET", "/api/offers/1", nil)
	wantCode(t, rec, 200)
	got := decodeBody[offerDetailResp](t, rec)
	if got.SnapshotCount != 2 || got.SnapshotDiff == nil {
		t.Fatalf("snapshot_count = %d, diff = %+v", got.SnapshotCount, got.SnapshotDiff)
	}
	if got.SnapshotDiff.From.ID == 0 || got.SnapshotDiff.To.ID == 0 {
		t.Errorf("diff refs = %+v", got.SnapshotDiff)
	}
	var added, removed []string
	for _, line := range got.SnapshotDiff.Lines {
		switch line.Op {
		case "added":
			added = append(added, line.Text)
		case "removed":
			removed = append(removed, line.Text)
		}
	}
	if strings.Join(removed, "|") != "two" || strings.Join(added, "|") != "TWO|four" {
		t.Errorf("diff = %+v", got.SnapshotDiff.Lines)
	}
}

func TestOfferSnapshotHistory(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	o := ts.seedOffer(t, acme, "SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	ctx := context.Background()

	if err := db.InsertSnapshot(ctx, ts.db, o.ID, "one", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	ts.clock.advance(time.Minute)
	if err := db.InsertSnapshot(ctx, ts.db, o.ID, "one\ntwo", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	ts.clock.advance(time.Minute)
	rec := ts.do(t, "PATCH", "/api/offers/1", map[string]any{"description": "manual text"})
	wantCode(t, rec, 200)

	rec = ts.do(t, "GET", "/api/offers/1", nil)
	wantCode(t, rec, 200)
	got := decodeBody[offerDetailResp](t, rec)
	if len(got.Snapshots) != 3 || got.SnapshotCount != 3 {
		t.Fatalf("snapshots = %+v, count = %d", got.Snapshots, got.SnapshotCount)
	}
	want := []struct {
		origin string
		length int
	}{
		{"scan", len("one")},
		{"scan", len("one\ntwo")},
		{"manual", len("manual text")},
	}
	for i, w := range want {
		s := got.Snapshots[i]
		if s.Origin != w.origin || s.TextLength != w.length || s.CapturedAt == "" {
			t.Errorf("snapshot %d = %+v, want origin=%s length=%d", i, s, w.origin, w.length)
		}
		if s.RawText != nil {
			t.Errorf("snapshot %d includes raw_text without snapshot_id: %q", i, *s.RawText)
		}
	}
	if got.LatestSnapshot == nil || got.LatestSnapshot.Origin != "manual" || got.LatestSnapshot.RawText != "manual text" {
		t.Errorf("latest_snapshot = %+v, want manual text", got.LatestSnapshot)
	}

	// The metadata payload stays small; the requested snapshot alone gets
	// its full text.
	firstID := got.Snapshots[0].ID
	rec = ts.do(t, "GET", fmt.Sprintf("/api/offers/1?snapshot_id=%d", firstID), nil)
	wantCode(t, rec, 200)
	selected := decodeBody[offerDetailResp](t, rec)
	if len(selected.Snapshots) != 3 {
		t.Fatalf("snapshots with snapshot_id = %+v", selected.Snapshots)
	}
	for i, s := range selected.Snapshots {
		if s.ID == firstID {
			if s.RawText == nil || *s.RawText != "one" {
				t.Errorf("requested snapshot raw_text = %v, want one", s.RawText)
			}
		} else if s.RawText != nil {
			t.Errorf("snapshot %d includes raw_text: %q", i, *s.RawText)
		}
	}

	wantErrorCode(t, ts.do(t, "GET", "/api/offers/1?snapshot_id=9999", nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/1?snapshot_id=abc", nil), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/1?snapshot_id=0", nil), 400, "validation_failed")
}

func TestSnapshotDiffEndpoint(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	o := ts.seedOffer(t, acme, "SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	ctx := context.Background()

	texts := []string{"alpha\nbeta", "alpha\ngamma", "alpha\ngamma\ndelta"}
	for i, text := range texts {
		if i > 0 {
			ts.clock.advance(time.Minute)
		}
		if err := db.InsertSnapshot(ctx, ts.db, o.ID, text, ts.clock.now()); err != nil {
			t.Fatal(err)
		}
	}
	infos, err := db.ListSnapshotInfos(ctx, ts.db, o.ID, 0)
	if err != nil || len(infos) != 3 {
		t.Fatalf("snapshot infos = %+v (err %v)", infos, err)
	}
	first, second, third := infos[0].ID, infos[1].ID, infos[2].ID

	// Reversed arguments are ordered oldest -> newest server-side.
	rec := ts.do(t, "GET",
		fmt.Sprintf("/api/offers/1/snapshot-diff?from=%d&to=%d", third, first), nil)
	wantCode(t, rec, 200)
	res := decodeBody[struct {
		SnapshotDiff snapshotDiffJSON `json:"snapshot_diff"`
	}](t, rec)
	if res.SnapshotDiff.From.ID != first || res.SnapshotDiff.To.ID != third {
		t.Errorf("diff refs = %+v, want %d -> %d", res.SnapshotDiff, first, third)
	}
	var removed, added []string
	for _, line := range res.SnapshotDiff.Lines {
		switch line.Op {
		case "removed":
			removed = append(removed, line.Text)
		case "added":
			added = append(added, line.Text)
		}
	}
	if strings.Join(removed, "|") != "beta" || strings.Join(added, "|") != "gamma|delta" {
		t.Errorf("arbitrary diff removed=%v added=%v", removed, added)
	}

	rec = ts.do(t, "GET",
		fmt.Sprintf("/api/offers/1/snapshot-diff?from=%d&to=%d", first, second), nil)
	wantCode(t, rec, 200)
	res = decodeBody[struct {
		SnapshotDiff snapshotDiffJSON `json:"snapshot_diff"`
	}](t, rec)
	if res.SnapshotDiff.From.ID != first || res.SnapshotDiff.To.ID != second {
		t.Errorf("forward diff refs = %+v, want %d -> %d", res.SnapshotDiff, first, second)
	}

	// Snapshots of another offer are rejected.
	other := ts.seedOffer(t, acme, "Platform Engineer", "Lyon", model.AppTodo, model.RemoteUnknown, 0, 0)
	if err := db.InsertSnapshot(ctx, ts.db, other.ID, "foreign", ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	foreignInfos, err := db.ListSnapshotInfos(ctx, ts.db, other.ID, 0)
	if err != nil || len(foreignInfos) != 1 {
		t.Fatalf("foreign snapshots = %+v (err %v)", foreignInfos, err)
	}
	wantErrorCode(t, ts.do(t, "GET",
		fmt.Sprintf("/api/offers/1/snapshot-diff?from=%d&to=%d", foreignInfos[0].ID, first), nil),
		404, "not_found")

	wantErrorCode(t, ts.do(t, "GET", "/api/offers/1/snapshot-diff?to=1", nil), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/1/snapshot-diff?from=abc&to=1", nil), 400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET",
		fmt.Sprintf("/api/offers/1/snapshot-diff?from=%d&to=%d", first, first), nil),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/99/snapshot-diff?from=1&to=2", nil),
		404, "not_found")
}

func TestSnapshotOriginsScanAndPatch(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	src, err := db.InsertSource(ctx, ts.db, model.Source{
		Name: "Scan Co", URL: "https://scan.example/board", Type: model.SourceGreenhouse,
		ConfigJSON: "{}", Enabled: true,
	}, ts.clock.now())
	if err != nil {
		t.Fatal(err)
	}
	ts.fetcher.offers[src.ID] = []sources.RawOffer{{
		ExternalUID: "1", Title: "SRE", Location: "Paris, France",
		URL: "https://scan.example/jobs/1", Description: "scan description",
	}}
	rec := ts.do(t, "POST", "/api/sources/"+strconv.FormatInt(src.ID, 10)+"/scan", nil)
	wantCode(t, rec, 200)

	var offerID int64
	if err := ts.db.QueryRowContext(ctx, `SELECT id FROM job_offers WHERE title = 'SRE'`).Scan(&offerID); err != nil {
		t.Fatal(err)
	}
	path := "/api/offers/" + strconv.FormatInt(offerID, 10)
	detail := decodeBody[offerDetailResp](t, ts.do(t, "GET", path, nil))
	if len(detail.Snapshots) != 1 || detail.Snapshots[0].Origin != "scan" {
		t.Fatalf("scan snapshots = %+v, want one scan snapshot", detail.Snapshots)
	}
	if detail.LatestSnapshot == nil || detail.LatestSnapshot.Origin != "scan" {
		t.Errorf("latest snapshot origin = %+v, want scan", detail.LatestSnapshot)
	}

	rec = ts.do(t, "PATCH", path, map[string]any{"description": "manual override"})
	wantCode(t, rec, 200)
	detail = decodeBody[offerDetailResp](t, ts.do(t, "GET", path, nil))
	if len(detail.Snapshots) != 2 ||
		detail.Snapshots[0].Origin != "scan" || detail.Snapshots[1].Origin != "manual" {
		t.Fatalf("origins after PATCH = %+v", detail.Snapshots)
	}
	if detail.LatestSnapshot == nil || detail.LatestSnapshot.Origin != "manual" ||
		detail.LatestSnapshot.RawText != "manual override" {
		t.Errorf("latest snapshot after PATCH = %+v", detail.LatestSnapshot)
	}
}

func TestMergeOffersAPI(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	target := ts.seedOffer(t, acme, "SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	source := ts.seedOffer(t, acme, "SRE - Platform", "Lyon", model.AppApplied, model.RemoteUnknown, 0, 0)
	ctx := context.Background()
	for _, p := range []struct {
		id  int64
		url string
	}{{target.ID, "https://ex.com/target"}, {source.ID, "https://ex.com/source"}} {
		if _, err := db.UpsertProvenance(ctx, ts.db, p.id, nil, nil, p.url, ts.clock.now()); err != nil {
			t.Fatal(err)
		}
	}
	ts.seedEvent(t, source.ID, model.EventCVSent, ts.clock.now(), "sent")
	if _, err := db.InsertNote(ctx, ts.db, model.Note{
		EntityType: model.EntityOffer, EntityID: source.ID, Body: "note",
	}, ts.clock.now()); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertSnapshot(ctx, ts.db, source.ID, "source text", ts.clock.now()); err != nil {
		t.Fatal(err)
	}

	rec := ts.do(t, "POST", "/api/offers/"+strconv.FormatInt(source.ID, 10)+"/merge", map[string]any{
		"into_id": target.ID,
	})
	wantCode(t, rec, 200)
	res := decodeBody[struct {
		Offer           offerJSON `json:"offer"`
		MergedOfferID   int64     `json:"merged_offer_id"`
		MovedProvenance int       `json:"moved_provenance"`
		MovedEvents     int       `json:"moved_events"`
		MovedNotes      int       `json:"moved_notes"`
		MovedSnapshots  int       `json:"moved_snapshots"`
	}](t, rec)
	if res.MergedOfferID != source.ID || res.Offer.ID != target.ID {
		t.Errorf("merge = %+v", res)
	}
	if res.MovedProvenance != 1 || res.MovedEvents != 1 || res.MovedNotes != 1 || res.MovedSnapshots != 1 {
		t.Errorf("moved counts = %+v", res)
	}
	detail := decodeBody[offerDetailResp](t, ts.do(t, "GET", "/api/offers/"+strconv.FormatInt(target.ID, 10), nil))
	if len(detail.Provenance) != 2 || len(detail.Events) != 1 || len(detail.Notes) != 1 {
		t.Errorf("detail after merge = %+v", detail)
	}
	wantErrorCode(t, ts.do(t, "GET", "/api/offers/"+strconv.FormatInt(source.ID, 10), nil), 404, "not_found")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/merge", map[string]any{"into_id": 1}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/merge", map[string]any{"into_id": 99}),
		404, "not_found")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/merge", map[string]any{}),
		400, "validation_failed")
}

func TestSplitOfferAPI(t *testing.T) {
	ts := newTestServer(t)
	acme := ts.seedCompany(t, "Acme")
	o := ts.seedOffer(t, acme, "SRE", "Paris", model.AppTodo, model.RemoteUnknown, 0, 0)
	ctx := context.Background()
	var provIDs []int64
	for _, url := range []string{"https://ex.com/a", "https://ex.com/b", "https://ex.com/c"} {
		if _, err := db.UpsertProvenance(ctx, ts.db, o.ID, nil, nil, url, ts.clock.now()); err != nil {
			t.Fatal(err)
		}
	}
	prov, err := db.ListProvenance(ctx, ts.db, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range prov {
		provIDs = append(provIDs, p.ID)
	}

	rec := ts.do(t, "POST", "/api/offers/1/split", map[string]any{
		"provenance_ids": []int64{provIDs[1]},
		"location":       "Lyon, France",
	})
	wantCode(t, rec, 201)
	res := decodeBody[struct {
		Offer           offerJSON `json:"offer"`
		MovedProvenance int       `json:"moved_provenance"`
	}](t, rec)
	if res.Offer.ID == o.ID || res.Offer.Location == nil || *res.Offer.Location != "Lyon, France" || res.MovedProvenance != 1 {
		t.Errorf("split = %+v", res)
	}
	if res.Offer.Title != o.Title {
		t.Errorf("split title = %q, want original", res.Offer.Title)
	}

	// The original still has two links; moving both away is refused.
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/split", map[string]any{
		"provenance_ids": []int64{provIDs[0], provIDs[2]},
		"title":          "SRE - Payments",
	}), 400, "validation_failed")

	// Same title and location collide with the original fingerprint.
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/split", map[string]any{
		"provenance_ids": []int64{provIDs[2]},
	}), 409, "offer_conflict")

	wantErrorCode(t, ts.do(t, "POST", "/api/offers/1/split", map[string]any{}),
		400, "validation_failed")
	wantErrorCode(t, ts.do(t, "POST", "/api/offers/99/split", map[string]any{
		"provenance_ids": []int64{1},
	}), 404, "not_found")
}

func TestImportClassifiesAndFilters(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Acme", "title": "Platform Engineer", "location": "Remote - France",
	})
	wantCode(t, rec, 201)
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{
		"company": "Acme", "title": "Marketing Platform Lead", "location": "Paris, France",
	})
	wantCode(t, rec, 201)

	rec = ts.do(t, "GET", "/api/offers?min_title_confidence=80&min_location_confidence=80", nil)
	wantCode(t, rec, 200)
	got := decodeBody[offerListResp](t, rec)
	if got.Total != 1 || len(got.Offers) != 1 {
		t.Fatalf("filtered offers = %+v", got)
	}
	o := got.Offers[0]
	if o.Title != "Platform Engineer" || o.TitleConfidence == nil || *o.TitleConfidence != 85 {
		t.Errorf("offer = %+v", o)
	}
	if o.LocationConfidence == nil || *o.LocationConfidence != 80 {
		t.Errorf("location_confidence = %v, want 80", o.LocationConfidence)
	}
	if o.RemotePolicy != "remote" {
		t.Errorf("remote_policy = %q, want remote", o.RemotePolicy)
	}
	var reasons MatchReasons
	if err := json.Unmarshal(o.MatchReasons, &reasons); err != nil {
		t.Fatalf("match_reasons %q: %v", o.MatchReasons, err)
	}
	if len(reasons.Title) == 0 || len(reasons.Location) == 0 {
		t.Errorf("match reasons = %+v", reasons)
	}
}
