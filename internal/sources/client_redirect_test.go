package sources

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func TestClientFollowsRobotsRedirect(t *testing.T) {
	var blockedHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.Redirect(w, r, "/canonical/robots.txt", http.StatusMovedPermanently)
		case "/canonical/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /blocked\n"))
		case "/ok":
			w.Write([]byte(`{"ok":true}`))
		case "/blocked":
			blockedHits++
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := testClient()
	var v struct{ OK bool }
	if err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/ok", &v); err != nil {
		t.Fatalf("GetJSON /ok: %v", err)
	}
	if !v.OK {
		t.Error("body not decoded after the robots redirect")
	}

	err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/blocked", &v)
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked from the redirected robots.txt", err)
	}
	if blockedHits != 0 {
		t.Errorf("disallowed path was contacted %d time(s)", blockedHits)
	}
}

func TestClientRejectsRobotsRedirectLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.Redirect(w, r, "/other-robots.txt", http.StatusFound)
		case "/other-robots.txt":
			http.Redirect(w, r, "/robots.txt", http.StatusFound)
		default:
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	c, _ := testClient()
	err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/ok", &struct{}{})
	if err == nil || !strings.Contains(err.Error(), "robots.txt redirect loop") {
		t.Fatalf("err = %v, want a robots redirect loop error", err)
	}
}

func TestClientFollowsGetRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case r.URL.Path == "/start":
			http.Redirect(w, r, "/final", http.StatusFound)
		case r.URL.Path == "/final":
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := testClient()
	var v struct{ OK bool }
	if err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/start", &v); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if !v.OK {
		t.Error("redirect target body not decoded")
	}
}

func TestClientRejectsBadRedirects(t *testing.T) {
	tests := []struct {
		name    string
		handler func(w http.ResponseWriter, r *http.Request)
		want    string
	}{
		{
			name: "too many hops",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.Write([]byte("User-agent: *\nAllow: /\n"))
					return
				}
				if r.URL.Path == "/start" {
					http.Redirect(w, r, "/r1", http.StatusFound)
					return
				}
				var n int
				if _, err := fmt.Sscanf(r.URL.Path, "/r%d", &n); err == nil && n < 6 {
					http.Redirect(w, r, "/r"+strconv.Itoa(n+1), http.StatusFound)
					return
				}
				w.Write([]byte("ok"))
			},
			want: "more than 5 hops",
		},
		{
			name: "redirect loop",
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/robots.txt":
					w.Write([]byte("User-agent: *\nAllow: /\n"))
				case "/start":
					http.Redirect(w, r, "/a", http.StatusFound)
				case "/a":
					http.Redirect(w, r, "/b", http.StatusFound)
				case "/b":
					http.Redirect(w, r, "/a", http.StatusFound)
				}
			},
			want: "redirect loop",
		},
		{
			name: "non-http scheme",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.Write([]byte("User-agent: *\nAllow: /\n"))
					return
				}
				w.Header().Set("Location", "ftp://example.com/file")
				w.WriteHeader(http.StatusFound)
			},
			want: "non-HTTP scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer srv.Close()
			c, _ := testClient()
			err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/start", &struct{}{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), srv.URL+"/start") {
				t.Errorf("err = %v, want it to name the original URL", err)
			}
		})
	}
}

func TestClientRedirectRechecksRobots(t *testing.T) {
	var privateHits int
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/private":
			mu.Lock()
			privateHits++
			mu.Unlock()
			w.Write([]byte("secret"))
		default:
			w.Write([]byte("ok"))
		}
	}))
	defer target.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nAllow: /\n"))
			return
		}
		http.Redirect(w, r, target.URL+"/private", http.StatusFound)
	}))
	defer origin.Close()

	c, _ := testClient()
	err := c.GetJSON(context.Background(), model.Source{ID: 1}, origin.URL+"/start", &struct{}{})
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if privateHits != 0 {
		t.Errorf("disallowed redirect target was contacted %d time(s)", privateHits)
	}
}

func TestClientRedirectRateLimitsSecondHost(t *testing.T) {
	type hit struct {
		host string
		path string
		at   time.Time
	}
	var mu sync.Mutex
	var hits []hit
	fc := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}

	record := func(host string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			hits = append(hits, hit{host: host, path: r.URL.Path, at: fc.now()})
			mu.Unlock()
			switch r.URL.Path {
			case "/robots.txt":
				w.Write([]byte("User-agent: *\nAllow: /\n"))
			case "/final", "/warm":
				w.Write([]byte("{}"))
			}
		}
	}

	target := httptest.NewServer(record("target"))
	defer target.Close()
	targetURL := target.URL

	origin := httptest.NewServer(func() http.HandlerFunc {
		base := record("origin")
		return func(w http.ResponseWriter, r *http.Request) {
			base(w, r)
			if r.URL.Path != "/robots.txt" {
				http.Redirect(w, r, targetURL+"/final", http.StatusFound)
			}
		}
	}())
	defer origin.Close()

	c := NewClient()
	c.now = fc.now
	c.sleep = fc.sleep
	c.DefaultInterval = 2 * time.Second

	// Simulate a recent request to the target host so the redirect has to
	// wait for that host's reserved slot.
	targetParsed, err := url.Parse(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	c.nextReq[targetParsed.Host] = fc.now().Add(2 * time.Second)
	c.mu.Unlock()

	if err := c.GetJSON(context.Background(), model.Source{ID: 1}, origin.URL+"/start", &struct{}{}); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	var lastOrigin, targetFinal time.Time
	for _, h := range hits {
		if h.host == "origin" {
			lastOrigin = h.at
		}
		if h.host == "target" && h.path == "/final" {
			targetFinal = h.at
		}
	}
	if lastOrigin.IsZero() || targetFinal.IsZero() {
		t.Fatalf("hits = %+v, want an origin request and the target final", hits)
	}
	if targetFinal.Sub(lastOrigin) < 2*time.Second {
		t.Errorf("redirect target came %v after the last origin request, want >= 2s",
			targetFinal.Sub(lastOrigin))
	}
}

func TestClientPostRedirectIsAnHTTPError(t *testing.T) {
	var finalHits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/post":
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
		case "/final":
			mu.Lock()
			finalHits++
			mu.Unlock()
			w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()

	c, _ := testClient()
	var v struct{ OK bool }
	err := c.PostJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/post", "application/json", nil, []byte(`{}`), &v)
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("err = %v, want an HTTPError", err)
	}
	if httpErr.Status != http.StatusTemporaryRedirect {
		t.Errorf("status = %d, want %d", httpErr.Status, http.StatusTemporaryRedirect)
	}
	mu.Lock()
	defer mu.Unlock()
	if finalHits != 0 {
		t.Errorf("POST redirect target was replayed %d time(s)", finalHits)
	}
}
