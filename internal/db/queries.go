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

// Querier is satisfied by both *sql.DB and *sql.Tx so queries can run
// inside or outside a transaction.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ErrNotFound is returned when a looked-up row does not exist.
var ErrNotFound = errors.New("not found")

func nullStr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func strPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	s := n.String
	return &s
}

func nullInt(p *int64) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *p, Valid: true}
}

func nullIntFromInt(p *int) sql.NullInt64 {
	if p == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*p), Valid: true}
}

func intPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	i := int(n.Int64)
	return &i
}

func int64Ptr(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	i := n.Int64
	return &i
}

func parseTime(s string) time.Time {
	t, err := model.ParseTime(s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseNullTime(n sql.NullString) time.Time {
	if !n.Valid {
		return time.Time{}
	}
	return parseTime(n.String)
}

// ---------- companies ----------

// ---------- sources ----------

const sourceColumns = `id, name, url, source_type, config_json, enabled, last_scan_at, last_scan_status, created_at`

func scanSource(row interface{ Scan(...any) error }) (model.Source, error) {
	var s model.Source
	var typ string
	var lastAt, lastStatus, created sql.NullString
	if err := row.Scan(&s.ID, &s.Name, &s.URL, &typ, &s.ConfigJSON, &s.Enabled, &lastAt, &lastStatus, &created); err != nil {
		return model.Source{}, err
	}
	s.Type = model.SourceType(typ)
	if lastAt.Valid {
		t := parseTime(lastAt.String)
		s.LastScanAt = &t
	}
	if lastStatus.Valid {
		st := model.ScanStatus(lastStatus.String)
		s.LastScanStatus = &st
	}
	s.CreatedAt = parseNullTime(created)
	return s, nil
}

// InsertSource stores a new source and returns it with its ID.
func InsertSource(ctx context.Context, q Querier, s model.Source, now time.Time) (model.Source, error) {
	if !s.Type.Valid() {
		return model.Source{}, fmt.Errorf("insert source: invalid source_type %q", s.Type)
	}
	if s.ConfigJSON == "" {
		s.ConfigJSON = "{}"
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO sources(name, url, source_type, config_json, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		s.Name, s.URL, string(s.Type), s.ConfigJSON, s.Enabled, model.FormatTime(now))
	if err != nil {
		return model.Source{}, fmt.Errorf("insert source %q: %w", s.URL, err)
	}
	s.ID, err = res.LastInsertId()
	if err != nil {
		return model.Source{}, fmt.Errorf("insert source %q: %w", s.URL, err)
	}
	s.CreatedAt = now.UTC().Truncate(time.Second)
	return s, nil
}

// GetSource returns the source with the given ID.
func GetSource(ctx context.Context, q Querier, id int64) (model.Source, error) {
	s, err := scanSource(q.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM sources WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, ErrNotFound
	}
	if err != nil {
		return model.Source{}, fmt.Errorf("get source %d: %w", id, err)
	}
	return s, nil
}

// FindSourceByURL returns the first source registered with url.
func FindSourceByURL(ctx context.Context, q Querier, url string) (model.Source, error) {
	s, err := scanSource(q.QueryRowContext(ctx, `SELECT `+sourceColumns+` FROM sources WHERE url = ? ORDER BY id LIMIT 1`, url))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Source{}, ErrNotFound
	}
	if err != nil {
		return model.Source{}, fmt.Errorf("find source %q: %w", url, err)
	}
	return s, nil
}

// ListSources returns sources ordered by ID, optionally only enabled ones.
func ListSources(ctx context.Context, q Querier, enabledOnly bool) ([]model.Source, error) {
	query := `SELECT ` + sourceColumns + ` FROM sources`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY id`
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()
	var out []model.Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, fmt.Errorf("list sources: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateSourceScan records the time and outcome of a scan.
func UpdateSourceScan(ctx context.Context, q Querier, id int64, at time.Time, status model.ScanStatus) error {
	if _, err := q.ExecContext(ctx,
		`UPDATE sources SET last_scan_at = ?, last_scan_status = ? WHERE id = ?`,
		model.FormatTime(at), string(status), id); err != nil {
		return fmt.Errorf("update source %d scan status: %w", id, err)
	}
	return nil
}

// UpdateSource replaces the editable fields of a source. Scan status
// columns are left untouched.
func UpdateSource(ctx context.Context, q Querier, s model.Source) error {
	if !s.Type.Valid() {
		return fmt.Errorf("update source %d: invalid source_type %q", s.ID, s.Type)
	}
	if s.ConfigJSON == "" {
		s.ConfigJSON = "{}"
	}
	if _, err := q.ExecContext(ctx,
		`UPDATE sources SET name = ?, url = ?, source_type = ?, config_json = ?, enabled = ? WHERE id = ?`,
		s.Name, s.URL, string(s.Type), s.ConfigJSON, s.Enabled, s.ID); err != nil {
		return fmt.Errorf("update source %d: %w", s.ID, err)
	}
	return nil
}

// DeleteSource removes a source. Its provenance rows are kept (the
// offers and their links are never deleted) but lose their source
// reference.
func DeleteSource(ctx context.Context, sqlDB *sql.DB, id int64) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin source delete: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`UPDATE job_offer_provenance SET source_id = NULL WHERE source_id = ?`, id); err != nil {
		return fmt.Errorf("detach provenance of source %d: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sources WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete source %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit source delete: %w", err)
	}
	return nil
}

// ---------- offers ----------

// offerColumns returns the job_offers column list in scanOffer order,
// each column qualified with prefix (e.g. "o.") when non-empty.
func offerColumns(prefix string) string {
	cols := []string{"id", "company_id", "title", "title_normalized", "location", "remote_policy", "url", "fingerprint",
		"first_seen_at", "last_seen_at", "offer_state", "application_state", "raw_text", "text_hash",
		"saved_at", "created_at", "updated_at",
		"incomplete_reason", "incomplete_raw"}
	for i, c := range cols {
		cols[i] = prefix + c
	}
	return strings.Join(cols, ", ")
}

func scanOffer(row interface{ Scan(...any) error }) (model.JobOffer, error) {
	var o model.JobOffer
	var companyID sql.NullInt64
	var location, url, rawText, textHash, savedAt, created, updated sql.NullString
	var incompleteReason, incompleteRaw sql.NullString
	var remote, firstSeen, lastSeen, offerState, appState string
	if err := row.Scan(&o.ID, &companyID, &o.Title, &o.TitleNormalized, &location, &remote, &url, &o.Fingerprint,
		&firstSeen, &lastSeen, &offerState, &appState, &rawText, &textHash,
		&savedAt, &created, &updated,
		&incompleteReason, &incompleteRaw); err != nil {
		return model.JobOffer{}, err
	}
	o.CompanyID = int64Ptr(companyID)
	o.Location = strPtr(location)
	o.RemotePolicy = model.RemotePolicy(remote)
	o.URL = strPtr(url)
	o.FirstSeenAt = parseTime(firstSeen)
	o.LastSeenAt = parseTime(lastSeen)
	o.OfferState = model.OfferState(offerState)
	o.ApplicationState = model.ApplicationState(appState)
	o.RawText = strPtr(rawText)
	o.TextHash = strPtr(textHash)
	if savedAt.Valid {
		t := parseTime(savedAt.String)
		o.SavedAt = &t
	}
	if incompleteReason.Valid {
		o.IncompleteReason = model.IncompleteReason(incompleteReason.String)
	}
	o.IncompleteRaw = strPtr(incompleteRaw)
	o.CreatedAt = parseNullTime(created)
	o.UpdatedAt = parseNullTime(updated)
	return o, nil
}

// FindOfferByProvenance returns the offer a sighting belongs to. A
// non-empty uid is looked up for that source first; url is the fallback
// for the same source. A nil sourceID is a manual import, which has no
// source of its own: its url is matched against every provenance row, so
// a manually imported link attaches to the offer a scan already tracks.
func FindOfferByProvenance(ctx context.Context, q Querier, sourceID *int64, externalUID, url string) (model.JobOffer, error) {
	uid := strings.TrimSpace(externalUID)
	url = strings.TrimSpace(url)

	var query string
	var args []any
	switch {
	case sourceID != nil && uid != "":
		query = `SELECT ` + offerColumns("o.") + ` FROM job_offers o
			JOIN job_offer_provenance p ON p.offer_id = o.id
			WHERE p.source_id = ? AND p.external_uid = ?
			ORDER BY p.last_seen_at DESC, p.id DESC LIMIT 1`
		args = []any{*sourceID, uid}
	case sourceID != nil && url != "":
		query = `SELECT ` + offerColumns("o.") + ` FROM job_offers o
			JOIN job_offer_provenance p ON p.offer_id = o.id
			WHERE p.source_id = ? AND p.url = ?
			ORDER BY p.last_seen_at DESC, p.id DESC LIMIT 1`
		args = []any{*sourceID, url}
	case sourceID == nil && url != "":
		query = `SELECT ` + offerColumns("o.") + ` FROM job_offers o
			JOIN job_offer_provenance p ON p.offer_id = o.id
			WHERE p.url = ?
			ORDER BY p.last_seen_at DESC, p.id DESC LIMIT 1`
		args = []any{url}
	default:
		return model.JobOffer{}, ErrNotFound
	}

	o, err := scanOffer(q.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return model.JobOffer{}, ErrNotFound
	}
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("find offer by provenance: %w", err)
	}
	return o, nil
}

// GetOffer returns the offer with the given ID.
func GetOffer(ctx context.Context, q Querier, id int64) (model.JobOffer, error) {
	o, err := scanOffer(q.QueryRowContext(ctx, `SELECT `+offerColumns("")+` FROM job_offers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.JobOffer{}, ErrNotFound
	}
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("get offer %d: %w", id, err)
	}
	return o, nil
}

// InsertOffer stores a new offer. FirstSeenAt/LastSeenAt/CreatedAt/
// UpdatedAt are all set to now; states default to active/none when empty.
func InsertOffer(ctx context.Context, q Querier, o model.JobOffer, now time.Time) (model.JobOffer, error) {
	if o.OfferState == "" {
		o.OfferState = model.OfferActive
	}
	if o.ApplicationState == "" {
		o.ApplicationState = model.AppNone
	}
	if o.RemotePolicy == "" {
		o.RemotePolicy = model.RemoteUnknown
	}
	ts := model.FormatTime(now)
	firstSeen, lastSeen := ts, ts
	if !o.FirstSeenAt.IsZero() {
		firstSeen = model.FormatTime(o.FirstSeenAt)
	}
	if !o.LastSeenAt.IsZero() {
		lastSeen = model.FormatTime(o.LastSeenAt)
	}
	res, err := q.ExecContext(ctx, `INSERT INTO job_offers(
		company_id, title, title_normalized, location, remote_policy, url, fingerprint,
		first_seen_at, last_seen_at, offer_state, application_state, raw_text, text_hash,
		created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullInt(o.CompanyID), o.Title, o.TitleNormalized, nullStr(o.Location), string(o.RemotePolicy), nullStr(o.URL), o.Fingerprint,
		firstSeen, lastSeen, string(o.OfferState), string(o.ApplicationState), nullStr(o.RawText), nullStr(o.TextHash),
		ts, ts)
	if err != nil {
		return model.JobOffer{}, fmt.Errorf("insert offer %q: %w", o.Title, err)
	}
	if o.ID, err = res.LastInsertId(); err != nil {
		return model.JobOffer{}, fmt.Errorf("insert offer %q: %w", o.Title, err)
	}
	t := now.UTC().Truncate(time.Second)
	o.CreatedAt, o.UpdatedAt = t, t
	if o.FirstSeenAt.IsZero() {
		o.FirstSeenAt = t
	}
	if o.LastSeenAt.IsZero() {
		o.LastSeenAt = t
	}
	return o, nil
}

// UpdateOfferSighting records that an existing offer was seen again:
// sets title, title_normalized, location, last_seen_at, offer_state,
// raw_text and text_hash.
func UpdateOfferSighting(ctx context.Context, q Querier, id int64, title, titleNormalized string,
	location *string, state model.OfferState, rawText, textHash *string, now time.Time) error {
	ts := model.FormatTime(now)
	if _, err := q.ExecContext(ctx, `UPDATE job_offers
		SET title = ?, title_normalized = ?, location = ?, last_seen_at = ?, offer_state = ?,
		    raw_text = ?, text_hash = ?, updated_at = ?
		WHERE id = ?`,
		title, titleNormalized, nullStr(location), ts, string(state),
		nullStr(rawText), nullStr(textHash), ts, id); err != nil {
		return fmt.Errorf("update offer %d sighting: %w", id, err)
	}
	return nil
}

// SetOfferRemotePolicy stores the derived remote policy of an offer. It
// deliberately does not bump updated_at: a re-derived fact is not
// job-search activity. Title and location affinity are not stored; they
// are computed from the current profile when the offer is read.
func SetOfferRemotePolicy(ctx context.Context, q Querier, id int64, policy model.RemotePolicy) error {
	if policy == "" {
		policy = model.RemoteUnknown
	}
	res, err := q.ExecContext(ctx, `UPDATE job_offers SET remote_policy = ? WHERE id = ?`, string(policy), id)
	if err != nil {
		return fmt.Errorf("set remote policy of offer %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchOfferLastSeen only bumps last_seen_at (manual re-import).
func TouchOfferLastSeen(ctx context.Context, q Querier, id int64, now time.Time) error {
	ts := model.FormatTime(now)
	if _, err := q.ExecContext(ctx, `UPDATE job_offers SET last_seen_at = ?, updated_at = ? WHERE id = ?`, ts, ts, id); err != nil {
		return fmt.Errorf("touch offer %d: %w", id, err)
	}
	return nil
}

// CountActiveSourceOffers returns how many active offers have provenance
// for the source. It is the known-offer count used by the scan guard that
// rejects empty or suspiciously small observations.
func CountActiveSourceOffers(ctx context.Context, q Querier, sourceID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT o.id)
		FROM job_offers o
		JOIN job_offer_provenance p ON p.offer_id = o.id
		WHERE p.source_id = ? AND o.offer_state = 'active'`, sourceID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count active offers for source %d: %w", sourceID, err)
	}
	return n, nil
}

// DisappearedOffer is an offer transitioned from active to disappeared by
// a scan, with the application state it held at that moment and the time
// it was last seen (the automatic ghost event time).
type DisappearedOffer struct {
	ID               int64
	ApplicationState model.ApplicationState
	LastSeenAt       time.Time
}

// MarkDisappeared sets offer_state = disappeared for every offer that has
// a provenance row for sourceID not seen since scanAt, unless the offer
// was seen by another source in that source's latest scan. Returns the
// offers transitioned together with their application state.
func MarkDisappeared(ctx context.Context, q Querier, sourceID int64, scanAt time.Time) ([]DisappearedOffer, error) {
	ts := model.FormatTime(scanAt)
	rows, err := q.QueryContext(ctx, `
		SELECT DISTINCT o.id, o.application_state, o.last_seen_at FROM job_offers o
		JOIN job_offer_provenance p ON p.offer_id = o.id
		WHERE p.source_id = ?
		  AND o.offer_state != 'disappeared'
		  AND NOT EXISTS (
		      SELECT 1 FROM job_offer_provenance p2
		      WHERE p2.offer_id = o.id AND p2.source_id = ? AND p2.last_seen_at >= ?)
		  AND NOT EXISTS (
		      SELECT 1 FROM job_offer_provenance p3
		      JOIN sources s ON s.id = p3.source_id
		      WHERE p3.offer_id = o.id AND p3.source_id != ?
		        AND s.last_scan_status = 'ok'
		        AND s.last_scan_at IS NOT NULL
		        AND p3.last_seen_at >= s.last_scan_at)
		ORDER BY o.id`, sourceID, sourceID, ts, sourceID)
	if err != nil {
		return nil, fmt.Errorf("find disappeared offers for source %d: %w", sourceID, err)
	}
	var gone []DisappearedOffer
	for rows.Next() {
		var o DisappearedOffer
		var state, seen string
		if err := rows.Scan(&o.ID, &state, &seen); err != nil {
			rows.Close()
			return nil, fmt.Errorf("find disappeared offers: %w", err)
		}
		o.ApplicationState = model.ApplicationState(state)
		seenAt, perr := model.ParseTime(seen)
		if perr != nil {
			rows.Close()
			return nil, fmt.Errorf("find disappeared offers: %w", perr)
		}
		o.LastSeenAt = seenAt
		gone = append(gone, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find disappeared offers: %w", err)
	}
	for _, o := range gone {
		if _, err := q.ExecContext(ctx,
			`UPDATE job_offers SET offer_state = 'disappeared', updated_at = ? WHERE id = ?`, ts, o.ID); err != nil {
			return nil, fmt.Errorf("mark offer %d disappeared: %w", o.ID, err)
		}
	}
	return gone, nil
}

// GhostDisappearedOffer records one ghosted event at the offer's last-seen
// time and closes the application for an offer that just transitioned to
// disappeared. now is the scan instant, used only for bookkeeping columns.
// It runs inside the caller's scan transaction. Only applied and interview
// offers are ghosted; callers decide that from
// DisappearedOffer.ApplicationState.
func GhostDisappearedOffer(ctx context.Context, q Querier, offerID int64, seenAt, now time.Time) error {
	desc := "Posting disappeared; marked ghosted automatically."
	if _, err := InsertEvent(ctx, q, model.Event{
		OfferID:     offerID,
		Type:        model.EventGhosted,
		Description: &desc,
		OccurredAt:  seenAt,
	}, now); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx,
		`UPDATE job_offers SET application_state = 'negative', saved_at = NULL, updated_at = ? WHERE id = ?`,
		model.FormatTime(now), offerID); err != nil {
		return fmt.Errorf("ghost offer %d: %w", offerID, err)
	}
	return nil
}

// OfferListItem is an offer joined with its company name for listings.
type OfferListItem struct {
	model.JobOffer
	CompanyName string
	LastEventAt *time.Time
}

// OfferFilter narrows ListOffers. Zero values mean "no filter".
type OfferFilter struct {
	OfferState       model.OfferState
	ApplicationState model.ApplicationState
	Limit            int
}

// ListOffers returns offers ordered by most recently seen first.
func ListOffers(ctx context.Context, q Querier, f OfferFilter) ([]OfferListItem, error) {
	var b strings.Builder
	b.WriteString(`SELECT c.name, ` + offerColumns("o.") + ` FROM job_offers o JOIN companies c ON c.id = o.company_id WHERE o.incomplete_reason IS NULL`)
	var args []any
	if f.OfferState != "" {
		b.WriteString(` AND o.offer_state = ?`)
		args = append(args, string(f.OfferState))
	}
	if f.ApplicationState != "" {
		b.WriteString(` AND o.application_state = ?`)
		args = append(args, string(f.ApplicationState))
	}
	b.WriteString(` ORDER BY o.last_seen_at DESC, o.id DESC`)
	if f.Limit > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, f.Limit)
	}
	rows, err := q.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list offers: %w", err)
	}
	defer rows.Close()
	var out []OfferListItem
	for rows.Next() {
		var item OfferListItem
		o, err := scanOffer(prefixScanner{rows, &item.CompanyName})
		if err != nil {
			return nil, fmt.Errorf("list offers: %w", err)
		}
		item.JobOffer = o
		out = append(out, item)
	}
	return out, rows.Err()
}

// prefixScanner scans one leading column into first before the rest.
type prefixScanner struct {
	rows  *sql.Rows
	first any
}

func (p prefixScanner) Scan(dest ...any) error {
	return p.rows.Scan(append([]any{p.first}, dest...)...)
}

// ---------- provenance ----------

// UpsertProvenance records a sighting of offerID at url. The row is
// found by (source_id, external_uid) when a uid is given, then by
// (offer_id, url). An existing row keeps its first_seen_at and gets the
// latest last_seen_at; the URL is not rewritten when the same uid appears
// at a new link, so the provenance keeps the first-seen link like the
// offer's canonical url. created reports insertion.
func UpsertProvenance(ctx context.Context, q Querier, offerID int64, sourceID *int64, externalUID *string, url string, now time.Time) (created bool, err error) {
	ts := model.FormatTime(now)
	uid := strings.TrimSpace(derefString(externalUID))

	if sourceID != nil && uid != "" {
		res, err := q.ExecContext(ctx, `UPDATE job_offer_provenance
			SET offer_id = ?, last_seen_at = ?
			WHERE source_id = ? AND external_uid = ?`,
			offerID, ts, *sourceID, uid)
		if err != nil {
			return false, fmt.Errorf("update provenance for offer %d: %w", offerID, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return false, nil
		}
	}

	res, err := q.ExecContext(ctx, `UPDATE job_offer_provenance
		SET last_seen_at = ?, source_id = COALESCE(?, source_id), external_uid = COALESCE(?, external_uid)
		WHERE offer_id = ? AND url = ?`,
		ts, nullInt(sourceID), nullStr(externalUID), offerID, url)
	if err != nil {
		return false, fmt.Errorf("update provenance for offer %d: %w", offerID, err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return false, nil
	}

	if _, err := q.ExecContext(ctx, `INSERT INTO job_offer_provenance(
		offer_id, source_id, external_uid, url, first_seen_at, last_seen_at) VALUES (?, ?, ?, ?, ?, ?)`,
		offerID, nullInt(sourceID), nullStr(externalUID), url, ts, ts); err != nil {
		return false, fmt.Errorf("insert provenance for offer %d: %w", offerID, err)
	}
	return true, nil
}

// ListProvenance returns all provenance rows of an offer.
func ListProvenance(ctx context.Context, q Querier, offerID int64) ([]model.Provenance, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, offer_id, source_id, external_uid, url, first_seen_at, last_seen_at
		FROM job_offer_provenance WHERE offer_id = ? ORDER BY id`, offerID)
	if err != nil {
		return nil, fmt.Errorf("list provenance for offer %d: %w", offerID, err)
	}
	defer rows.Close()
	var out []model.Provenance
	for rows.Next() {
		var p model.Provenance
		var src sql.NullInt64
		var uid, first, last sql.NullString
		if err := rows.Scan(&p.ID, &p.OfferID, &src, &uid, &p.URL, &first, &last); err != nil {
			return nil, fmt.Errorf("list provenance: %w", err)
		}
		p.SourceID = int64Ptr(src)
		p.ExternalUID = strPtr(uid)
		p.FirstSeenAt = parseNullTime(first)
		p.LastSeenAt = parseNullTime(last)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------- snapshots ----------

// InsertSnapshot stores a scan-origin snapshot. It is the default used by
// existing callers; new code should pass an explicit origin through
// InsertSnapshotOrigin.
func InsertSnapshot(ctx context.Context, q Querier, offerID int64, rawText string, now time.Time) error {
	return InsertSnapshotOrigin(ctx, q, offerID, rawText, model.SnapshotOriginScan, now)
}

// InsertSnapshotOrigin stores a description snapshot with its origin.
// An empty origin defaults to "scan".
func InsertSnapshotOrigin(ctx context.Context, q Querier, offerID int64, rawText string, origin model.SnapshotOrigin, now time.Time) error {
	if origin == "" {
		origin = model.SnapshotOriginScan
	}
	if !origin.Valid() {
		return fmt.Errorf("insert snapshot for offer %d: invalid origin %q", offerID, origin)
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO snapshots(offer_id, raw_text, origin, captured_at) VALUES (?, ?, ?, ?)`,
		offerID, rawText, string(origin), model.FormatTime(now)); err != nil {
		return fmt.Errorf("insert snapshot for offer %d: %w", offerID, err)
	}
	return nil
}

// CountSnapshots returns the number of snapshots of an offer.
func CountSnapshots(ctx context.Context, q Querier, offerID int64) (int, error) {
	var n int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM snapshots WHERE offer_id = ?`, offerID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count snapshots for offer %d: %w", offerID, err)
	}
	return n, nil
}

// SnapshotInfo is a lightweight snapshot row for version history. RawText
// is only populated for the requested snapshot (nil otherwise).
type SnapshotInfo struct {
	ID         int64
	Origin     model.SnapshotOrigin
	CapturedAt time.Time
	TextLength int
	RawText    *string
}

// ListSnapshotInfos returns an offer's snapshots oldest first. When
// rawTextID is positive, that snapshot's raw text is included; the other
// rows only carry metadata.
func ListSnapshotInfos(ctx context.Context, q Querier, offerID, rawTextID int64) ([]SnapshotInfo, error) {
	query := `SELECT id, origin, captured_at, length(raw_text), NULL
	          FROM snapshots WHERE offer_id = ? ORDER BY captured_at, id`
	args := []any{offerID}
	if rawTextID > 0 {
		query = `SELECT id, origin, captured_at, length(raw_text),
		                CASE WHEN id = ? THEN raw_text END
		         FROM snapshots WHERE offer_id = ? ORDER BY captured_at, id`
		args = []any{rawTextID, offerID}
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list snapshots of offer %d: %w", offerID, err)
	}
	defer rows.Close()
	out := []SnapshotInfo{}
	for rows.Next() {
		var s SnapshotInfo
		var origin, captured string
		var length int64
		var raw sql.NullString
		if err := rows.Scan(&s.ID, &origin, &captured, &length, &raw); err != nil {
			return nil, fmt.Errorf("list snapshots of offer %d: %w", offerID, err)
		}
		s.Origin = model.SnapshotOrigin(origin)
		s.CapturedAt = parseTime(captured)
		s.TextLength = int(length)
		s.RawText = strPtr(raw)
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSnapshotForOffer returns one snapshot, enforcing that it belongs to
// the offer.
func GetSnapshotForOffer(ctx context.Context, q Querier, offerID, snapshotID int64) (model.Snapshot, error) {
	var s model.Snapshot
	var origin, captured string
	err := q.QueryRowContext(ctx,
		`SELECT id, offer_id, origin, raw_text, captured_at
		 FROM snapshots WHERE id = ? AND offer_id = ?`, snapshotID, offerID).
		Scan(&s.ID, &s.OfferID, &origin, &s.RawText, &captured)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Snapshot{}, ErrNotFound
	}
	if err != nil {
		return model.Snapshot{}, fmt.Errorf("get snapshot %d of offer %d: %w", snapshotID, offerID, err)
	}
	s.Origin = model.SnapshotOrigin(origin)
	s.CapturedAt = parseTime(captured)
	return s, nil
}
