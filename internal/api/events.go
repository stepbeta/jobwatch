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

func (s *Server) offerForRequest(w http.ResponseWriter, r *http.Request) (model.JobOffer, bool) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "offer id must be a positive integer")
		return model.JobOffer{}, false
	}
	o, err := db.GetOffer(r.Context(), s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("offer %d not found", id))
		return model.JobOffer{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return model.JobOffer{}, false
	}
	return o, true
}

func (s *Server) eventForRequest(w http.ResponseWriter, r *http.Request, offerID int64) (model.Event, bool) {
	eventID, ok := pathID(r, "eventID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "event id must be a positive integer")
		return model.Event{}, false
	}
	e, err := db.GetEvent(r.Context(), s.DB, eventID)
	if errors.Is(err, db.ErrNotFound) || (err == nil && e.OfferID != offerID) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("event %d not found on offer %d", eventID, offerID))
		return model.Event{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return model.Event{}, false
	}
	return e, true
}

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	o, ok := s.offerForRequest(w, r)
	if !ok {
		return
	}
	events, err := db.ListEvents(r.Context(), s.DB, o.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]eventJSON, 0, len(events))
	for _, e := range events {
		out = append(out, eventToJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": out})
}

type eventInput struct {
	Type        string  `json:"type"`
	Description *string `json:"description"`
	OccurredAt  string  `json:"occurred_at"`
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	o, ok := s.offerForRequest(w, r)
	if !ok {
		return
	}
	var in eventInput
	if !decodeJSON(w, r, &in) {
		return
	}
	typ := model.EventType(strings.TrimSpace(in.Type))
	if !typ.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid event type %q", typ))
		return
	}
	occurred := s.now()
	if strings.TrimSpace(in.OccurredAt) != "" {
		t, err := parseTimeInput(in.OccurredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		occurred = t
	}
	var desc *string
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if d != "" {
			desc = &d
		}
	}
	e, err := db.InsertEvent(r.Context(), s.DB, model.Event{
		OfferID:     o.ID,
		Type:        typ,
		Description: desc,
		OccurredAt:  occurred,
	}, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"event": eventToJSON(e)})
}

type eventPatch struct {
	Type        *string         `json:"type"`
	Description json.RawMessage `json:"description"`
	OccurredAt  *string         `json:"occurred_at"`
}

func (s *Server) patchEvent(w http.ResponseWriter, r *http.Request) {
	o, ok := s.offerForRequest(w, r)
	if !ok {
		return
	}
	e, ok := s.eventForRequest(w, r, o.ID)
	if !ok {
		return
	}
	var in eventPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.EventUpdate
	if in.Type != nil {
		typ := model.EventType(strings.TrimSpace(*in.Type))
		if !typ.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid event type %q", typ))
			return
		}
		u.Type = &typ
	}
	if len(in.Description) > 0 {
		var desc string
		if err := json.Unmarshal(in.Description, &desc); err != nil {
			if string(in.Description) != "null" {
				writeError(w, http.StatusBadRequest, "validation_failed", "description must be a string or null")
				return
			}
			desc = ""
		}
		desc = strings.TrimSpace(desc)
		u.Description = &desc
	}
	if in.OccurredAt != nil {
		t, err := parseTimeInput(*in.OccurredAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		u.OccurredAt = &t
	}
	if err := db.UpdateEvent(r.Context(), s.DB, e.ID, u); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	e, err := db.GetEvent(r.Context(), s.DB, e.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"event": eventToJSON(e)})
}

func (s *Server) deleteEvent(w http.ResponseWriter, r *http.Request) {
	o, ok := s.offerForRequest(w, r)
	if !ok {
		return
	}
	e, ok := s.eventForRequest(w, r, o.ID)
	if !ok {
		return
	}
	if err := db.DeleteEvent(r.Context(), s.DB, e.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
