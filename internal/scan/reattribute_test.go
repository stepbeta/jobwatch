package scan

import (
	"context"
	"testing"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

func TestReattributeSource(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()
	src := addSource(t, sqlDB, "Feed", `{"company":"Feed"}`)

	ff.offers[src.ID] = []sources.RawOffer{
		{ExternalUID: "1", Title: "Feed Role", Location: "Remote", URL: "https://x/1", Description: "first"},
		{ExternalUID: "2", Title: "Second Role", URL: "https://x/2"},
	}
	clk.advance()
	if r := sc.ScanSource(ctx, src); r.New != 2 || r.Err != nil {
		t.Fatalf("seed scan = %+v, want 2 new", r)
	}
	o1 := offerByProvenance(t, sqlDB, src.ID, "1")
	o2 := offerByProvenance(t, sqlDB, src.ID, "2")
	feedID := *o1.CompanyID
	if o2.CompanyID == nil || *o2.CompanyID != feedID {
		t.Fatalf("seed offers are not both under the feed company: %+v / %+v", o1, o2)
	}

	// History the re-attribution must preserve.
	ev, err := db.InsertEvent(ctx, sqlDB, model.Event{
		OfferID: o1.ID, Type: model.EventCVSent, OccurredAt: clk.now(),
	}, clk.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityOffer, EntityID: o1.ID, Body: "keep me",
	}, clk.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE job_offers SET application_state = 'applied' WHERE id = ?`, o1.ID); err != nil {
		t.Fatal(err)
	}
	snapshots := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, o1.ID)

	// The adapter now extracts the employer for uid 1 only. uid 2 has no
	// sighting in this output and must stay where it is.
	ff.offers[src.ID] = []sources.RawOffer{
		{ExternalUID: "1", Title: "Underwriter", Company: "Hemp",
			URL: "https://x/1", Description: "second"},
	}
	clk.advance()
	res := sc.ReattributeSource(ctx, src)
	if res.Err != nil || res.Reattributed != 1 || res.Unchanged != 0 || res.Skipped != 1 {
		t.Fatalf("first reattribution = %+v, want 1 moved, 1 skipped", res)
	}
	if len(res.EmptiedCompanies) != 0 {
		t.Errorf("emptied companies = %v, want none (the feed still has uid 2)", res.EmptiedCompanies)
	}

	got := offerByProvenance(t, sqlDB, src.ID, "1")
	if got.Title != "Underwriter" || got.CompanyID == nil {
		t.Fatalf("moved offer = %+v", got)
	}
	var name string
	if err := sqlDB.QueryRow(`SELECT name FROM companies WHERE id = ?`, *got.CompanyID).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name != "Hemp" {
		t.Errorf("moved offer company = %q, want Hemp", name)
	}
	if got.ApplicationState != model.AppApplied || got.OfferState != model.OfferActive {
		t.Errorf("moved offer states = %q/%q, want applied/active", got.ApplicationState, got.OfferState)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM events WHERE offer_id = ? AND id = ?`, got.ID, ev.ID); n != 1 {
		t.Errorf("event was not preserved")
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM notes WHERE entity_type = 'offer' AND entity_id = ?`, got.ID); n != 1 {
		t.Errorf("note was not preserved")
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, got.ID); n != snapshots {
		t.Errorf("snapshots = %d, want unchanged %d", n, snapshots)
	}
	if got.LastSeenAt != o1.LastSeenAt {
		t.Errorf("last_seen_at moved from %v to %v; re-attribution is not a sighting", o1.LastSeenAt, got.LastSeenAt)
	}

	untouched := offerByProvenance(t, sqlDB, src.ID, "2")
	if untouched.Title != "Second Role" || untouched.CompanyID == nil || *untouched.CompanyID != feedID {
		t.Errorf("offer without a sighting changed: %+v", untouched)
	}

	// A second run with the same output is a no-op.
	clk.advance()
	res = sc.ReattributeSource(ctx, src)
	if res.Err != nil || res.Reattributed != 0 || res.Unchanged != 1 || res.Skipped != 1 {
		t.Fatalf("second reattribution = %+v, want no move", res)
	}
	again := offerByProvenance(t, sqlDB, src.ID, "1")
	if again.Title != got.Title || again.UpdatedAt != got.UpdatedAt {
		t.Errorf("second run changed the offer: %+v -> %+v", got, again)
	}

	// Moving the last offer too reports the emptied feed company; the row
	// itself stays.
	ff.offers[src.ID] = []sources.RawOffer{
		{ExternalUID: "1", Title: "Underwriter", Company: "Hemp", URL: "https://x/1"},
		{ExternalUID: "2", Title: "Second Role", Company: "Other", URL: "https://x/2"},
	}
	clk.advance()
	res = sc.ReattributeSource(ctx, src)
	if res.Err != nil || res.Reattributed != 1 || res.Unchanged != 1 || res.Skipped != 0 {
		t.Fatalf("third reattribution = %+v, want 1 moved", res)
	}
	if len(res.EmptiedCompanies) != 1 || res.EmptiedCompanies[0] != "Feed" {
		t.Errorf("emptied companies = %v, want [Feed]", res.EmptiedCompanies)
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM companies WHERE id = ?`, feedID); n != 1 {
		t.Errorf("emptied company was deleted")
	}
	if n := count(t, sqlDB, `SELECT COUNT(*) FROM job_offers WHERE company_id = ?`, feedID); n != 0 {
		t.Errorf("feed company still has %d offer(s)", n)
	}
}

func TestReattributeSkipsRepairedOffers(t *testing.T) {
	sqlDB, sc, ff, clk := setup(t)
	ctx := context.Background()
	src := addSource(t, sqlDB, "Feed", "")

	// An item without company evidence is stored incomplete, then repaired
	// by the user.
	ff.offers[src.ID] = []sources.RawOffer{{ExternalUID: "1", Title: "Role", URL: "https://x/1"}}
	clk.advance()
	if r := sc.ScanSource(ctx, src); r.Incomplete != 1 {
		t.Fatalf("scan = %+v, want 1 incomplete", r)
	}
	o := offerByProvenance(t, sqlDB, src.ID, "1")
	company, err := db.CreateCompany(ctx, sqlDB, "User Co", nil, clk.now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RepairOffer(ctx, sqlDB, o.ID, company.ID, "User Title", nil, clk.now()); err != nil {
		t.Fatal(err)
	}

	// The adapter now reports an employer, but the user's repair wins.
	ff.offers[src.ID] = []sources.RawOffer{{
		ExternalUID: "1", Title: "Adapter Title", Company: "Adapter Co", URL: "https://x/1",
	}}
	res := sc.ReattributeSource(ctx, src)
	if res.Err != nil || res.Reattributed != 0 || res.Skipped != 1 {
		t.Fatalf("reattribution = %+v, want the repaired offer skipped", res)
	}
	got := offerByProvenance(t, sqlDB, src.ID, "1")
	if got.Title != "User Title" || got.CompanyID == nil || *got.CompanyID != company.ID {
		t.Errorf("repaired offer was rewritten: %+v", got)
	}
}
