package scan

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/db"
)

// Reclassify re-runs the deterministic classifier over stored offers in
// one transaction and returns the number of offers updated. With all
// false only offers without scores are touched; with all true every offer
// is recomputed (useful after tweaking keyword tables).
func Reclassify(ctx context.Context, sqlDB *sql.DB, all bool) (int, error) {
	offers, err := db.ListOffersForClassification(ctx, sqlDB, !all)
	if err != nil {
		return 0, err
	}
	if len(offers) == 0 {
		return 0, nil
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin reclassification: %w", err)
	}
	defer tx.Rollback()
	for _, o := range offers {
		scores := classify.Classify(o.Title, strValue(o.Location), strValue(o.RawText)).Scores()
		if err := db.SetOfferClassification(ctx, tx, o.ID, scores); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit reclassification: %w", err)
	}
	return len(offers), nil
}
