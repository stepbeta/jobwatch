package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/scan"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// normalizeConfig validates a config object and returns its storage
// form. nil means an empty config.
func normalizeConfig(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return "", fmt.Errorf("config must be a JSON object")
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return "", fmt.Errorf("config must be a JSON object")
	}
	return string(b), nil
}

func validateSourceURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("url is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("url must be an http(s) URL")
	}
	return nil
}

type sourceInput struct {
	Name       string          `json:"name"`
	URL        string          `json:"url"`
	SourceType string          `json:"source_type"`
	Config     json.RawMessage `json:"config"`
	Enabled    *bool           `json:"enabled"`
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	list, err := db.ListSources(r.Context(), s.DB, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	out := make([]sourceJSON, 0, len(list))
	for _, src := range list {
		out = append(out, sourceToJSON(src))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sources": out})
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var in sourceInput
	if !decodeJSON(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "name is required")
		return
	}
	typ := model.SourceType(strings.TrimSpace(in.SourceType))
	if typ == "" {
		detected, _, ok := sources.Detect(in.URL)
		if !ok {
			writeError(w, http.StatusBadRequest, "validation_failed",
				"cannot detect source_type from url; provide source_type explicitly")
			return
		}
		typ = detected
	}
	if !typ.Valid() {
		writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid source_type %q", typ))
		return
	}
	if typ != model.SourceManual {
		if err := validateSourceURL(in.URL); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	}
	cfg, err := normalizeConfig(in.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	ctx := r.Context()
	if existing, err := db.FindSourceByURL(ctx, s.DB, in.URL); err == nil {
		writeError(w, http.StatusConflict, "duplicate_source",
			fmt.Sprintf("a source with this url already exists (id %d)", existing.ID))
		return
	} else if !errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	src, err := db.InsertSource(ctx, s.DB, model.Source{
		Name:       in.Name,
		URL:        strings.TrimSpace(in.URL),
		Type:       typ,
		ConfigJSON: cfg,
		Enabled:    enabled,
	}, s.now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"source": sourceToJSON(src)})
}

func (s *Server) getSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a positive integer")
		return
	}
	src, err := db.GetSource(r.Context(), s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("source %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": sourceToJSON(src)})
}

type sourcePatch struct {
	Name       *string         `json:"name"`
	URL        *string         `json:"url"`
	SourceType *string         `json:"source_type"`
	Config     json.RawMessage `json:"config"`
	Enabled    *bool           `json:"enabled"`
}

func (s *Server) patchSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a positive integer")
		return
	}
	var in sourcePatch
	if !decodeJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	src, err := db.GetSource(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("source %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "name cannot be empty")
			return
		}
		src.Name = name
	}
	if in.URL != nil {
		src.URL = strings.TrimSpace(*in.URL)
	}
	if in.SourceType != nil {
		typ := model.SourceType(strings.TrimSpace(*in.SourceType))
		if !typ.Valid() {
			writeError(w, http.StatusBadRequest, "validation_failed", fmt.Sprintf("invalid source_type %q", typ))
			return
		}
		src.Type = typ
	}
	if src.Type != model.SourceManual {
		if err := validateSourceURL(src.URL); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	}
	if len(in.Config) > 0 {
		cfg, err := normalizeConfig(in.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		src.ConfigJSON = cfg
	}
	if in.Enabled != nil {
		src.Enabled = *in.Enabled
	}
	if existing, err := db.FindSourceByURL(ctx, s.DB, src.URL); err == nil && existing.ID != src.ID {
		writeError(w, http.StatusConflict, "duplicate_source",
			fmt.Sprintf("a source with this url already exists (id %d)", existing.ID))
		return
	} else if err != nil && !errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := db.UpdateSource(ctx, s.DB, src); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	src, err = db.GetSource(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"source": sourceToJSON(src)})
}

func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a positive integer")
		return
	}
	ctx := r.Context()
	if _, err := db.GetSource(ctx, s.DB, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("source %d not found", id))
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := db.DeleteSource(ctx, s.DB, id); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type scanResultJSON struct {
	SourceID    int64  `json:"source_id"`
	SourceName  string `json:"source_name"`
	Status      string `json:"status"`
	Fetched     int    `json:"fetched"`
	Skipped     int    `json:"skipped"`
	New         int    `json:"new"`
	Seen        int    `json:"seen"`
	Reappeared  int    `json:"reappeared"`
	Disappeared int    `json:"disappeared"`
	Incomplete  int    `json:"incomplete"`
	Error       string `json:"error,omitempty"`
}

func (s *Server) scanSource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_id", "source id must be a positive integer")
		return
	}
	ctx := r.Context()
	src, err := db.GetSource(ctx, s.DB, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("source %d not found", id))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	sc := &scan.Scanner{DB: s.DB, Fetcher: s.fetcher(), Now: s.Now}
	res := sc.ScanSource(ctx, src)
	out := scanResultJSON{
		SourceID:    res.SourceID,
		SourceName:  res.SourceName,
		Status:      string(res.Status),
		Fetched:     res.Fetched,
		Skipped:     res.Skipped,
		New:         res.New,
		Seen:        res.Seen,
		Reappeared:  res.Reappeared,
		Disappeared: res.Disappeared,
		Incomplete:  res.Incomplete,
	}
	if out.Status == "" {
		out.Status = "skipped"
	}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}
	updated, err := db.GetSource(ctx, s.DB, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": out, "source": sourceToJSON(updated)})
}
