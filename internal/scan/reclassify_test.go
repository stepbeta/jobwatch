package scan

import (
	"context"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

func TestReclassify(t *testing.T) {
	sqlDB, _, _, _ := setup(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	company, err := db.CreateCompany(ctx, sqlDB, "Acme", nil, now)
	if err != nil {
		t.Fatal(err)
	}
	location := "Remote - France"
	o, err := db.InsertOffer(ctx, sqlDB, model.JobOffer{
		CompanyID:       &company.ID,
		Title:           "Platform Engineer",
		TitleNormalized: model.NormalizeTitle("Platform Engineer"),
		Location:        &location,
		Fingerprint:     model.Fingerprint(company.NameNormalized, model.NormalizeTitle("Platform Engineer"), ""),
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	n, err := Reclassify(ctx, sqlDB, false)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("reclassified = %d, want 1", n)
	}
	got, err := db.GetOffer(ctx, sqlDB, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TitleConfidence == nil || *got.TitleConfidence != 85 {
		t.Errorf("title_confidence = %v, want 85", got.TitleConfidence)
	}
	if got.LocationConfidence == nil || *got.LocationConfidence != 80 {
		t.Errorf("location_confidence = %v, want 80", got.LocationConfidence)
	}
	if got.RemotePolicy != model.RemoteFull {
		t.Errorf("remote_policy = %q, want remote", got.RemotePolicy)
	}
	if n, err := Reclassify(ctx, sqlDB, false); err != nil || n != 0 {
		t.Errorf("second reclassify = %d, %v; want 0", n, err)
	}
	if n, err := Reclassify(ctx, sqlDB, true); err != nil || n != 1 {
		t.Errorf("reclassify all = %d, %v; want 1", n, err)
	}
}
