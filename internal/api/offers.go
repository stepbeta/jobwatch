package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/diff"
	"github.com/stepbeta/jobwatch/internal/model"
)

const (
	defaultOfferLimit = 50
	maxOfferLimit     = 200
)

// parseOfferQuery builds a db.OfferQuery from the request parameters.
func parseOfferQuery(r *http.Request) (db.OfferQuery, error) {
	var f db.OfferQuery
	f.Search = strings.TrimSpace(r.URL.Query().Get("q"))

	apps, err := queryEnumList(r, "application_state", model.ApplicationState.Valid)
	if err != nil {
		return f, err
	}
	f.ApplicationStates = apps
	offers, err := queryEnumList(r, "offer_state", model.OfferState.Valid)
	if err != nil {
		return f, err
	}
	f.OfferStates = offers
	remotes, err := queryEnumList(r, "remote_policy", model.RemotePolicy.Valid)
	if err != nil {
		return f, err
	}
	f.RemotePolicies = remotes
	if f.MinTitleConfidence, err = queryInt(r, "min_title_confidence", 0, 100); err != nil {
		return f, err
	}
	if f.MinLocationConfidence, err = queryInt(r, "min_location_confidence", 0, 100); err != nil {
		return f, err
	}
	if f.CompanyID, err = queryInt64(r, "company_id"); err != nil {
		return f, err
	}
	f.HasApplication = queryBool(r, "has_application")
	if raw := strings.TrimSpace(r.URL.Query().Get("saved")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return f, fmt.Errorf("saved must be true or false")
		}
		f.Saved = &v
	}

	f.Sort = strings.TrimSpace(r.URL.Query().Get("sort"))
	if f.Sort == "" {
		f.Sort = db.SortLastSeen
	}
	if !db.ValidOfferSort(f.Sort) {
		return f, fmt.Errorf("sort must be one of last_seen, first_seen, recent_event, title, company, location, title_confidence, location_confidence, saved")
	}
	f.Order = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))
	if !db.ValidOfferOrder(f.Order) {
		return f, fmt.Errorf("order must be asc or desc")
	}
	f.Limit = defaultOfferLimit
	if v, err := queryInt(r, "limit", 1, maxOfferLimit); err != nil {
		return f, err
	} else if v != nil {
		f.Limit = *v
	}
	if v, err := queryInt(r, "offset", 0, 1_000_000); err != nil {
		return f, err
	} else if v != nil {
		f.Offset = *v
	}
	return f, nil
}

func (s *Server) listOffers(w http.ResponseWriter, r *http.Request) {
	f, err := parseOfferQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	res, err := db.QueryOffers(r.Context(), s.DB, f)
	if errors.Is(err, db.ErrInvalidQuery) {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]offerJSON, 0, len(res.Offers))
	for _, o := range res.Offers {
		out = append(out, offerToJSON(o))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offers": out,
		"total":  res.Total,
		"limit":  f.Limit,
		"offset": f.Offset,
	})
}

func (s *Server) getOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	ctx := r.Context()
	snapshotID, err := queryInt64(r, "snapshot_id")
	if err != nil || (snapshotID != nil && *snapshotID <= 0) {
		writeError(w, http.StatusBadRequest, "validation_failed", "snapshot_id must be a positive integer")
		return
	}
	o, err := db.GetOffer(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var companyOut any
	companyName := ""
	if o.CompanyID != nil {
		company, err := db.GetCompany(ctx, s.DB, *o.CompanyID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		companyName = company.Name
		out := companyToJSON(company)
		if out.OfferCount, out.NoteCount, err = db.CompanyCounts(ctx, s.DB, company.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		companyOut = out
	}
	prov, err := db.ListProvenanceDetailed(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	events, err := db.ListEvents(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	notes, err := db.ListNotes(ctx, s.DB, model.EntityOffer, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	snapCount, err := db.CountSnapshots(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var rawTextID int64
	if snapshotID != nil {
		rawTextID = *snapshotID
	}
	infos, err := db.ListSnapshotInfos(ctx, s.DB, id, rawTextID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if snapshotID != nil {
		found := false
		for _, s := range infos {
			if s.ID == *snapshotID {
				found = true
				break
			}
		}
		if !found {
			writeError(w, http.StatusNotFound, "not_found",
				fmt.Sprintf("snapshot %d not found on offer %d", *snapshotID, id))
			return
		}
	}
	snaps, err := db.LatestSnapshots(ctx, s.DB, id, 2)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var snapshot, snapshotDiff any
	if len(snaps) > 0 {
		snapshot = snapshotToJSON(snaps[0])
	}
	if len(snaps) > 1 {
		snapshotDiff = diffToJSON(snaps[1], snaps[0])
	}
	snapshotsOut := make([]snapshotSummaryJSON, 0, len(infos))
	for _, s := range infos {
		snapshotsOut = append(snapshotsOut, snapshotInfoToJSON(s))
	}

	provOut := make([]provenanceJSON, 0, len(prov))
	for _, p := range prov {
		provOut = append(provOut, provenanceToJSON(p))
	}
	eventsOut := make([]eventJSON, 0, len(events))
	for _, e := range events {
		eventsOut = append(eventsOut, eventToJSON(e))
	}
	notesOut := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		notesOut = append(notesOut, noteToJSON(n))
	}
	offerOut := offerModelToJSON(o, companyName)
	offerOut.RawText = o.RawText
	if len(events) > 0 {
		offerOut.LastEventAt = timeString(events[len(events)-1].OccurredAt)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"offer":           offerOut,
		"company":         companyOut,
		"provenance":      provOut,
		"events":          eventsOut,
		"notes":           notesOut,
		"snapshots":       snapshotsOut,
		"latest_snapshot": snapshot,
		"snapshot_diff":   snapshotDiff,
		"snapshot_count":  snapCount,
	})
}

// offerCompanyName returns the display name of the offer's company, or an
// empty string for an incomplete offer with no company yet.
func (s *Server) offerCompanyName(ctx context.Context, o model.JobOffer) (string, error) {
	if o.CompanyID == nil {
		return "", nil
	}
	c, err := db.GetCompany(ctx, s.DB, *o.CompanyID)
	if err != nil {
		return "", err
	}
	return c.Name, nil
}

type offerPatch struct {
	ApplicationState *string         `json:"application_state"`
	OfferState       *string         `json:"offer_state"`
	Location         json.RawMessage `json:"location"`
	Description      json.RawMessage `json:"description"`
	Saved            *bool           `json:"saved"`
}

func (s *Server) patchOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in offerPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.OfferUpdate
	if in.ApplicationState != nil {
		st := model.ApplicationState(strings.TrimSpace(*in.ApplicationState))
		if !st.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid application_state %q", st))
			return
		}
		u.ApplicationState = &st
	}
	if in.OfferState != nil {
		st := model.OfferState(strings.TrimSpace(*in.OfferState))
		if !st.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid offer_state %q", st))
			return
		}
		u.OfferState = &st
	}
	if len(in.Location) > 0 {
		var loc string
		if err := json.Unmarshal(in.Location, &loc); err != nil {
			if string(in.Location) != "null" {
				writeError(w, http.StatusBadRequest, "validation_failed", "location must be a string or null")
				return
			}
			loc = ""
		}
		loc = strings.TrimSpace(loc)
		u.Location = &loc
	}
	if len(in.Description) > 0 {
		if string(in.Description) == "null" {
			writeError(w, http.StatusBadRequest, "validation_failed",
				"description cannot be null; send a string to replace it")
			return
		}
		var desc string
		if err := json.Unmarshal(in.Description, &desc); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "description must be a string")
			return
		}
		desc = strings.TrimSpace(desc)
		u.Description = &desc
	}
	if in.Saved != nil {
		u.Saved = in.Saved
	}
	if u.ApplicationState == nil && u.OfferState == nil && u.Location == nil && u.Description == nil && u.Saved == nil {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"nothing to update: provide application_state, offer_state, location, description or saved")
		return
	}

	ctx := r.Context()
	if err := db.UpdateOfferFields(ctx, s.DB, id, u, s.now()); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	o, err := db.GetOffer(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, o)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	offerOut := offerModelToJSON(o, companyName)
	offerOut.RawText = o.RawText
	writeJSON(w, http.StatusOK, map[string]any{"offer": offerOut})
}

type offerRepair struct {
	CompanyID int64   `json:"company_id"`
	Title     string  `json:"title"`
	Location  *string `json:"location"`
}

// repairOffer assigns a company and title to an incomplete offer. The raw
// reported values stay in incomplete_raw, which later scans use to avoid
// overwriting the user's title and location.
func (s *Server) repairOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in offerRepair
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Title = strings.TrimSpace(in.Title)
	if in.CompanyID <= 0 || in.Title == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "company_id and title are required")
		return
	}
	var location *string
	if in.Location != nil {
		if v := strings.TrimSpace(*in.Location); v != "" {
			location = &v
		}
	}
	ctx := r.Context()
	o, err := db.RepairOffer(ctx, s.DB, id, in.CompanyID, in.Title, location, s.now())
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "offer or company not found")
		return
	case errors.Is(err, db.ErrOfferNotIncomplete):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, o)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offer": offerModelToJSON(o, companyName)})
}

func diffToJSON(from, to model.Snapshot) snapshotDiffJSON {
	lines := diff.Lines(from.RawText, to.RawText)
	out := snapshotDiffJSON{
		From:  snapshotRefJSON{ID: from.ID, CapturedAt: model.FormatTime(from.CapturedAt)},
		To:    snapshotRefJSON{ID: to.ID, CapturedAt: model.FormatTime(to.CapturedAt)},
		Lines: make([]diffLineJSON, 0, len(lines)),
	}
	for _, line := range lines {
		out.Lines = append(out.Lines, diffLineJSON{Op: string(line.Op), Text: line.Text})
	}
	return out
}

// snapshotDiff returns the line diff between any two snapshots of an
// offer. The pair is ordered oldest -> newest server-side.
func (s *Server) snapshotDiff(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	fromID, err := queryInt64(r, "from")
	if err != nil || fromID == nil || *fromID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "from must be a positive integer")
		return
	}
	toID, err := queryInt64(r, "to")
	if err != nil || toID == nil || *toID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "to must be a positive integer")
		return
	}
	if *fromID == *toID {
		writeError(w, http.StatusBadRequest, "validation_failed", "from and to must be different snapshots")
		return
	}
	ctx := r.Context()
	if _, err := db.GetOffer(ctx, s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	from, err := db.GetSnapshotForOffer(ctx, s.DB, id, *fromID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("snapshot %d not found on offer %d", *fromID, id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	to, err := db.GetSnapshotForOffer(ctx, s.DB, id, *toID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("snapshot %d not found on offer %d", *toID, id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	older, newer := from, to
	if newer.CapturedAt.Before(older.CapturedAt) ||
		(newer.CapturedAt.Equal(older.CapturedAt) && newer.ID < older.ID) {
		older, newer = newer, older
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot_diff": diffToJSON(older, newer)})
}

type mergeOfferInput struct {
	IntoID int64 `json:"into_id"`
}

// mergeOffer merges :id into into_id after an explicit user confirmation.
func (s *Server) mergeOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in mergeOfferInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.IntoID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "into_id must be a positive integer")
		return
	}
	ctx := r.Context()
	res, err := db.MergeOffers(ctx, s.DB, id, in.IntoID, s.now())
	switch {
	case errors.Is(err, db.ErrOfferMergeSelf):
		writeError(w, http.StatusBadRequest, "validation_failed", "an offer cannot be merged into itself")
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "one of the offers does not exist")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, res.Offer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"offer":            offerModelToJSON(res.Offer, companyName),
		"merged_offer_id":  res.MergedOfferID,
		"moved_provenance": res.MovedProvenance,
		"moved_events":     res.MovedEvents,
		"moved_notes":      res.MovedNotes,
		"moved_snapshots":  res.MovedSnapshots,
	})
}

type splitOfferInput struct {
	ProvenanceIDs []int64         `json:"provenance_ids"`
	Title         string          `json:"title"`
	Location      json.RawMessage `json:"location"`
}

// splitOffer moves selected provenance rows to a new offer in the same
// company. location is optional: omitted copies the original location,
// null clears it.
func (s *Server) splitOffer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return
	}
	var in splitOfferInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.ProvenanceIDs) == 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "provenance_ids must contain at least one row")
		return
	}
	var location *string
	if len(in.Location) > 0 {
		if string(in.Location) == "null" {
			empty := ""
			location = &empty
		} else {
			var value string
			if err := json.Unmarshal(in.Location, &value); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "location must be a string or null")
				return
			}
			value = strings.TrimSpace(value)
			location = &value
		}
	}
	ctx := r.Context()
	res, err := db.SplitOffer(ctx, s.DB, id, db.OfferSplitInput{
		ProvenanceIDs: in.ProvenanceIDs,
		Title:         in.Title,
		Location:      location,
	}, s.now())
	switch {
	case errors.Is(err, db.ErrOfferSplitNoProvenance),
		errors.Is(err, db.ErrOfferSplitProvenance),
		errors.Is(err, db.ErrOfferSplitLastLink):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyName, err := s.offerCompanyName(ctx, res.Offer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"offer":            offerModelToJSON(res.Offer, companyName),
		"moved_provenance": res.MovedProvenance,
	})
}
