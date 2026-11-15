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
	Company                                        string // from the source, may override the source's own company
}

// Fetcher retrieves the current list of offers of a source.
type Fetcher interface {
	Fetch(ctx context.Context, src model.Source) ([]RawOffer, error)
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

	BoardToken string `json:"board_token,omitempty"` // greenhouse
	Site       string `json:"site,omitempty"`        // lever
	Region     string `json:"region,omitempty"`      // lever: "" (global) or "eu"
	Org        string `json:"org,omitempty"`         // ashby job board name
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

// Registry maps source types to their fetcher.
type Registry map[model.SourceType]Fetcher

// NewRegistry returns the adapters implemented so far, sharing client.
func NewRegistry(client *Client) Registry {
	return Registry{
		model.SourceGreenhouse: &Greenhouse{Client: client},
		model.SourceLever:      &Lever{Client: client},
		model.SourceAshby:      &Ashby{Client: client},
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
