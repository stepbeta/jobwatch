package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stepbeta/jobwatch/internal/importmeta"
)

type importResp struct {
	Error    errorDetail         `json:"error"`
	Metadata importmeta.Metadata `json:"metadata"`
}

func importPage(t *testing.T, page string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nAllow: /\n"))
		case "/job":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page))
		case "/empty":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head></head><body>nothing</body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestImportOfferFromURL(t *testing.T) {
	ts := newTestServer(t)
	page := `<!doctype html><html><head>
	  <script type="application/ld+json">{"@context":"https://schema.org","@type":"JobPosting",
	    "title":"Site Reliability Engineer","hiringOrganization":{"name":"Qonto"},
	    "jobLocation":{"address":{"addressLocality":"Paris","addressCountry":"France"}},
	    "description":"<p>Run Kubernetes and Terraform.</p>"}</script>
	</head></html>`
	srv := importPage(t, page)

	rec := ts.do(t, "POST", "/api/offers/import", map[string]any{"url": srv.URL + "/job"})
	wantCode(t, rec, 201)
	created := decodeBody[struct {
		OfferID int64 `json:"offer_id"`
		Created bool  `json:"created"`
	}](t, rec)
	if !created.Created || created.OfferID == 0 {
		t.Fatalf("import = %+v", created)
	}

	rec = ts.do(t, "GET", "/api/offers/"+strconv.FormatInt(created.OfferID, 10), nil)
	wantCode(t, rec, 200)
	detail := decodeBody[offerDetailResp](t, rec)
	if detail.Offer.CompanyName != "Qonto" || detail.Offer.Title != "Site Reliability Engineer" {
		t.Errorf("offer = %+v", detail.Offer)
	}
	if detail.Offer.Location == nil || *detail.Offer.Location != "Paris, France" {
		t.Errorf("location = %v", detail.Offer.Location)
	}
	if len(detail.Provenance) != 1 || detail.Provenance[0].URL != srv.URL+"/job" {
		t.Errorf("provenance = %+v", detail.Provenance)
	}
	if detail.Offer.TitleAffinity != 85 || detail.Offer.LocationAffinity != 60 {
		t.Errorf("read-time affinity = %d/%d, want 85/60", detail.Offer.TitleAffinity, detail.Offer.LocationAffinity)
	}
	if len(detail.Offer.MatchReasons.Title) == 0 || len(detail.Offer.MatchReasons.Location) == 0 {
		t.Errorf("match reasons missing: %+v", detail.Offer.MatchReasons)
	}

	// Explicit fields override extracted ones and skip the fetch: this
	// unreachable host would fail if a metadata request were attempted.
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{
		"url": "http://127.0.0.1:1/job", "company": "Other", "title": "Platform Engineer",
	})
	wantCode(t, rec, 201)
}

func TestImportOfferFromURLIncomplete(t *testing.T) {
	ts := newTestServer(t)
	srv := importPage(t, `<html><head><title>Acme careers</title></head></html>`)

	rec := ts.do(t, "POST", "/api/offers/import", map[string]any{"url": srv.URL + "/empty"})
	wantErrorCode(t, rec, 422, "metadata_incomplete")
	body := decodeBody[importResp](t, rec)
	if body.Metadata.URL != srv.URL+"/empty" {
		t.Errorf("metadata url = %q", body.Metadata.URL)
	}

	// A page with a title but no company also needs the manual form.
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{"url": srv.URL + "/job"})
	wantErrorCode(t, rec, 422, "metadata_incomplete")

	// Metadata can be completed by the user in the same request.
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{
		"url": srv.URL + "/empty", "company": "Acme", "title": "SRE",
	})
	wantCode(t, rec, 201)

	// A dead host falls back to metadata_unavailable.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	rec = ts.do(t, "POST", "/api/offers/import", map[string]any{"url": deadURL + "/job"})
	wantErrorCode(t, rec, 422, "metadata_unavailable")
	if !strings.Contains(decodeBody[importResp](t, rec).Error.Message, "fetch") {
		t.Errorf("message = %q", decodeBody[importResp](t, rec).Error.Message)
	}
}
