package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stepbeta/jobwatch/internal/db"
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

	f.Sort = strings.TrimSpace(r.URL.Query().Get("sort"))
	if f.Sort == "" {
		f.Sort = db.SortLastSeen
	}
	if !db.ValidOfferSort(f.Sort) {
		return f, fmt.Errorf("sort must be one of last_seen, first_seen, recent_event, title, company")
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
	o, err := db.GetOffer(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	company, err := db.GetCompany(ctx, s.DB, o.CompanyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
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
	var snapshot any
	latest, err := db.LatestSnapshot(ctx, s.DB, id)
	switch {
	case err == nil:
		snapshot = snapshotToJSON(latest)
	case errors.Is(err, db.ErrNotFound):
		snapshot = nil
	default:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
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
	offerOut := offerModelToJSON(o, company.Name)
	offerOut.RawText = o.RawText
	if len(events) > 0 {
		offerOut.LastEventAt = timeString(events[len(events)-1].OccurredAt)
	}

	companyOut := companyToJSON(company)
	if companyOut.OfferCount, companyOut.NoteCount, err = db.CompanyCounts(ctx, s.DB, company.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"offer":           offerOut,
		"company":         companyOut,
		"provenance":      provOut,
		"events":          eventsOut,
		"notes":           notesOut,
		"latest_snapshot": snapshot,
		"snapshot_count":  snapCount,
	})
}

type offerPatch struct {
	ApplicationState *string         `json:"application_state"`
	OfferState       *string         `json:"offer_state"`
	Location         json.RawMessage `json:"location"`
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
	if u.ApplicationState == nil && u.OfferState == nil && u.Location == nil {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"nothing to update: provide application_state, offer_state or location")
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
	company, err := db.GetCompany(ctx, s.DB, o.CompanyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offer": offerModelToJSON(o, company.Name)})
}
