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

// validateEntity checks that a note's target exists.
func (s *Server) validateEntity(r *http.Request, typ model.EntityType, id int64) error {
	switch typ {
	case model.EntityOffer:
		_, err := db.GetOffer(r.Context(), s.DB, id)
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("offer %d not found", id)
		}
		return err
	case model.EntityCompany:
		_, err := db.GetCompany(r.Context(), s.DB, id)
		if errors.Is(err, db.ErrNotFound) {
			return fmt.Errorf("company %d not found", id)
		}
		return err
	default:
		return fmt.Errorf("invalid entity_type %q", typ)
	}
}

// validatePinnedEvent checks that an event exists and belongs to the
// note's entity (an offer, or any offer of a company).
func (s *Server) validatePinnedEvent(r *http.Request, typ model.EntityType, entityID, eventID int64) error {
	ctx := r.Context()
	e, err := db.GetEvent(ctx, s.DB, eventID)
	if errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("event %d not found", eventID)
	}
	if err != nil {
		return err
	}
	if typ == model.EntityOffer {
		if e.OfferID != entityID {
			return fmt.Errorf("event %d does not belong to offer %d", eventID, entityID)
		}
		return nil
	}
	o, err := db.GetOffer(ctx, s.DB, e.OfferID)
	if err != nil {
		return err
	}
	if o.CompanyID == nil || *o.CompanyID != entityID {
		return fmt.Errorf("event %d does not belong to an offer of company %d", eventID, entityID)
	}
	return nil
}

func (s *Server) listNotes(w http.ResponseWriter, r *http.Request) {
	typ := model.EntityType(strings.TrimSpace(r.URL.Query().Get("entity_type")))
	id, err := queryInt64(r, "entity_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	if typ == "" || id == nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "entity_type and entity_id are required")
		return
	}
	if !typ.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid entity_type %q", typ))
		return
	}
	if err := s.validateEntity(r, typ, *id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	notes, err := db.ListNotes(r.Context(), s.DB, typ, *id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		out = append(out, noteToJSON(n))
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": out})
}

type noteInput struct {
	EntityType    string `json:"entity_type"`
	EntityID      int64  `json:"entity_id"`
	Body          string `json:"body"`
	PinnedEventID *int64 `json:"pinned_event_id"`
}

func (s *Server) createNote(w http.ResponseWriter, r *http.Request) {
	var in noteInput
	if !decodeJSON(w, r, &in) {
		return
	}
	typ := model.EntityType(strings.TrimSpace(in.EntityType))
	if !typ.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid entity_type %q", typ))
		return
	}
	if strings.TrimSpace(in.Body) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "body is required")
		return
	}
	if in.EntityID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "entity_id must be a positive integer")
		return
	}
	if err := s.validateEntity(r, typ, in.EntityID); err != nil {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	if in.PinnedEventID != nil {
		if err := s.validatePinnedEvent(r, typ, in.EntityID, *in.PinnedEventID); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	}
	n, err := db.InsertNote(r.Context(), s.DB, model.Note{
		EntityType:    typ,
		EntityID:      in.EntityID,
		Body:          in.Body,
		PinnedEventID: in.PinnedEventID,
	}, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"note": noteToJSON(n)})
}

func (s *Server) getNote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "note id must be a positive integer")
		return
	}
	n, err := db.GetNote(r.Context(), s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("note %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": noteToJSON(n)})
}

type notePatch struct {
	Body          *string         `json:"body"`
	PinnedEventID json.RawMessage `json:"pinned_event_id"`
}

func (s *Server) patchNote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "note id must be a positive integer")
		return
	}
	ctx := r.Context()
	n, err := db.GetNote(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("note %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var in notePatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.NoteUpdate
	if in.Body != nil {
		u.Body = in.Body
	}
	if len(in.PinnedEventID) > 0 {
		if string(in.PinnedEventID) == "null" {
			u.ClearPinned = true
		} else {
			var eventID int64
			if err := json.Unmarshal(in.PinnedEventID, &eventID); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "pinned_event_id must be an integer or null")
				return
			}
			if err := s.validatePinnedEvent(r, n.EntityType, n.EntityID, eventID); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			u.PinnedEventID = &eventID
		}
	}
	if u.Body == nil && u.PinnedEventID == nil && !u.ClearPinned {
		writeError(w, http.StatusBadRequest, "validation_failed", "nothing to update: provide body or pinned_event_id")
		return
	}
	if err := db.UpdateNote(ctx, s.DB, id, u, s.now()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	n, err = db.GetNote(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"note": noteToJSON(n)})
}

func (s *Server) deleteNote(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "note id must be a positive integer")
		return
	}
	if err := db.DeleteNote(r.Context(), s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("note %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
