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
	defaultCompanyLimit = 100
	maxCompanyLimit     = 500
)

func (s *Server) listCompanies(w http.ResponseWriter, r *http.Request) {
	f := db.CompanyFilter{Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	f.Limit = defaultCompanyLimit
	if v, err := queryInt(r, "limit", 1, maxCompanyLimit); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	} else if v != nil {
		f.Limit = *v
	}
	if v, err := queryInt(r, "offset", 0, 1_000_000); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	} else if v != nil {
		f.Offset = *v
	}
	list, total, err := db.ListCompanies(r.Context(), s.DB, f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]companyJSON, 0, len(list))
	for _, c := range list {
		out = append(out, companyItemToJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"companies": out,
		"total":     total,
		"limit":     f.Limit,
		"offset":    f.Offset,
	})
}

type companyInput struct {
	Name    string  `json:"name"`
	Website *string `json:"website"`
}

func (s *Server) createCompany(w http.ResponseWriter, r *http.Request) {
	var in companyInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	var website *string
	if in.Website != nil {
		if v := strings.TrimSpace(*in.Website); v != "" {
			website = &v
		}
	}
	c, err := db.CreateCompany(r.Context(), s.DB, in.Name, website, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"company": companyToJSON(c)})
}

func (s *Server) getCompany(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "company id must be a positive integer")
		return
	}
	ctx := r.Context()
	c, err := db.GetCompany(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("company %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	offers, err := db.QueryOffers(ctx, s.DB, db.OfferQuery{
		CompanyID: &c.ID,
		Sort:      db.SortLastSeen,
		Limit:     maxOfferLimit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	notes, err := db.ListNotes(ctx, s.DB, model.EntityCompany, c.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	candidates, err := db.SimilarCompanies(ctx, s.DB, c)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	offerOut := make([]offerJSON, 0, len(offers.Offers))
	for _, o := range offers.Offers {
		offerOut = append(offerOut, offerToJSON(o))
	}
	noteOut := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		noteOut = append(noteOut, noteToJSON(n))
	}
	candOut := make([]companyCandidateJSON, 0, len(candidates))
	for _, cand := range candidates {
		candOut = append(candOut, companyCandidateJSON{
			ID: cand.ID, Name: cand.Name, NameNormalized: cand.NameNormalized,
			Score: cand.Score, Reason: cand.Reason,
		})
	}
	out := companyToJSON(c)
	out.OfferCount = offers.Total
	out.NoteCount = len(notes)
	writeJSON(w, http.StatusOK, map[string]any{
		"company":             out,
		"offers":              offerOut,
		"notes":               noteOut,
		"possible_duplicates": candOut,
	})
}

type companyPatch struct {
	Name    *string         `json:"name"`
	Website json.RawMessage `json:"website"`
}

func (s *Server) patchCompany(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "company id must be a positive integer")
		return
	}
	var in companyPatch
	if !decodeJSON(w, r, &in) {
		return
	}
	var u db.CompanyUpdate
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "name cannot be empty")
			return
		}
		u.Name = &name
	}
	if len(in.Website) > 0 {
		var website string
		if err := json.Unmarshal(in.Website, &website); err != nil {
			if string(in.Website) != "null" {
				writeError(w, http.StatusBadRequest, "validation_failed", "website must be a string or null")
				return
			}
			website = ""
		}
		website = strings.TrimSpace(website)
		u.Website = &website
	}
	if u.Name == nil && u.Website == nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "nothing to update: provide name or website")
		return
	}
	c, err := db.UpdateCompany(r.Context(), s.DB, id, u, s.now())
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("company %d not found", id))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := companyToJSON(c)
	if out.OfferCount, out.NoteCount, err = db.CompanyCounts(r.Context(), s.DB, c.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"company": out})
}

func (s *Server) deleteCompany(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "company id must be a positive integer")
		return
	}
	err := db.DeleteCompany(r.Context(), s.DB, id)
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("company %d not found", id))
	case errors.Is(err, db.ErrCompanyHasOffers):
		writeError(w, http.StatusConflict, "company_has_offers",
			"this company still has offers; merge it into another company instead of deleting it")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

type mergeInput struct {
	IntoID int64 `json:"into_id"`
}

func (s *Server) mergeCompany(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "company id must be a positive integer")
		return
	}
	var in mergeInput
	if !decodeJSON(w, r, &in) {
		return
	}
	if in.IntoID <= 0 {
		writeError(w, http.StatusBadRequest, "validation_failed", "into_id must be a positive integer")
		return
	}
	res, err := db.MergeCompanies(r.Context(), s.DB, id, in.IntoID, s.now())
	switch {
	case errors.Is(err, db.ErrMergeSelf):
		writeError(w, http.StatusBadRequest, "validation_failed", "a company cannot be merged into itself")
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "one of the companies does not exist")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	companyOut := companyToJSON(res.Company)
	if companyOut.OfferCount, companyOut.NoteCount, err = db.CompanyCounts(r.Context(), s.DB, res.Company.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"company":           companyOut,
		"merged_company_id": id,
		"moved_offers":      res.MovedOffers,
		"moved_notes":       res.MovedNotes,
	})
}
