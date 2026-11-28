package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

// doRaw sends a request through the policy-wrapped handler with explicit
// Host, Origin and Content-Type.
func doRaw(ts *testServer, method, path, host, origin, contentType string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	ts.handler.ServeHTTP(rec, r)
	return rec
}

func noteCount(t *testing.T, ts *testServer) int {
	t.Helper()
	var n int
	if err := ts.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM notes`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRequestPolicyHost(t *testing.T) {
	ts := newTestServer(t)
	seedOfferSet(t, ts)

	tests := []struct {
		name string
		host string
		want int
	}{
		{"loopback IP", "127.0.0.1:8080", http.StatusOK},
		{"loopback IPv6", "[::1]:8080", http.StatusOK},
		{"localhost with development port", "localhost:5173", http.StatusOK},
		{"rebound host", "attacker.example", http.StatusForbidden},
		{"public IP", "203.0.113.7:8080", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := doRaw(ts, http.MethodGet, "/api/health", tt.host, "", "", nil)
			if rec.Code != tt.want {
				t.Fatalf("code = %d, want %d (body %s)", rec.Code, tt.want, rec.Body.String())
			}
			if tt.want == http.StatusForbidden {
				var body errorBody
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if body.Error.Code != "forbidden_host" {
					t.Errorf("error code = %q, want forbidden_host", body.Error.Code)
				}
			}
		})
	}

	// An explicitly bound non-loopback address is added to the allowlist.
	ts.Server.BindAddr = "192.168.1.20"
	if rec := doRaw(ts, http.MethodGet, "/api/health", "192.168.1.20:8080", "", "", nil); rec.Code != http.StatusOK {
		t.Errorf("bound address code = %d, want 200", rec.Code)
	}
	if rec := doRaw(ts, http.MethodGet, "/api/health", "198.51.100.5:8080", "", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("other address code = %d, want 403", rec.Code)
	}
}

func TestRequestPolicyOrigin(t *testing.T) {
	ts := newTestServer(t)
	sre, _ := seedOfferSet(t, ts)
	body, err := json.Marshal(map[string]any{
		"entity_type": "offer", "entity_id": sre.ID, "body": "policy test",
	})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:8080"

	rec := doRaw(ts, http.MethodPost, "/api/notes", host, "https://example.com", "application/json", body)
	wantErrorCode(t, rec, http.StatusForbidden, "forbidden_origin")
	if n := noteCount(t, ts); n != 0 {
		t.Errorf("notes = %d, want 0 after a rejected write", n)
	}

	rec = doRaw(ts, http.MethodPost, "/api/notes", host, "", "application/json", body)
	wantCode(t, rec, http.StatusCreated)

	rec = doRaw(ts, http.MethodPost, "/api/notes", host, "http://localhost:5173", "application/json", body)
	wantCode(t, rec, http.StatusCreated)

	// A foreign Origin on a safe request is not a write attempt.
	if rec := doRaw(ts, http.MethodGet, "/api/health", host, "https://example.com", "", nil); rec.Code != http.StatusOK {
		t.Errorf("GET with foreign origin code = %d, want 200", rec.Code)
	}

	// An explicitly bound address is allowed as an origin too.
	ts.Server.BindAddr = "192.168.1.20"
	rec = doRaw(ts, http.MethodPost, "/api/notes", "192.168.1.20:8080", "http://192.168.1.20:8080", "application/json", body)
	wantCode(t, rec, http.StatusCreated)
}

func TestDecodeJSONContentType(t *testing.T) {
	ts := newTestServer(t)
	sre, _ := seedOfferSet(t, ts)
	body, err := json.Marshal(map[string]any{
		"entity_type": "offer", "entity_id": sre.ID, "body": "content type test",
	})
	if err != nil {
		t.Fatal(err)
	}
	const host = "127.0.0.1:8080"

	rec := doRaw(ts, http.MethodPost, "/api/notes", host, "", "text/plain", body)
	wantErrorCode(t, rec, http.StatusUnsupportedMediaType, "unsupported_media_type")
	if n := noteCount(t, ts); n != 0 {
		t.Errorf("notes = %d, want 0 after a rejected body", n)
	}

	rec = doRaw(ts, http.MethodPost, "/api/notes", host, "", "application/json; charset=utf-8", body)
	wantCode(t, rec, http.StatusCreated)

	// A bodyless write needs no Content-Type.
	rec = doRaw(ts, http.MethodPost, "/api/sources", host, "", "application/json",
		[]byte(`{"name":"Acme","url":"https://boards.greenhouse.io/acme"}`))
	wantCode(t, rec, http.StatusCreated)
	src := decodeBody[sourceResp](t, rec).Source
	ts.fetcher.offers[src.ID] = nil
	rec = doRaw(ts, http.MethodPost, "/api/sources/"+fmt.Sprintf("%d", src.ID)+"/scan", host, "", "", nil)
	wantCode(t, rec, http.StatusOK)
	if got := decodeBody[scanResp](t, rec).Result.Status; got != string(model.ScanOK) {
		t.Errorf("bodyless scan status = %q, want ok", got)
	}
}
