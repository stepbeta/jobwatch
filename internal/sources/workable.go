package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

const workableBaseURL = "https://apply.workable.com"

// Workable fetches the public (keyless) careers widget endpoint:
// GET https://apply.workable.com/api/v1/widget/accounts/{account}?details=true.
// A job open in several places appears once per location entry; entries
// are grouped by shortcode so the offer is reported once.
type Workable struct {
	Client  *Client
	BaseURL string // empty means the public widget host
}

type workableResponse struct {
	Name string        `json:"name"`
	Jobs []workableJob `json:"jobs"`
}

type workableJob struct {
	Title          string             `json:"title"`
	Shortcode      string             `json:"shortcode"`
	Code           string             `json:"code"`
	EmploymentType string             `json:"employment_type"`
	Telecommuting  bool               `json:"telecommuting"`
	URL            string             `json:"url"`
	Shortlink      string             `json:"shortlink"`
	ApplicationURL string             `json:"application_url"`
	City           string             `json:"city"`
	State          string             `json:"state"`
	Country        string             `json:"country"`
	Description    string             `json:"description"`
	Locations      []workableLocation `json:"locations"`
}

type workableLocation struct {
	City        string `json:"city"`
	Region      string `json:"region"`
	State       string `json:"state"`
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

func (wk *Workable) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	account, err := workableAccount(src)
	if err != nil {
		return nil, err
	}
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	base := wk.BaseURL
	if base == "" {
		base = workableBaseURL
	}
	base = strings.TrimSuffix(base, "/")
	u := fmt.Sprintf("%s/api/v1/widget/accounts/%s?details=true", base, url.PathEscape(account))

	var resp workableResponse
	if err := wk.Client.GetJSON(ctx, src, u, &resp); err != nil {
		return nil, fmt.Errorf("workable %s: %w", account, err)
	}
	// Keep the first position of each shortcode and merge its locations.
	type grouped struct {
		job       workableJob
		locations []string
	}
	order := make([]string, 0, len(resp.Jobs))
	groups := make(map[string]*grouped, len(resp.Jobs))
	for _, j := range resp.Jobs {
		key := strings.TrimSpace(j.Shortcode)
		if key == "" {
			key = strings.TrimSpace(j.URL)
		}
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &grouped{job: j}
			groups[key] = g
			order = append(order, key)
		}
		g.locations = append(g.locations, workableLocations(j)...)
	}

	companyID := companyKey(model.SourceWorkable, account)
	out := make([]RawOffer, 0, len(order))
	for _, key := range order {
		g := groups[key]
		j := g.job
		link := workableJobURL(base, account, j)
		if link == "" {
			continue
		}
		out = append(out, finalizeOffer(RawOffer{
			ExternalUID: strings.TrimSpace(j.Shortcode),
			Title:       strings.TrimSpace(j.Title),
			Location:    joinLocations(g.locations, j.Telecommuting),
			URL:         link,
			Description: HTMLToText(j.Description),
			Company:     strings.TrimSpace(resp.Name),
			CompanyKey:  companyID,
		}, cfg.Company))
	}
	return out, nil
}

func workableAccount(src model.Source) (string, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return "", err
	}
	if a := strings.TrimSpace(cfg.Account); a != "" {
		return a, nil
	}
	if typ, dc, ok := Detect(src.URL); ok && typ == model.SourceWorkable && dc.Account != "" {
		return dc.Account, nil
	}
	return "", fmt.Errorf("source %d: workable account not set in config_json and not derivable from url", src.ID)
}

func workableJobURL(base, account string, j workableJob) string {
	for _, raw := range []string{j.URL, j.Shortlink, j.ApplicationURL} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if u, err := url.Parse(raw); err == nil && u.IsAbs() && (u.Scheme == "http" || u.Scheme == "https") {
			return strings.TrimSuffix(u.String(), "/apply")
		}
		if rel, err := url.Parse(raw); err == nil {
			if abs := resolveURL(base+"/", rel.String()); abs != "" {
				return strings.TrimSuffix(abs, "/apply")
			}
		}
	}
	if key := strings.TrimSpace(j.Shortcode); key != "" {
		return fmt.Sprintf("%s/j/%s", base, url.PathEscape(key))
	}
	return ""
}

func workableLocations(j workableJob) []string {
	var out []string
	for _, l := range j.Locations {
		if s := locationLabel(l.City, firstNonEmpty(l.Region, l.State), l.Country); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		if s := locationLabel(j.City, j.State, j.Country); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func locationLabel(city, region, country string) string {
	var parts []string
	for _, p := range []string{city, region, country} {
		if s := strings.TrimSpace(p); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// joinLocations deduplicates location labels and marks telecommuting
// postings, which the classifier uses to derive the remote policy.
func joinLocations(locations []string, remote bool) string {
	seen := map[string]bool{}
	var out []string
	for _, l := range locations {
		l = strings.TrimSpace(l)
		key := strings.ToLower(l)
		if l == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, l)
	}
	loc := strings.Join(out, "; ")
	if remote && !strings.Contains(strings.ToLower(loc), "remote") {
		if loc == "" {
			return "Remote"
		}
		loc += " (remote)"
	}
	return loc
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveURL resolves a possibly relative reference against base.
func resolveURL(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	abs := b.ResolveReference(r)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	return abs.String()
}
