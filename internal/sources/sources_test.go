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
		{"https://acme.jobs.personio.de/xml", model.SourcePersonio, Config{Tenant: "acme"}, true},
		{"https://acme.jobs.personio.com/careers", model.SourcePersonio, Config{Tenant: "acme"}, true},
		{"https://apply.workable.com/acme", model.SourceWorkable, Config{Account: "acme"}, true},
		{"https://apply.workable.com/api/v1/widget/accounts/acme?details=true", model.SourceWorkable, Config{Account: "acme"}, true},
		{"https://acme.workable.com/", model.SourceWorkable, Config{Account: "acme"}, true},
		{"https://semble.bamboohr.com/careers", model.SourceBambooHR, Config{Subdomain: "semble"}, true},
		{"https://semble.bamboohr.com/careers/list", model.SourceBambooHR, Config{Subdomain: "semble"}, true},
		{"https://Semble.BambooHR.com/careers/211", model.SourceBambooHR, Config{Subdomain: "semble"}, true},
		{"https://www.welcometothejungle.com/en/companies/fabriq/jobs", model.SourceWTTJ, Config{}, true},
		{"https://welcometothejungle.com/fr/companies/joko", model.SourceWTTJ, Config{}, true},
		{"https://www.welcometothejungle.com/en/jobs?query=platform%20engineer", "", Config{}, false},
		{"https://www.welcometothejungle.com/fr/jobs-matches", "", Config{}, false},
		{"https://www.welcometothejungle.com/en/companies-v1/joko/jobs", "", Config{}, false},
		{"https://www.bamboohr.com/careers", "", Config{}, false},
		{"https://semble.bamboohr.com/", "", Config{}, false},
		{"https://my.greenhouse.io/dashboard", "", Config{}, false},
		{"https://www.greenhouse.io/careers", "", Config{}, false},
		{"https://apply.workable.com/j/ABC123", "", Config{}, false},
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

func TestPersonioAdapter(t *testing.T) {
	srv := fixtureServer(t, "/xml", "personio.xml")
	c, _ := testClient()
	p := &Personio{Client: c, BaseURL: srv.URL}
	got, err := p.Fetch(context.Background(), model.Source{
		ID: 1, Type: model.SourcePersonio, URL: "https://acme.jobs.personio.de",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d offers, want 3", len(got))
	}
	first := got[0]
	if first.ExternalUID != "2885178" || first.Title != "Business Development Representative (d/f/m)" {
		t.Errorf("first offer = %+v", first)
	}
	if first.Location != "Berlin" || first.URL != srv.URL+"/job/2885178" {
		t.Errorf("first location/url = %q %q", first.Location, first.URL)
	}
	for _, want := range []string{"Your mission", "Kubernetes", "Your profile", "SRE or Platform Engineer"} {
		if !strings.Contains(first.Description, want) {
			t.Errorf("description missing %q: %q", want, first.Description)
		}
	}
	if strings.Contains(first.Description, "<") || strings.Contains(first.Description, "&lt;") {
		t.Errorf("description still contains markup: %.200q", first.Description)
	}
	if got[2].Location != "Remote - Germany" {
		t.Errorf("third location = %q", got[2].Location)
	}

	// The tenant can also come from config_json when the URL is not a feed URL.
	if _, err := (&Personio{Client: c, BaseURL: srv.URL}).Fetch(context.Background(),
		model.Source{ID: 2, URL: "https://example.invalid", ConfigJSON: `{"tenant":"acme"}`}); err != nil {
		t.Errorf("tenant from config: %v", err)
	}
	if _, err := (&Personio{Client: c}).Fetch(context.Background(), model.Source{ID: 3, URL: "https://example.invalid"}); err == nil ||
		!strings.Contains(err.Error(), "tenant") {
		t.Errorf("missing tenant error = %v", err)
	}
}

func TestWorkableAdapter(t *testing.T) {
	srv := fixtureServer(t, "/api/v1/widget/accounts/acme?details=true", "workable_jobs.json")
	c, _ := testClient()
	wk := &Workable{Client: c, BaseURL: srv.URL}
	got, err := wk.Fetch(context.Background(), model.Source{
		ID: 1, Type: model.SourceWorkable, URL: "https://apply.workable.com/acme",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d offers, want 3", len(got))
	}
	first := got[0]
	if first.ExternalUID != "81B46579FE" || first.Title != "Open-Source Machine Learning Engineer - EMEA Remote" {
		t.Errorf("first offer = %+v", first)
	}
	if first.Company != "Hugging Face" {
		t.Errorf("company = %q", first.Company)
	}
	if first.URL != "https://apply.workable.com/j/81B46579FE" {
		t.Errorf("url = %q", first.URL)
	}
	if !strings.Contains(first.Location, "Paris, Île-de-France, France") ||
		!strings.Contains(first.Location, "Berlin, Berlin, Germany") ||
		!strings.Contains(first.Location, "(remote)") {
		t.Errorf("location = %q", first.Location)
	}
	if !strings.Contains(first.Description, "Hugging Face") {
		t.Errorf("description missing company text: %.160q", first.Description)
	}
	if strings.Contains(first.Description, "<") {
		t.Errorf("description still contains markup: %.160q", first.Description)
	}
	if got[2].Location != "Paris, Île-de-France, France (remote)" {
		t.Errorf("third location = %q", got[2].Location)
	}

	// An account can come from config_json, and an unknown account is an
	// HTTP error rather than an empty board.
	if _, err := (&Workable{Client: c, BaseURL: srv.URL}).Fetch(context.Background(),
		model.Source{ID: 2, URL: "https://example.invalid", ConfigJSON: `{"account":"acme"}`}); err != nil {
		t.Errorf("account from config: %v", err)
	}
	if _, err := (&Workable{Client: c}).Fetch(context.Background(), model.Source{ID: 3, URL: "https://example.invalid"}); err == nil ||
		!strings.Contains(err.Error(), "account") {
		t.Errorf("missing account error = %v", err)
	}
}

func TestBambooHRAdapter(t *testing.T) {
	srv := fixtureServer(t, "/careers/list", "bamboohr_careers.json")
	c, _ := testClient()
	b := &BambooHR{Client: c, BaseURL: srv.URL}
	got, err := b.Fetch(context.Background(), model.Source{
		ID: 1, Type: model.SourceBambooHR, URL: "https://semble.bamboohr.com/careers",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 9 {
		t.Fatalf("got %d offers, want 9", len(got))
	}
	first := got[0]
	if first.ExternalUID != "117" ||
		first.Title != "Don't see a role for you? Apply here to join our Talent Pool" {
		t.Errorf("first offer = %+v", first)
	}
	if first.Location != "" || first.Description != "" || first.Company != "" {
		t.Errorf("first offer location/description/company = %q %q %q", first.Location, first.Description, first.Company)
	}
	if first.URL != srv.URL+"/careers/117" {
		t.Errorf("first url = %q", first.URL)
	}
	byID := map[string]RawOffer{}
	for _, offer := range got {
		byID[offer.ExternalUID] = offer
	}
	if o := byID["211"]; o.Location != "Paris, France" {
		t.Errorf("211 location = %q", o.Location)
	}
	if o := byID["214"]; o.Location != "Paris, ile de France, France" {
		t.Errorf("214 location = %q", o.Location)
	}
	if o := byID["202"]; o.Location != "London, United Kingdom" {
		t.Errorf("202 location = %q", o.Location)
	}
	if o := byID["201"]; o.Location != "United Kingdom" {
		t.Errorf("201 location = %q", o.Location)
	}
	if o := byID["211"]; o.URL != srv.URL+"/careers/211" {
		t.Errorf("211 url = %q", o.URL)
	}

	// The subdomain can also come from config_json when the URL is not a
	// BambooHR careers URL.
	if _, err := (&BambooHR{Client: c, BaseURL: srv.URL}).Fetch(context.Background(),
		model.Source{ID: 2, URL: "https://example.invalid", ConfigJSON: `{"subdomain":"semble"}`}); err != nil {
		t.Errorf("subdomain from config: %v", err)
	}
	if _, err := (&BambooHR{Client: c}).Fetch(context.Background(), model.Source{ID: 3, URL: "https://example.invalid"}); err == nil ||
		!strings.Contains(err.Error(), "subdomain") {
		t.Errorf("missing subdomain error = %v", err)
	}
}

func TestBambooHRParsing(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCount int
		wantLoc   string
		wantUID   string
		wantErr   string
	}{
		{
			name:      "empty listing",
			body:      `{"meta":{"totalCount":0},"result":[]}`,
			wantCount: 0,
		},
		{
			name:      "null result",
			body:      `{"meta":{},"result":null}`,
			wantCount: 0,
		},
		{
			name: "missing optional fields",
			body: `{"result":[
				{"id":"7","jobOpeningName":"SRE","departmentLabel":null,
				 "location":{"city":null,"state":null},"atsLocation":null,"isRemote":null},
				{"jobOpeningName":"No id"},
				{"id":"8"}]}`,
			wantCount: 1,
			wantUID:   "7",
		},
		{
			name:      "remote flag without location",
			body:      `{"result":[{"id":"9","jobOpeningName":"Platform Engineer","isRemote":true}]}`,
			wantCount: 1,
			wantLoc:   "Remote",
			wantUID:   "9",
		},
		{
			name: "remote flag with location",
			body: `{"result":[{"id":"10","jobOpeningName":"SRE","isRemote":true,
				"atsLocation":{"city":"Paris","province":null,"state":null,"country":"France"}}]}`,
			wantCount: 1,
			wantLoc:   "Paris, France (remote)",
			wantUID:   "10",
		},
		{
			name:    "malformed json",
			body:    `{"result":`,
			wantErr: "decode",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.Write([]byte("User-agent: *\nAllow: /\n"))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c, _ := testClient()
			got, err := (&BambooHR{Client: c, BaseURL: srv.URL}).Fetch(context.Background(), model.Source{
				ID: 1, Type: model.SourceBambooHR, URL: "https://semble.bamboohr.com/careers",
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d offers, want %d", len(got), tt.wantCount)
			}
			if tt.wantCount > 0 && (got[0].Location != tt.wantLoc || got[0].ExternalUID != tt.wantUID) {
				t.Errorf("first offer = %+v, want location %q uid %q", got[0], tt.wantLoc, tt.wantUID)
			}
		})
	}
}

func TestRSSAdapter(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		fixture    string
		config     string
		wantCount  int
		wantTitle  string
		wantURL    string
		wantUID    string
		wantDescIn string
		wantLoc    string
		wantComp   string
	}{
		{
			name:       "rss defaults",
			path:       "/feed.xml",
			fixture:    "rss_feed.xml",
			wantCount:  3,
			wantTitle:  "Hometap: Underwriter",
			wantURL:    "https://weworkremotely.com/remote-jobs/hometap-underwriter",
			wantUID:    "https://weworkremotely.com/remote-jobs/hometap-underwriter",
			wantDescIn: "Kubernetes",
		},
		{
			name:       "rss custom mapping",
			path:       "/feed.xml",
			fixture:    "rss_feed.xml",
			config:     `{"location_field":"region","company_field":"category","description_field":"description"}`,
			wantCount:  3,
			wantTitle:  "Hometap: Underwriter",
			wantURL:    "https://weworkremotely.com/remote-jobs/hometap-underwriter",
			wantUID:    "https://weworkremotely.com/remote-jobs/hometap-underwriter",
			wantDescIn: "Terraform",
			wantLoc:    "USA Only",
			wantComp:   "Management and Finance",
		},
		{
			name:       "atom alternate link",
			path:       "/atom.xml",
			fixture:    "atom_feed.xml",
			config:     `{"description_field":"content"}`,
			wantCount:  3,
			wantTitle:  "Go Turns 17",
			wantURL:    "https://go.dev/blog/17years",
			wantUID:    "tag:blog.golang.org,2013:blog.golang.org/17years",
			wantDescIn: "Remote platform engineering",
		},
		{
			name:       "atom attribute mapping",
			path:       "/atom.xml",
			fixture:    "atom_feed.xml",
			config:     `{"link_field":"link@href","description_field":"summary","company_field":"author/name"}`,
			wantCount:  3,
			wantTitle:  "Go Turns 17",
			wantURL:    "https://go.dev/blog/17years",
			wantUID:    "tag:blog.golang.org,2013:blog.golang.org/17years",
			wantDescIn: "Happy Birthday",
			wantComp:   "Austin Clements, for the Go team",
		},
		{
			// Teamtailor nests locations under tt:locations/tt:location
			// and exposes the display name as tt:name. The path uses the
			// local names, so the namespace prefix is not part of it.
			name:       "teamtailor nested location",
			path:       "/teamtailor.xml",
			fixture:    "teamtailor_feed.xml",
			config:     `{"location_field":"locations/location/name"}`,
			wantCount:  2,
			wantTitle:  "Team Manager - Monitoring & Incident Management – Remote-First",
			wantURL:    "https://careers.epicompany.eu/jobs/8622189-team-manager-monitoring-incident-management-remote-first",
			wantUID:    "9f9434ef-8881-46ab-91b4-ba85e0978b15",
			wantDescIn: "monitoring and incident management",
			wantLoc:    "Remote-First in Belgium",
		},
		{
			// Homerun publishes Atom entries with a non-standard
			// <description> element; the default mapping reads it.
			name:       "homerun atom description",
			path:       "/homerun.xml",
			fixture:    "homerun_feed.xml",
			wantCount:  2,
			wantTitle:  "Staff Product Engineer",
			wantURL:    "https://careers.ghost.org/staff-product-engineer",
			wantUID:    "job_w9iLO3e0iMEITMxpk6bQ",
			wantDescIn: "Build the Ghost product",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := fixtureServer(t, tt.path, tt.fixture)
			c, _ := testClient()
			got, err := (&RSS{Client: c}).Fetch(context.Background(), model.Source{
				ID: 1, Type: model.SourceRSS, URL: srv.URL + tt.path, ConfigJSON: tt.config,
			})
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(got) != tt.wantCount {
				t.Fatalf("got %d offers, want %d", len(got), tt.wantCount)
			}
			first := got[0]
			if first.Title != tt.wantTitle || first.URL != tt.wantURL || first.ExternalUID != tt.wantUID {
				t.Errorf("first offer = %+v", first)
			}
			if !strings.Contains(first.Description, tt.wantDescIn) {
				t.Errorf("description missing %q: %.200q", tt.wantDescIn, first.Description)
			}
			if first.Location != tt.wantLoc || first.Company != tt.wantComp {
				t.Errorf("location/company = %q %q, want %q %q", first.Location, first.Company, tt.wantLoc, tt.wantComp)
			}
		})
	}

	// We Work Remotely publishes RSS 2.0 items with the region in a
	// non-standard <region> element and no company element. The registered
	// source maps only location_field; titles such as "Company: Role" are
	// kept verbatim (the RSS known gap), and the company stays empty.
	t.Run("weworkremotely registered mapping", func(t *testing.T) {
		srv := fixtureServer(t, "/remote-jobs.rss", "rss_feed.xml")
		c, _ := testClient()
		got, err := (&RSS{Client: c}).Fetch(context.Background(), model.Source{
			ID: 36, Type: model.SourceRSS, URL: srv.URL + "/remote-jobs.rss",
			ConfigJSON: `{"location_field":"region"}`,
		})
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("got %d offers, want 3", len(got))
		}
		want := []struct{ title, region string }{
			{"Hometap: Underwriter", "USA Only"},
			{"Jadeclass Education: Online English Writing and Analysis Teacher - All Levels", "Oceania Only"},
			{"DeepSeas: Lead Incident Response Analyst", "Americas Only"},
		}
		for i, w := range want {
			offer := got[i]
			if offer.Title != w.title || offer.Location != w.region {
				t.Errorf("offer %d title/location = %q/%q, want %q/%q", i, offer.Title, offer.Location, w.title, w.region)
			}
			if offer.Company != "" {
				t.Errorf("offer %d company = %q, want empty", i, offer.Company)
			}
			if !strings.HasPrefix(offer.URL, "https://weworkremotely.com/remote-jobs/") {
				t.Errorf("offer %d URL = %q", i, offer.URL)
			}
			if offer.ExternalUID != offer.URL {
				t.Errorf("offer %d UID = %q, want the guid/link %q", i, offer.ExternalUID, offer.URL)
			}
		}
	})

	// An XML document without known item elements is an empty feed, not an error.
	srv := fixtureServer(t, "/empty.xml", "personio.xml")
	c, _ := testClient()
	got, err := (&RSS{Client: c}).Fetch(context.Background(), model.Source{ID: 1, URL: srv.URL + "/empty.xml"})
	if err != nil || len(got) != 0 {
		t.Errorf("empty feed = %v, %d offers", err, len(got))
	}
}

func TestHTMLAdapter(t *testing.T) {
	srv := fixtureServer(t, "/careers", "html_listing.html")
	c, _ := testClient()
	cfg := `{"item_selector":"li.job-card","title_selector":"h2.job-title",
	         "link_selector":"h2.job-title a","location_selector":".job-location",
	         "description_selector":".job-summary","uid_attr":"data-id"}`
	got, err := (&HTML{Client: c}).Fetch(context.Background(), model.Source{
		ID: 1, Type: model.SourceHTML, URL: srv.URL + "/careers", ConfigJSON: cfg,
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d offers, want 3", len(got))
	}
	first := got[0]
	if first.ExternalUID != "acme-101" || first.Title != "Site Reliability Engineer" {
		t.Errorf("first offer = %+v", first)
	}
	if first.URL != srv.URL+"/jobs/site-reliability-engineer" {
		t.Errorf("relative link = %q", first.URL)
	}
	if first.Location != "Remote - France" {
		t.Errorf("location = %q", first.Location)
	}
	for _, want := range []string{"Run Kubernetes and Terraform platforms.", "Fully remote."} {
		if !strings.Contains(first.Description, want) {
			t.Errorf("description missing %q: %q", want, first.Description)
		}
	}
	if got[1].URL != "https://careers.example.com/jobs/platform-engineer" {
		t.Errorf("absolute link = %q", got[1].URL)
	}

	// A title selector without an explicit link selector falls back to the
	// first href inside the title element.
	loose, err := (&HTML{Client: c}).Fetch(context.Background(), model.Source{
		ID: 1, URL: srv.URL + "/careers",
		ConfigJSON: `{"item_selector":"li.job-card","title_selector":"h2.job-title"}`,
	})
	if err != nil || len(loose) != 3 || loose[0].URL != srv.URL+"/jobs/site-reliability-engineer" {
		t.Errorf("loose selectors = %v, %d offers (err %v)", loose, len(loose), err)
	}

	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{"missing item selector", `{"title_selector":"h2"}`, "item_selector"},
		{"missing title selector", `{"item_selector":"li"}`, "title_selector"},
		{"bad item selector", `{"item_selector":"li[","title_selector":"h2"}`, "item_selector"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (&HTML{Client: c}).Fetch(context.Background(), model.Source{
				ID: 1, URL: srv.URL + "/careers", ConfigJSON: tt.config,
			})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntervalFor(t *testing.T) {
	tests := []struct {
		name string
		typ  model.SourceType
		cfg  string
		want time.Duration
	}{
		{"default", model.SourceGreenhouse, `{}`, DefaultInterval},
		{"configured", model.SourceWorkable, `{"rate_limit_ms":5000}`, 5 * time.Second},
		{"html minimum", model.SourceHTML, `{}`, HTMLMinInterval},
		{"html ignores smaller config", model.SourceHTML, `{"rate_limit_ms":1000}`, HTMLMinInterval},
		{"html accepts larger config", model.SourceHTML, `{"rate_limit_ms":8000}`, 8 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfig(model.Source{ConfigJSON: tt.cfg})
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.IntervalFor(tt.typ, DefaultInterval); got != tt.want {
				t.Errorf("IntervalFor = %v, want %v", got, tt.want)
			}
		})
	}
}
