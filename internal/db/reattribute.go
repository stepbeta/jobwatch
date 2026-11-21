package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// SourceProvenanceOffer is one (source, external_uid) link together with
// the offer fields the re-attribution pass needs to decide whether the
// offer should move to another company or title.
type SourceProvenanceOffer struct {
	OfferID       int64
	ExternalUID   string
	Title         string
	CompanyID     *int64
	Location      *string
	RawText       *string
	IncompleteRaw *string
}

// ListSourceProvenanceOffers returns the offers linked to sourceID by a
// non-empty external uid. Links without a uid are not re-attributable:
// they are matched by URL only, and re-attribution is defined on
// (source, uid).
func ListSourceProvenanceOffers(ctx context.Context, q Querier, sourceID int64) ([]SourceProvenanceOffer, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT p.offer_id, p.external_uid, o.title, o.company_id, o.location, o.raw_text, o.incomplete_raw
		FROM job_offer_provenance p
		JOIN job_offers o ON o.id = p.offer_id
		WHERE p.source_id = ? AND p.external_uid IS NOT NULL AND TRIM(p.external_uid) <> ''
		ORDER BY p.id`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list provenance of source %d: %w", sourceID, err)
	}
	defer rows.Close()

	var out []SourceProvenanceOffer
	for rows.Next() {
		var link SourceProvenanceOffer
		var companyID sql.NullInt64
		var location, rawText, incompleteRaw sql.NullString
		if err := rows.Scan(&link.OfferID, &link.ExternalUID, &link.Title, &companyID,
			&location, &rawText, &incompleteRaw); err != nil {
			return nil, fmt.Errorf("list provenance of source %d: %w", sourceID, err)
		}
		link.CompanyID = int64Ptr(companyID)
		link.Location = strPtr(location)
		link.RawText = strPtr(rawText)
		link.IncompleteRaw = strPtr(incompleteRaw)
		out = append(out, link)
	}
	return out, rows.Err()
}

// ReattributeOffer moves an offer to the company and title the source now
// reports. It touches no event, note, snapshot, application state or
// listing state, and does not change last_seen_at.
func ReattributeOffer(ctx context.Context, q Querier, id, companyID int64,
	title, titleNormalized string, now time.Time) error {
	res, err := q.ExecContext(ctx, `UPDATE job_offers
		SET company_id = ?, title = ?, title_normalized = ?, updated_at = ?
		WHERE id = ?`,
		companyID, title, titleNormalized, model.FormatTime(now), id)
	if err != nil {
		return fmt.Errorf("reattribute offer %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CompaniesWithoutOffers returns the names of the given companies that no
// longer have any offer. Used to report the companies a re-attribution
// pass emptied; the rows stay visible for the user to delete.
func CompaniesWithoutOffers(ctx context.Context, q Querier, ids []int64) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := q.QueryContext(ctx, `
		SELECT c.name FROM companies c
		WHERE c.id IN (`+placeholders(len(ids))+`)
		  AND NOT EXISTS (SELECT 1 FROM job_offers o WHERE o.company_id = c.id)
		ORDER BY c.name COLLATE NOCASE, c.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list emptied companies: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("list emptied companies: %w", err)
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
