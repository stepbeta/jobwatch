package scan

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ImportInput is a manually entered offer (source_type manual: no fetch,
// provenance rows have a NULL source_id).
type ImportInput struct {
	Company     string `json:"company"`
	Title       string `json:"title"`
	Location    string `json:"location,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

// ImportResult reports the offer an import landed on.
type ImportResult struct {
	OfferID int64
	Created bool // false when an existing offer matched by fingerprint
}

// ParseImport decodes arg as a JSON object, a JSON array of objects, or
// a bare http(s) URL (which then needs company/title from elsewhere).
func ParseImport(arg string) ([]ImportInput, error) {
	s := strings.TrimSpace(arg)
	switch {
	case strings.HasPrefix(s, "{"):
		var in ImportInput
		if err := decodeStrict(s, &in); err != nil {
			return nil, err
		}
		return []ImportInput{in}, nil
	case strings.HasPrefix(s, "["):
		var ins []ImportInput
		if err := decodeStrict(s, &ins); err != nil {
			return nil, err
		}
		return ins, nil
	default:
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("import: %q is neither JSON nor an http(s) URL", arg)
		}
		return []ImportInput{{URL: s}}, nil
	}
}

func decodeStrict(s string, v any) error {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("import: invalid JSON: %w", err)
	}
	return nil
}

// FillMissing fills empty fields from extracted metadata. Values already
// present (typically user input) win.
func (in *ImportInput) FillMissing(company, title, location, rawURL, description string) {
	if in.Company == "" {
		in.Company = company
	}
	if in.Title == "" {
		in.Title = title
	}
	if in.Location == "" {
		in.Location = location
	}
	if in.URL == "" {
		in.URL = rawURL
	}
	if in.Description == "" {
		in.Description = description
	}
}

// Validate checks the required fields.
func (in ImportInput) Validate() error {
	var missing []string
	if strings.TrimSpace(in.Company) == "" {
		missing = append(missing, "company")
	}
	if strings.TrimSpace(in.Title) == "" {
		missing = append(missing, "title")
	}
	if len(missing) > 0 {
		return fmt.Errorf("import: missing %s", strings.Join(missing, ", "))
	}
	return ValidateImportURL(in.URL)
}

// ValidateImportURL checks that raw is empty or an absolute http(s) URL.
func ValidateImportURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("import: invalid url %q", raw)
	}
	return nil
}

// Import stores a manual offer through the same dedup path as scans, in
// one transaction. If the fingerprint matches an existing offer, the URL
// is added as a provenance row of that offer instead.
func Import(ctx context.Context, sqlDB *sql.DB, in ImportInput, now time.Time) (ImportResult, error) {
	if err := in.Validate(); err != nil {
		return ImportResult{}, err
	}
	now = now.UTC().Truncate(time.Second)
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, fmt.Errorf("begin import: %w", err)
	}
	defer tx.Rollback()

	id, outcome, err := applySighting(ctx, tx, Sighting{
		Company:     in.Company,
		Title:       in.Title,
		Location:    in.Location,
		URL:         in.URL,
		Description: in.Description,
	}, now, map[int64]bool{})
	if err != nil {
		return ImportResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("commit import: %w", err)
	}
	return ImportResult{OfferID: id, Created: outcome == OutcomeNew}, nil
}
