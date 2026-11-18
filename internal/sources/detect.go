package sources

import (
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Detect infers the source type and per-type config from a public board
// URL (e.g. https://boards.greenhouse.io/acme, https://jobs.lever.co/acme,
// https://jobs.ashbyhq.com/acme) or from the matching API URL. ok is
// false when the URL is not recognized.
func Detect(rawURL string) (typ model.SourceType, cfg Config, ok bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return "", Config{}, false
	}
	host := strings.ToLower(u.Hostname())
	segs := pathSegments(u.Path)

	switch {
	case host == "boards-api.greenhouse.io":
		// /v1/boards/{token}/...
		if len(segs) >= 3 && segs[0] == "v1" && segs[1] == "boards" {
			return model.SourceGreenhouse, Config{BoardToken: segs[2]}, true
		}
	case host == "my.greenhouse.io" || host == "www.greenhouse.io":
		// The greenhouse.io corporate site and the candidate dashboard
		// are not job boards.
		return "", Config{}, false
	case strings.HasSuffix(host, "greenhouse.io"):
		if len(segs) >= 1 && segs[0] == "embed" {
			if t := u.Query().Get("for"); t != "" {
				return model.SourceGreenhouse, Config{BoardToken: t}, true
			}
			return "", Config{}, false
		}
		if len(segs) >= 1 {
			return model.SourceGreenhouse, Config{BoardToken: segs[0]}, true
		}
	case host == "api.lever.co" || host == "api.eu.lever.co":
		// /v0/postings/{site}
		if len(segs) >= 3 && segs[0] == "v0" && segs[1] == "postings" {
			return model.SourceLever, Config{Site: segs[2], Region: leverRegion(host)}, true
		}
	case host == "jobs.lever.co" || host == "jobs.eu.lever.co":
		if len(segs) >= 1 {
			return model.SourceLever, Config{Site: segs[0], Region: leverRegion(host)}, true
		}
	case host == "api.ashbyhq.com":
		// /posting-api/job-board/{org}
		if len(segs) >= 3 && segs[0] == "posting-api" && segs[1] == "job-board" {
			return model.SourceAshby, Config{Org: segs[2]}, true
		}
	case host == "jobs.ashbyhq.com":
		if len(segs) >= 1 {
			return model.SourceAshby, Config{Org: segs[0]}, true
		}
	case strings.Contains(host, ".jobs.personio."):
		if i := strings.Index(host, ".jobs.personio."); i > 0 {
			return model.SourcePersonio, Config{Tenant: host[:i]}, true
		}
	case host == "apply.workable.com":
		// /api/v1/widget/accounts/{account} or /{account}/...
		if len(segs) >= 5 && segs[0] == "api" && segs[1] == "v1" && segs[2] == "widget" && segs[3] == "accounts" {
			return model.SourceWorkable, Config{Account: segs[4]}, true
		}
		if len(segs) >= 1 && segs[0] != "j" {
			return model.SourceWorkable, Config{Account: segs[0]}, true
		}
	case strings.HasSuffix(host, ".workable.com") && host != "www.workable.com" && host != "jobs.workable.com":
		return model.SourceWorkable, Config{Account: strings.TrimSuffix(host, ".workable.com")}, true
	case strings.HasSuffix(host, ".bamboohr.com") && host != "www.bamboohr.com" && len(segs) >= 1 && segs[0] == "careers":
		// /careers, /careers/list and /careers/{id} are all the tenant board.
		return model.SourceBambooHR, Config{Subdomain: strings.TrimSuffix(host, ".bamboohr.com")}, true
	case (host == "www.welcometothejungle.com" || host == "welcometothejungle.com") &&
		strings.Contains(u.Path, "/companies/"):
		// Company careers page, e.g. /en/companies/fabriq/jobs. The
		// marketplace paths (/en/jobs, /fr/jobs-matches, ...) are not
		// company pages and stay unrecognized.
		return model.SourceWTTJ, Config{}, true
	}
	return "", Config{}, false
}

func leverRegion(host string) string {
	if strings.Contains(host, ".eu.") {
		return "eu"
	}
	return ""
}

func pathSegments(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
