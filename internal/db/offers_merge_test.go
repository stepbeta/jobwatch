package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func seedMergeOffer(t *testing.T, q Querier, company model.Company, title, location string) model.JobOffer {
	t.Helper()
	ctx := context.Background()
	o := model.JobOffer{
		CompanyID:       company.ID,
		Title:           title,
		TitleNormalized: model.NormalizeTitle(title),
		Fingerprint:     model.Fingerprint(company.NameNormalized, model.NormalizeTitle(title), ""),
	}
	if location != "" {
		o.Location = &location
	}
	o, err := InsertOffer(ctx, q, o, time.Now().UTC())
	if err != nil {
		t.Fatalf("insert offer %q: %v", title, err)
	}
	return o
}

func TestMergeOffersMovesEverything(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	company, err := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	target := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "Lyon")
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE job_offers SET application_state = 'applied', offer_state = 'disappeared' WHERE id = ?`, source.ID); err != nil {
		t.Fatal(err)
	}

	for _, p := range []struct {
		offer model.JobOffer
		url   string
	}{{target, "https://ex.com/a"}, {source, "https://ex.com/b"}} {
		if _, err := UpsertProvenance(ctx, sqlDB, p.offer.ID, nil, nil, p.url, now); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := InsertEvent(ctx, sqlDB, model.Event{
		OfferID: source.ID, Type: model.EventCVSent, OccurredAt: now,
	}, now); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertNote(ctx, sqlDB, model.Note{
		EntityType: model.EntityOffer, EntityID: source.ID, Body: "moved note",
	}, now); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{source.ID, target.ID} {
		if err := InsertSnapshot(ctx, sqlDB, id, "description", now); err != nil {
			t.Fatal(err)
		}
	}

	res, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.MergedOfferID != source.ID || res.MovedProvenance != 1 || res.MovedEvents != 1 ||
		res.MovedNotes != 1 || res.MovedSnapshots != 1 {
		t.Errorf("merge result = %+v", res)
	}
	if _, err := GetOffer(ctx, sqlDB, source.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("absorbed offer still exists: %v", err)
	}
	merged, err := GetOffer(ctx, sqlDB, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if merged.OfferState != model.OfferActive {
		t.Errorf("offer_state = %q, want active", merged.OfferState)
	}
	if merged.ApplicationState != model.AppApplied {
		t.Errorf("application_state = %q, want applied", merged.ApplicationState)
	}
	prov, err := ListProvenance(ctx, sqlDB, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(prov) != 2 {
		t.Errorf("provenance = %+v", prov)
	}
	events, err := ListEvents(ctx, sqlDB, target.ID)
	if err != nil || len(events) != 1 {
		t.Errorf("events = %+v, err %v", events, err)
	}
	notes, err := ListNotes(ctx, sqlDB, model.EntityOffer, target.ID)
	if err != nil || len(notes) != 1 {
		t.Errorf("notes = %+v, err %v", notes, err)
	}
	if n, err := CountSnapshots(ctx, sqlDB, target.ID); err != nil || n != 2 {
		t.Errorf("snapshots = %d, err %v", n, err)
	}
}

func TestMergeOffersCollapsesDuplicateURLs(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	target := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "Lyon")
	if _, err := UpsertProvenance(ctx, sqlDB, target.ID, nil, nil, "https://ex.com/same", now); err != nil {
		t.Fatal(err)
	}
	if _, err := UpsertProvenance(ctx, sqlDB, source.ID, nil, nil, "https://ex.com/same", now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE job_offer_provenance SET first_seen_at = ? WHERE offer_id = ?`,
		model.FormatTime(now.Add(-48*time.Hour)), source.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.ExecContext(ctx,
		`UPDATE job_offer_provenance SET first_seen_at = ?, last_seen_at = ? WHERE offer_id = ?`,
		model.FormatTime(now.Add(-24*time.Hour)), model.FormatTime(now), target.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now); err != nil {
		t.Fatal(err)
	}
	prov, err := ListProvenance(ctx, sqlDB, target.ID)
	if err != nil || len(prov) != 1 {
		t.Fatalf("provenance = %+v, err %v", prov, err)
	}
	if !prov[0].FirstSeenAt.Equal(now.Add(-48 * time.Hour)) {
		t.Errorf("first_seen = %v, want earliest", prov[0].FirstSeenAt)
	}
	if !prov[0].LastSeenAt.Equal(now) {
		t.Errorf("last_seen = %v, want latest", prov[0].LastSeenAt)
	}
}

func TestMergeOffersValidation(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	o := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	if _, err := MergeOffers(ctx, sqlDB, o.ID, o.ID, now); !errors.Is(err, ErrOfferMergeSelf) {
		t.Errorf("self merge err = %v", err)
	}
	if _, err := MergeOffers(ctx, sqlDB, o.ID, 999, now); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing target err = %v", err)
	}
}

func TestSplitOfferMovesProvenance(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	original := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	for _, url := range []string{"https://ex.com/a", "https://ex.com/b"} {
		if _, err := UpsertProvenance(ctx, sqlDB, original.ID, nil, nil, url, now); err != nil {
			t.Fatal(err)
		}
	}
	prov, _ := ListProvenance(ctx, sqlDB, original.ID)
	if len(prov) != 2 {
		t.Fatal(prov)
	}
	lyon := "Lyon, France"
	res, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{
		ProvenanceIDs: []int64{prov[1].ID},
		Location:      &lyon,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.MovedProvenance != 1 {
		t.Errorf("moved = %d", res.MovedProvenance)
	}
	if res.Offer.ID == original.ID || res.Offer.Location == nil || *res.Offer.Location != lyon {
		t.Errorf("new offer = %+v", res.Offer)
	}
	if res.Offer.Fingerprint == original.Fingerprint {
		t.Errorf("new offer reused the original fingerprint")
	}
	if res.Offer.URL == nil || *res.Offer.URL != "https://ex.com/b" {
		t.Errorf("new offer URL = %v", res.Offer.URL)
	}
	remaining, _ := ListProvenance(ctx, sqlDB, original.ID)
	if len(remaining) != 1 || remaining[0].URL != "https://ex.com/a" {
		t.Errorf("remaining provenance = %+v", remaining)
	}
	if n, _ := ListProvenance(ctx, sqlDB, res.Offer.ID); len(n) != 1 {
		t.Errorf("new provenance = %+v", n)
	}
}

func TestSplitOfferValidation(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	original := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	for _, url := range []string{"https://ex.com/a", "https://ex.com/b"} {
		if _, err := UpsertProvenance(ctx, sqlDB, original.ID, nil, nil, url, now); err != nil {
			t.Fatal(err)
		}
	}
	prov, _ := ListProvenance(ctx, sqlDB, original.ID)

	if _, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{}, now); !errors.Is(err, ErrOfferSplitNoProvenance) {
		t.Errorf("no provenance err = %v", err)
	}
	// Same title and location collides with the original offer.
	if _, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{
		ProvenanceIDs: []int64{prov[0].ID},
	}, now); !errors.Is(err, ErrOfferFingerprintTaken) {
		t.Errorf("fingerprint err = %v", err)
	}
	// All links cannot move away.
	ids := []int64{prov[0].ID, prov[1].ID}
	if _, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{
		ProvenanceIDs: ids, Title: "SRE - Payments",
	}, now); !errors.Is(err, ErrOfferSplitLastLink) {
		t.Errorf("last link err = %v", err)
	}
	// Foreign provenance row is rejected.
	other := seedMergeOffer(t, sqlDB, company, "Other", "")
	if _, err := UpsertProvenance(ctx, sqlDB, other.ID, nil, nil, "https://ex.com/other", now); err != nil {
		t.Fatal(err)
	}
	otherProv, _ := ListProvenance(ctx, sqlDB, other.ID)
	if _, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{
		ProvenanceIDs: []int64{otherProv[0].ID}, Title: "SRE - Payments",
	}, now); !errors.Is(err, ErrOfferSplitProvenance) {
		t.Errorf("foreign provenance err = %v", err)
	}
}
