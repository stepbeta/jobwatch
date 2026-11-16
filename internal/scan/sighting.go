package scan

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/internal/classify"
	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

// Sighting is one observation of an offer, from a source scan or a
// manual import (SourceID nil).
type Sighting struct {
	Company, Title, Location, URL, Description, ExternalUID string
	SourceID                                                *int64
}

// Outcome describes what a sighting did to the offer table.
type Outcome int

const (
	OutcomeNew        Outcome = iota // offer created
	OutcomeSeen                      // existing offer, no content change
	OutcomeChanged                   // existing offer, description changed
	OutcomeReappeared                // existing offer was disappeared, now active
	OutcomeDuplicate                 // offer already updated by this scan; provenance only
)

// applySighting resolves the company, finds or creates the offer by
// fingerprint, applies the offer state transition and records the
// provenance row. touched holds offers already updated in the current
// scan so that two postings mapping to the same offer do not compare
// their texts against each other.
func applySighting(ctx context.Context, q db.Querier, in Sighting, now time.Time, touched map[int64]bool) (int64, Outcome, error) {
	company, _, err := db.ResolveCompany(ctx, q, in.Company, now)
	if err != nil {
		return 0, 0, err
	}
	title := strings.TrimSpace(in.Title)
	titleNorm := model.NormalizeTitle(title)
	if titleNorm == "" {
		return 0, 0, fmt.Errorf("offer %q: empty normalized title", in.Title)
	}
	location := strings.TrimSpace(in.Location)
	desc := strings.TrimSpace(in.Description)

	existing, fp, err := resolveOffer(ctx, q, company.NameNormalized, titleNorm, location)
	if err != nil {
		return 0, 0, err
	}

	var offerID int64
	var outcome Outcome
	switch {
	case existing == nil:
		scores := classify.Classify(title, location, desc).Scores()
		titleConf, locConf := scores.TitleConfidence, scores.LocationConfidence
		reasons := scores.MatchReasonsJSON
		o := model.JobOffer{
			CompanyID:          company.ID,
			Title:              title,
			TitleNormalized:    titleNorm,
			Location:           optStr(location),
			RemotePolicy:       scores.RemotePolicy,
			URL:                optStr(strings.TrimSpace(in.URL)),
			Fingerprint:        fp,
			RawText:            optStr(desc),
			TitleConfidence:    &titleConf,
			LocationConfidence: &locConf,
			MatchReasonsJSON:   &reasons,
		}
		if desc != "" {
			h := model.TextHash(desc)
			o.TextHash = &h
		}
		o, err = db.InsertOffer(ctx, q, o, now)
		if err != nil {
			return 0, 0, err
		}
		if err := db.InsertSnapshot(ctx, q, o.ID, desc, now); err != nil {
			return 0, 0, err
		}
		offerID, outcome = o.ID, OutcomeNew

	case touched[existing.ID]:
		offerID, outcome = existing.ID, OutcomeDuplicate

	default:
		offerID = existing.ID
		state, rawText, textHash := existing.OfferState, existing.RawText, existing.TextHash
		outcome = OutcomeSeen
		snapshot := false
		if desc != "" {
			h := model.TextHash(desc)
			switch {
			case textHash == nil:
				// first description for this offer: fill in, not a change
				rawText, textHash, snapshot = &desc, &h, true
			case *textHash != h:
				rawText, textHash, snapshot = &desc, &h, true
				state, outcome = model.OfferChanged, OutcomeChanged
			}
		}
		if outcome != OutcomeChanged && state == model.OfferDisappeared {
			state, outcome = model.OfferActive, OutcomeReappeared
		}
		if err := db.UpdateOfferSighting(ctx, q, offerID, state, rawText, textHash, now); err != nil {
			return 0, 0, err
		}
		// Recompute the scores from the values that are now current: the
		// stored location when the sighting has none, and the description
		// stored on the offer (which may predate this sighting).
		effectiveLocation := location
		if existing.Location != nil {
			effectiveLocation = *existing.Location
		}
		scores := classify.Classify(existing.Title, effectiveLocation, strValue(rawText)).Scores()
		if err := db.SetOfferClassification(ctx, q, offerID, scores); err != nil {
			return 0, 0, err
		}
		if snapshot {
			if err := db.InsertSnapshot(ctx, q, offerID, desc, now); err != nil {
				return 0, 0, err
			}
		}
	}
	touched[offerID] = true

	if u := strings.TrimSpace(in.URL); u != "" {
		if _, err := db.UpsertProvenance(ctx, q, offerID, in.SourceID, optStr(strings.TrimSpace(in.ExternalUID)), u, now); err != nil {
			return 0, 0, err
		}
	}
	return offerID, outcome, nil
}

// resolveOffer finds the offer matching company+title. The base
// fingerprint matches unless the stored offer has a different, known
// location bucket; in that case the location-bucketed fingerprint is
// used instead (SPEC section 5 collision rule). It returns the existing
// offer (nil if none) and the fingerprint a new offer must use.
func resolveOffer(ctx context.Context, q db.Querier, companyNorm, titleNorm, location string) (*model.JobOffer, string, error) {
	base := model.Fingerprint(companyNorm, titleNorm, "")
	o, err := db.GetOfferByFingerprint(ctx, q, base)
	if errors.Is(err, db.ErrNotFound) {
		return nil, base, nil
	}
	if err != nil {
		return nil, "", err
	}
	stored := ""
	if o.Location != nil {
		stored = model.LocationBucket(*o.Location)
	}
	bucket := model.LocationBucket(location)
	if stored == "" || bucket == "" || stored == bucket {
		return &o, base, nil
	}

	fp := model.Fingerprint(companyNorm, titleNorm, bucket)
	o2, err := db.GetOfferByFingerprint(ctx, q, fp)
	if errors.Is(err, db.ErrNotFound) {
		return nil, fp, nil
	}
	if err != nil {
		return nil, "", err
	}
	return &o2, fp, nil
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
