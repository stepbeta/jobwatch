//go:build !ui

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaceholderHandler(t *testing.T) {
	h := Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "npm run build") {
		t.Errorf("placeholder page lacks build instructions: %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/offers/1", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-root code = %d, want 404", rec.Code)
	}
}
