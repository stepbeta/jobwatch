package importmeta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/stepbeta/jobwatch/internal/sources"
)

func parseDoc(t *testing.T, body string) *xhtml.Node {
	t.Helper()
	doc, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestExtractHTML(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Metadata
	}{
		{
			name: "json-ld job posting",
			body: `<!doctype html><html><head>
			  <script type="application/ld+json">{"@context":"https://schema.org","@type":"JobPosting",
			    "title":"Site Reliability Engineer","hiringOrganization":{"@type":"Organization","name":"Acme"},
			    "jobLocation":{"@type":"Place","address":{"addressLocality":"Paris","addressRegion":"Ile-de-France","addressCountry":{"name":"France"}}},
			    "description":"<p>Run Kubernetes and Terraform.</p>","url":"/jobs/42"}</script>
			</head><body></body></html>`,
			want: Metadata{
				Title:       "Site Reliability Engineer",
				Company:     "Acme",
				Location:    "Paris, Ile-de-France, France",
				URL:         "/jobs/42",
				Description: "Run Kubernetes and Terraform.",
			},
		},
		{
			name: "json-ld graph",
			body: `<html><head><script type="application/ld+json">
			  {"@graph":[{"@type":"Organization","name":"Other"},{"@type":["JobPosting"],"title":"Platform Engineer","hiringOrganization":"Other Co"}]}
			</script></head></html>`,
			want: Metadata{Title: "Platform Engineer", Company: "Other Co"},
		},
		{
			name: "open graph fallback",
			body: `<html><head>
			  <meta property="og:title" content="SRE (H/F)">
			  <meta property="og:site_name" content="Qonto">
			  <meta property="og:description" content="Full remote platform role in France">
			</head><body><h1>ignored because og:title wins</h1></body></html>`,
			want: Metadata{
				Title:       "SRE (H/F)",
				Company:     "Qonto",
				Description: "Full remote platform role in France",
			},
		},
		{
			name: "title tag and meta description",
			body: `<html><head><title>Platform Engineer - Acme</title>
			  <meta name="description" content="Build platforms"></head></html>`,
			want: Metadata{Title: "Platform Engineer - Acme", Description: "Build platforms"},
		},
		{
			name: "empty page",
			body: `<html><head></head><body><div>hello</div></body></html>`,
			want: Metadata{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHTML(parseDoc(t, tt.body))
			if got != tt.want {
				t.Errorf("extractHTML = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestExtractFetch(t *testing.T) {
	page := `<!doctype html><html><head>
	  <meta property="og:title" content="SRE">
	  <meta property="og:site_name" content="Acme">
	</head></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/job":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := sources.NewClient()
	client.DefaultInterval = 0
	meta, err := Extract(context.Background(), client, srv.URL+"/job")
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if meta.Title != "SRE" || meta.Company != "Acme" || meta.URL != srv.URL+"/job" {
		t.Errorf("metadata = %+v", meta)
	}

	// A robots.txt disallow is a fetch error, not a silent empty result.
	blocked := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /\n"))
			return
		}
		w.Write([]byte(page))
	}))
	defer blocked.Close()
	if _, err := Extract(context.Background(), client, blocked.URL+"/job"); err == nil {
		t.Error("robots-disallowed page was fetched")
	}

	if _, err := Extract(context.Background(), client, "ftp://example.com/x"); err == nil {
		t.Error("non-http URL accepted")
	}
}
