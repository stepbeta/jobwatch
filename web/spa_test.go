package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func testFS() fs.FS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html>index</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
}

func TestSPAHandler(t *testing.T) {
	h := spaHandler(testFS())
	tests := []struct {
		name      string
		method    string
		path      string
		wantCode  int
		wantBody  string
		wantCache string
	}{
		{"index", "GET", "/", 200, "<html>index</html>", "no-cache"},
		{"asset", "GET", "/assets/app.js", 200, "console.log(1)", "public, max-age=31536000, immutable"},
		{"client route falls back to index", "GET", "/offers/1", 200, "<html>index</html>", "no-cache"},
		{"unknown file falls back to index", "GET", "/missing.txt", 200, "<html>index</html>", "no-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantCode {
				t.Fatalf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("cache-control = %q, want %q", got, tt.wantCache)
			}
		})
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST / code = %d, want 405", rec.Code)
	}
}
