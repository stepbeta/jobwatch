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
	"reflect"
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
			name: "hit organization slug wins over the source slug",
			hit: wttjHit{
				Reference:    "PARTO_PqAo88l",
				Name:         "Lead Developer (Python/React) - CDI - Paris - M/F/D",
				Slug:         "lead-developer-python-react-cdi-paris-m-f-d_paris",
				Organization: wttjOrganization{Name: "Partoo", Slug: "partoo"},
			},
			search: wttjSearch{Language: "en", OrgSlug: "partoo-italia"},
			want: RawOffer{
				ExternalUID: "PARTO_PqAo88l",
				Title:       "Lead Developer (Python/React) - CDI - Paris - M/F/D",
				URL:         "https://www.welcometothejungle.com/en/companies/partoo/jobs/lead-developer-python-react-cdi-paris-m-f-d_paris",
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

// TestWTTJMapPartooHit maps the recorded Partoo hit. Discovery reports a
// sibling organization as the fallback, so the URL and company key must
// follow the hit's own organization slug.
func TestWTTJMapPartooHit(t *testing.T) {
	var resp wttjAlgoliaResponse
	if err := json.Unmarshal(readFixture(t, "wttj_hits_partoo.json"), &resp); err != nil {
		t.Fatalf("decode hits: %v", err)
	}
	if len(resp.Results) != 1 || len(resp.Results[0].Hits) != 1 {
		t.Fatalf("fixture results = %d/%d, want 1/1", len(resp.Results), len(resp.Results[0].Hits))
	}
	hit := resp.Results[0].Hits[0]
	offer, ok := mapWTTJHit(hit, wttjSearch{Language: "en", OrgSlug: "localoo"})
	if !ok {
		t.Fatal("hit was dropped")
	}
	want := RawOffer{
		ExternalUID: "PARTO_PqAo88l",
		Title:       "Lead Developer (Python/React) - CDI - Paris - M/F/D",
		Location:    "Paris, France",
		URL:         "https://www.welcometothejungle.com/en/companies/partoo/jobs/lead-developer-python-react-cdi-paris-m-f-d_paris",
		Company:     "Partoo",
		CompanyKey:  companyKey(model.SourceWTTJ, "partoo"),
	}
	if offer != want {
		t.Errorf("offer = %+v, want %+v", offer, want)
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
		if search.ExpectedItems != 7 {
			t.Errorf("expected items = %d, want 7", search.ExpectedItems)
		}
		if !reflect.DeepEqual(search.Filters, []wttjFilterGroup{{Attribute: "organization.slug", Values: []string{"fabriq"}}}) {
			t.Errorf("filters = %+v", search.Filters)
		}
		if !strings.HasPrefix(search.APIKey, "NDU3MmUxZGRj") {
			t.Errorf("api key not read verbatim: %q", search.APIKey)
		}
	})

	t.Run("nested layout block (Partoo)", func(t *testing.T) {
		search, err := parseWTTJPages(readFixture(t, "wttj_page_partoo.json"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if search.AppID != "CSEKHVMS53" || search.Index != "wk_cms_jobs_production" {
			t.Errorf("app/index = %q/%q", search.AppID, search.Index)
		}
		if search.Language != "en" || search.OrgSlug != "partoo" {
			t.Errorf("language/org = %q/%q", search.Language, search.OrgSlug)
		}
		if search.ExpectedItems != 15 {
			t.Errorf("expected items = %d, want 15", search.ExpectedItems)
		}
		want := []wttjFilterGroup{{
			Attribute: "organization.slug",
			Values:    []string{"localoo", "partoo-italia", "partoo"},
		}}
		if !reflect.DeepEqual(search.Filters, want) {
			t.Errorf("filters = %+v, want %+v", search.Filters, want)
		}
		if !strings.HasPrefix(search.APIKey, "NDU3MmUxZGRj") {
			t.Errorf("api key not read verbatim: %q", search.APIKey)
		}
	})

	t.Run("stale organization slug (Swile)", func(t *testing.T) {
		search, err := parseWTTJPages(readFixture(t, "wttj_page_swile.json"))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if search.AppID != "CSEKHVMS53" || search.Index != "wk_cms_jobs_production" {
			t.Errorf("app/index = %q/%q", search.AppID, search.Index)
		}
		if search.Language != "en" || search.OrgSlug != "swile" {
			t.Errorf("language/org = %q/%q", search.Language, search.OrgSlug)
		}
		if search.ExpectedItems != 15 {
			t.Errorf("expected items = %d, want 15", search.ExpectedItems)
		}
		want := []wttjFilterGroup{{
			Attribute: "organization.slug",
			Values:    []string{"swile"},
		}}
		if !reflect.DeepEqual(search.Filters, want) {
			t.Errorf("filters = %+v, want %+v", search.Filters, want)
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
			!reflect.DeepEqual(search.Filters, []wttjFilterGroup{{Attribute: "website.reference", Values: []string{"wttj_fr"}}}) {
			t.Errorf("search = %+v", search)
		}
	})

	t.Run("credentials under a container that is not jobs-search", func(t *testing.T) {
		body := `{"page":{"sections":[{"containers":[{"blocks":[
			{"kind":"layout","properties":{"columns":2},"contents":[
				{"kind":"promo","properties":{"title":"Jobs"}},
				{"kind":"promo","properties":{
					"algolia_app_id":"APP","algolia_api_key":"KEY","algolia_index_name":"IDX",
					"language":"en","filters":{"organization.slug":["one","two"]}}}]}]}]}]}}`
		search, err := parseWTTJPages([]byte(body))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if search.AppID != "APP" || search.Index != "IDX" || search.Language != "en" {
			t.Errorf("search = %+v", search)
		}
		if !reflect.DeepEqual(search.Filters, []wttjFilterGroup{{Attribute: "organization.slug", Values: []string{"one", "two"}}}) {
			t.Errorf("filters = %+v", search.Filters)
		}
	})

	t.Run("missing credentials object is permanent", func(t *testing.T) {
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
		filters []wttjFilterGroup
		page    int
		want    string
	}{
		{
			name:    "fabriq",
			filters: []wttjFilterGroup{{Attribute: "organization.slug", Values: []string{"fabriq"}}},
			want:    "facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=0",
		},
		{
			name:    "second page",
			filters: []wttjFilterGroup{{Attribute: "organization.slug", Values: []string{"fabriq"}}},
			page:    1,
			want:    "facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=1",
		},
		{
			name: "several values of one attribute are OR-ed",
			filters: []wttjFilterGroup{{
				Attribute: "organization.slug",
				Values:    []string{"localoo", "partoo-italia", "partoo"},
			}},
			want: "facetFilters=%5B%5B%22organization.slug%3Alocaloo%22%2C%22organization.slug%3Apartoo-italia%22%2C%22organization.slug%3Apartoo%22%5D%5D&hitsPerPage=50&page=0",
		},
		{
			name: "separate attributes are AND-ed as separate groups",
			filters: []wttjFilterGroup{
				{Attribute: "organization.slug", Values: []string{"fabriq"}},
				{Attribute: "remote", Values: []string{"fulltime"}},
			},
			want: "facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%2C%5B%22remote%3Afulltime%22%5D%5D&hitsPerPage=50&page=0",
		},
		{
			name: "the key-embedded website.reference filter is omitted",
			filters: []wttjFilterGroup{
				{Attribute: "organization.slug", Values: []string{"fabriq"}},
				{Attribute: "website.reference", Values: []string{"wttj_fr"}},
			},
			want: "facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=0",
		},
		{
			name:    "only the key-embedded filter leaves no facetFilters",
			filters: []wttjFilterGroup{{Attribute: "website.reference", Values: []string{"wttj_fr"}}},
			want:    "hitsPerPage=50&page=0",
		},
		{
			name:    "empty group is ignored",
			filters: []wttjFilterGroup{{Attribute: "organization.slug"}},
			want:    "hitsPerPage=50&page=0",
		},
		{
			name: "no filters",
			want: "hitsPerPage=50&page=0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := wttjAlgoliaParams(tt.filters, tt.page)
			if err != nil {
				t.Fatalf("params: %v", err)
			}
			if got != tt.want {
				t.Errorf("params = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWTTJQueryBody(t *testing.T) {
	params := "facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=0"
	body, err := wttjQueryBody("wk_cms_jobs_production", params)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"requests":[{"indexName":"wk_cms_jobs_production","params":"` + params + `"}]}`
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
	orgRequests   int
	pagesURL      string
}

func newWTTJTestServer(t *testing.T) *wttjTestServer {
	t.Helper()
	ts := &wttjTestServer{}
	pagesBody := readFixture(t, "wttj_page_fabriq.json")
	page0 := readFixture(t, "wttj_hits_page0.json")
	page1 := readFixture(t, "wttj_hits_page1.json")
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case r.URL.Path == "/api/v1/pages":
			ts.mu.Lock()
			ts.discoveryPath = r.URL.Query().Get("path")
			ts.discoveryUA = r.Header.Get("User-Agent")
			ts.mu.Unlock()
			w.Write(pagesBody)
		case r.URL.Path == "/1/indexes/*/queries":
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
		case strings.HasPrefix(r.URL.Path, "/api/v1/organizations/"):
			ts.mu.Lock()
			ts.orgRequests++
			ts.mu.Unlock()
			http.NotFound(w, r)
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
	if ts.orgRequests != 0 {
		t.Errorf("organization lookups = %d, want 0 for a resolved page", ts.orgRequests)
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
	for i, body := range ts.bodies {
		if !strings.Contains(body, `"indexName":"wk_cms_jobs_production"`) {
			t.Errorf("request %d body = %s, want the discovered index", i, body)
		}
		want := `"params":"facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=` + strconv.Itoa(i) + `"`
		if !strings.Contains(body, want) {
			t.Errorf("request %d body = %s, want it to contain %s", i, body, want)
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

// TestWTTJResolveStaleOrgSlug covers a renamed organization: the page
// filter (swile) lags behind the index (lunchr), so an empty first page
// triggers one organization lookup and a retry with the canonical slug.
func TestWTTJResolveStaleOrgSlug(t *testing.T) {
	tests := []struct {
		name        string
		page        []byte
		orgSlug     string
		orgStatus   int
		wantOffers  int
		wantQueries int
		wantOrgReqs int
		wantErr     string
	}{
		{
			name:        "stale slug is retried with the canonical one",
			page:        readFixture(t, "wttj_page_swile.json"),
			orgSlug:     "lunchr",
			wantOffers:  1,
			wantQueries: 2,
			wantOrgReqs: 1,
		},
		{
			name:        "canonical slug already matches",
			page:        readFixture(t, "wttj_page_swile.json"),
			orgSlug:     "swile",
			wantQueries: 1,
			wantOrgReqs: 1,
		},
		{
			name: "page reports no jobs",
			page: []byte(`{"page":{"path":[{"items_count":0}],"sections":[{"containers":[{"blocks":[
				{"kind":"jobs-search","properties":{
					"algolia_app_id":"APP","algolia_api_key":"KEY","algolia_index_name":"IDX",
					"language":"en","filters":{"organization.slug":["swile"]}}}]}]}]}}`),
			orgSlug:     "lunchr",
			wantQueries: 1,
		},
		{
			name:        "organization lookup failure is retryable",
			page:        readFixture(t, "wttj_page_swile.json"),
			orgStatus:   http.StatusInternalServerError,
			wantQueries: 1,
			wantOrgReqs: 1,
			wantErr:     "resolve organization",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var queries, orgReqs int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/robots.txt":
					w.Write([]byte("User-agent: *\nAllow: /\n"))
				case r.URL.Path == "/api/v1/pages":
					w.Write(tt.page)
				case strings.HasPrefix(r.URL.Path, "/api/v1/organizations/"):
					orgReqs++
					if tt.orgStatus != 0 {
						http.Error(w, "boom", tt.orgStatus)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"organization":{"name":"Swile","slug":"` + tt.orgSlug + `"}}`))
				case r.URL.Path == "/1/indexes/*/queries":
					queries++
					body, _ := io.ReadAll(r.Body)
					if strings.Contains(string(body), "organization.slug%3Aswile") {
						w.Write([]byte(`{"results":[{"hits":[],"nbPages":0,"nbHits":0}]}`))
						return
					}
					w.Write(readFixture(t, "wttj_hits_swile.json"))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			c, _ := testClient()
			wttj := &WTTJ{
				Client:           c,
				PagesURL:         srv.URL + "/api/v1/pages",
				OrganizationsURL: srv.URL + "/api/v1/organizations",
				AlgoliaBaseURL:   srv.URL,
			}
			offers, err := wttj.Fetch(context.Background(), model.Source{
				ID: 1, Type: model.SourceWTTJ,
				URL: "https://www.welcometothejungle.com/en/companies/swile/jobs",
			})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(offers) != tt.wantOffers {
				t.Fatalf("offers = %d, want %d: %+v", len(offers), tt.wantOffers, offers)
			}
			if queries != tt.wantQueries {
				t.Errorf("algolia queries = %d, want %d", queries, tt.wantQueries)
			}
			if orgReqs != tt.wantOrgReqs {
				t.Errorf("organization requests = %d, want %d", orgReqs, tt.wantOrgReqs)
			}
			if tt.wantOffers == 1 {
				got := offers[0]
				if got.CompanyKey != companyKey(model.SourceWTTJ, "lunchr") ||
					!strings.Contains(got.URL, "/companies/lunchr/jobs/") {
					t.Errorf("offer = %+v, want the hit's canonical organization", got)
				}
			}
		})
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
