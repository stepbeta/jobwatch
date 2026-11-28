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
	Skipped     int // raw offers ignored (no URL to carry identity)
	New         int
	Seen        int // existing offers seen again (includes Reappeared)
	Reappeared  int // disappeared -> active
	Disappeared int
	Ghosted     int // disappeared applied/interview offers closed automatically
	Incomplete  int // items stored as incomplete offers
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

// ErrUntrustworthyScan reports a successful fetch whose item count is too
// small to trust: zero items for a source with active offers, or fewer than
// half of four or more known active offers. Such an observation is recorded
// as an error and must not mark offers disappeared.
var ErrUntrustworthyScan = errors.New("untrustworthy scan observation")

// ScanSource fetches src and applies the result in a single transaction.
// Fetch failures mark the source "error" (or "blocked" when access was
// refused); offers are then left untouched. An observation rejected by the
// trust guard is likewise recorded as "error" without touching offers.
func (s *Scanner) ScanSource(ctx context.Context, src model.Source) Result {
	res := Result{SourceID: src.ID, SourceName: src.Name}
	now := s.now()

	raws, err := s.fetchWithKnown(ctx, src)
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

	known, err := db.CountActiveSourceOffers(ctx, s.DB, src.ID)
	if err != nil {
		res.Status = model.ScanError
		res.Err = err
		if uerr := db.UpdateSourceScan(ctx, s.DB, src.ID, now, model.ScanError); uerr != nil {
			res.Err = errors.Join(err, uerr)
		}
		return res
	}
	if gerr := untrustworthyObservation(usableItems(raws), known); gerr != nil {
		res.Status = model.ScanError
		res.Err = gerr
		if uerr := db.UpdateSourceScan(ctx, s.DB, src.ID, now, model.ScanError); uerr != nil {
			res.Err = errors.Join(gerr, uerr)
		}
		return res
	}

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

// usableItems counts fetched items that carry the URL needed for
// provenance identity. Items without a URL are skipped by apply, so the
// guard must not count them as evidence of a healthy scan.
func usableItems(raws []sources.RawOffer) int {
	n := 0
	for _, raw := range raws {
		if strings.TrimSpace(raw.URL) != "" {
			n++
		}
	}
	return n
}

// untrustworthyObservation applies the disappearance guard. It returns nil
// when the observation may be applied.
func untrustworthyObservation(fetched, known int) error {
	if known == 0 {
		// A source with no known offers cannot be a drop.
		return nil
	}
	if fetched == 0 {
		return fmt.Errorf("%w: returned no usable items while %d active offers are known", ErrUntrustworthyScan, known)
	}
	if known >= 4 && fetched*2 < known {
		return fmt.Errorf("%w: returned %d usable items for %d active offers", ErrUntrustworthyScan, fetched, known)
	}
	return nil
}

// fetchWithKnown loads the descriptions already stored for src's offers
// and passes them to the adapter, so a detail-page fallback can skip
// items whose text is known.
func (s *Scanner) fetchWithKnown(ctx context.Context, src model.Source) ([]sources.RawOffer, error) {
	known, err := db.KnownDescriptions(ctx, s.DB, src.ID)
	if err != nil {
		return nil, err
	}
	return s.Fetcher.Fetch(sources.WithKnownDescriptions(ctx, known), src)
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
			Company:     firstNonEmpty(raw.Company, cfg.Company),
			CompanyKey:  raw.CompanyKey,
			Title:       raw.Title,
			Location:    raw.Location,
			URL:         raw.URL,
			Description: raw.Description,
			ExternalUID: raw.ExternalUID,
			SourceID:    &srcID,
		}
		if strings.TrimSpace(in.URL) == "" {
			// Without a URL there is no provenance identity to store.
			res.Skipped++
			continue
		}
		// The fields decide whether the item is usable; the adapter's
		// IncompleteReason is a hint and the pipeline recomputes it.
		if reason := sources.IncompleteReasonFor(in.Company, in.CompanyKey, in.Title); reason != "" {
			if err := applyIncompleteSighting(ctx, tx, in, reason, now, res); err != nil {
				return err
			}
			continue
		}
		_, outcome, err := applySighting(ctx, tx, in, now, touched)
		if errors.Is(err, db.ErrCompanyUnknown) {
			// The item has a title but no usable company evidence.
			reason := model.IncompleteMissingCompany
			if strings.TrimSpace(in.Title) == "" {
				reason = model.IncompleteMissingCompanyTitle
			}
			if err := applyIncompleteSighting(ctx, tx, in, reason, now, res); err != nil {
				return err
			}
			continue
		}
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
	for _, o := range gone {
		if o.ApplicationState != model.AppApplied && o.ApplicationState != model.AppInterview {
			continue
		}
		if err := db.GhostDisappearedOffer(ctx, tx, o.ID, o.LastSeenAt, now); err != nil {
			return err
		}
		res.Ghosted++
	}

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
