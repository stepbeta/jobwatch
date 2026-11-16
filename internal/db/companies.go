package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// ErrCompanyNameTaken is returned when a company rename would collide
// with the normalized name of another company.
var ErrCompanyNameTaken = errors.New("company name already exists")

// ErrCompanyHasOffers is returned when deleting a company that still owns
// offers. Offers are never deleted, so the company must be merged instead.
var ErrCompanyHasOffers = errors.New("company still has offers")

// ErrMergeSelf is returned when a company merge names the same company on
// both sides.
var ErrMergeSelf = errors.New("cannot merge a company into itself")

// ConflictError reports offers whose fingerprints would collide with an
// existing offer during a company rename or merge. The operation is
// rejected; the user must resolve the offers explicitly.
type ConflictError struct {
	Operation string
	Offers    []int64
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s: %d offer fingerprint(s) already exist in the target company (offer IDs %v)",
		e.Operation, len(e.Offers), e.Offers)
}

func scanCompanyRow(row interface{ Scan(...any) error }) (model.Company, error) {
	var c model.Company
	var website, created sql.NullString
	if err := row.Scan(&c.ID, &c.Name, &c.NameNormalized, &website, &created); err != nil {
		return model.Company{}, err
	}
	c.Website = strPtr(website)
	c.CreatedAt = parseNullTime(created)
	return c, nil
}

// GetCompany returns the company with the given ID.
func GetCompany(ctx context.Context, q Querier, id int64) (model.Company, error) {
	c, err := scanCompanyRow(q.QueryRowContext(ctx,
		`SELECT id, name, name_normalized, website, created_at FROM companies WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Company{}, ErrNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("get company %d: %w", id, err)
	}
	return c, nil
}

// CreateCompany inserts a company. It refuses a name whose normalized
// form already exists instead of silently returning the existing row.
func CreateCompany(ctx context.Context, q Querier, name string, website *string, now time.Time) (model.Company, error) {
	name = strings.TrimSpace(name)
	norm := model.NormalizeCompanyName(name)
	if norm == "" {
		return model.Company{}, fmt.Errorf("create company: empty name")
	}
	if _, err := GetCompanyByNormalized(ctx, q, norm); err == nil {
		return model.Company{}, ErrCompanyNameTaken
	} else if !errors.Is(err, ErrNotFound) {
		return model.Company{}, err
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO companies(name, name_normalized, website, created_at) VALUES (?, ?, ?, ?)`,
		name, norm, nullStr(website), model.FormatTime(now))
	if err != nil {
		return model.Company{}, fmt.Errorf("create company %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Company{}, fmt.Errorf("create company %q: %w", name, err)
	}
	return model.Company{
		ID:             id,
		Name:           name,
		NameNormalized: norm,
		Website:        website,
		CreatedAt:      now.UTC().Truncate(time.Second),
	}, nil
}

// CompanyUpdate is a partial update of a company. nil fields are left
// unchanged; an empty Website clears the column.
type CompanyUpdate struct {
	Name    *string
	Website *string
}

// UpdateCompany applies u in one transaction. Renaming a company rewrites
// the fingerprints of its offers (they embed the normalized company
// name); the update is rejected when a fingerprint would collide.
func UpdateCompany(ctx context.Context, sqlDB *sql.DB, id int64, u CompanyUpdate, now time.Time) (model.Company, error) {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return model.Company{}, fmt.Errorf("begin company update: %w", err)
	}
	defer tx.Rollback()

	c, err := GetCompany(ctx, tx, id)
	if err != nil {
		return model.Company{}, err
	}
	name, norm := c.Name, c.NameNormalized
	if u.Name != nil {
		name = strings.TrimSpace(*u.Name)
		norm = model.NormalizeCompanyName(name)
		if norm == "" {
			return model.Company{}, fmt.Errorf("update company %d: empty name", id)
		}
	}
	if norm != c.NameNormalized {
		if other, err := GetCompanyByNormalized(ctx, tx, norm); err == nil && other.ID != id {
			return model.Company{}, ErrCompanyNameTaken
		} else if err != nil && !errors.Is(err, ErrNotFound) {
			return model.Company{}, err
		}
		updates, conflicts, err := planFingerprintUpdates(ctx, tx, id, c.NameNormalized, norm)
		if err != nil {
			return model.Company{}, err
		}
		if len(conflicts) > 0 {
			return model.Company{}, &ConflictError{Operation: "rename company", Offers: conflicts}
		}
		if err := applyFingerprintUpdates(ctx, tx, updates, now); err != nil {
			return model.Company{}, err
		}
	}

	website := c.Website
	if u.Website != nil {
		website = nil
		if *u.Website != "" {
			website = u.Website
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE companies SET name = ?, name_normalized = ?, website = ? WHERE id = ?`,
		name, norm, nullStr(website), id); err != nil {
		return model.Company{}, fmt.Errorf("update company %d: %w", id, err)
	}
	if err := tx.Commit(); err != nil {
		return model.Company{}, fmt.Errorf("commit company update: %w", err)
	}
	return model.Company{ID: id, Name: name, NameNormalized: norm, Website: website, CreatedAt: c.CreatedAt}, nil
}

// DeleteCompany removes a company and its company-level notes. It
// refuses when the company still has offers.
func DeleteCompany(ctx context.Context, sqlDB *sql.DB, id int64) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin company delete: %w", err)
	}
	defer tx.Rollback()
	if _, err := GetCompany(ctx, tx, id); err != nil {
		return err
	}
	var offers int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM job_offers WHERE company_id = ?`, id).Scan(&offers); err != nil {
		return fmt.Errorf("count offers of company %d: %w", id, err)
	}
	if offers > 0 {
		return ErrCompanyHasOffers
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM notes WHERE entity_type = 'company' AND entity_id = ?`, id); err != nil {
		return fmt.Errorf("delete notes of company %d: %w", id, err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM companies WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete company %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit company delete: %w", err)
	}
	return nil
}

// CompanyListItem is a company with its offer and note counts.
type CompanyListItem struct {
	model.Company
	OfferCount int
	NoteCount  int
}

// CompanyFilter narrows ListCompanies. Zero values mean "no filter".
type CompanyFilter struct {
	Q      string
	Limit  int
	Offset int
}

// ListCompanies returns companies with counts, ordered by normalized
// name, plus the total number of matching rows.
func ListCompanies(ctx context.Context, q Querier, f CompanyFilter) ([]CompanyListItem, int, error) {
	where := " WHERE 1=1"
	var args []any
	if s := strings.TrimSpace(f.Q); s != "" {
		where += " AND (c.name_normalized LIKE ? OR lower(c.name) LIKE ?)"
		like := "%" + model.NormalizeCompanyName(s) + "%"
		args = append(args, like, "%"+strings.ToLower(s)+"%")
	}
	var total int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM companies c`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count companies: %w", err)
	}
	query := `SELECT c.id, c.name, c.name_normalized, c.website, c.created_at,
		(SELECT COUNT(*) FROM job_offers o WHERE o.company_id = c.id),
		(SELECT COUNT(*) FROM notes n WHERE n.entity_type = 'company' AND n.entity_id = c.id)
		FROM companies c` + where + ` ORDER BY c.name_normalized, c.id`
	qargs := append([]any{}, args...)
	if f.Limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		qargs = append(qargs, f.Limit, f.Offset)
	}
	rows, err := q.QueryContext(ctx, query, qargs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list companies: %w", err)
	}
	defer rows.Close()
	out := []CompanyListItem{}
	for rows.Next() {
		var item CompanyListItem
		var website, created sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.NameNormalized, &website, &created,
			&item.OfferCount, &item.NoteCount); err != nil {
			return nil, 0, fmt.Errorf("list companies: %w", err)
		}
		item.Website = strPtr(website)
		item.CreatedAt = parseNullTime(created)
		out = append(out, item)
	}
	return out, total, rows.Err()
}

// SimilarCompanies proposes companies whose normalized name is close to
// c's (see model.SimilarCompanyNames). Candidates are never merged
// automatically.
func SimilarCompanies(ctx context.Context, q Querier, c model.Company) ([]model.CompanyCandidate, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, name, name_normalized FROM companies WHERE id != ? ORDER BY name_normalized`, c.ID)
	if err != nil {
		return nil, fmt.Errorf("list companies for similarity: %w", err)
	}
	defer rows.Close()
	var out []model.CompanyCandidate
	for rows.Next() {
		var cand model.CompanyCandidate
		if err := rows.Scan(&cand.ID, &cand.Name, &cand.NameNormalized); err != nil {
			return nil, fmt.Errorf("scan company candidate: %w", err)
		}
		score, reason := model.SimilarCompanyNames(c.NameNormalized, cand.NameNormalized)
		if score < model.MinCompanyCandidateScore {
			continue
		}
		cand.Score, cand.Reason = score, reason
		out = append(out, cand)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].NameNormalized < out[j].NameNormalized
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out, nil
}

// CompanyCounts returns the number of offers and company-level notes of
// a company.
func CompanyCounts(ctx context.Context, q Querier, id int64) (offers, notes int, err error) {
	if err := q.QueryRowContext(ctx,
		`SELECT (SELECT COUNT(*) FROM job_offers WHERE company_id = ?),
		        (SELECT COUNT(*) FROM notes WHERE entity_type = 'company' AND entity_id = ?)`,
		id, id).Scan(&offers, &notes); err != nil {
		return 0, 0, fmt.Errorf("count company %d relations: %w", id, err)
	}
	return offers, notes, nil
}

// MergeResult reports what a company merge moved.
type MergeResult struct {
	Company     model.Company
	MovedOffers int
	MovedNotes  int
}

// MergeCompanies moves every offer and company-level note of fromID to
// intoID, recomputes the offers' fingerprints with the target company
// name, and deletes the emptied company. It is only called after an
// explicit user confirmation. If any fingerprint would collide, nothing
// is written and a *ConflictError lists the offers.
func MergeCompanies(ctx context.Context, sqlDB *sql.DB, fromID, intoID int64, now time.Time) (MergeResult, error) {
	if fromID == intoID {
		return MergeResult{}, ErrMergeSelf
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return MergeResult{}, fmt.Errorf("begin company merge: %w", err)
	}
	defer tx.Rollback()

	from, err := GetCompany(ctx, tx, fromID)
	if err != nil {
		return MergeResult{}, err
	}
	into, err := GetCompany(ctx, tx, intoID)
	if err != nil {
		return MergeResult{}, err
	}
	updates, conflicts, err := planFingerprintUpdates(ctx, tx, fromID, from.NameNormalized, into.NameNormalized)
	if err != nil {
		return MergeResult{}, err
	}
	if len(conflicts) > 0 {
		return MergeResult{}, &ConflictError{Operation: "merge companies", Offers: conflicts}
	}
	if err := applyFingerprintUpdates(ctx, tx, updates, now); err != nil {
		return MergeResult{}, err
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE job_offers SET company_id = ? WHERE company_id = ?`, intoID, fromID)
	if err != nil {
		return MergeResult{}, fmt.Errorf("move offers of company %d: %w", fromID, err)
	}
	movedOffers, _ := res.RowsAffected()
	res, err = tx.ExecContext(ctx,
		`UPDATE notes SET entity_id = ?, updated_at = ? WHERE entity_type = 'company' AND entity_id = ?`,
		intoID, model.FormatTime(now), fromID)
	if err != nil {
		return MergeResult{}, fmt.Errorf("move notes of company %d: %w", fromID, err)
	}
	movedNotes, _ := res.RowsAffected()
	if from.Website != nil {
		if _, err := tx.ExecContext(ctx,
			`UPDATE companies SET website = COALESCE(website, ?) WHERE id = ?`, *from.Website, intoID); err != nil {
			return MergeResult{}, fmt.Errorf("copy website of company %d: %w", fromID, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM companies WHERE id = ?`, fromID); err != nil {
		return MergeResult{}, fmt.Errorf("delete merged company %d: %w", fromID, err)
	}
	if err := tx.Commit(); err != nil {
		return MergeResult{}, fmt.Errorf("commit company merge: %w", err)
	}
	into, err = GetCompany(ctx, sqlDB, intoID)
	if err != nil {
		return MergeResult{}, err
	}
	return MergeResult{Company: into, MovedOffers: int(movedOffers), MovedNotes: int(movedNotes)}, nil
}

// planFingerprintUpdates computes the new fingerprints of the offers of
// companyID when its normalized name changes from oldNorm to newNorm.
// Offers whose current fingerprint is the base fingerprint keep the base
// form; bucketed offers keep their location bucket. It returns the
// offer IDs whose new fingerprint already exists in another company (or
// twice in the plan) as conflicts, without writing anything.
func planFingerprintUpdates(ctx context.Context, q Querier, companyID int64, oldNorm, newNorm string) (map[int64]string, []int64, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, title_normalized, location, fingerprint FROM job_offers WHERE company_id = ? ORDER BY id`, companyID)
	if err != nil {
		return nil, nil, fmt.Errorf("plan fingerprints for company %d: %w", companyID, err)
	}
	defer rows.Close()
	type offerFingerprint struct {
		id       int64
		location sql.NullString
		old      string
		title    string
	}
	var offers []offerFingerprint
	for rows.Next() {
		var o offerFingerprint
		if err := rows.Scan(&o.id, &o.title, &o.location, &o.old); err != nil {
			return nil, nil, fmt.Errorf("plan fingerprints: %w", err)
		}
		offers = append(offers, o)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Fingerprints already used outside this company.
	existing := map[string]int64{}
	orows, err := q.QueryContext(ctx, `SELECT id, fingerprint FROM job_offers WHERE company_id != ?`, companyID)
	if err != nil {
		return nil, nil, fmt.Errorf("plan fingerprints: %w", err)
	}
	for orows.Next() {
		var id int64
		var fp string
		if err := orows.Scan(&id, &fp); err != nil {
			orows.Close()
			return nil, nil, fmt.Errorf("plan fingerprints: %w", err)
		}
		existing[fp] = id
	}
	orows.Close()
	if err := orows.Err(); err != nil {
		return nil, nil, err
	}

	updates := make(map[int64]string, len(offers))
	planned := map[string]int64{}
	var conflicts []int64
	for _, o := range offers {
		var fp string
		if o.old == model.Fingerprint(oldNorm, o.title, "") {
			fp = model.Fingerprint(newNorm, o.title, "")
		} else {
			bucket := ""
			if o.location.Valid {
				bucket = model.LocationBucket(o.location.String)
			}
			if bucket == "" {
				conflicts = append(conflicts, o.id)
				continue
			}
			fp = model.Fingerprint(newNorm, o.title, bucket)
		}
		if _, ok := existing[fp]; ok {
			conflicts = append(conflicts, o.id)
			continue
		}
		if prev, ok := planned[fp]; ok {
			conflicts = append(conflicts, o.id, prev)
			continue
		}
		planned[fp] = o.id
		updates[o.id] = fp
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i] < conflicts[j] })
	uniq := conflicts[:0]
	for i, id := range conflicts {
		if i == 0 || id != conflicts[i-1] {
			uniq = append(uniq, id)
		}
	}
	return updates, uniq, nil
}

func applyFingerprintUpdates(ctx context.Context, q Querier, updates map[int64]string, now time.Time) error {
	ts := model.FormatTime(now)
	for id, fp := range updates {
		if _, err := q.ExecContext(ctx,
			`UPDATE job_offers SET fingerprint = ?, updated_at = ? WHERE id = ?`, fp, ts, id); err != nil {
			return fmt.Errorf("update fingerprint of offer %d: %w", id, err)
		}
	}
	return nil
}
