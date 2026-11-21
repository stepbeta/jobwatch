package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Welcome to the Jungle (WTTJ) company careers pages are server-rendered
// shells: the offer list is served by an Algolia index whose credentials
// are published per page by the site's own bootstrap endpoint. The two
// calls are documented in docs/SOURCE-DECISIONS.md.
const (
	wttjSiteURL  = "https://www.welcometothejungle.com"
	wttjPagesURL = "https://api.welcometothejungle.com/api/v1/pages"
	// wttjReferer is required: the Algolia secured key is
	// referrer-restricted and omitting it makes Algolia answer 403.
	wttjReferer = "https://www.welcometothejungle.com/"
	// wttjBrowserUA is a normal desktop browser UA. The bootstrap
	// endpoint and Algolia are reachable with it; everywhere else the
	// shared client keeps sending the identifying JobWatch UA.
	wttjBrowserUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	wttjHitsPerPage = 50
	wttjMaxPages    = 100 // runaway guard for a corrupt nbPages
)

// ErrNotCareersPage marks a bootstrap response with no usable
// jobs-search block: the URL is not a WTTJ company careers page. It is
// permanent (retrying cannot help). Network and JSON decoding failures
// are ordinary, retryable errors.
var ErrNotCareersPage = errors.New("URL is not a WTTJ careers page")

// WTTJ fetches the offers of a Welcome to the Jungle company careers
// page. Every scan performs one in-memory discovery call (bootstrap
// page tree) and then one Algolia multi-query per result page.
type WTTJ struct {
	Client *Client
	// PagesURL and AlgoliaBaseURL override the public endpoints; tests
	// point them at httptest servers. Empty means the real endpoints.
	PagesURL       string
	AlgoliaBaseURL string
}

// wttjFilter is one Algolia facet filter, e.g. organization.slug:fabriq.
type wttjFilter struct {
	Attribute, Value string
}

// wttjSearch is what discovery yields for one careers page.
type wttjSearch struct {
	AppID, APIKey, Index string
	Language             string
	OrgSlug              string // organization.slug: WTTJ job URL segment
	Filters              []wttjFilter
}

// wttjOffice is one hit location.
type wttjOffice struct {
	City    string `json:"city"`
	Country string `json:"country"`
}

// wttjHit is the subset of an Algolia hit the mapper reads. Two index
// shapes are known: reference/objectID may be a business id or a CMS
// row id, organizations may or may not carry a slug, and locations may
// come as offices[] or office. Extra hit fields (salary, remote,
// contract_type, sectors, highlights) have no offer column in this
// milestone and are deliberately not decoded.
type wttjHit struct {
	ObjectID     json.RawMessage `json:"objectID"` // string or number
	Reference    string          `json:"reference"`
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	Language     string          `json:"language"`
	PublishedAt  string          `json:"published_at"`
	Profile      string          `json:"profile"`
	Summary      string          `json:"summary"`
	Offices      []wttjOffice    `json:"offices"`
	Office       *wttjOffice     `json:"office"`
	Organization struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"organization"`
}

type wttjQuery struct {
	IndexName string `json:"indexName"`
	Params    string `json:"params"`
}

type wttjQueryRequest struct {
	Requests []wttjQuery `json:"requests"`
}

type wttjResult struct {
	Hits    []wttjHit `json:"hits"`
	Page    int       `json:"page"`
	NbPages int       `json:"nbPages"`
	NbHits  int       `json:"nbHits"`
}

type wttjAlgoliaResponse struct {
	Results []wttjResult `json:"results"`
	Message string       `json:"message"`
}

// Fetch discovers the credentials of the careers page behind src and
// pages through its Algolia results.
func (w *WTTJ) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	pageURL := firstNonEmpty(cfg.URL, src.URL)
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("wttj: %q is not an http(s) careers page URL", pageURL)
	}

	search, err := w.discover(ctx, src, u, cfg)
	if err != nil {
		return nil, err
	}

	out := make([]RawOffer, 0, wttjHitsPerPage)
	for page := 0; page < wttjMaxPages; page++ {
		res, err := w.query(ctx, src, search, page)
		if err != nil {
			return nil, err
		}
		for _, hit := range res.Hits {
			if offer, ok := mapWTTJHit(hit, search); ok {
				out = append(out, finalizeOffer(offer, cfg.Company))
			}
		}
		// Algolia reports nbPages once per query; stop after the last
		// page or an empty one, whichever comes first. Exceeding the
		// guard is an error rather than a silent truncation, which would
		// make the scan mark the missing offers disappeared.
		if page+1 >= wttjMaxPages && res.NbPages > wttjMaxPages {
			return nil, fmt.Errorf("wttj: %d result pages exceed the %d page limit", res.NbPages, wttjMaxPages)
		}
		if len(res.Hits) == 0 || page+1 >= res.NbPages {
			break
		}
	}
	return out, nil
}

// discover calls the page bootstrap and returns the search settings.
// Config overrides for the Algolia app id and key are applied last: they
// exist for debugging only.
func (w *WTTJ) discover(ctx context.Context, src model.Source, pageURL *url.URL, cfg Config) (wttjSearch, error) {
	base := w.PagesURL
	if base == "" {
		base = wttjPagesURL
	}
	u := base + "?path=" + url.QueryEscape(pageURL.EscapedPath())
	body, err := w.Client.fetch(ctx, src, http.MethodGet, u, map[string]string{
		"Accept":     "application/json",
		"User-Agent": wttjBrowserUA,
	}, nil)
	if err != nil {
		return wttjSearch{}, fmt.Errorf("wttj: discover %s: %w", pageURL.Path, err)
	}

	search, err := parseWTTJPages(body)
	if err != nil {
		return wttjSearch{}, err
	}
	if search.Language == "" {
		search.Language = wttjPathLanguage(pageURL.Path)
	}
	if search.OrgSlug == "" {
		search.OrgSlug = wttjPathSlug(pageURL.Path)
	}
	if cfg.AlgoliaAppID != "" {
		search.AppID = cfg.AlgoliaAppID
	}
	if cfg.AlgoliaAPIKey != "" {
		search.APIKey = cfg.AlgoliaAPIKey
	}
	if search.AppID == "" || search.APIKey == "" || search.Index == "" {
		return wttjSearch{}, fmt.Errorf("wttj: jobs-search block has no Algolia credentials or index name")
	}
	return search, nil
}

// query performs one Algolia multi-query for the given result page.
func (w *WTTJ) query(ctx context.Context, src model.Source, search wttjSearch, page int) (wttjResult, error) {
	body, err := wttjQueryBody(search.Index, wttjAlgoliaParams(search.Filters, page))
	if err != nil {
		return wttjResult{}, err
	}
	base := w.AlgoliaBaseURL
	if base == "" {
		base = "https://" + search.AppID + "-dsn.algolia.net"
	}
	// The form content type with a JSON body is what the site's Algolia
	// JS client sends; Algolia parses the body as JSON regardless.
	var resp wttjAlgoliaResponse
	if err := w.Client.PostJSON(ctx, src, base+"/1/indexes/*/queries",
		"application/x-www-form-urlencoded", map[string]string{
			"x-algolia-application-id": search.AppID,
			"x-algolia-api-key":        search.APIKey,
			"Referer":                  wttjReferer,
			"User-Agent":               wttjBrowserUA,
		}, body, &resp); err != nil {
		return wttjResult{}, fmt.Errorf("wttj: algolia page %d: %w", page, err)
	}
	if len(resp.Results) == 0 {
		if resp.Message != "" {
			return wttjResult{}, fmt.Errorf("wttj: algolia page %d: %s", page, resp.Message)
		}
		return wttjResult{}, fmt.Errorf("wttj: algolia page %d returned no results", page)
	}
	return resp.Results[0], nil
}

// parseWTTJPages extracts the jobs-search settings from the bootstrap
// page tree. The credentials live in the properties of the
// kind=="jobs-search" block (nested in its contents on the live site,
// directly in properties on the documented shape), so the whole subtree
// is searched for the object that carries algolia_app_id.
func parseWTTJPages(body []byte) (wttjSearch, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return wttjSearch{}, fmt.Errorf("wttj: decode page tree: %w", err)
	}
	props, ok := findJobSearchProperties(root)
	if !ok {
		return wttjSearch{}, fmt.Errorf("wttj: %w", ErrNotCareersPage)
	}
	return wttjSearchFromProperties(props)
}

// findJobSearchProperties walks the tree (map keys in sorted order, so
// the result is deterministic) and returns the first properties object
// carrying Algolia settings that sits inside a jobs-search block.
func findJobSearchProperties(v any) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if kind, _ := t["kind"].(string); kind == "jobs-search" {
			if p, ok := findMapWithKey(t, "algolia_app_id"); ok {
				return p, true
			}
		}
		for _, k := range sortedKeys(t) {
			if p, ok := findJobSearchProperties(t[k]); ok {
				return p, true
			}
		}
	case []any:
		for _, x := range t {
			if p, ok := findJobSearchProperties(x); ok {
				return p, true
			}
		}
	}
	return nil, false
}

func findMapWithKey(v any, key string) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if _, ok := t[key]; ok {
			return t, true
		}
		for _, k := range sortedKeys(t) {
			if p, ok := findMapWithKey(t[k], key); ok {
				return p, true
			}
		}
	case []any:
		for _, x := range t {
			if p, ok := findMapWithKey(x, key); ok {
				return p, true
			}
		}
	}
	return nil, false
}

func wttjSearchFromProperties(p map[string]any) (wttjSearch, error) {
	search := wttjSearch{
		AppID:    wttjString(p["algolia_app_id"]),
		APIKey:   wttjString(p["algolia_api_key"]),
		Index:    wttjString(p["algolia_index_name"]),
		Language: wttjString(p["language"]),
	}
	filters, err := wttjFilters(p["filters"])
	if err != nil {
		return wttjSearch{}, err
	}
	search.Filters = filters
	for _, f := range filters {
		if f.Attribute == "organization.slug" && search.OrgSlug == "" {
			search.OrgSlug = f.Value
		}
	}
	if search.OrgSlug == "" {
		if org, ok := p["website_organization"].(map[string]any); ok {
			search.OrgSlug = wttjString(org["slug"])
		}
	}
	return search, nil
}

// wttjFilters flattens the discovered filters object into ordered
// attribute/value pairs.
func wttjFilters(v any) ([]wttjFilter, error) {
	if v == nil {
		return nil, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New(`wttj: "filters" in the jobs-search block is not an object`)
	}
	out := make([]wttjFilter, 0, len(m))
	for _, k := range sortedKeys(m) {
		switch val := m[k].(type) {
		case string:
			if val == "" {
				return nil, fmt.Errorf("wttj: filter %q is empty", k)
			}
			out = append(out, wttjFilter{Attribute: k, Value: val})
		case []any:
			for _, x := range val {
				s, _ := x.(string)
				if s == "" {
					return nil, fmt.Errorf("wttj: filter %q holds a non-string value", k)
				}
				out = append(out, wttjFilter{Attribute: k, Value: s})
			}
		default:
			return nil, fmt.Errorf("wttj: filter %q has an unsupported value", k)
		}
	}
	return out, nil
}

// wttjAlgoliaParams serializes the discovered filters and paging into
// the `params` string of an Algolia multi-query request, e.g.
// "filters=organization.slug:fabriq&hitsPerPage=50&page=0".
func wttjAlgoliaParams(filters []wttjFilter, page int) string {
	paging := "hitsPerPage=" + strconv.Itoa(wttjHitsPerPage) + "&page=" + strconv.Itoa(page)
	if len(filters) == 0 {
		return paging
	}
	parts := make([]string, 0, len(filters))
	for _, f := range filters {
		parts = append(parts, f.Attribute+":"+f.Value)
	}
	return "filters=" + strings.Join(parts, " ") + "&" + paging
}

func wttjQueryBody(index, params string) ([]byte, error) {
	// The site's client sends the params unescaped; encoding with the
	// default HTML escaping would still be valid JSON but would send
	// \u0026 instead of &.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(wttjQueryRequest{Requests: []wttjQuery{{IndexName: index, Params: params}}}); err != nil {
		return nil, fmt.Errorf("wttj: encode query: %w", err)
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// mapWTTJHit converts one hit into a RawOffer. Hits without a title or
// slug cannot be tracked and are dropped by the caller (counted as
// skipped).
func mapWTTJHit(h wttjHit, search wttjSearch) (RawOffer, bool) {
	title := strings.TrimSpace(h.Name)
	slug := strings.TrimSpace(h.Slug)
	if slug == "" {
		return RawOffer{}, false
	}
	// reference is the stable business id. objectID is only a fallback:
	// on some indexes it is a numeric CMS row id.
	uid := strings.TrimSpace(h.Reference)
	if uid == "" {
		uid = wttjRawID(h.ObjectID)
	}
	org := strings.TrimSpace(h.Organization.Slug)
	if org == "" {
		org = search.OrgSlug
	}
	// The page locale from discovery builds every URL (verified to
	// resolve); the hit language is only a fallback.
	link := wttjOfferURL(firstNonEmpty(search.Language, h.Language, "en"), org, slug)
	if link == "" {
		return RawOffer{}, false
	}
	// profile is markdown; it is stored verbatim, summary is the
	// fallback when profile is absent or blank.
	desc := h.Profile
	if strings.TrimSpace(desc) == "" {
		desc = h.Summary
	}
	company := strings.TrimSpace(h.Organization.Name)
	key := companyKey(model.SourceWTTJ, org)
	return RawOffer{
		ExternalUID:      uid,
		Title:            title,
		Location:         wttjLocation(h),
		URL:              link,
		Description:      desc,
		Company:          company,
		CompanyKey:       key,
		IncompleteReason: incompleteReason(company, key, title),
	}, true
}

// wttjOfferURL builds the public job page URL, e.g.
// https://www.welcometothejungle.com/en/companies/fabriq/jobs/<slug>.
func wttjOfferURL(language, org, slug string) string {
	if org == "" || slug == "" {
		return ""
	}
	return wttjSiteURL + "/" + url.PathEscape(language) +
		"/companies/" + url.PathEscape(org) + "/jobs/" + url.PathEscape(slug)
}

// wttjLocation joins offices as "City, Country" ("; " between several
// offices) and falls back to the single office field, then to the
// city/country that is present.
func wttjLocation(h wttjHit) string {
	offices := h.Offices
	if len(offices) == 0 && h.Office != nil {
		offices = []wttjOffice{*h.Office}
	}
	parts := make([]string, 0, len(offices))
	for _, o := range offices {
		switch {
		case o.City != "" && o.Country != "":
			parts = append(parts, o.City+", "+o.Country)
		case o.City != "":
			parts = append(parts, o.City)
		case o.Country != "":
			parts = append(parts, o.Country)
		}
	}
	return strings.Join(parts, "; ")
}

// wttjRawID returns a JSON string or number as a plain string.
func wttjRawID(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if unq, err := strconv.Unquote(s); err == nil {
		return unq
	}
	return s
}

// wttjPathSlug returns the company slug of a careers path
// (/en/companies/fabriq/jobs -> "fabriq").
func wttjPathSlug(p string) string {
	segs := pathSegments(p)
	for i := 0; i+1 < len(segs); i++ {
		if segs[i] == "companies" {
			return segs[i+1]
		}
	}
	return ""
}

// WTTJSlug returns the company slug of a Welcome to the Jungle careers
// path (/en/companies/fabriq/jobs -> "fabriq"), or "" when the path is
// not a company page. It is used to name new sources.
func WTTJSlug(p string) string { return wttjPathSlug(p) }

// wttjPathLanguage returns a two-letter leading locale segment
// (/en/companies/... -> "en").
func wttjPathLanguage(p string) string {
	segs := pathSegments(p)
	if len(segs) == 0 || len(segs[0]) != 2 {
		return ""
	}
	for _, r := range segs[0] {
		if r < 'a' || r > 'z' {
			return ""
		}
	}
	return segs[0]
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func wttjString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
