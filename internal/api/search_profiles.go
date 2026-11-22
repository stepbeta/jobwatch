package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
)

type searchProfileTermJSON struct {
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
	Affinity  string `json:"affinity"`
	Bucket    string `json:"bucket"`
	Term      string `json:"term"`
	Weight    int    `json:"weight"`
	SortOrder int    `json:"sort_order"`
}

type searchProfileRemoteJSON struct {
	Policy   string `json:"policy"`
	Accepted bool   `json:"accepted"`
}

type searchProfileJSON struct {
	ID            int64                     `json:"id"`
	Name          string                    `json:"name"`
	TitleLabel    string                    `json:"title_label"`
	LocationLabel string                    `json:"location_label"`
	BucketScale   string                    `json:"bucket_scale"`
	IsDefault     bool                      `json:"is_default"`
	Terms         []searchProfileTermJSON   `json:"terms"`
	Remote        []searchProfileRemoteJSON `json:"remote"`
	CreatedAt     *string                   `json:"created_at"`
	UpdatedAt     *string                   `json:"updated_at"`
}

func searchProfileToJSON(p model.SearchProfile) searchProfileJSON {
	out := searchProfileJSON{
		ID:            p.ID,
		Name:          p.Name,
		TitleLabel:    p.TitleLabel,
		LocationLabel: p.LocationLabel,
		BucketScale:   string(p.BucketScale),
		IsDefault:     p.IsDefault,
		Terms:         make([]searchProfileTermJSON, 0, len(p.Terms)),
		Remote:        make([]searchProfileRemoteJSON, 0, len(p.Remote)),
		CreatedAt:     timeString(p.CreatedAt),
		UpdatedAt:     timeString(p.UpdatedAt),
	}
	for _, t := range p.Terms {
		out.Terms = append(out.Terms, searchProfileTermJSON{
			ID: t.ID, ProfileID: t.ProfileID, Affinity: string(t.Affinity),
			Bucket: string(t.Bucket), Term: t.Term, Weight: t.Weight, SortOrder: t.SortOrder,
		})
	}
	for _, r := range p.Remote {
		out.Remote = append(out.Remote, searchProfileRemoteJSON{Policy: string(r.Policy), Accepted: r.Accepted})
	}
	return out
}

// searchProfileInput accepts a full profile object, so a client can send
// back what it read. id, profile_id, is_default and timestamps are
// ignored: the path id and the stored row own them.
type searchProfileInput struct {
	ID            int64                     `json:"id"`
	Name          string                    `json:"name"`
	TitleLabel    string                    `json:"title_label"`
	LocationLabel string                    `json:"location_label"`
	BucketScale   string                    `json:"bucket_scale"`
	IsDefault     *bool                     `json:"is_default"`
	Terms         []searchProfileTermInput  `json:"terms"`
	Remote        []searchProfileRemoteJSON `json:"remote"`
	CreatedAt     *string                   `json:"created_at"`
	UpdatedAt     *string                   `json:"updated_at"`
}

type searchProfileTermInput struct {
	ID        int64  `json:"id"`
	ProfileID int64  `json:"profile_id"`
	Affinity  string `json:"affinity"`
	Bucket    string `json:"bucket"`
	Term      string `json:"term"`
	Weight    int    `json:"weight"`
	SortOrder int    `json:"sort_order"`
}

func (in searchProfileInput) profile(id int64) model.SearchProfile {
	p := model.SearchProfile{
		ID:            id,
		Name:          in.Name,
		TitleLabel:    in.TitleLabel,
		LocationLabel: in.LocationLabel,
		BucketScale:   model.BucketScale(in.BucketScale),
		Terms:         make([]model.SearchProfileTerm, 0, len(in.Terms)),
		Remote:        make([]model.SearchProfileRemote, 0, len(in.Remote)),
	}
	for _, t := range in.Terms {
		p.Terms = append(p.Terms, model.SearchProfileTerm{
			Affinity:  model.TermAffinity(t.Affinity),
			Bucket:    model.TermBucket(t.Bucket),
			Term:      t.Term,
			Weight:    t.Weight,
			SortOrder: t.SortOrder,
		})
	}
	for _, r := range in.Remote {
		p.Remote = append(p.Remote, model.SearchProfileRemote{
			Policy:   model.RemotePolicy(r.Policy),
			Accepted: r.Accepted,
		})
	}
	return p
}

func (s *Server) listSearchProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := db.ListSearchProfiles(r.Context(), s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]searchProfileJSON, 0, len(profiles))
	for _, p := range profiles {
		out = append(out, searchProfileToJSON(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": out})
}

func (s *Server) getSearchProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "profile id must be a positive integer")
		return
	}
	p, err := db.GetSearchProfile(r.Context(), s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("search profile %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": searchProfileToJSON(p)})
}

// replaceSearchProfile replaces name, labels, bucket scale, terms and
// remote selection in one transaction. Validation failures leave the
// stored profile untouched.
func (s *Server) replaceSearchProfile(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "profile id must be a positive integer")
		return
	}
	var in searchProfileInput
	if !decodeJSON(w, r, &in) {
		return
	}
	p, err := db.UpdateSearchProfile(r.Context(), s.DB, in.profile(id), s.now())
	switch {
	case errors.Is(err, db.ErrInvalidProfile):
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("search profile %d not found", id))
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": searchProfileToJSON(p)})
}
