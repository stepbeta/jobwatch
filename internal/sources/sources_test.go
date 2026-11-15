package sources

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

// fakeClock records requested sleeps and advances virtual time instead
// of sleeping.
type fakeClock struct {
	mu     sync.Mutex
	t      time.Time
	sleeps []time.Duration
}

func (f *fakeClock) now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

func (f *fakeClock) sleep(_ context.Context, d time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sleeps = append(f.sleeps, d)
	if d > 0 {
		f.t = f.t.Add(d)
	}
	return nil
}

func testClient() (*Client, *fakeClock) {
	c := NewClient()
	fc := &fakeClock{t: time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)}
	c.now = fc.now
	c.sleep = fc.sleep
	return c, fc
}

func TestParseRobots(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		path      string
		want      bool
		wantDelay time.Duration
	}{
		{"empty file allows", "", "/anything", true, 0},
		{"star disallow all", "User-agent: *\nDisallow: /", "/v1/boards/x/jobs", false, 0},
		{"star empty disallow allows", "User-agent: *\nDisallow:", "/x", true, 0},
		{"greenhouse real", "# comment\n\nUser-agent: *\nDisallow: /embed/\n", "/v1/boards/acme/jobs?content=true", true, 0},
		{"greenhouse real embed", "User-agent: *\nDisallow: /embed/\n", "/embed/job_board", false, 0},
		{"lever real crawl delay", "User-agent: *\nAllow: /\nCrawl-delay: 1\n", "/v0/postings/acme?mode=json", true, time.Second},
		{"specific group wins over star", "User-agent: *\nDisallow: /\n\nUser-agent: JobWatch\nAllow: /\n", "/x", true, 0},
		{"specific group disallow", "User-agent: *\nAllow: /\n\nUser-agent: jobwatch\nDisallow: /private\n", "/private/a", false, 0},
		{"other bot group ignored", "User-agent: Googlebot\nDisallow: /\n", "/x", true, 0},
		{"multi agent group", "User-agent: foo\nUser-agent: JobWatch\nDisallow: /a\n", "/a", false, 0},
		{"longest match wins allow", "User-agent: *\nDisallow: /jobs\nAllow: /jobs/public\n", "/jobs/public/1", true, 0},
		{"longest match wins disallow", "User-agent: *\nAllow: /jobs\nDisallow: /jobs/private\n", "/jobs/private/1", false, 0},
		{"wildcard", "User-agent: *\nDisallow: /*.json$\n", "/feed.json", false, 0},
		{"wildcard anchor no match", "User-agent: *\nDisallow: /*.json$\n", "/feed.json?x=1", true, 0},
		{"query rule", "User-agent: *\nDisallow: /*?content=true\n", "/v1/boards/a/jobs?content=true", false, 0},
		{"robots.txt always allowed", "User-agent: *\nDisallow: /\n", "/robots.txt", true, 0},
		{"fractional crawl delay", "User-agent: *\nCrawl-delay: 2.5\n", "/", true, 2500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := parseRobots(strings.NewReader(tt.body), RobotsToken)
			if got := rr.allowed(tt.path); got != tt.want {
				t.Errorf("allowed(%q) = %v, want %v", tt.path, got, tt.want)
			}
			if rr.crawlDelay != tt.wantDelay {
				t.Errorf("crawlDelay = %v, want %v", rr.crawlDelay, tt.wantDelay)
			}
		})
	}
}

func TestClientGet(t *testing.T) {
	tests := []struct {
		name        string
		robotsCode  int
		robotsBody  string
		apiCode     int
		wantErr     bool
		wantBlocked bool
	}{
		{"ok", 200, "User-agent: *\nAllow: /\n", 200, false, false},
		{"robots 404 means allowed", 404, "", 200, false, false},
		{"robots 401 means allowed", 401, "", 200, false, false},
		{"robots disallow is blocked", 200, "User-agent: *\nDisallow: /api\n", 200, true, true},
		{"robots 500 is error", 500, "", 200, true, false},
		{"robots 429 is blocked", 429, "", 200, true, true},
		{"api 403 is blocked", 200, "", 403, true, true},
		{"api 429 is blocked", 200, "", 429, true, true},
		{"api 500 is error", 200, "", 500, true, false},
		{"api 404 is error", 200, "", 404, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotUA string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.WriteHeader(tt.robotsCode)
					w.Write([]byte(tt.robotsBody))
					return
				}
				gotUA = r.Header.Get("User-Agent")
				w.WriteHeader(tt.apiCode)
				w.Write([]byte(`{"ok":true}`))
			}))
			defer srv.Close()

			c, _ := testClient()
			var v struct{ OK bool }
			err := c.GetJSON(context.Background(), model.Source{ID: 1}, srv.URL+"/api/x", &v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if errors.Is(err, ErrBlocked) != tt.wantBlocked {
				t.Errorf("errors.Is(ErrBlocked) = %v, want %v (err=%v)", errors.Is(err, ErrBlocked), tt.wantBlocked, err)
			}
			if !tt.wantErr {
				if !v.OK {
					t.Error("body not decoded")
				}
				if gotUA != UserAgent {
					t.Errorf("User-Agent = %q", gotUA)
				}
			}
		})
	}
}

func TestClientRateLimit(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		robots     string
		wantSpaced time.Duration
	}{
		{"default interval", `{}`, "", DefaultInterval},
		{"per-source interval", `{"rate_limit_ms": 5000}`, "", 5 * time.Second},
		{"crawl-delay larger than interval", `{"rate_limit_ms": 500}`, "User-agent: *\nCrawl-delay: 3\n", 3 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.Write([]byte(tt.robots))
					return
				}
				w.Write([]byte(`{}`))
			}))
			defer srv.Close()
			c, fc := testClient()
			src := model.Source{ID: 1, ConfigJSON: tt.config}
			var v map[string]any
			for i := 0; i < 3; i++ {
				if err := c.GetJSON(context.Background(), src, srv.URL+"/a", &v); err != nil {
					t.Fatal(err)
				}
			}
			// sleeps: robots(0), req1, req2, req3. robots is fetched once.
			if len(fc.sleeps) != 4 {
				t.Fatalf("sleeps = %v, want 4 entries", fc.sleeps)
			}
			if fc.sleeps[0] != 0 {
				t.Errorf("first request waited %v", fc.sleeps[0])
			}
			for i, d := range fc.sleeps[2:] {
				if d != tt.wantSpaced {
					t.Errorf("request %d waited %v, want %v", i+2, d, tt.wantSpaced)
				}
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		url    string
		typ    model.SourceType
		cfg    Config
		wantOK bool
	}{
		{"https://boards.greenhouse.io/gitlab", model.SourceGreenhouse, Config{BoardToken: "gitlab"}, true},
		{"https://job-boards.greenhouse.io/gitlab/jobs/123", model.SourceGreenhouse, Config{BoardToken: "gitlab"}, true},
		{"https://job-boards.eu.greenhouse.io/acme", model.SourceGreenhouse, Config{BoardToken: "acme"}, true},
		{"https://boards.greenhouse.io/embed/job_board?for=acme", model.SourceGreenhouse, Config{BoardToken: "acme"}, true},
		{"https://boards-api.greenhouse.io/v1/boards/acme/jobs?content=true", model.SourceGreenhouse, Config{BoardToken: "acme"}, true},
		{"https://jobs.lever.co/spotify", model.SourceLever, Config{Site: "spotify"}, true},
		{"https://jobs.eu.lever.co/acme/abc", model.SourceLever, Config{Site: "acme", Region: "eu"}, true},
		{"https://api.lever.co/v0/postings/acme?mode=json", model.SourceLever, Config{Site: "acme"}, true},
		{"https://jobs.ashbyhq.com/linear", model.SourceAshby, Config{Org: "linear"}, true},
		{"https://api.ashbyhq.com/posting-api/job-board/linear", model.SourceAshby, Config{Org: "linear"}, true},
		{"https://careers.example.com/jobs", "", Config{}, false},
		{"https://jobs.lever.co/", "", Config{}, false},
		{"not a url", "", Config{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			typ, cfg, ok := Detect(tt.url)
			if ok != tt.wantOK || typ != tt.typ || cfg != tt.cfg {
				t.Errorf("Detect = (%q, %+v, %v), want (%q, %+v, %v)", typ, cfg, ok, tt.typ, tt.cfg, tt.wantOK)
			}
		})
	}
}

func TestHTMLToText(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"plain", "hello", "hello"},
		{"paragraphs", "<p>One</p><p>Two &amp; three</p>", "One\n\nTwo & three"},
		{"list", "<ul><li>Go</li><li>K8s</li></ul>", "- Go\n- K8s"},
		{"br and nbsp", "a<br/>b&nbsp;c", "a\nb c"},
		{"script dropped", "<script>alert(1)</script><div>x</div>", "x"},
		{"inline tags", "<p><strong>Bold</strong> <em>it</em></p>", "Bold it"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HTMLToText(tt.in); got != tt.want {
				t.Errorf("HTMLToText(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// fixtureServer serves testdata/<file> at path and an allow-all robots.txt.
func fixtureServer(t *testing.T, path, file string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.RequestURI() {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case path:
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestAdapters(t *testing.T) {
	tests := []struct {
		name       string
		reqPath    string
		fixture    string
		fetcher    func(c *Client, base string) Fetcher
		src        model.Source
		wantCount  int
		wantFirst  RawOffer // Description checked by prefix
		wantSecLoc string   // Location of the second offer
		wantDescIn string   // substring of the first description
	}{
		{
			name:      "greenhouse",
			reqPath:   "/v1/boards/gitlab/jobs?content=true",
			fixture:   "greenhouse_jobs.json",
			fetcher:   func(c *Client, b string) Fetcher { return &Greenhouse{Client: c, BaseURL: b} },
			src:       model.Source{ID: 1, Type: model.SourceGreenhouse, URL: "https://boards.greenhouse.io/gitlab"},
			wantCount: 3,
			wantFirst: RawOffer{
				ExternalUID: "8638232002",
				Title:       "AI Transformation Owner, CRO",
				Location:    "Remote, United States",
				URL:         "https://job-boards.greenhouse.io/gitlab/jobs/8638232002",
				Description: "GitLab is the intelligent orchestration platform for DevSecOps.",
				Company:     "GitLab",
			},
			wantSecLoc: "Remote, Canada; Remote, United States",
		},
		{
			name:      "lever",
			reqPath:   "/v0/postings/spotify?mode=json",
			fixture:   "lever_postings.json",
			fetcher:   func(c *Client, b string) Fetcher { return &Lever{Client: c, BaseURL: b} },
			src:       model.Source{ID: 2, Type: model.SourceLever, URL: "https://example.invalid", ConfigJSON: `{"site":"spotify"}`},
			wantCount: 3,
			wantFirst: RawOffer{
				ExternalUID: "2a55d6c9-c564-443a-be3e-a3851db5a93b",
				Title:       "Account Executive - Commercial, Spotify Technology",
				Location:    "New York, NY",
				URL:         "https://jobs.lever.co/spotify/2a55d6c9-c564-443a-be3e-a3851db5a93b",
				Description: "Spotify Technology is our business-to-business software team",
			},
			wantSecLoc: "London",
			wantDescIn: "What You'll Do\n• Identify and pursue new business opportunities",
		},
		{
			name:      "ashby skips unlisted",
			reqPath:   "/posting-api/job-board/linear",
			fixture:   "ashby_job_board.json",
			fetcher:   func(c *Client, b string) Fetcher { return &Ashby{Client: c, BaseURL: b} },
			src:       model.Source{ID: 3, Type: model.SourceAshby, URL: "https://jobs.ashbyhq.com/linear"},
			wantCount: 2,
			wantFirst: RawOffer{
				ExternalUID: "d3bc1ced-3ce4-4086-a050-555055dbb1ff",
				Title:       "Senior / Staff Fullstack Engineer",
				Location:    "Europe",
				URL:         "https://jobs.ashbyhq.com/linear/d3bc1ced-3ce4-4086-a050-555055dbb1ff",
				Description: "At Linear, we're building the product development system",
			},
			wantSecLoc: "North America; Toronto",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := fixtureServer(t, tt.reqPath, tt.fixture)
			c, _ := testClient()
			got, err := tt.fetcher(c, srv.URL).Fetch(context.Background(), tt.src)
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d offers, want %d", len(got), tt.wantCount)
			}
			first := got[0]
			if !strings.HasPrefix(first.Description, tt.wantFirst.Description) {
				t.Errorf("description = %.120q..., want prefix %q", first.Description, tt.wantFirst.Description)
			}
			if strings.Contains(first.Description, "<") || strings.Contains(first.Description, "&lt;") {
				t.Errorf("description still contains markup: %.200q", first.Description)
			}
			if tt.wantDescIn != "" && !strings.Contains(first.Description, tt.wantDescIn) {
				t.Errorf("description missing %q", tt.wantDescIn)
			}
			first.Description, tt.wantFirst.Description = "", ""
			if first != tt.wantFirst {
				t.Errorf("first offer = %+v, want %+v", first, tt.wantFirst)
			}
			if got[1].Location != tt.wantSecLoc {
				t.Errorf("second location = %q, want %q", got[1].Location, tt.wantSecLoc)
			}
		})
	}
}

func TestAdapterErrors(t *testing.T) {
	c, _ := testClient()
	tests := []struct {
		name    string
		fetcher Fetcher
		src     model.Source
		wantErr string
	}{
		{"greenhouse missing token", &Greenhouse{Client: c}, model.Source{ID: 1, URL: "https://example.com"}, "board_token"},
		{"lever missing site", &Lever{Client: c}, model.Source{ID: 1, URL: "https://example.com"}, "site"},
		{"ashby missing org", &Ashby{Client: c}, model.Source{ID: 1, URL: "https://example.com"}, "org"},
		{"bad config json", &Ashby{Client: c}, model.Source{ID: 1, ConfigJSON: "{"}, "invalid config_json"},
		{"registry unknown type", Registry{}, model.Source{Type: model.SourceRSS}, "not implemented"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fetcher.Fetch(context.Background(), tt.src)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}

	// Unknown board: the API answers 404, which is an error but not "blocked".
	srv := fixtureServer(t, "/nothing", "lever_postings.json")
	_, err := (&Lever{Client: c, BaseURL: srv.URL}).Fetch(context.Background(),
		model.Source{ID: 9, ConfigJSON: `{"site":"missing"}`})
	var he *HTTPError
	if !errors.As(err, &he) || he.Status != 404 || errors.Is(err, ErrBlocked) {
		t.Errorf("404 board err = %v", err)
	}
}
