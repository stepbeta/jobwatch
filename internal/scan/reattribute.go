package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// ReattributeResult summarizes one source's re-attribution pass.
type ReattributeResult struct {
	SourceID   int64
	SourceName string
	// Status is empty when the source has no adapter and nothing was
	// fetched; otherwise ok or the fetch error state.
	Status model.ScanStatus
	Err    error

	Fetched          int      // offers returned by the adapter
	Reattributed     int      // stored offers moved to another company or title
	Unchanged        int      // matched offers the adapter already agrees with
	Skipped          int      // matched offers left untouched (repaired, incomplete, unmatched raw)
	EmptiedCompanies []string // companies that lost their last offer in this pass
}

// ReattributeAll re-fetches every enabled source and re-attributes its
// offers. A failing source never stops the loop.
func (s *Scanner) ReattributeAll(ctx context.Context) ([]ReattributeResult, error) {
	srcs, err := db.ListSources(ctx, s.DB, true)
	if err != nil {
		return nil, err
	}
	out := make([]ReattributeResult, 0, len(srcs))
	for _, src := range srcs {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		out = append(out, s.ReattributeSource(ctx, src))
	}
	return out, nil
}

// ReattributeSource re-fetches src and rewrites the company and title of
// the offers linked by (source, uid) to the adapter's current output.
// Events, notes, snapshots, application state and listing state are not
// touched. The pass is idempotent: a second run with the same output
// changes nothing. Offers without a uid link, and repaired offers, are
// left untouched.
func (s *Scanner) ReattributeSource(ctx context.Context, src model.Source) ReattributeResult {
	res := ReattributeResult{SourceID: src.ID, SourceName: src.Name}
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
		return res
	}
	res.Fetched = len(raws)

	cfg, err := sources.ParseConfig(src)
	if err != nil {
		res.Status, res.Err = model.ScanError, err
		return res
	}

	byUID := make(map[string]sources.RawOffer, len(raws))
	for _, raw := range raws {
		if uid := strings.TrimSpace(raw.ExternalUID); uid != "" {
			byUID[uid] = raw
		}
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		res.Status, res.Err = model.ScanError, fmt.Errorf("begin reattribution of source %d: %w", src.ID, err)
		return res
	}
	defer tx.Rollback()

	links, err := db.ListSourceProvenanceOffers(ctx, tx, src.ID)
	if err != nil {
		res.Status, res.Err = model.ScanError, err
		return res
	}

	lost := map[int64]bool{}
	srcID := src.ID
	for _, link := range links {
		raw, ok := byUID[strings.TrimSpace(link.ExternalUID)]
		if !ok {
			// No sighting under this uid in the current output: history.
			res.Skipped++
			continue
		}
		if link.IncompleteRaw != nil {
			// Incomplete or user-repaired: the user owns this row.
			res.Skipped++
			continue
		}
		companyName := firstNonEmpty(raw.Company, cfg.Company)
		if sources.IncompleteReasonFor(companyName, raw.CompanyKey, raw.Title) != "" {
			res.Skipped++
			continue
		}
		title := strings.TrimSpace(raw.Title)
		titleNorm := model.NormalizeTitle(title)
		if titleNorm == "" {
			res.Skipped++
			continue
		}

		company, _, err := db.ResolveCompany(ctx, tx, db.CompanyInput{
			Key: raw.CompanyKey, Name: companyName, SourceID: &srcID,
		}, now)
		if errors.Is(err, db.ErrCompanyUnknown) {
			res.Skipped++
			continue
		}
		if err != nil {
			res.Status, res.Err = model.ScanError, err
			return res
		}

		companyChanged := link.CompanyID == nil || *link.CompanyID != company.ID
		if !companyChanged && link.Title == title {
			res.Unchanged++
			continue
		}
		if link.CompanyID != nil {
			lost[*link.CompanyID] = true
		}
		if err := db.ReattributeOffer(ctx, tx, link.OfferID, company.ID, title, titleNorm, now); err != nil {
			res.Status, res.Err = model.ScanError, err
			return res
		}
		scores := classify.Classify(title, strValue(link.Location), strValue(link.RawText)).Scores()
		if err := db.SetOfferClassification(ctx, tx, link.OfferID, scores); err != nil {
			res.Status, res.Err = model.ScanError, err
			return res
		}
		res.Reattributed++
	}

	ids := make([]int64, 0, len(lost))
	for id := range lost {
		ids = append(ids, id)
	}
	if res.EmptiedCompanies, err = db.CompaniesWithoutOffers(ctx, tx, ids); err != nil {
		res.Status, res.Err = model.ScanError, err
		return res
	}
	if err := tx.Commit(); err != nil {
		res.Status, res.Err = model.ScanError, fmt.Errorf("commit reattribution of source %d: %w", src.ID, err)
		return res
	}
	res.Status = model.ScanOK
	return res
}
