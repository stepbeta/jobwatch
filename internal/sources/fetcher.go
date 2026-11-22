// Package sources defines the Fetcher interface, the shared polite HTTP
// client and one adapter per source type.
package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// RawOffer is an offer as parsed from a source, before company
// resolution and deduplication.
type RawOffer struct {
	ExternalUID, Title, Location, URL, Description string
	// Company is the display-name evidence the source reported. CompanyKey
	// is the namespaced identifier it already holds (greenhouse:acme,
	// lever:acme, …). Either may be empty; a source with neither cannot
	// attribute the item.
	Company    string
	CompanyKey string
	// IncompleteReason is set when the adapter could not normalize the
	// item (missing title or company). The item is still returned so the
	// pipeline can store it instead of silently dropping it.
	IncompleteReason model.IncompleteReason
}

// Fetcher retrieves the current list of offers of a source.
type Fetcher interface {
	Fetch(ctx context.Context, src model.Source) ([]RawOffer, error)
}

type knownDescriptionsKey struct{}

// WithKnownDescriptions attaches the descriptions already stored for a
// source's offers, keyed by external UID. Adapters that would otherwise
// fetch a detail page per item use it to skip items whose text is
// already known, so a scan spends its requests on genuinely missing
// descriptions.
func WithKnownDescriptions(ctx context.Context, known map[string]string) context.Context {
	if len(known) == 0 {
		return ctx
	}
	return context.WithValue(ctx, knownDescriptionsKey{}, known)
}

// KnownDescriptions returns the map attached by WithKnownDescriptions,
// or nil when the caller has no stored descriptions.
func KnownDescriptions(ctx context.Context) map[string]string {
	known, _ := ctx.Value(knownDescriptionsKey{}).(map[string]string)
	return known
}

// ErrBlocked is wrapped by errors caused by the source refusing access
// (robots.txt disallow, HTTP 401/403/429). Such sources are marked
// "blocked" rather than "error".
var ErrBlocked = errors.New("access blocked")

// ErrNotImplemented is returned for source types without an adapter yet.
var ErrNotImplemented = errors.New("source type not implemented")

// Config is the union of per-type settings stored in sources.config_json.
// Unused fields are ignored by each adapter.
type Config struct {
	// Company overrides the company name used for offers of this source
	// (default: the source name).
	Company string `json:"company,omitempty"`
	// RateLimitMS is the minimum interval between requests to this
	// source's host, in milliseconds (default 2000).
	RateLimitMS int `json:"rate_limit_ms,omitempty"`
	// Note is an optional human-readable explanation shown in the UI,
	// used by the seeded disabled sources for Workday/SuccessFactors.
	Note string `json:"note,omitempty"`
	// URL optionally overrides the source row URL (wttj: the careers
	// page whose bootstrap is read to discover the search index).
	URL string `json:"url,omitempty"`

	BoardToken string `json:"board_token,omitempty"` // greenhouse
	Site       string `json:"site,omitempty"`        // lever
	Region     string `json:"region,omitempty"`      // lever: "" (global) or "eu"
	Org        string `json:"org,omitempty"`         // ashby job board name

	Tenant    string `json:"tenant,omitempty"`    // personio subdomain
	Language  string `json:"language,omitempty"`  // personio feed language (optional)
	Account   string `json:"account,omitempty"`   // workable account slug
	Subdomain string `json:"subdomain,omitempty"` // bamboohr tenant subdomain

	// wttj: debug-only overrides of the Algolia credentials that are
	// normally discovered from the careers page bootstrap.
	AlgoliaAppID  string `json:"algolia_app_id,omitempty"`
	AlgoliaAPIKey string `json:"algolia_api_key,omitempty"`

	// rss: item element and field paths. A path is a chain of child
	// element local names; "@name" reads an attribute.
	Item             string `json:"item,omitempty"`
	TitleField       string `json:"title_field,omitempty"`
	LinkField        string `json:"link_field,omitempty"`
	DescriptionField string `json:"description_field,omitempty"`
	LocationField    string `json:"location_field,omitempty"`
	CompanyField     string `json:"company_field,omitempty"`
	UIDField         string `json:"uid_field,omitempty"`

	// Employer is an optional declarative rule that derives the employer
	// from the title (or a separate field/selector) and rewrites the title
	// without the employer prefix. Used by RSS and HTML sources that list
	// several employers.
	Employer *EmployerRule `json:"employer,omitempty"`

	// html: CSS selectors for the listing page.
	ItemSelector        string `json:"item_selector,omitempty"`
	TitleSelector       string `json:"title_selector,omitempty"`
	LinkSelector        string `json:"link_selector,omitempty"`
	LinkAttr            string `json:"link_attr,omitempty"`
	LocationSelector    string `json:"location_selector,omitempty"`
	DescriptionSelector string `json:"description_selector,omitempty"`
	CompanySelector     string `json:"company_selector,omitempty"`
	UIDSelector         string `json:"uid_selector,omitempty"`
	UIDAttr             string `json:"uid_attr,omitempty"`
}

// ParseConfig decodes src.ConfigJSON. An empty string is an empty config.
func ParseConfig(src model.Source) (Config, error) {
	var c Config
	raw := strings.TrimSpace(src.ConfigJSON)
	if raw == "" {
		return c, nil
	}
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return c, fmt.Errorf("source %d: invalid config_json: %w", src.ID, err)
	}
	return c, nil
}

// Interval returns the configured per-source request interval, or def.
func (c Config) Interval(def time.Duration) time.Duration {
	if c.RateLimitMS > 0 {
		return time.Duration(c.RateLimitMS) * time.Millisecond
	}
	return def
}

// IntervalFor returns the request interval for a source type. HTML pages
// are the most fragile and least structured target, so their polling
// never runs faster than 5 s even when config_json asks for less. A
// larger rate_limit_ms always wins.
func (c Config) IntervalFor(typ model.SourceType, def time.Duration) time.Duration {
	d := c.Interval(def)
	if typ == model.SourceHTML && d < HTMLMinInterval {
		return HTMLMinInterval
	}
	return d
}

// HTMLMinInterval is the conservative lower bound for HTML scraping.
const HTMLMinInterval = 5 * time.Second

// Registry maps source types to their fetcher.
type Registry map[model.SourceType]Fetcher

// NewRegistry returns the adapters implemented so far, sharing client.
func NewRegistry(client *Client) Registry {
	return Registry{
		model.SourceGreenhouse: &Greenhouse{Client: client},
		model.SourceLever:      &Lever{Client: client},
		model.SourceAshby:      &Ashby{Client: client},
		model.SourcePersonio:   &Personio{Client: client},
		model.SourceWorkable:   &Workable{Client: client},
		model.SourceBambooHR:   &BambooHR{Client: client},
		model.SourceWTTJ:       &WTTJ{Client: client},
		model.SourceRSS:        &RSS{Client: client},
		model.SourceHTML:       &HTML{Client: client},
	}
}

// Fetch dispatches to the fetcher registered for src.Type.
func (r Registry) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	f, ok := r[src.Type]
	if !ok {
		return nil, fmt.Errorf("%s: %w", src.Type, ErrNotImplemented)
	}
	return f.Fetch(ctx, src)
}
