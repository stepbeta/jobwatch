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

// Offer merge and split errors.
var (
	ErrOfferMergeSelf         = errors.New("cannot merge an offer into itself")
	ErrOfferSplitProvenance   = errors.New("selected provenance rows do not all belong to the offer")
	ErrOfferSplitLastLink     = errors.New("at least one provenance row must stay on the original offer")
	ErrOfferSplitNoProvenance = errors.New("select at least one provenance row to move")
)

// OfferMergeResult reports what an explicit offer merge moved.
type OfferMergeResult struct {
	Offer           model.JobOffer
	MergedOfferID   int64
	MovedProvenance int
	MovedEvents     int
	MovedNotes      int
	MovedSnapshots  int
}

// MergeOffers moves every provenance row, event, note and snapshot of
// fromID to intoID, keeps the earliest first-seen and latest last-seen
// timestamps, and removes the absorbed offer row. It only runs after an
// explicit user confirmation (SPEC section 5); offers are never merged by
// scans.
func MergeOffers(ctx context.Context, sqlDB *sql.DB, fromID, intoID int64, now time.Time) (OfferMergeResult, error) {
	if fromID == intoID {
		return OfferMergeResult{}, ErrOfferMergeSelf
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return OfferMergeResult{}, fmt.Errorf("begin offer merge: %w", err)
	}
	defer tx.Rollback()

	from, err := GetOffer(ctx, tx, fromID)
	if err != nil {
		return OfferMergeResult{}, err
	}
	into, err := GetOffer(ctx, tx, intoID)
	if err != nil {
		return OfferMergeResult{}, err
	}

	res := OfferMergeResult{MergedOfferID: fromID}
	if res.MovedProvenance, err = mergeProvenance(ctx, tx, fromID, intoID); err != nil {
		return OfferMergeResult{}, err
	}
	rows, err := tx.ExecContext(ctx, `UPDATE events SET offer_id = ? WHERE offer_id = ?`, intoID, fromID)
	if err != nil {
		return OfferMergeResult{}, fmt.Errorf("move events of offer %d: %w", fromID, err)
	}
	n, _ := rows.RowsAffected()
	res.MovedEvents = int(n)

	rows, err = tx.ExecContext(ctx,
		`UPDATE notes SET entity_id = ?, updated_at = ? WHERE entity_type = 'offer' AND entity_id = ?`,
		intoID, model.FormatTime(now), fromID)
	if err != nil {
		return OfferMergeResult{}, fmt.Errorf("move notes of offer %d: %w", fromID, err)
	}
	n, _ = rows.RowsAffected()
	res.MovedNotes = int(n)

	rows, err = tx.ExecContext(ctx, `UPDATE snapshots SET offer_id = ? WHERE offer_id = ?`, intoID, fromID)
	if err != nil {
		return OfferMergeResult{}, fmt.Errorf("move snapshots of offer %d: %w", fromID, err)
	}
	n, _ = rows.RowsAffected()
	res.MovedSnapshots = int(n)

	if err := updateMergedOffer(ctx, tx, from, into, now); err != nil {
		return OfferMergeResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM job_offers WHERE id = ?`, fromID); err != nil {
		return OfferMergeResult{}, fmt.Errorf("delete merged offer %d: %w", fromID, err)
	}
	if err := tx.Commit(); err != nil {
		return OfferMergeResult{}, fmt.Errorf("commit offer merge: %w", err)
	}
	merged, err := GetOffer(ctx, sqlDB, intoID)
	if err != nil {
		return OfferMergeResult{}, err
	}
	res.Offer = merged
	return res, nil
}

// mergeProvenance moves provenance rows and collapses duplicate URLs:
// when the target already has the same URL, the target row keeps the
// earliest first-seen and latest last-seen timestamps instead of creating
// a duplicate.
func mergeProvenance(ctx context.Context, q Querier, fromID, intoID int64) (int, error) {
	type prov struct {
		id        int64
		sourceID  sql.NullInt64
		uid       sql.NullString
		url       string
		firstSeen sql.NullString
		lastSeen  sql.NullString
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, source_id, external_uid, url, first_seen_at, last_seen_at
		 FROM job_offer_provenance WHERE offer_id = ? ORDER BY id`, fromID)
	if err != nil {
		return 0, fmt.Errorf("list provenance of offer %d: %w", fromID, err)
	}
	var provs []prov
	for rows.Next() {
		var p prov
		if err := rows.Scan(&p.id, &p.sourceID, &p.uid, &p.url, &p.firstSeen, &p.lastSeen); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan provenance of offer %d: %w", fromID, err)
		}
		provs = append(provs, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	moved := 0
	for _, p := range provs {
		var targetID int64
		var sourceID sql.NullInt64
		var uid, first, last sql.NullString
		err := q.QueryRowContext(ctx,
			`SELECT id, source_id, external_uid, first_seen_at, last_seen_at
			 FROM job_offer_provenance WHERE offer_id = ? AND url = ?`, intoID, p.url).
			Scan(&targetID, &sourceID, &uid, &first, &last)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			if _, err := q.ExecContext(ctx,
				`UPDATE job_offer_provenance SET offer_id = ? WHERE id = ?`, intoID, p.id); err != nil {
				return 0, fmt.Errorf("move provenance %d: %w", p.id, err)
			}
		case err != nil:
			return 0, fmt.Errorf("look up target provenance: %w", err)
		default:
			if _, err := q.ExecContext(ctx,
				`UPDATE job_offer_provenance SET source_id = ?, external_uid = ?, first_seen_at = ?, last_seen_at = ?
				 WHERE id = ?`,
				firstValidInt(sourceID, p.sourceID), firstValidStr(uid, p.uid),
				earlierTimestamp(first, p.firstSeen), laterTimestamp(last, p.lastSeen), targetID); err != nil {
				return 0, fmt.Errorf("merge provenance %d: %w", targetID, err)
			}
			if _, err := q.ExecContext(ctx, `DELETE FROM job_offer_provenance WHERE id = ?`, p.id); err != nil {
				return 0, fmt.Errorf("delete duplicate provenance %d: %w", p.id, err)
			}
		}
		moved++
	}
	return moved, nil
}

func updateMergedOffer(ctx context.Context, q Querier, from, into model.JobOffer, now time.Time) error {
	firstSeen := from.FirstSeenAt
	if into.FirstSeenAt.Before(firstSeen) {
		firstSeen = into.FirstSeenAt
	}
	lastSeen := from.LastSeenAt
	if into.LastSeenAt.After(lastSeen) {
		lastSeen = into.LastSeenAt
	}
	url := into.URL
	if url == nil {
		url = from.URL
	}
	location := into.Location
	if location == nil {
		location = from.Location
	}
	rawText := into.RawText
	if from.RawText != nil && (into.RawText == nil || from.LastSeenAt.After(into.LastSeenAt)) {
		rawText = from.RawText
	}
	textHash := into.TextHash
	if rawText == from.RawText {
		textHash = from.TextHash
	}
	appState := into.ApplicationState
	if appState == model.AppTodo && from.ApplicationState != model.AppTodo {
		appState = from.ApplicationState
	}
	scores := classify.Classify(into.Title, derefString(location), derefString(rawText)).Scores()
	titleConf, locConf := scores.TitleConfidence, scores.LocationConfidence
	reasons := scores.MatchReasonsJSON
	if _, err := q.ExecContext(ctx, `UPDATE job_offers SET
		url = ?, location = ?, remote_policy = ?, first_seen_at = ?, last_seen_at = ?,
		offer_state = ?, application_state = ?, raw_text = ?, text_hash = ?,
		title_confidence = ?, location_confidence = ?, match_reasons_json = ?, updated_at = ?
		WHERE id = ?`,
		nullStr(url), nullStr(location), string(scores.RemotePolicy), model.FormatTime(firstSeen), model.FormatTime(lastSeen),
		string(mergeOfferState(from.OfferState, into.OfferState)), string(appState), nullStr(rawText), nullStr(textHash),
		titleConf, locConf, reasons, model.FormatTime(now), into.ID); err != nil {
		return fmt.Errorf("update merged offer %d: %w", into.ID, err)
	}
	return nil
}

// mergeOfferState keeps the most informative lifecycle state: an active
// offer stays active, and only two disappeared offers stay disappeared.
func mergeOfferState(a, b model.OfferState) model.OfferState {
	if a == model.OfferActive || b == model.OfferActive {
		return model.OfferActive
	}
	return model.OfferDisappeared
}

// OfferSplitInput describes an explicit split of an offer: provenance
// rows move to a new offer, whose title/location may be adjusted. An empty
// Title copies the original title; a nil Location copies the original
// location, while a pointer to an empty string clears it.
type OfferSplitInput struct {
	ProvenanceIDs []int64
	Title         string
	Location      *string
}

// OfferSplitResult reports the new offer created by a split.
type OfferSplitResult struct {
	Offer           model.JobOffer
	MovedProvenance int
}

// SplitOffer creates a second offer in the same company and moves the
// selected provenance rows to it. Events, notes and snapshots stay on the
// original offer because they belong to the tracked application; at least
// one provenance row must remain on the original. The new offer may share
// a company, title and location with another offer: identity is the
// provenance, not the fingerprint.
func SplitOffer(ctx context.Context, sqlDB *sql.DB, offerID int64, in OfferSplitInput, now time.Time) (OfferSplitResult, error) {
	ids := uniqueIDs(in.ProvenanceIDs)
	if len(ids) == 0 {
		return OfferSplitResult{}, ErrOfferSplitNoProvenance
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return OfferSplitResult{}, fmt.Errorf("begin offer split: %w", err)
	}
	defer tx.Rollback()

	original, err := GetOffer(ctx, tx, offerID)
	if err != nil {
		return OfferSplitResult{}, err
	}
	if original.CompanyID == nil {
		return OfferSplitResult{}, fmt.Errorf("split offer: the offer has no company")
	}
	company, err := GetCompany(ctx, tx, *original.CompanyID)
	if err != nil {
		return OfferSplitResult{}, err
	}

	type prov struct {
		id        int64
		url       string
		firstSeen sql.NullString
		lastSeen  sql.NullString
	}
	args := []any{offerID}
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := tx.QueryContext(ctx,
		`SELECT id, url, first_seen_at, last_seen_at FROM job_offer_provenance
		 WHERE offer_id = ? AND id IN (`+placeholders(len(ids))+`) ORDER BY id`, args...)
	if err != nil {
		return OfferSplitResult{}, fmt.Errorf("list selected provenance: %w", err)
	}
	var selected []prov
	for rows.Next() {
		var p prov
		if err := rows.Scan(&p.id, &p.url, &p.firstSeen, &p.lastSeen); err != nil {
			rows.Close()
			return OfferSplitResult{}, fmt.Errorf("scan selected provenance: %w", err)
		}
		selected = append(selected, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return OfferSplitResult{}, err
	}
	if len(selected) != len(ids) {
		return OfferSplitResult{}, ErrOfferSplitProvenance
	}
	var total int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM job_offer_provenance WHERE offer_id = ?`, offerID).Scan(&total); err != nil {
		return OfferSplitResult{}, fmt.Errorf("count provenance of offer %d: %w", offerID, err)
	}
	if len(selected) >= total {
		return OfferSplitResult{}, ErrOfferSplitLastLink
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		title = original.Title
	}
	titleNorm := model.NormalizeTitle(title)
	if titleNorm == "" {
		return OfferSplitResult{}, fmt.Errorf("split offer: empty normalized title")
	}
	location := original.Location
	if in.Location != nil {
		loc := strings.TrimSpace(*in.Location)
		location = nil
		if loc != "" {
			location = &loc
		}
	}

	fp := model.Fingerprint(company.NameNormalized, titleNorm, "")

	firstSeen := parseNullTime(selected[0].firstSeen)
	lastSeen := parseNullTime(selected[0].lastSeen)
	for _, p := range selected[1:] {
		if t := parseNullTime(p.firstSeen); !t.IsZero() && (firstSeen.IsZero() || t.Before(firstSeen)) {
			firstSeen = t
		}
		if t := parseNullTime(p.lastSeen); t.After(lastSeen) {
			lastSeen = t
		}
	}
	if firstSeen.IsZero() {
		firstSeen = now
	}
	if lastSeen.IsZero() {
		lastSeen = now
	}

	state := model.OfferActive
	if original.OfferState == model.OfferDisappeared {
		state = model.OfferDisappeared
	}
	scores := classify.Classify(title, derefString(location), "").Scores()
	titleConf, locConf := scores.TitleConfidence, scores.LocationConfidence
	reasons := scores.MatchReasonsJSON
	newOffer := model.JobOffer{
		CompanyID:          &company.ID,
		Title:              title,
		TitleNormalized:    titleNorm,
		Location:           location,
		RemotePolicy:       scores.RemotePolicy,
		URL:                &selected[0].url,
		Fingerprint:        fp,
		FirstSeenAt:        firstSeen,
		LastSeenAt:         lastSeen,
		OfferState:         state,
		ApplicationState:   model.AppTodo,
		TitleConfidence:    &titleConf,
		LocationConfidence: &locConf,
		MatchReasonsJSON:   &reasons,
	}
	created, err := InsertOffer(ctx, tx, newOffer, now)
	if err != nil {
		return OfferSplitResult{}, err
	}

	moveArgs := []any{created.ID, offerID}
	for _, id := range ids {
		moveArgs = append(moveArgs, id)
	}
	rowsAffected, err := tx.ExecContext(ctx,
		`UPDATE job_offer_provenance SET offer_id = ? WHERE offer_id = ? AND id IN (`+placeholders(len(ids))+`)`,
		moveArgs...)
	if err != nil {
		return OfferSplitResult{}, fmt.Errorf("move provenance to offer %d: %w", created.ID, err)
	}
	moved, _ := rowsAffected.RowsAffected()

	if err := repairOriginalURL(ctx, tx, original, now); err != nil {
		return OfferSplitResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return OfferSplitResult{}, fmt.Errorf("commit offer split: %w", err)
	}
	created, err = GetOffer(ctx, sqlDB, created.ID)
	if err != nil {
		return OfferSplitResult{}, err
	}
	return OfferSplitResult{Offer: created, MovedProvenance: int(moved)}, nil
}

// repairOriginalURL keeps the original offer's canonical URL pointing at
// one of its remaining links after a split.
func repairOriginalURL(ctx context.Context, q Querier, original model.JobOffer, now time.Time) error {
	if original.URL == nil {
		return nil
	}
	var n int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM job_offer_provenance WHERE offer_id = ? AND url = ?`,
		original.ID, *original.URL).Scan(&n); err != nil {
		return fmt.Errorf("check original URL: %w", err)
	}
	if n > 0 {
		return nil
	}
	var next sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT url FROM job_offer_provenance WHERE offer_id = ? ORDER BY id LIMIT 1`, original.ID).Scan(&next)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("find replacement URL: %w", err)
	}
	if _, err := q.ExecContext(ctx,
		`UPDATE job_offers SET url = ?, updated_at = ? WHERE id = ?`,
		nullStr(strPtr(next)), model.FormatTime(now), original.ID); err != nil {
		return fmt.Errorf("repair URL of offer %d: %w", original.ID, err)
	}
	return nil
}

func uniqueIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func firstValidInt(a, b sql.NullInt64) sql.NullInt64 {
	if a.Valid {
		return a
	}
	return b
}

func firstValidStr(a, b sql.NullString) sql.NullString {
	if a.Valid {
		return a
	}
	return b
}

func earlierTimestamp(a, b sql.NullString) sql.NullString {
	if !a.Valid {
		return b
	}
	if !b.Valid {
		return a
	}
	if parseTime(b.String).Before(parseTime(a.String)) {
		return b
	}
	return a
}

func laterTimestamp(a, b sql.NullString) sql.NullString {
	if !a.Valid {
		return b
	}
	if !b.Valid {
		return a
	}
	if parseTime(b.String).After(parseTime(a.String)) {
		return b
	}
	return a
}
