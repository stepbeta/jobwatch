package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// ErrInvalidQuery is returned when a full-text search expression is not
// valid FTS5 syntax.
var ErrInvalidQuery = errors.New("invalid search query")

// OfferQuery narrows QueryOffers. Zero values mean "no filter". Search
// runs against the job_offers FTS5 index (title and description).
type OfferQuery struct {
	Search                string
	ApplicationStates     []model.ApplicationState
	OfferStates           []model.OfferState
	RemotePolicies        []model.RemotePolicy
	MinTitleConfidence    *int
	MinLocationConfidence *int
	CompanyID             *int64
	HasApplication        bool // application_state != 'todo'
	Sort                  string
	Limit                 int
	Offset                int
}

// Sort values accepted by QueryOffers.
const (
	SortLastSeen    = "last_seen"
	SortFirstSeen   = "first_seen"
	SortRecentEvent = "recent_event"
	SortTitle       = "title"
	SortCompany     = "company"
)

// ValidOfferSort reports whether s is a supported offer sort.
func ValidOfferSort(s string) bool {
	switch s {
	case SortLastSeen, SortFirstSeen, SortRecentEvent, SortTitle, SortCompany:
		return true
	}
	return false
}

// OfferList is a page of offers plus the total number of matches.
type OfferList struct {
	Offers []OfferListItem
	Total  int
}

// QueryOffers returns offers matching f, ordered and paginated.
func QueryOffers(ctx context.Context, q Querier, f OfferQuery) (OfferList, error) {
	where, args, err := offerWhere(ctx, q, f)
	if err != nil {
		return OfferList{}, err
	}
	var total int
	if err := q.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM job_offers o JOIN companies c ON c.id = o.company_id`+where, args...).Scan(&total); err != nil {
		return OfferList{}, fmt.Errorf("count offers: %w", err)
	}

	query := `SELECT c.name, (SELECT MAX(e.occurred_at) FROM events e WHERE e.offer_id = o.id), ` +
		offerColumns("o.") + ` FROM job_offers o JOIN companies c ON c.id = o.company_id` + where +
		` ORDER BY ` + offerOrder(f.Sort)
	qargs := append([]any{}, args...)
	if f.Limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		qargs = append(qargs, f.Limit, f.Offset)
	}
	rows, err := q.QueryContext(ctx, query, qargs...)
	if err != nil {
		return OfferList{}, fmt.Errorf("list offers: %w", err)
	}
	defer rows.Close()

	out := OfferList{Offers: []OfferListItem{}, Total: total}
	for rows.Next() {
		var item OfferListItem
		var lastEvent sql.NullString
		o, err := scanOffer(listScanner{rows: rows, company: &item.CompanyName, lastEvent: &lastEvent})
		if err != nil {
			return OfferList{}, fmt.Errorf("list offers: %w", err)
		}
		item.JobOffer = o
		if lastEvent.Valid {
			t := parseTime(lastEvent.String)
			item.LastEventAt = &t
		}
		out.Offers = append(out.Offers, item)
	}
	return out, rows.Err()
}

// listScanner scans the two leading list columns before the offer
// columns.
type listScanner struct {
	rows      *sql.Rows
	company   *string
	lastEvent *sql.NullString
}

func (l listScanner) Scan(dest ...any) error {
	return l.rows.Scan(append([]any{l.company, l.lastEvent}, dest...)...)
}

func offerWhere(ctx context.Context, q Querier, f OfferQuery) (string, []any, error) {
	var b strings.Builder
	b.WriteString(" WHERE 1=1")
	var args []any

	if s := strings.TrimSpace(f.Search); s != "" {
		// Validate the FTS5 expression up front so a syntax error is a
		// client error rather than an internal one.
		rows, err := q.QueryContext(ctx,
			`SELECT COUNT(*) FROM job_offers_fts WHERE job_offers_fts MATCH ?`, s)
		if err != nil {
			return "", nil, fmt.Errorf("%w: %v", ErrInvalidQuery, err)
		}
		rows.Close()
		b.WriteString(` AND o.id IN (SELECT rowid FROM job_offers_fts WHERE job_offers_fts MATCH ?)`)
		args = append(args, s)
	}
	if len(f.ApplicationStates) > 0 {
		b.WriteString(` AND o.application_state IN (` + placeholders(len(f.ApplicationStates)) + `)`)
		for _, s := range f.ApplicationStates {
			args = append(args, string(s))
		}
	}
	if len(f.OfferStates) > 0 {
		b.WriteString(` AND o.offer_state IN (` + placeholders(len(f.OfferStates)) + `)`)
		for _, s := range f.OfferStates {
			args = append(args, string(s))
		}
	}
	if len(f.RemotePolicies) > 0 {
		b.WriteString(` AND o.remote_policy IN (` + placeholders(len(f.RemotePolicies)) + `)`)
		for _, s := range f.RemotePolicies {
			args = append(args, string(s))
		}
	}
	if f.MinTitleConfidence != nil {
		b.WriteString(` AND o.title_confidence IS NOT NULL AND o.title_confidence >= ?`)
		args = append(args, *f.MinTitleConfidence)
	}
	if f.MinLocationConfidence != nil {
		b.WriteString(` AND o.location_confidence IS NOT NULL AND o.location_confidence >= ?`)
		args = append(args, *f.MinLocationConfidence)
	}
	if f.CompanyID != nil {
		b.WriteString(` AND o.company_id = ?`)
		args = append(args, *f.CompanyID)
	}
	if f.HasApplication {
		b.WriteString(` AND o.application_state != 'todo'`)
	}
	return b.String(), args, nil
}

func offerOrder(sortBy string) string {
	switch sortBy {
	case SortFirstSeen:
		return `o.first_seen_at DESC, o.id DESC`
	case SortRecentEvent:
		return `COALESCE((SELECT MAX(e.occurred_at) FROM events e WHERE e.offer_id = o.id), o.updated_at) DESC, o.id DESC`
	case SortTitle:
		return `o.title COLLATE NOCASE ASC, o.id ASC`
	case SortCompany:
		return `c.name COLLATE NOCASE ASC, o.title COLLATE NOCASE ASC, o.id ASC`
	default:
		return `o.last_seen_at DESC, o.id DESC`
	}
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// OfferUpdate is a partial update of an offer. nil fields are left
// unchanged; an empty Location clears the column. A non-nil Description
// replaces raw_text (empty stores no description), recomputes text_hash
// and appends a snapshot in the same transaction.
type OfferUpdate struct {
	ApplicationState *model.ApplicationState
	OfferState       *model.OfferState
	Location         *string
	Description      *string
}

// UpdateOfferFields applies u atomically and bumps updated_at. Offer
// fingerprints are not recomputed: location is a display/collision-bucket
// field, and the fingerprint stays stable for dedup. A description edit
// is versioned with a new snapshot row, exactly like a scan-captured
// description.
func UpdateOfferFields(ctx context.Context, sqlDB *sql.DB, id int64, u OfferUpdate, now time.Time) error {
	var sets []string
	var args []any
	if u.ApplicationState != nil {
		sets = append(sets, "application_state = ?")
		args = append(args, string(*u.ApplicationState))
	}
	if u.OfferState != nil {
		sets = append(sets, "offer_state = ?")
		args = append(args, string(*u.OfferState))
	}
	if u.Location != nil {
		sets = append(sets, "location = ?")
		var loc *string
		if *u.Location != "" {
			loc = u.Location
		}
		args = append(args, nullStr(loc))
	}
	if u.Description != nil {
		sets = append(sets, "raw_text = ?", "text_hash = ?")
		desc := strings.TrimSpace(*u.Description)
		var rawText, textHash *string
		if desc != "" {
			h := model.TextHash(desc)
			rawText, textHash = &desc, &h
		}
		args = append(args, nullStr(rawText), nullStr(textHash))
	}
	if len(sets) == 0 {
		return nil
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin offer %d update: %w", id, err)
	}
	defer tx.Rollback()

	sets = append(sets, "updated_at = ?")
	args = append(args, model.FormatTime(now), id)
	res, err := tx.ExecContext(ctx,
		`UPDATE job_offers SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return fmt.Errorf("update offer %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if u.Description != nil {
		if err := InsertSnapshotOrigin(ctx, tx, id, strings.TrimSpace(*u.Description), model.SnapshotOriginManual, now); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit offer %d update: %w", id, err)
	}
	return nil
}

// LatestSnapshot returns the most recent description snapshot of an
// offer, or ErrNotFound when none exists.
func LatestSnapshot(ctx context.Context, q Querier, offerID int64) (model.Snapshot, error) {
	var s model.Snapshot
	var origin, captured sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT id, offer_id, origin, raw_text, captured_at FROM snapshots WHERE offer_id = ? ORDER BY captured_at DESC, id DESC LIMIT 1`,
		offerID).Scan(&s.ID, &s.OfferID, &origin, &s.RawText, &captured)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Snapshot{}, ErrNotFound
	}
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("latest snapshot of offer %d: %w", offerID, err)
	}
	s.Origin = model.SnapshotOrigin(origin.String)
	s.CapturedAt = parseNullTime(captured)
	return s, nil
}

// LatestSnapshots returns up to limit snapshots of an offer, newest
// first. It is used for the changed-offer diff view.
func LatestSnapshots(ctx context.Context, q Querier, offerID int64, limit int) ([]model.Snapshot, error) {
	if limit <= 0 {
		return []model.Snapshot{}, nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, offer_id, origin, raw_text, captured_at FROM snapshots
		 WHERE offer_id = ? ORDER BY captured_at DESC, id DESC LIMIT ?`, offerID, limit)
	if err != nil {
		return nil, fmt.Errorf("latest snapshots of offer %d: %w", offerID, err)
	}
	defer rows.Close()
	out := []model.Snapshot{}
	for rows.Next() {
		var s model.Snapshot
		var origin, captured sql.NullString
		if err := rows.Scan(&s.ID, &s.OfferID, &origin, &s.RawText, &captured); err != nil {
			return nil, fmt.Errorf("latest snapshots of offer %d: %w", offerID, err)
		}
		s.Origin = model.SnapshotOrigin(origin.String)
		s.CapturedAt = parseNullTime(captured)
		out = append(out, s)
	}
	return out, rows.Err()
}

// ProvenanceItem is a provenance row joined with its source name.
type ProvenanceItem struct {
	model.Provenance
	SourceName *string
}

// ListProvenanceDetailed returns every link an offer was seen at,
// including the source name for display.
func ListProvenanceDetailed(ctx context.Context, q Querier, offerID int64) ([]ProvenanceItem, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT p.id, p.offer_id, p.source_id, p.external_uid, p.url, p.first_seen_at, p.last_seen_at, s.name
		 FROM job_offer_provenance p
		 LEFT JOIN sources s ON s.id = p.source_id
		 WHERE p.offer_id = ? ORDER BY p.id`, offerID)
	if err != nil {
		return nil, fmt.Errorf("list provenance for offer %d: %w", offerID, err)
	}
	defer rows.Close()
	out := []ProvenanceItem{}
	for rows.Next() {
		var p ProvenanceItem
		var src sql.NullInt64
		var uid, first, last, sourceName sql.NullString
		if err := rows.Scan(&p.ID, &p.OfferID, &src, &uid, &p.URL, &first, &last, &sourceName); err != nil {
			return nil, fmt.Errorf("list provenance: %w", err)
		}
		p.SourceID = int64Ptr(src)
		p.ExternalUID = strPtr(uid)
		p.FirstSeenAt = parseNullTime(first)
		p.LastSeenAt = parseNullTime(last)
		p.SourceName = strPtr(sourceName)
		out = append(out, p)
	}
	return out, rows.Err()
}
