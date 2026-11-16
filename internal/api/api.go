// Package api holds the REST handlers served under /api.
package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// Server serves the REST API.
type Server struct {
	DB      *sql.DB
	Version string

	// Fetcher is used by the synchronous scan endpoint. When nil, a
	// registry with one shared polite client is created on first use.
	Fetcher sources.Fetcher
	// HTTPClient is the polite client used for URL metadata extraction
	// during imports. When nil it is created on first use and shared with
	// the default Fetcher.
	HTTPClient *sources.Client
	// Now defaults to time.Now.
	Now func() time.Time

	fetcherOnce    sync.Once
	defaultFetcher sources.Fetcher
	clientOnce     sync.Once
	defaultClient  *sources.Client
}

// Handler returns the HTTP handler for the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)

	mux.HandleFunc("GET /api/sources", s.listSources)
	mux.HandleFunc("POST /api/sources", s.createSource)
	mux.HandleFunc("GET /api/sources/{id}", s.getSource)
	mux.HandleFunc("PATCH /api/sources/{id}", s.patchSource)
	mux.HandleFunc("DELETE /api/sources/{id}", s.deleteSource)
	mux.HandleFunc("POST /api/sources/{id}/scan", s.scanSource)

	mux.HandleFunc("GET /api/offers", s.listOffers)
	mux.HandleFunc("POST /api/offers/import", s.importOffer)
	mux.HandleFunc("GET /api/offers/{id}", s.getOffer)
	mux.HandleFunc("PATCH /api/offers/{id}", s.patchOffer)
	mux.HandleFunc("GET /api/offers/{id}/snapshot-diff", s.snapshotDiff)
	mux.HandleFunc("POST /api/offers/{id}/merge", s.mergeOffer)
	mux.HandleFunc("POST /api/offers/{id}/split", s.splitOffer)
	mux.HandleFunc("GET /api/offers/{id}/events", s.listEvents)
	mux.HandleFunc("POST /api/offers/{id}/events", s.createEvent)
	mux.HandleFunc("PATCH /api/offers/{id}/events/{eventID}", s.patchEvent)
	mux.HandleFunc("DELETE /api/offers/{id}/events/{eventID}", s.deleteEvent)

	mux.HandleFunc("GET /api/notes", s.listNotes)
	mux.HandleFunc("POST /api/notes", s.createNote)
	mux.HandleFunc("GET /api/notes/{id}", s.getNote)
	mux.HandleFunc("PATCH /api/notes/{id}", s.patchNote)
	mux.HandleFunc("DELETE /api/notes/{id}", s.deleteNote)

	mux.HandleFunc("GET /api/companies", s.listCompanies)
	mux.HandleFunc("POST /api/companies", s.createCompany)
	mux.HandleFunc("GET /api/companies/{id}", s.getCompany)
	mux.HandleFunc("PATCH /api/companies/{id}", s.patchCompany)
	mux.HandleFunc("DELETE /api/companies/{id}", s.deleteCompany)
	mux.HandleFunc("POST /api/companies/{id}/merge", s.mergeCompany)

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	return mux
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC().Truncate(time.Second)
	}
	return time.Now().UTC().Truncate(time.Second)
}

func (s *Server) httpClient() *sources.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	s.clientOnce.Do(func() {
		s.defaultClient = sources.NewClient()
	})
	return s.defaultClient
}

func (s *Server) fetcher() sources.Fetcher {
	if s.Fetcher != nil {
		return s.Fetcher
	}
	s.fetcherOnce.Do(func() {
		s.defaultFetcher = sources.NewRegistry(s.httpClient())
	})
	return s.defaultFetcher
}

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	DB      string `json:"db"`
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{Status: "ok", Version: s.Version, DB: "ok"}
	code := http.StatusOK
	if err := s.DB.PingContext(r.Context()); err != nil {
		resp.Status, resp.DB = "degraded", err.Error()
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, resp)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	writeJSON(w, code, errorBody{Error: errorDetail{Code: errCode, Message: msg}})
}

// maxBodyBytes caps JSON request bodies.
const maxBodyBytes = 1 << 20

// decodeJSON decodes a strict JSON object into v. It writes a 400
// response and returns false on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	body := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		msg := "invalid JSON body: " + err.Error()
		if errors.Is(err, io.EOF) {
			msg = "request body is empty"
		}
		writeError(w, http.StatusBadRequest, "invalid_json", msg)
		return false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// queryInt64 parses an optional integer query parameter.
func queryInt64(r *http.Request, name string) (*int64, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer", name)
	}
	return &v, nil
}

// queryInt parses an optional integer query parameter within [lo, hi].
func queryInt(r *http.Request, name string, lo, hi int) (*int, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < lo || v > hi {
		return nil, fmt.Errorf("%s must be an integer between %d and %d", name, lo, hi)
	}
	return &v, nil
}

// queryEnumList parses a comma-separated list of enum values and checks
// each one with valid.
func queryEnumList[T ~string](r *http.Request, name string, valid func(T) bool) ([]T, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	var out []T
	for _, part := range strings.Split(raw, ",") {
		v := T(strings.TrimSpace(part))
		if v == "" {
			continue
		}
		if !valid(v) {
			return nil, fmt.Errorf("invalid %s %q", name, v)
		}
		out = append(out, v)
	}
	return out, nil
}

func queryBool(r *http.Request, name string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name))) {
	case "1", "true", "yes":
		return true
	}
	return false
}

// timeString formats a non-zero time, or returns nil.
func timeString(t time.Time) *string {
	if t.IsZero() {
		return nil
	}
	s := model.FormatTime(t)
	return &s
}

// parseTimeInput accepts RFC3339, "YYYY-MM-DDTHH:MM" (datetime-local) and
// "YYYY-MM-DD".
func parseTimeInput(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("timestamp is required")
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q (use RFC3339)", s)
}
