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

// interviewEntryInput is the POST body. question is required; answer may
// be empty at creation. answer_format is enum-like and validated in Go.
// category is optional; omitted or null stores NULL (uncategorized).
type interviewEntryInput struct {
	Question     string  `json:"question"`
	Answer       string  `json:"answer"`
	AnswerFormat string  `json:"answer_format"`
	Category     *string `json:"category"`
	DisplayOrder *int    `json:"display_order"`
}

// interviewEntryPatch is the PATCH body. answer_format is intentionally
// absent: changing the stored format is not part of this milestone.
// category is a raw message so an absent field, null and a string are
// distinguishable.
type interviewEntryPatch struct {
	Question     *string         `json:"question"`
	Answer       *string         `json:"answer"`
	Category     json.RawMessage `json:"category"`
	DisplayOrder json.RawMessage `json:"display_order"`
}

func (s *Server) listInterviewEntries(w http.ResponseWriter, r *http.Request) {
	entries, err := db.ListInterviewEntries(r.Context(), s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]interviewEntryJSON, 0, len(entries))
	for _, e := range entries {
		out = append(out, interviewEntryToJSON(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"interview_entries": out})
}

func (s *Server) createInterviewEntry(w http.ResponseWriter, r *http.Request) {
	var in interviewEntryInput
	if !decodeJSON(w, r, &in) {
		return
	}
	question := strings.TrimSpace(in.Question)
	if question == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "question is required")
		return
	}
	format := model.AnswerFormat(strings.TrimSpace(in.AnswerFormat))
	if format == "" {
		format = model.AnswerFormatMarkdown
	}
	if !format.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid answer_format %q", format))
		return
	}
	var category *string
	if in.Category != nil {
		trimmed := strings.TrimSpace(*in.Category)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "category must not be blank")
			return
		}
		category = &trimmed
	}
	e, err := db.InsertInterviewEntry(r.Context(), s.DB, model.InterviewEntry{
		Question:     question,
		Answer:       in.Answer,
		AnswerFormat: format,
		Category:     category,
		DisplayOrder: in.DisplayOrder,
	}, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"interview_entry": interviewEntryToJSON(e)})
}

func (s *Server) getInterviewEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "interview entry id must be a positive integer")
		return
	}
	e, err := db.GetInterviewEntry(r.Context(), s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("interview entry %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interview_entry": interviewEntryToJSON(e)})
}

func (s *Server) patchInterviewEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "interview entry id must be a positive integer")
		return
	}
	ctx := r.Context()
	if _, err := db.GetInterviewEntry(ctx, s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("interview entry %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var in interviewEntryPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.InterviewEntryUpdate
	if in.Question != nil {
		if strings.TrimSpace(*in.Question) == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "question must not be empty")
			return
		}
		u.Question = in.Question
	}
	if in.Answer != nil {
		u.Answer = in.Answer
	}
	if len(in.Category) > 0 {
		if string(in.Category) == "null" {
			u.ClearCategory = true
		} else {
			var category string
			if err := json.Unmarshal(in.Category, &category); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "category must be a string or null")
				return
			}
			category = strings.TrimSpace(category)
			if category == "" {
				writeError(w, http.StatusBadRequest, "validation_failed", "category must not be blank")
				return
			}
			u.Category = &category
		}
	}
	if len(in.DisplayOrder) > 0 {
		if string(in.DisplayOrder) == "null" {
			u.ClearDisplayOrder = true
		} else {
			var order int
			if err := json.Unmarshal(in.DisplayOrder, &order); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", "display_order must be an integer or null")
				return
			}
			u.DisplayOrder = &order
		}
	}
	if u.Question == nil && u.Answer == nil && u.Category == nil && !u.ClearCategory &&
		u.DisplayOrder == nil && !u.ClearDisplayOrder {
		writeError(w, http.StatusBadRequest, "validation_failed",
			"nothing to update: provide question, answer, category or display_order")
		return
	}
	if err := db.UpdateInterviewEntry(ctx, s.DB, id, u, s.now()); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("interview entry %d not found", id))
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	e, err := db.GetInterviewEntry(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interview_entry": interviewEntryToJSON(e)})
}

func (s *Server) deleteInterviewEntry(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "interview entry id must be a positive integer")
		return
	}
	if err := db.DeleteInterviewEntry(r.Context(), s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("interview entry %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
