package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/model"
)

// ErrOfferNotIncomplete is returned when a repair is requested for an
// offer that is already complete.
var ErrOfferNotIncomplete = errors.New("offer is not incomplete")

// InsertIncompleteOffer stores an item a source could not normalize. The
// company may be nil and the title may be empty; incompleteRaw is the
// JSON of the values the source reported. The offer is still a normal
// row: it keeps provenance, snapshots and notes, and a later scan can
// find it by identity.
func InsertIncompleteOffer(ctx context.Context, q Querier, o model.JobOffer,
	reason model.IncompleteReason, incompleteRaw string, now time.Time) (model.JobOffer, error) {
	if !reason.Valid() {
		return model.JobOffer{}, fmt.Errorf("insert incomplete offer: invalid reason %q", reason)
	}
	if o.OfferState == "" {
		o.OfferState = model.OfferActive
	}
	if o.ApplicationState == "" {
		o.ApplicationState = model.AppNone
	}
	if o.RemotePolicy == "" {
		o.RemotePolicy = model.RemoteUnknown
	}
	if strings.TrimSpace(incompleteRaw) == "" {
		incompleteRaw = "{}"
	}
	ts := model.FormatTime(now)
	res, err := q.ExecContext(ctx, `INSERT INTO job_offers(
		company_id, title, title_normalized, location, remote_policy, url, fingerprint,
		first_seen_at, last_seen_at, offer_state, application_state, raw_text, text_hash,
		incomplete_reason, incomplete_raw, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(o.CompanyID), o.Title, o.TitleNormalized, nullStr(o.Location), string(o.RemotePolicy),
		nullStr(o.URL), o.Fingerprint, ts, ts, string(o.OfferState), string(o.ApplicationState),
		nullStr(o.RawText), nullStr(o.TextHash), string(reason), incompleteRaw, ts, ts)
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("insert incomplete offer %q: %w", o.Title, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("insert incomplete offer %q: %w", o.Title, err)
	}
	t := now.UTC().Truncate(time.Second)
	o.ID = id
	o.IncompleteReason = reason
	o.IncompleteRaw = &incompleteRaw
	o.FirstSeenAt, o.LastSeenAt = t, t
	o.CreatedAt, o.UpdatedAt = t, t
	return o, nil
}

// RepairOffer assigns a company and title to an incomplete offer, clears
// the incomplete reason and re-runs the classifier. The raw reported
// values stay in incomplete_raw: that column is the durable marker that
// tells later scans not to overwrite the user's title and location.
func RepairOffer(ctx context.Context, sqlDB *sql.DB, id, companyID int64, title string,
	location *string, now time.Time) (model.JobOffer, error) {
	title = strings.TrimSpace(title)
	norm := model.NormalizeTitle(title)
	if norm == "" {
		return model.JobOffer{}, fmt.Errorf("repair offer: title is required")
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("begin offer %d repair: %w", id, err)
	}
	defer tx.Rollback()

	o, err := GetOffer(ctx, tx, id)
	if err != nil {
		return model.JobOffer{}, err
	}
	if o.IncompleteReason == "" {
		return model.JobOffer{}, ErrOfferNotIncomplete
	}
	if _, err := GetCompany(ctx, tx, companyID); err != nil {
		return model.JobOffer{}, err
	}
	loc := o.Location
	if location != nil {
		loc = nil
		if v := strings.TrimSpace(*location); v != "" {
			loc = &v
		}
	}
	policy := classify.DeriveRemotePolicy(derefString(loc), derefString(o.RawText))
	if _, err := tx.ExecContext(ctx, `UPDATE job_offers
		SET company_id = ?, title = ?, title_normalized = ?, location = ?, remote_policy = ?,
		    incomplete_reason = NULL, updated_at = ?
		WHERE id = ?`,
		companyID, title, norm, nullStr(loc), string(policy), model.FormatTime(now), id); err != nil {
		return model.JobOffer{}, fmt.Errorf("repair offer %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return model.JobOffer{}, fmt.Errorf("commit offer %d repair: %w", id, err)
	}
	return GetOffer(ctx, sqlDB, id)
}
