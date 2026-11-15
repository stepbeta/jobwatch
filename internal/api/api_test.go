package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stepbeta/jobwatch/internal/db"
)

func TestHandler(t *testing.T) {
	sqlDB, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	h := (&Server{DB: sqlDB, Version: "test"}).Handler()

	tests := []struct {
		method, path string
		wantCode     int
		wantKey      string // top-level JSON key expected in the body
	}{
		{"GET", "/api/health", 200, "status"},
		{"POST", "/api/health", 404, "error"},
		{"GET", "/api/nope", 404, "error"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			if tt.wantKey == "" {
				return
			}
			var body map[string]json.RawMessage
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if _, ok := body[tt.wantKey]; !ok {
				t.Errorf("body %s lacks %q", rec.Body.String(), tt.wantKey)
			}
		})
	}
}
