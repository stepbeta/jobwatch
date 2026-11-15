// Package api holds the REST handlers served under /api.
package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// Server serves the REST API.
type Server struct {
	DB      *sql.DB
	Version string
}

// Handler returns the HTTP handler for the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such endpoint: "+r.Method+" "+r.URL.Path)
	})
	return mux
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

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, code int, errCode, msg string) {
	var b errorBody
	b.Error.Code, b.Error.Message = errCode, msg
	writeJSON(w, code, b)
}
