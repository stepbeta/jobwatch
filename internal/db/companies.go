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

// ErrCompanyHasOffers is returned when deleting a company that still owns
// offers. Offers are never deleted, so the company must be merged instead.
var ErrCompanyHasOffers = errors.New("company still has offers")

// ErrMergeSelf is returned when a company merge names the same company on
// both sides.
var ErrMergeSelf = errors.New("cannot merge a company into itself")

// ErrCompanyUnknown is returned when a sighting carries neither an
// attested company key nor a usable company name. The item cannot be
// attributed; callers store it as incomplete or skip it.
var ErrCompanyUnknown = errors.New("company cannot be determined")

// CompanyInput is the evidence a sighting carries about its company.
// Key is the adapter's namespaced token (greenhouse:acme, lever:acme, …);
// Name is the display name the source reported; SourceID scopes a
// name-only claim to the source that made it (nil for manual imports).
type CompanyInput struct {
	Key      string
	Name     string
	SourceID *int64
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

// GetCompanyByKey returns the company that owns an attested identifier.
func GetCompanyByKey(ctx context.Context, q Querier, key string) (model.Company, error) {
	c, err := scanCompanyRow(q.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.name_normalized, c.website, c.created_at
		 FROM companies c JOIN company_keys k ON k.company_id = c.id
		 WHERE k.key = ? LIMIT 1`, key))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Company{}, ErrNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("get company by key %q: %w", key, err)
	}
	return c, nil
}

// GetCompanyByAlias returns the company a source claimed a name for.
func GetCompanyByAlias(ctx context.Context, q Querier, nameNormalized string, sourceID int64) (model.Company, error) {
	c, err := scanCompanyRow(q.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.name_normalized, c.website, c.created_at
		 FROM companies c JOIN company_aliases a ON a.company_id = c.id
		 WHERE a.name_normalized = ? AND a.source_id = ? LIMIT 1`, nameNormalized, sourceID))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Company{}, ErrNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("get company by alias %q: %w", nameNormalized, err)
	}
	return c, nil
}

// GetCompanyByGlobalAlias returns the company a name was confirmed for
// across all sources.
func GetCompanyByGlobalAlias(ctx context.Context, q Querier, nameNormalized string) (model.Company, error) {
	c, err := scanCompanyRow(q.QueryRowContext(ctx,
		`SELECT c.id, c.name, c.name_normalized, c.website, c.created_at
		 FROM companies c JOIN company_aliases a ON a.company_id = c.id
		 WHERE a.name_normalized = ? AND a.source_id IS NULL LIMIT 1`, nameNormalized))
	if errors.Is(err, sql.ErrNoRows) {
		return model.Company{}, ErrNotFound
	}
	if err != nil {
		return model.Company{}, fmt.Errorf("get company by global alias %q: %w", nameNormalized, err)
	}
	return c, nil
}

func insertCompany(ctx context.Context, q Querier, name, norm string, website *string, now time.Time) (model.Company, error) {
	res, err := q.ExecContext(ctx,
		`INSERT INTO companies(name, name_normalized, website, created_at) VALUES (?, ?, ?, ?)`,
		name, norm, nullStr(website), model.FormatTime(now))
	if err != nil {
		return model.Company{}, fmt.Errorf("insert company %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Company{}, fmt.Errorf("insert company %q: %w", name, err)
	}
	return model.Company{
		ID:             id,
		Name:           name,
		NameNormalized: norm,
		Website:        website,
		CreatedAt:      now.UTC().Truncate(time.Second),
	}, nil
}

// linkCompanyKey records an attested identifier. A key already linked to
// another company is left alone; the caller resolves by key first, so a
// conflict is a data issue for the user to merge.
func linkCompanyKey(ctx context.Context, q Querier, companyID int64, key string, now time.Time) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	if _, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO company_keys(company_id, key, created_at) VALUES (?, ?, ?)`,
		companyID, key, model.FormatTime(now)); err != nil {
		return fmt.Errorf("link key %q to company %d: %w", key, companyID, err)
	}
	return nil
}

// linkCompanyAlias records a name claim. A nil sourceID is a confirmed
// global alias; a non-nil one is scoped to that source.
func linkCompanyAlias(ctx context.Context, q Querier, companyID int64, nameNormalized string, sourceID *int64, now time.Time) error {
	nameNormalized = strings.TrimSpace(nameNormalized)
	if nameNormalized == "" {
		return nil
	}
	if _, err := q.ExecContext(ctx,
		`INSERT OR IGNORE INTO company_aliases(company_id, name_normalized, source_id, created_at)
		 VALUES (?, ?, ?, ?)`,
		companyID, nameNormalized, nullInt(sourceID), model.FormatTime(now)); err != nil {
		return fmt.Errorf("link alias %q to company %d: %w", nameNormalized, companyID, err)
	}
	return nil
}

// ResolveCompany returns the company a sighting belongs to. Resolution is
// key, then source-scoped alias, then confirmed global alias, then create.
// It never matches on a bare name across sources: a name-only claim is
// scoped to the source that made it. created reports a new row.
func ResolveCompany(ctx context.Context, q Querier, in CompanyInput, now time.Time) (model.Company, bool, error) {
	key := strings.TrimSpace(in.Key)
	name := strings.TrimSpace(in.Name)
	norm := model.NormalizeCompanyName(name)

	if key != "" {
		c, err := GetCompanyByKey(ctx, q, key)
		switch {
		case err == nil:
			if err := linkCompanyAlias(ctx, q, c.ID, norm, in.SourceID, now); err != nil {
				return model.Company{}, false, err
			}
			return c, false, nil
		case !errors.Is(err, ErrNotFound):
			return model.Company{}, false, err
		}
	}

	if norm != "" && in.SourceID != nil {
		c, err := GetCompanyByAlias(ctx, q, norm, *in.SourceID)
		switch {
		case err == nil:
			if err := linkCompanyKey(ctx, q, c.ID, key, now); err != nil {
				return model.Company{}, false, err
			}
			return c, false, nil
		case !errors.Is(err, ErrNotFound):
			return model.Company{}, false, err
		}
	}

	if norm != "" {
		c, err := GetCompanyByGlobalAlias(ctx, q, norm)
		switch {
		case err == nil:
			if err := linkCompanyAlias(ctx, q, c.ID, norm, in.SourceID, now); err != nil {
				return model.Company{}, false, err
			}
			if err := linkCompanyKey(ctx, q, c.ID, key, now); err != nil {
				return model.Company{}, false, err
			}
			return c, false, nil
		case !errors.Is(err, ErrNotFound):
			return model.Company{}, false, err
		}
	}

	if norm == "" {
		return model.Company{}, false, ErrCompanyUnknown
	}

	c, err := insertCompany(ctx, q, name, norm, nil, now)
	if err != nil {
		return model.Company{}, false, err
	}
	if err := linkCompanyAlias(ctx, q, c.ID, norm, in.SourceID, now); err != nil {
		return model.Company{}, false, err
	}
	if err := linkCompanyKey(ctx, q, c.ID, key, now); err != nil {
		return model.Company{}, false, err
	}
	return c, true, nil
}

// CreateCompany inserts a company and a confirmed global alias for its
// name. Names are not unique: an existing alias with the same normalized
// name is left on its company and the new row simply does not claim it.
func CreateCompany(ctx context.Context, sqlDB *sql.DB, name string, website *string, now time.Time) (model.Company, error) {
	name = strings.TrimSpace(name)
	norm := model.NormalizeCompanyName(name)
	if norm == "" {
		return model.Company{}, fmt.Errorf("create company: empty name")
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return model.Company{}, fmt.Errorf("begin company create: %w", err)
	}
	defer tx.Rollback()
	c, err := insertCompany(ctx, tx, name, norm, website, now)
	if err != nil {
		return model.Company{}, err
	}
	if err := linkCompanyAlias(ctx, tx, c.ID, norm, nil, now); err != nil {
		return model.Company{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Company{}, fmt.Errorf("commit company create: %w", err)
	}
	return c, nil
}

// CompanyUpdate is a partial update of a company. nil fields are left
// unchanged; an empty Website clears the column.
type CompanyUpdate struct {
	Name    *string
	Website *string
}

// UpdateCompany applies u in one transaction. Renaming changes the
// display name and records both the old and the new name as aliases;
// offers keep their identity, company and fingerprint.
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
	if err := linkCompanyAlias(ctx, tx, id, c.NameNormalized, nil, now); err != nil {
		return model.Company{}, err
	}
	if err := linkCompanyAlias(ctx, tx, id, norm, nil, now); err != nil {
		return model.Company{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Company{}, fmt.Errorf("commit company update: %w", err)
	}
	return model.Company{ID: id, Name: name, NameNormalized: norm, Website: website, CreatedAt: c.CreatedAt}, nil
}

// DeleteCompany removes a company, its company-level notes and (through
// foreign keys) its keys and aliases. It refuses when the company still
// has offers.
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

// MergeCompanies moves every offer, company-level note, key and alias of
// fromID to intoID and deletes the emptied company. It is only called
// after an explicit user confirmation. The absorbed display name becomes
// a confirmed alias of the target, so later scans resolve it there and no
// ghost company is addressable. Offers keep their fingerprints; identity
// is provenance, not the company display name.
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

	if _, err := tx.ExecContext(ctx,
		`UPDATE OR IGNORE company_keys SET company_id = ? WHERE company_id = ?`, intoID, fromID); err != nil {
		return MergeResult{}, fmt.Errorf("move keys of company %d: %w", fromID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM company_keys WHERE company_id = ?`, fromID); err != nil {
		return MergeResult{}, fmt.Errorf("delete keys of company %d: %w", fromID, err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE OR IGNORE company_aliases SET company_id = ? WHERE company_id = ?`, intoID, fromID); err != nil {
		return MergeResult{}, fmt.Errorf("move aliases of company %d: %w", fromID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM company_aliases WHERE company_id = ?`, fromID); err != nil {
		return MergeResult{}, fmt.Errorf("delete aliases of company %d: %w", fromID, err)
	}
	if err := linkCompanyAlias(ctx, tx, intoID, from.NameNormalized, nil, now); err != nil {
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
