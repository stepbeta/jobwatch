package sources

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestWTTJMapHit(t *testing.T) {
	search := wttjSearch{Language: "en", OrgSlug: "fabriq"}
	tests := []struct {
		name   string
		hit    wttjHit
		search wttjSearch
		want   RawOffer
		wantOK bool
	}{
		{
			name: "reference, offices and verbatim markdown profile",
			hit: wttjHit{
				Reference: "FABRI_wqJ1xND",
				ObjectID:  json.RawMessage(`4386482`),
				Name:      "Senior Front-end Engineer (F/H)",
				Slug:      "senior-full-stack-engineer-front-end-focus_paris",
				Language:  "en",
				Profile:   "### What we're looking for\n\n* **5+ years**",
				Summary:   "short summary",
				Offices:   []wttjOffice{{City: "Paris", Country: "France"}},
			},
			search: search,
			want: RawOffer{
				ExternalUID: "FABRI_wqJ1xND",
				Title:       "Senior Front-end Engineer (F/H)",
				Location:    "Paris, France",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/senior-full-stack-engineer-front-end-focus_paris",
				Description: "### What we're looking for\n\n* **5+ years**",
			},
			wantOK: true,
		},
		{
			name: "objectID string fallback when reference is empty",
			hit: wttjHit{
				ObjectID: json.RawMessage(`"3e6f18f5-97fb-404a-9ff7-700c7ab6629d"`),
				Name:     "SRE",
				Slug:     "sre_paris",
			},
			search: search,
			want: RawOffer{
				ExternalUID: "3e6f18f5-97fb-404a-9ff7-700c7ab6629d",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre_paris",
			},
			wantOK: true,
		},
		{
			name: "objectID numeric fallback",
			hit: wttjHit{
				ObjectID: json.RawMessage(`4386482`),
				Name:     "SRE",
				Slug:     "sre_paris",
			},
			search: search,
			want: RawOffer{
				ExternalUID: "4386482",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre_paris",
			},
			wantOK: true,
		},
		{
			name: "single office field fallback",
			hit: wttjHit{
				Reference: "A",
				Name:      "SRE",
				Slug:      "sre_paris",
				Office:    &wttjOffice{City: "Lyon", Country: "France"},
			},
			search: search,
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				Location:    "Lyon, France",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre_paris",
			},
			wantOK: true,
		},
		{
			name: "several offices are joined",
			hit: wttjHit{
				Reference: "A",
				Name:      "SRE",
				Slug:      "sre_paris",
				Offices: []wttjOffice{
					{City: "Paris", Country: "France"},
					{City: "Berlin"},   // country missing
					{Country: "Spain"}, // city missing
				},
			},
			search: search,
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				Location:    "Paris, France; Berlin; Spain",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre_paris",
			},
			wantOK: true,
		},
		{
			name: "empty profile falls back to summary",
			hit: wttjHit{
				Reference: "A",
				Name:      "SRE",
				Slug:      "sre",
				Profile:   "   \n ",
				Summary:   "Join Fabriq.",
			},
			search: search,
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre",
				Description: "Join Fabriq.",
			},
			wantOK: true,
		},
		{
			name:   "organization slug falls back to the discovered slug",
			hit:    wttjHit{Reference: "A", Name: "SRE", Slug: "sre"},
			search: search,
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre",
			},
			wantOK: true,
		},
		{
			name:   "page language wins over the hit language",
			hit:    wttjHit{Reference: "A", Name: "SRE", Slug: "sre", Language: "fr"},
			search: search,
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre",
			},
			wantOK: true,
		},
		{
			name:   "hit language is the fallback when discovery has none",
			hit:    wttjHit{Reference: "A", Name: "SRE", Slug: "sre", Language: "fr"},
			search: wttjSearch{OrgSlug: "fabriq"},
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/fr/companies/fabriq/jobs/sre",
			},
			wantOK: true,
		},
		{
			name:   "en is the last-resort language",
			hit:    wttjHit{Reference: "A", Name: "SRE", Slug: "sre"},
			search: wttjSearch{OrgSlug: "fabriq"},
			want: RawOffer{
				ExternalUID: "A",
				Title:       "SRE",
				URL:         "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre",
			},
			wantOK: true,
		},
		{
			name:   "no title is kept as incomplete",
			hit:    wttjHit{Reference: "A", Slug: "sre"},
			search: search,
			want: RawOffer{
				ExternalUID:      "A",
				URL:              "https://www.welcometothejungle.com/en/companies/fabriq/jobs/sre",
				IncompleteReason: model.IncompleteMissingTitle,
			},
			wantOK: true,
		},
		{
			name:   "no slug is dropped",
			hit:    wttjHit{Reference: "A", Name: "SRE"},
			search: search,
			wantOK: false,
		},
		{
			name:   "no organization slug anywhere cannot build a URL",
			hit:    wttjHit{Reference: "A", Name: "SRE", Slug: "sre"},
			search: wttjSearch{Language: "en"},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := mapWTTJHit(tt.hit, tt.search)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (offer %+v)", ok, tt.wantOK, got)
			}
			if !tt.wantOK {
				return
			}
			// Company comes from organization.name; the table sets it below.
			tt.want.Company = strings.TrimSpace(tt.hit.Organization.Name)
			tt.want.CompanyKey = companyKey(model.SourceWTTJ,
				firstNonEmpty(tt.hit.Organization.Slug, tt.search.OrgSlug))
			if got != tt.want {
				t.Errorf("offer = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestWTTJMapHitJSON covers the hit fields that are decoded from JSON:
// both timestamp shapes, explicit nulls for the fields the mapper does
// not store (salary, remote, contract_type), and organization names.
func TestWTTJMapHitJSON(t *testing.T) {
	tests := []struct {
		name, body, wantUID, wantTitle, wantLocation, wantCompany string
	}{
		{
			name: "Z timestamp and null extras",
			body: `{"objectID":"3e6f18f5-97fb-404a-9ff7-700c7ab6629d",
				"reference":"3e6f18f5-97fb-404a-9ff7-700c7ab6629d",
				"name":"Senior Front-end Engineer (F/H)",
				"slug":"senior-full-stack-engineer-front-end-focus_paris",
				"published_at":"2026-11-17T17:07:44Z",
				"profile":"### What we're looking for",
				"summary":"Join Fabriq.",
				"offices":[{"city":"Paris","country":"France"}],
				"remote":null,"contract_type":null,
				"salary_yearly_minimum":null,"salary_yearly_maximum":null,"salary_currency":null,
				"organization":{"name":"Fabriq","slug":"fabriq"}}`,
			wantUID:      "3e6f18f5-97fb-404a-9ff7-700c7ab6629d",
			wantTitle:    "Senior Front-end Engineer (F/H)",
			wantLocation: "Paris, France",
			wantCompany:  "Fabriq",
		},
		{
			name: "offset timestamp, numeric objectID and extras",
			body: `{"objectID":4386482,"reference":"FABRI_wqJ1xND",
				"name":"AI Software Engineer (F/H)","slug":"ai-software-engineer-f-h_paris",
				"published_at":"2026-10-27T09:00:00.000+01:00",
				"remote":"fulltime","contract_type":"FULL_TIME","salary_currency":"EUR",
				"salary_yearly_minimum":70000,"salary_yearly_maximum":null,
				"office":{"city":"Paris","country":"France"},
				"organization":{"name":"Fabriq","slug":"fabriq"}}`,
			wantUID:      "FABRI_wqJ1xND",
			wantTitle:    "AI Software Engineer (F/H)",
			wantLocation: "Paris, France",
			wantCompany:  "Fabriq",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hit wttjHit
			if err := json.Unmarshal([]byte(tt.body), &hit); err != nil {
				t.Fatalf("decode hit: %v", err)
			}
			offer, ok := mapWTTJHit(hit, wttjSearch{Language: "en", OrgSlug: "fabriq"})
			if !ok {
				t.Fatal("hit was dropped")
			}
			if offer.ExternalUID != tt.wantUID || offer.Title != tt.wantTitle ||
				offer.Location != tt.wantLocation || offer.Company != tt.wantCompany {
				t.Errorf("offer = %+v", offer)
			}
		})
	}
}

func TestParseWTTJPages(t *testing.T) {
	t.Run("real page tree", func(t *testing.T) {
		search, err := parseWTTJPages(readFixture(t, "wttj_page_fabriq.json"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if search.AppID != "CSEKHVMS53" || search.Index != "wk_cms_jobs_production" {
			t.Errorf("app/index = %q/%q", search.AppID, search.Index)
		}
		if search.Language != "en" || search.OrgSlug != "fabriq" {
			t.Errorf("language/org = %q/%q", search.Language, search.OrgSlug)
		}
		if len(search.Filters) != 1 || search.Filters[0] != (wttjFilter{"organization.slug", "fabriq"}) {
			t.Errorf("filters = %+v", search.Filters)
		}
		if !strings.HasPrefix(search.APIKey, "NDU3MmUxZGRj") {
			t.Errorf("api key not read verbatim: %q", search.APIKey)
		}
	})

	t.Run("credentials directly in the block properties", func(t *testing.T) {
		body := `{"page":{"sections":[{"containers":[{"blocks":[
			{"kind":"text","properties":{"title":"Jobs"}},
			{"kind":"jobs-search","properties":{
				"algolia_app_id":"APP","algolia_api_key":"KEY","algolia_index_name":"IDX",
				"language":"fr","filters":{"website.reference":"wttj_fr"}}}]}]}]}}`
		search, err := parseWTTJPages([]byte(body))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if search.AppID != "APP" || search.APIKey != "KEY" || search.Index != "IDX" ||
			search.Language != "fr" || search.OrgSlug != "" ||
			len(search.Filters) != 1 || search.Filters[0] != (wttjFilter{"website.reference", "wttj_fr"}) {
			t.Errorf("search = %+v", search)
		}
	})

	t.Run("missing jobs-search block is permanent", func(t *testing.T) {
		body := `{"page":{"sections":[{"containers":[{"blocks":[{"kind":"text"}]}]}]}}`
		_, err := parseWTTJPages([]byte(body))
		if !errors.Is(err, ErrNotCareersPage) {
			t.Errorf("err = %v, want ErrNotCareersPage", err)
		}
	})

	t.Run("malformed JSON is retryable", func(t *testing.T) {
		_, err := parseWTTJPages([]byte(`{"page":`))
		if err == nil || errors.Is(err, ErrNotCareersPage) {
			t.Errorf("err = %v, want a decoding error", err)
		}
	})

	t.Run("unsupported filters shape is an error", func(t *testing.T) {
		body := `{"page":{"blocks":[{"kind":"jobs-search","properties":{
			"algolia_app_id":"APP","algolia_api_key":"KEY","algolia_index_name":"IDX",
			"filters":["organization.slug:fabriq"]}}]}}`
		if _, err := parseWTTJPages([]byte(body)); err == nil || !strings.Contains(err.Error(), "filters") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestWTTJAlgoliaParams(t *testing.T) {
	tests := []struct {
		name    string
		filters []wttjFilter
		page    int
		want    string
	}{
		{
			name:    "fabriq",
			filters: []wttjFilter{{"organization.slug", "fabriq"}},
			want:    "filters=organization.slug:fabriq&hitsPerPage=50&page=0",
		},
		{
			name:    "second page",
			filters: []wttjFilter{{"organization.slug", "fabriq"}},
			page:    1,
			want:    "filters=organization.slug:fabriq&hitsPerPage=50&page=1",
		},
		{
			name: "several filters are AND-joined in attribute order",
			filters: []wttjFilter{
				{"organization.slug", "fabriq"},
				{"website.reference", "wttj_fr"},
			},
			want: "filters=organization.slug:fabriq website.reference:wttj_fr&hitsPerPage=50&page=0",
		},
		{
			name: "no filters",
			want: "hitsPerPage=50&page=0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wttjAlgoliaParams(tt.filters, tt.page); got != tt.want {
				t.Errorf("params = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWTTJQueryBody(t *testing.T) {
	body, err := wttjQueryBody("wk_cms_jobs_production", "filters=organization.slug:fabriq&hitsPerPage=50&page=0")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"requests":[{"indexName":"wk_cms_jobs_production","params":"filters=organization.slug:fabriq&hitsPerPage=50&page=0"}]}`
	if string(body) != want {
		t.Errorf("body = %s, want %s", body, want)
	}
}

// wttjTestServer serves the bootstrap page tree and the two Algolia
// result pages, recording what the adapter sent.
type wttjTestServer struct {
	*httptest.Server
	mu            sync.Mutex
	discoveryPath string
	discoveryUA   string
	headers       []http.Header
	bodies        []string
	pagesURL      string
}

func newWTTJTestServer(t *testing.T) *wttjTestServer {
	t.Helper()
	ts := &wttjTestServer{}
	pagesBody := readFixture(t, "wttj_page_fabriq.json")
	page0 := readFixture(t, "wttj_hits_page0.json")
	page1 := readFixture(t, "wttj_hits_page1.json")
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/api/v1/pages":
			ts.mu.Lock()
			ts.discoveryPath = r.URL.Query().Get("path")
			ts.discoveryUA = r.Header.Get("User-Agent")
			ts.mu.Unlock()
			w.Write(pagesBody)
		case "/1/indexes/*/queries":
			body, _ := io.ReadAll(r.Body)
			ts.mu.Lock()
			ts.headers = append(ts.headers, r.Header.Clone())
			ts.bodies = append(ts.bodies, string(body))
			ts.mu.Unlock()
			var req wttjQueryRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			switch {
			case strings.HasSuffix(req.Requests[0].Params, "page=0"):
				w.Write(page0)
			case strings.HasSuffix(req.Requests[0].Params, "page=1"):
				w.Write(page1)
			default:
				http.Error(w, "unexpected page", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	ts.pagesURL = ts.URL + "/api/v1/pages"
	return ts
}

func TestWTTJFetch(t *testing.T) {
	ts := newWTTJTestServer(t)
	c, _ := testClient()
	w := &WTTJ{Client: c, PagesURL: ts.pagesURL, AlgoliaBaseURL: ts.URL}
	src := model.Source{
		ID:   1,
		Type: model.SourceWTTJ,
		URL:  "https://www.welcometothejungle.com/en/companies/fabriq/jobs",
	}

	got, err := w.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Discovery: one GET of the bootstrap with the careers path and a
	// browser User-Agent.
	if ts.discoveryPath != "/en/companies/fabriq/jobs" {
		t.Errorf("discovery path = %q", ts.discoveryPath)
	}
	if ts.discoveryUA != wttjBrowserUA {
		t.Errorf("discovery User-Agent = %q", ts.discoveryUA)
	}

	// Algolia: one POST per result page, exact headers, form content
	// type with a JSON body.
	if len(ts.bodies) != 2 {
		t.Fatalf("algolia requests = %d, want 2 (page 0 and 1)", len(ts.bodies))
	}
	for i, h := range ts.headers {
		if got, want := h.Get("x-algolia-application-id"), "CSEKHVMS53"; got != want {
			t.Errorf("request %d application id = %q, want %q", i, got, want)
		}
		if got := h.Get("x-algolia-api-key"); !strings.HasPrefix(got, "NDU3MmUxZGRj") {
			t.Errorf("request %d api key = %q", i, got)
		}
		if got, want := h.Get("Referer"), wttjReferer; got != want {
			t.Errorf("request %d Referer = %q, want %q", i, got, want)
		}
		if got, want := h.Get("Content-Type"), "application/x-www-form-urlencoded"; got != want {
			t.Errorf("request %d Content-Type = %q, want %q", i, got, want)
		}
		if got, want := h.Get("User-Agent"), wttjBrowserUA; got != want {
			t.Errorf("request %d User-Agent = %q", i, got)
		}
	}
	for i, want := range []string{
		`"indexName":"wk_cms_jobs_production"`,
		`"params":"filters=organization.slug:fabriq&hitsPerPage=50&page=`,
	} {
		if !strings.Contains(ts.bodies[i], want) {
			t.Errorf("request %d body = %s, want it to contain %s", i, ts.bodies[i], want)
		}
	}

	// Both index shapes are mapped; pagination stops after nbPages.
	if len(got) != 2 {
		t.Fatalf("offers = %d, want 2: %+v", len(got), got)
	}
	first := got[0]
	if first.ExternalUID != "3e6f18f5-97fb-404a-9ff7-700c7ab6629d" ||
		first.Title != "Senior Front-end Engineer (F/H)" ||
		first.Company != "Fabriq" ||
		first.Location != "Paris, France" ||
		first.URL != "https://www.welcometothejungle.com/en/companies/fabriq/jobs/senior-full-stack-engineer-front-end-focus_paris" {
		t.Errorf("first offer = %+v", first)
	}
	if !strings.HasPrefix(first.Description, "### What we're looking for") {
		t.Errorf("first description = %.80q", first.Description)
	}
	second := got[1]
	if second.ExternalUID != "FABRI_wqJ1xND" || second.Title != "Senior Front-end Engineer (F/H)" ||
		second.URL != first.URL {
		t.Errorf("second offer = %+v", second)
	}
	again, err := w.Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if !sameUIDs(got, again) {
		t.Errorf("uids are not stable across fetches: %v vs %v", uidsOf(got), uidsOf(again))
	}
}

func TestWTTJFetchConfigOverrides(t *testing.T) {
	ts := newWTTJTestServer(t)
	c, _ := testClient()
	w := &WTTJ{Client: c, PagesURL: ts.pagesURL, AlgoliaBaseURL: ts.URL}
	src := model.Source{
		ID:         1,
		Type:       model.SourceWTTJ,
		URL:        "https://www.welcometothejungle.com/en/companies/fabriq/jobs",
		ConfigJSON: `{"algolia_app_id":"OVERRIDEAPP","algolia_api_key":"OVERRIDEKEY"}`,
	}
	if _, err := w.Fetch(context.Background(), src); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(ts.headers) == 0 {
		t.Fatal("no algolia request")
	}
	if got := ts.headers[0].Get("x-algolia-application-id"); got != "OVERRIDEAPP" {
		t.Errorf("application id = %q, want the override", got)
	}
	if got := ts.headers[0].Get("x-algolia-api-key"); got != "OVERRIDEKEY" {
		t.Errorf("api key = %q, want the override", got)
	}
}

func TestWTTJErrors(t *testing.T) {
	src := model.Source{
		ID:   1,
		Type: model.SourceWTTJ,
		URL:  "https://www.welcometothejungle.com/en/companies/fabriq/jobs",
	}
	tests := []struct {
		name          string
		pages         func(w http.ResponseWriter)
		algolia       func(w http.ResponseWriter)
		wantErr       string
		wantPermanent bool
		wantBlocked   bool
	}{
		{
			name:          "not a careers page",
			pages:         func(w http.ResponseWriter) { w.Write([]byte(`{"page":{"sections":[]}}`)) },
			wantErr:       "URL is not a WTTJ careers page",
			wantPermanent: true,
		},
		{
			name:    "bootstrap HTTP error is retryable",
			pages:   func(w http.ResponseWriter) { http.Error(w, "boom", http.StatusInternalServerError) },
			wantErr: "HTTP 500",
		},
		{
			name:        "bootstrap blocked",
			pages:       func(w http.ResponseWriter) { http.Error(w, "no", http.StatusForbidden) },
			wantErr:     "HTTP 403",
			wantBlocked: true,
		},
		{
			name:    "malformed page tree is retryable",
			pages:   func(w http.ResponseWriter) { w.Write([]byte(`{"page":`)) },
			wantErr: "decode page tree",
		},
		{
			name:        "algolia blocked",
			algolia:     func(w http.ResponseWriter) { http.Error(w, "no", http.StatusForbidden) },
			wantErr:     "HTTP 403",
			wantBlocked: true,
		},
		{
			name:    "algolia without results",
			algolia: func(w http.ResponseWriter) { w.Write([]byte(`{"results":[]}`)) },
			wantErr: "no results",
		},
		{
			name:    "algolia error message",
			algolia: func(w http.ResponseWriter) { w.Write([]byte(`{"message":"bad request","results":[]}`)) },
			wantErr: "bad request",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pagesBody := readFixture(t, "wttj_page_fabriq.json")
			hits := readFixture(t, "wttj_hits_page0.json")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/robots.txt":
					w.Write([]byte("User-agent: *\nAllow: /\n"))
				case "/api/v1/pages":
					if tt.pages != nil {
						tt.pages(w)
						return
					}
					w.Write(pagesBody)
				case "/1/indexes/*/queries":
					if tt.algolia != nil {
						tt.algolia(w)
						return
					}
					w.Write(hits)
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			c, _ := testClient()
			wttj := &WTTJ{Client: c, PagesURL: srv.URL + "/api/v1/pages", AlgoliaBaseURL: srv.URL}
			_, err := wttj.Fetch(context.Background(), src)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
			if errors.Is(err, ErrNotCareersPage) != tt.wantPermanent {
				t.Errorf("errors.Is(ErrNotCareersPage) = %v, want %v", errors.Is(err, ErrNotCareersPage), tt.wantPermanent)
			}
			if errors.Is(err, ErrBlocked) != tt.wantBlocked {
				t.Errorf("errors.Is(ErrBlocked) = %v, want %v", errors.Is(err, ErrBlocked), tt.wantBlocked)
			}
		})
	}
}

// The URL is built from what discovery reported, whatever the source
// row's base origin is.
func TestWTTJFetchUsesConfigURL(t *testing.T) {
	ts := newWTTJTestServer(t)
	c, _ := testClient()
	w := &WTTJ{Client: c, PagesURL: ts.pagesURL, AlgoliaBaseURL: ts.URL}
	src := model.Source{
		ID:         1,
		Type:       model.SourceWTTJ,
		URL:        "https://example.invalid/",
		ConfigJSON: `{"url":"https://www.welcometothejungle.com/en/companies/fabriq/jobs"}`,
	}
	if _, err := w.Fetch(context.Background(), src); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if ts.discoveryPath != "/en/companies/fabriq/jobs" {
		t.Errorf("discovery path = %q", ts.discoveryPath)
	}

	// A non-http URL is rejected before any request.
	if _, err := w.Fetch(context.Background(), model.Source{ID: 2, URL: "not a url"}); err == nil ||
		!strings.Contains(err.Error(), "careers page URL") {
		t.Errorf("invalid URL err = %v", err)
	}
}

func TestWTTJHelpers(t *testing.T) {
	tests := []struct {
		path, wantSlug, wantLang string
	}{
		{"/en/companies/fabriq/jobs", "fabriq", "en"},
		{"/fr/companies/joko/jobs/123", "joko", "fr"},
		{"/companies/fabriq", "fabriq", ""},
		{"/en/jobs", "", "en"},
		{"/en/companies-v1/joko/jobs", "", "en"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := wttjPathSlug(tt.path); got != tt.wantSlug {
				t.Errorf("slug = %q, want %q", got, tt.wantSlug)
			}
			if got := wttjPathLanguage(tt.path); got != tt.wantLang {
				t.Errorf("language = %q, want %q", got, tt.wantLang)
			}
		})
	}
	u, err := url.Parse("https://www.welcometothejungle.com/en/companies/fabriq/jobs")
	if err != nil {
		t.Fatal(err)
	}
	if got := WTTJSlug(u.Path); got != "fabriq" {
		t.Errorf("WTTJSlug = %q", got)
	}
}

// A corrupt or absurd nbPages must fail the scan instead of silently
// truncating: a truncated offer list would mark the rest disappeared.
func TestWTTJPageLimit(t *testing.T) {
	pagesBody := readFixture(t, "wttj_page_fabriq.json")
	hit := `{"reference":"A","name":"SRE","slug":"sre","organization":{"name":"Fabriq","slug":"fabriq"}}`
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/api/v1/pages":
			w.Write(pagesBody)
		case "/1/indexes/*/queries":
			requests++
			w.Write([]byte(`{"results":[{"hits":[` + hit + `],"nbPages":` + strconv.Itoa(wttjMaxPages+1) + `}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, _ := testClient()
	wttj := &WTTJ{Client: c, PagesURL: srv.URL + "/api/v1/pages", AlgoliaBaseURL: srv.URL}
	_, err := wttj.Fetch(context.Background(), model.Source{
		ID: 1, Type: model.SourceWTTJ,
		URL: "https://www.welcometothejungle.com/en/companies/fabriq/jobs",
	})
	if err == nil || !strings.Contains(err.Error(), "page limit") {
		t.Fatalf("err = %v, want a page-limit error", err)
	}
	if requests != wttjMaxPages {
		t.Errorf("algolia requests = %d, want %d", requests, wttjMaxPages)
	}
}
