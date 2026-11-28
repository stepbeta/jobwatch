package db

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func insertTestOffer(t *testing.T, sqlDB *sql.DB, title string, state model.OfferState, now time.Time) int64 {
	t.Helper()
	ts := model.FormatTime(now)
	res, err := sqlDB.Exec(
		`INSERT INTO job_offers(title, title_normalized, first_seen_at, last_seen_at, offer_state)
		 VALUES (?, ?, ?, ?, ?)`,
		title, model.NormalizeTitle(title), ts, ts, string(state))
	if err != nil {
		t.Fatalf("insert offer %q: %v", title, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertTestProvenance(t *testing.T, sqlDB *sql.DB, offerID, sourceID int64, now time.Time) {
	t.Helper()
	ts := model.FormatTime(now)
	if _, err := sqlDB.Exec(
		`INSERT INTO job_offer_provenance(offer_id, source_id, url, first_seen_at, last_seen_at)
		 VALUES (?, ?, ?, ?, ?)`,
		offerID, sourceID, "https://example.com/"+model.FormatTime(now), ts, ts); err != nil {
		t.Fatalf("insert provenance %d/%d: %v", offerID, sourceID, err)
	}
}

func TestCountActiveSourceOffers(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		setup func(t *testing.T, sqlDB *sql.DB, sourceID int64)
		want  int
	}{
		{
			name:  "source with no offers",
			setup: func(*testing.T, *sql.DB, int64) {},
			want:  0,
		},
		{
			name: "source with active offers",
			setup: func(t *testing.T, sqlDB *sql.DB, sourceID int64) {
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "SRE", model.OfferActive, now), sourceID, now)
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "Platform Engineer", model.OfferActive, now), sourceID, now)
			},
			want: 2,
		},
		{
			name: "source whose offers are all disappeared",
			setup: func(t *testing.T, sqlDB *sql.DB, sourceID int64) {
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "Old SRE", model.OfferDisappeared, now), sourceID, now)
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "Old Platform", model.OfferDisappeared, now), sourceID, now)
			},
			want: 0,
		},
		{
			name: "offers of another source are not counted",
			setup: func(t *testing.T, sqlDB *sql.DB, sourceID int64) {
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "Mine", model.OfferActive, now), sourceID, now)
				other, err := InsertSource(context.Background(), sqlDB, model.Source{
					Name: "Other", URL: "https://example.com/other", Type: model.SourceLever, Enabled: true,
				}, now)
				if err != nil {
					t.Fatal(err)
				}
				insertTestProvenance(t, sqlDB, insertTestOffer(t, sqlDB, "Theirs", model.OfferActive, now), other.ID, now)
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB := openTest(t)
			src, err := InsertSource(ctx, sqlDB, model.Source{
				Name: "Acme", URL: "https://example.com/acme", Type: model.SourceGreenhouse, Enabled: true,
			}, now)
			if err != nil {
				t.Fatal(err)
			}
			tt.setup(t, sqlDB, src.ID)
			got, err := CountActiveSourceOffers(ctx, sqlDB, src.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("CountActiveSourceOffers = %d, want %d", got, tt.want)
			}
		})
	}
}
