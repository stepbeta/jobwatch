package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func setAppState(t *testing.T, sqlDB *sql.DB, id int64, state model.ApplicationState, now time.Time) {
	t.Helper()
	if err := UpdateOfferFields(context.Background(), sqlDB, id, OfferUpdate{ApplicationState: &state}, now); err != nil {
		t.Fatalf("set state %q: %v", state, err)
	}
}

func TestUpdateOfferFieldsSavedAtInvariant(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	o := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")

	if o.ApplicationState != model.AppNone {
		t.Fatalf("new offer state = %q, want none", o.ApplicationState)
	}
	if o.SavedAt != nil {
		t.Fatalf("new offer saved_at = %v, want nil", o.SavedAt)
	}

	setAppState(t, sqlDB, o.ID, model.AppSaved, now)
	got, _ := GetOffer(ctx, sqlDB, o.ID)
	if got.ApplicationState != model.AppSaved || got.SavedAt == nil || !got.SavedAt.Equal(now) {
		t.Errorf("saved: state=%q saved_at=%v, want saved at %v", got.ApplicationState, got.SavedAt, now)
	}

	// Re-saving refreshes the timestamp.
	later := now.Add(time.Hour)
	setAppState(t, sqlDB, o.ID, model.AppSaved, later)
	got, _ = GetOffer(ctx, sqlDB, o.ID)
	if got.SavedAt == nil || !got.SavedAt.Equal(later) {
		t.Errorf("re-saved saved_at = %v, want %v", got.SavedAt, later)
	}

	// Any other state clears it.
	setAppState(t, sqlDB, o.ID, model.AppApplied, later.Add(time.Hour))
	got, _ = GetOffer(ctx, sqlDB, o.ID)
	if got.ApplicationState != model.AppApplied || got.SavedAt != nil {
		t.Errorf("applied: state=%q saved_at=%v, want applied with nil", got.ApplicationState, got.SavedAt)
	}

	setAppState(t, sqlDB, o.ID, model.AppSaved, later.Add(2*time.Hour))
	setAppState(t, sqlDB, o.ID, model.AppNone, later.Add(3*time.Hour))
	got, _ = GetOffer(ctx, sqlDB, o.ID)
	if got.ApplicationState != model.AppNone || got.SavedAt != nil {
		t.Errorf("none: state=%q saved_at=%v, want none with nil", got.ApplicationState, got.SavedAt)
	}
}

func TestMergeOffersSavedAt(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)

	t.Run("saved source fills an unset target", func(t *testing.T) {
		sqlDB := openTest(t)
		company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
		target := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
		source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "Lyon")
		setAppState(t, sqlDB, source.ID, model.AppSaved, now)

		if _, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now); err != nil {
			t.Fatal(err)
		}
		merged, _ := GetOffer(ctx, sqlDB, target.ID)
		if merged.ApplicationState != model.AppSaved || merged.SavedAt == nil || !merged.SavedAt.Equal(now) {
			t.Errorf("merged state=%q saved_at=%v, want saved at %v", merged.ApplicationState, merged.SavedAt, now)
		}
	})

	t.Run("saved target yields to an applied source", func(t *testing.T) {
		sqlDB := openTest(t)
		company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
		target := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
		source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "Lyon")
		setAppState(t, sqlDB, target.ID, model.AppSaved, now)
		setAppState(t, sqlDB, source.ID, model.AppApplied, now)

		if _, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now); err != nil {
			t.Fatal(err)
		}
		merged, _ := GetOffer(ctx, sqlDB, target.ID)
		if merged.ApplicationState != model.AppApplied || merged.SavedAt != nil {
			t.Errorf("merged state=%q saved_at=%v, want applied with nil", merged.ApplicationState, merged.SavedAt)
		}
	})

	t.Run("applied target keeps its stage", func(t *testing.T) {
		sqlDB := openTest(t)
		company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
		target := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
		source := seedMergeOffer(t, sqlDB, company, "SRE - Platform", "Lyon")
		setAppState(t, sqlDB, target.ID, model.AppApplied, now)
		setAppState(t, sqlDB, source.ID, model.AppSaved, now)

		if _, err := MergeOffers(ctx, sqlDB, source.ID, target.ID, now); err != nil {
			t.Fatal(err)
		}
		merged, _ := GetOffer(ctx, sqlDB, target.ID)
		if merged.ApplicationState != model.AppApplied || merged.SavedAt != nil {
			t.Errorf("merged state=%q saved_at=%v, want applied with nil", merged.ApplicationState, merged.SavedAt)
		}
	})
}

func TestSplitOfferDefaultsToNone(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)
	original := seedMergeOffer(t, sqlDB, company, "SRE", "Paris")
	for _, url := range []string{"https://ex.com/a", "https://ex.com/b"} {
		if _, err := UpsertProvenance(ctx, sqlDB, original.ID, nil, nil, url, now); err != nil {
			t.Fatal(err)
		}
	}
	prov, _ := ListProvenance(ctx, sqlDB, original.ID)
	res, err := SplitOffer(ctx, sqlDB, original.ID, OfferSplitInput{ProvenanceIDs: []int64{prov[1].ID}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if res.Offer.ApplicationState != model.AppNone || res.Offer.SavedAt != nil {
		t.Errorf("split offer state=%q saved_at=%v, want none with nil", res.Offer.ApplicationState, res.Offer.SavedAt)
	}
}

func TestHasApplicationExcludesNoneAndSaved(t *testing.T) {
	sqlDB := openTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	company, _ := CreateCompany(ctx, sqlDB, "Acme", nil, now)

	states := []model.ApplicationState{
		model.AppNone, model.AppSaved, model.AppApplied, model.AppInterview,
		model.AppOffer, model.AppAccepted, model.AppTurnedDown, model.AppNegative,
	}
	for i, st := range states {
		o := seedMergeOffer(t, sqlDB, company, "Offer "+string(rune('A'+i)), "Paris")
		setAppState(t, sqlDB, o.ID, st, now)
	}

	got, err := QueryOffers(ctx, sqlDB, OfferQuery{HasApplication: true, Sort: SortLastSeen})
	if err != nil {
		t.Fatal(err)
	}
	if got.Total != 6 {
		t.Errorf("has_application total = %d, want 6", got.Total)
	}
	for _, o := range got.Offers {
		if o.ApplicationState == model.AppNone || o.ApplicationState == model.AppSaved {
			t.Errorf("has_application returned %q", o.ApplicationState)
		}
	}
}
