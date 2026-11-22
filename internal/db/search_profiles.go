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

// ErrInvalidProfile wraps every search-profile validation failure so the
// API can answer 400 instead of 500.
var ErrInvalidProfile = errors.New("invalid search profile")

const searchProfileColumns = `id, name, title_label, location_label, bucket_scale, is_default, created_at, updated_at`

func scanSearchProfile(row interface{ Scan(...any) error }) (model.SearchProfile, error) {
	var p model.SearchProfile
	var scale string
	var isDefault int
	var created, updated string
	if err := row.Scan(&p.ID, &p.Name, &p.TitleLabel, &p.LocationLabel, &scale, &isDefault,
		&created, &updated); err != nil {
		return model.SearchProfile{}, err
	}
	p.BucketScale = model.BucketScale(scale)
	p.IsDefault = isDefault == 1
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return p, nil
}

// ListSearchProfiles returns every profile with its terms and remote
// selection, default profile first.
func ListSearchProfiles(ctx context.Context, q Querier) ([]model.SearchProfile, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+searchProfileColumns+` FROM search_profiles
		 ORDER BY is_default DESC, name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("list search profiles: %w", err)
	}
	defer rows.Close()
	out := []model.SearchProfile{}
	for rows.Next() {
		p, err := scanSearchProfile(rows)
		if err != nil {
			return nil, fmt.Errorf("list search profiles: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list search profiles: %w", err)
	}
	for i := range out {
		if err := loadProfileDetails(ctx, q, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// GetSearchProfile returns one profile with its terms and remote
// selection.
func GetSearchProfile(ctx context.Context, q Querier, id int64) (model.SearchProfile, error) {
	p, err := scanSearchProfile(q.QueryRowContext(ctx,
		`SELECT `+searchProfileColumns+` FROM search_profiles WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return model.SearchProfile{}, ErrNotFound
	}
	if err != nil {
		return model.SearchProfile{}, fmt.Errorf("get search profile %d: %w", id, err)
	}
	if err := loadProfileDetails(ctx, q, &p); err != nil {
		return model.SearchProfile{}, err
	}
	return p, nil
}

// DefaultSearchProfile returns the profile marked is_default.
func DefaultSearchProfile(ctx context.Context, q Querier) (model.SearchProfile, error) {
	p, err := scanSearchProfile(q.QueryRowContext(ctx,
		`SELECT `+searchProfileColumns+` FROM search_profiles WHERE is_default = 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return model.SearchProfile{}, ErrNotFound
	}
	if err != nil {
		return model.SearchProfile{}, fmt.Errorf("get default search profile: %w", err)
	}
	if err := loadProfileDetails(ctx, q, &p); err != nil {
		return model.SearchProfile{}, err
	}
	return p, nil
}

func loadProfileDetails(ctx context.Context, q Querier, p *model.SearchProfile) error {
	rows, err := q.QueryContext(ctx,
		`SELECT id, profile_id, affinity, bucket, term, weight, sort_order
		 FROM search_profile_terms WHERE profile_id = ?
		 ORDER BY affinity, sort_order, id`, p.ID)
	if err != nil {
		return fmt.Errorf("list terms of search profile %d: %w", p.ID, err)
	}
	terms := []model.SearchProfileTerm{}
	for rows.Next() {
		var t model.SearchProfileTerm
		var affinity, bucket string
		if err := rows.Scan(&t.ID, &t.ProfileID, &affinity, &bucket, &t.Term, &t.Weight, &t.SortOrder); err != nil {
			rows.Close()
			return fmt.Errorf("list terms of search profile %d: %w", p.ID, err)
		}
		t.Affinity = model.TermAffinity(affinity)
		t.Bucket = model.TermBucket(bucket)
		terms = append(terms, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("list terms of search profile %d: %w", p.ID, err)
	}

	remoteRows, err := q.QueryContext(ctx,
		`SELECT policy, accepted FROM search_profile_remote WHERE profile_id = ? ORDER BY policy`, p.ID)
	if err != nil {
		return fmt.Errorf("list remote selection of search profile %d: %w", p.ID, err)
	}
	defer remoteRows.Close()
	remote := []model.SearchProfileRemote{}
	for remoteRows.Next() {
		var r model.SearchProfileRemote
		var policy string
		var accepted int
		if err := remoteRows.Scan(&policy, &accepted); err != nil {
			return fmt.Errorf("list remote selection of search profile %d: %w", p.ID, err)
		}
		r.Policy = model.RemotePolicy(policy)
		r.Accepted = accepted == 1
		remote = append(remote, r)
	}
	if err := remoteRows.Err(); err != nil {
		return fmt.Errorf("list remote selection of search profile %d: %w", p.ID, err)
	}
	p.Terms = terms
	p.Remote = remote
	return nil
}

// UpdateSearchProfile replaces the editable fields of a profile — name,
// labels, bucket scale, terms and remote selection — in one transaction.
// IsDefault, ID and timestamps are not editable; a validation failure or
// any database error leaves the stored profile untouched.
func UpdateSearchProfile(ctx context.Context, sqlDB *sql.DB, p model.SearchProfile, now time.Time) (model.SearchProfile, error) {
	if err := p.Validate(); err != nil {
		return model.SearchProfile{}, fmt.Errorf("%w: %v", ErrInvalidProfile, err)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return model.SearchProfile{}, fmt.Errorf("begin search profile %d update: %w", p.ID, err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`UPDATE search_profiles SET name = ?, title_label = ?, location_label = ?, bucket_scale = ?, updated_at = ?
		 WHERE id = ?`,
		strings.TrimSpace(p.Name), strings.TrimSpace(p.TitleLabel), strings.TrimSpace(p.LocationLabel),
		string(p.BucketScale), model.FormatTime(now), p.ID)
	if err != nil {
		return model.SearchProfile{}, fmt.Errorf("update search profile %d: %w", p.ID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.SearchProfile{}, ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM search_profile_terms WHERE profile_id = ?`, p.ID); err != nil {
		return model.SearchProfile{}, fmt.Errorf("replace terms of search profile %d: %w", p.ID, err)
	}
	for i, t := range p.Terms {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_profile_terms(profile_id, affinity, bucket, term, weight, sort_order)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			p.ID, string(t.Affinity), string(t.Bucket), strings.TrimSpace(t.Term), t.Weight, i); err != nil {
			return model.SearchProfile{}, fmt.Errorf("insert term %q of search profile %d: %w", t.Term, p.ID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM search_profile_remote WHERE profile_id = ?`, p.ID); err != nil {
		return model.SearchProfile{}, fmt.Errorf("replace remote selection of search profile %d: %w", p.ID, err)
	}
	for _, r := range p.Remote {
		accepted := 0
		if r.Accepted {
			accepted = 1
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_profile_remote(profile_id, policy, accepted) VALUES (?, ?, ?)`,
			p.ID, string(r.Policy), accepted); err != nil {
			return model.SearchProfile{}, fmt.Errorf("insert remote policy %q of search profile %d: %w", r.Policy, p.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return model.SearchProfile{}, fmt.Errorf("commit search profile %d update: %w", p.ID, err)
	}
	return GetSearchProfile(ctx, sqlDB, p.ID)
}
