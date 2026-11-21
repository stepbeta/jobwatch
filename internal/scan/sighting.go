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
	Company, CompanyKey, Title, Location, URL, Description, ExternalUID string
	SourceID                                                            *int64
}

// Outcome describes what a sighting did to the offer table.
type Outcome int

const (
	OutcomeNew        Outcome = iota // offer created
	OutcomeSeen                      // existing offer, refreshed
	OutcomeReappeared                // existing offer was disappeared, now active
	OutcomeDuplicate                 // offer already updated by this scan; provenance only
)

// applySighting resolves the sighting through provenance identity, updates
// or creates the offer, applies the listing-state transition and records
// the provenance row. A sighting is resolved by (source, uid) first, then
// (source, url); the fingerprint no longer participates. touched holds
// offers already updated in the current scan so that two postings mapping
// to the same offer do not compare their texts against each other.
func applySighting(ctx context.Context, q db.Querier, in Sighting, now time.Time, touched map[int64]bool) (int64, Outcome, error) {
	title := strings.TrimSpace(in.Title)
	titleNorm := model.NormalizeTitle(title)
	if titleNorm == "" {
		return 0, 0, fmt.Errorf("offer %q: empty normalized title", in.Title)
	}
	location := strings.TrimSpace(in.Location)
	desc := strings.TrimSpace(in.Description)
	link := strings.TrimSpace(in.URL)
	uid := strings.TrimSpace(in.ExternalUID)

	existing, err := db.FindOfferByProvenance(ctx, q, in.SourceID, uid, link)
	if err != nil && !errors.Is(err, db.ErrNotFound) {
		return 0, 0, err
	}
	var found *model.JobOffer
	if err == nil {
		found = &existing
	}

	var offerID int64
	var outcome Outcome
	switch {
	case found == nil:
		// A miss is the only path that resolves a company and creates a row.
		company, _, err := db.ResolveCompany(ctx, q, db.CompanyInput{
			Key:      in.CompanyKey,
			Name:     in.Company,
			SourceID: in.SourceID,
		}, now)
		if err != nil {
			return 0, 0, err
		}
		scores := classify.Classify(title, location, desc).Scores()
		titleConf, locConf := scores.TitleConfidence, scores.LocationConfidence
		reasons := scores.MatchReasonsJSON
		o := model.JobOffer{
			CompanyID:          company.ID,
			Title:              title,
			TitleNormalized:    titleNorm,
			Location:           optStr(location),
			RemotePolicy:       scores.RemotePolicy,
			URL:                optStr(link),
			Fingerprint:        model.Fingerprint(company.NameNormalized, titleNorm, ""),
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
		if err := db.InsertSnapshotOrigin(ctx, q, o.ID, desc, model.SnapshotOriginScan, now); err != nil {
			return 0, 0, err
		}
		offerID, outcome = o.ID, OutcomeNew

	case touched[found.ID]:
		offerID, outcome = found.ID, OutcomeDuplicate

	default:
		offerID = found.ID
		state, rawText, textHash := found.OfferState, found.RawText, found.TextHash
		outcome = OutcomeSeen
		snapshot := false
		if desc != "" {
			h := model.TextHash(desc)
			switch {
			case textHash == nil:
				// first description for this offer: fill in, not a change
				rawText, textHash, snapshot = &desc, &h, true
			case *textHash != h:
				// A description edit is not a listing state: the new text
				// is kept as a snapshot.
				rawText, textHash, snapshot = &desc, &h, true
			}
		}
		if state == model.OfferDisappeared {
			state, outcome = model.OfferActive, OutcomeReappeared
		}

		// The title and a reported location follow a source. An empty
		// location keeps the stored value; the company and canonical url
		// are attributes of the offer and are not rewritten by a sighting.
		// A manual import (no source) only adds a link: it does not rewrite
		// the offer's title or location.
		effectiveLocation := location
		if effectiveLocation == "" && found.Location != nil {
			effectiveLocation = *found.Location
		}
		updateTitle, updateTitleNorm := title, titleNorm
		if in.SourceID == nil {
			updateTitle, updateTitleNorm = found.Title, found.TitleNormalized
			effectiveLocation = strValue(found.Location)
		}
		if err := db.UpdateOfferSighting(ctx, q, offerID, updateTitle, updateTitleNorm,
			optStr(effectiveLocation), state, rawText, textHash, now); err != nil {
			return 0, 0, err
		}
		scores := classify.Classify(updateTitle, effectiveLocation, strValue(rawText)).Scores()
		if err := db.SetOfferClassification(ctx, q, offerID, scores); err != nil {
			return 0, 0, err
		}
		if snapshot {
			if err := db.InsertSnapshotOrigin(ctx, q, offerID, desc, model.SnapshotOriginScan, now); err != nil {
				return 0, 0, err
			}
		}
	}
	touched[offerID] = true

	if link != "" {
		if _, err := db.UpsertProvenance(ctx, q, offerID, in.SourceID, optStr(uid), link, now); err != nil {
			return 0, 0, err
		}
	}
	return offerID, outcome, nil
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
