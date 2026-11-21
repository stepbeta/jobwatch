// Package scan runs the per-source scan pipeline (fetch, company
// resolution, dedup, offer/provenance upsert, snapshots, state
// transitions) and manual imports.
package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// Scanner scans sources into the database.
type Scanner struct {
	DB      *sql.DB
	Fetcher sources.Fetcher
	Now     func() time.Time // defaults to time.Now
}

// Result summarizes the scan of one source.
type Result struct {
	SourceID   int64
	SourceName string
	// Status is the recorded scan status; empty when the source was
	// skipped (no adapter yet) and nothing was recorded.
	Status model.ScanStatus
	Err    error

	Fetched     int // offers returned by the adapter
	Skipped     int // raw offers ignored (missing title or URL)
	New         int
	Seen        int // existing offers seen again (includes Reappeared)
	Reappeared  int // disappeared -> active
	Disappeared int
}

func (s *Scanner) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

// ScanAll scans every enabled source in ID order. A failing source never
// stops the loop; its error is reported in its Result.
func (s *Scanner) ScanAll(ctx context.Context) ([]Result, error) {
	srcs, err := db.ListSources(ctx, s.DB, true)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(srcs))
	for _, src := range srcs {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out = append(out, s.ScanSource(ctx, src))
	}
	return out, nil
}

// ScanSource fetches src and applies the result in a single transaction.
// Fetch failures mark the source "error" (or "blocked" when access was
// refused); offers are then left untouched.
func (s *Scanner) ScanSource(ctx context.Context, src model.Source) Result {
	res := Result{SourceID: src.ID, SourceName: src.Name}
	now := s.now()

	raws, err := s.Fetcher.Fetch(ctx, src)
	if errors.Is(err, sources.ErrNotImplemented) {
		res.Err = err
		return res
	}
	if err != nil {
		res.Err = err
		res.Status = model.ScanError
		if errors.Is(err, sources.ErrBlocked) {
			res.Status = model.ScanBlocked
		}
		if uerr := db.UpdateSourceScan(ctx, s.DB, src.ID, now, res.Status); uerr != nil {
			res.Err = errors.Join(err, uerr)
		}
		return res
	}
	res.Fetched = len(raws)

	if err := s.apply(ctx, src, raws, now, &res); err != nil {
		res = Result{SourceID: src.ID, SourceName: src.Name, Fetched: len(raws), Status: model.ScanError, Err: err}
		if uerr := db.UpdateSourceScan(ctx, s.DB, src.ID, now, model.ScanError); uerr != nil {
			res.Err = errors.Join(err, uerr)
		}
		return res
	}
	res.Status = model.ScanOK
	return res
}

func (s *Scanner) apply(ctx context.Context, src model.Source, raws []sources.RawOffer, now time.Time, res *Result) error {
	cfg, err := sources.ParseConfig(src)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin scan of source %d: %w", src.ID, err)
	}
	defer tx.Rollback()

	touched := map[int64]bool{}
	srcID := src.ID
	for _, raw := range raws {
		in := Sighting{
			Company:     firstNonEmpty(raw.Company, cfg.Company, src.Name),
			Title:       raw.Title,
			Location:    raw.Location,
			URL:         raw.URL,
			Description: raw.Description,
			ExternalUID: raw.ExternalUID,
			SourceID:    &srcID,
		}
		if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.URL) == "" {
			res.Skipped++
			continue
		}
		_, outcome, err := applySighting(ctx, tx, in, now, touched)
		if err != nil {
			return err
		}
		res.count(outcome)
	}

	gone, err := db.MarkDisappeared(ctx, tx, src.ID, now)
	if err != nil {
		return err
	}
	res.Disappeared = len(gone)

	if err := db.UpdateSourceScan(ctx, tx, src.ID, now, model.ScanOK); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit scan of source %d: %w", src.ID, err)
	}
	return nil
}

func (r *Result) count(o Outcome) {
	switch o {
	case OutcomeNew:
		r.New++
	case OutcomeSeen:
		r.Seen++
	case OutcomeReappeared:
		r.Seen++
		r.Reappeared++
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
