package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// BambooHR fetches the public careers listing endpoint:
// GET https://{subdomain}.bamboohr.com/careers/list.
//
// The XML feed variant ({subdomain}.bamboohr.com/careers/jobs_feed.xml)
// answers 404, so the JSON listing is the only supported surface. The
// listing carries the posting id, title, department and structured
// location but no description; descriptions would need one additional
// request per posting, which scans do not make.
type BambooHR struct {
	Client  *Client
	BaseURL string // empty means https://{subdomain}.bamboohr.com
}

type bambooHRResponse struct {
	Meta struct {
		TotalCount int `json:"totalCount"`
	} `json:"meta"`
	Result []bambooHRJob `json:"result"`
}

type bambooHRJob struct {
	ID              string           `json:"id"`
	JobOpeningName  string           `json:"jobOpeningName"`
	DepartmentLabel string           `json:"departmentLabel"`
	Location        bambooHRLocation `json:"location"`
	ATSLocation     bambooHRLocation `json:"atsLocation"`
	IsRemote        bool             `json:"isRemote"`
}

type bambooHRLocation struct {
	City     string `json:"city"`
	State    string `json:"state"`
	Province string `json:"province"`
	Country  string `json:"country"`
}

func (b *BambooHR) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	subdomain, err := bambooHRSubdomain(src)
	if err != nil {
		return nil, err
	}
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	base := b.BaseURL
	if base == "" {
		base = fmt.Sprintf("https://%s.bamboohr.com", subdomain)
	}
	base = strings.TrimSuffix(base, "/")

	var resp bambooHRResponse
	if err := b.Client.GetJSON(ctx, src, base+"/careers/list", &resp); err != nil {
		return nil, fmt.Errorf("bamboohr %s: %w", subdomain, err)
	}

	key := companyKey(model.SourceBambooHR, subdomain)
	out := make([]RawOffer, 0, len(resp.Result))
	for _, job := range resp.Result {
		id := strings.TrimSpace(job.ID)
		if id == "" {
			continue
		}
		// DepartmentLabel is available but RawOffer has no department
		// field; it is deliberately not mixed into the description.
		out = append(out, finalizeOffer(RawOffer{
			ExternalUID: id,
			Title:       strings.TrimSpace(job.JobOpeningName),
			Location:    bambooHRJobLocation(job),
			URL:         fmt.Sprintf("%s/careers/%s", base, url.PathEscape(id)),
			CompanyKey:  key,
		}, cfg.Company))
	}
	return out, nil
}

// bambooHRSubdomain returns the tenant subdomain from config_json or the
// source URL.
func bambooHRSubdomain(src model.Source) (string, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return "", err
	}
	if s := strings.TrimSpace(cfg.Subdomain); s != "" {
		return s, nil
	}
	if typ, dc, ok := Detect(src.URL); ok && typ == model.SourceBambooHR && dc.Subdomain != "" {
		return dc.Subdomain, nil
	}
	return "", fmt.Errorf("source %d: bamboohr subdomain not set in config_json and not derivable from url", src.ID)
}

// bambooHRJobLocation joins the populated location fields from both the
// legacy `location` object and the structured `atsLocation`, dropping
// empty and redundant parts (e.g. a state that repeats the city).
func bambooHRJobLocation(job bambooHRJob) string {
	var parts []string
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		lower := strings.ToLower(p)
		for _, existing := range parts {
			el := strings.ToLower(existing)
			// Drop a new part that merely repeats an earlier one (state
			// "London, City of" after city "London"), but keep a broader
			// part such as the country after province "ile de France".
			if el == lower || strings.Contains(lower, el) {
				return
			}
		}
		parts = append(parts, p)
	}
	for _, loc := range []bambooHRLocation{job.Location, job.ATSLocation} {
		add(loc.City)
		add(loc.Province)
		add(loc.State)
		add(loc.Country)
	}
	label := strings.Join(parts, ", ")
	if job.IsRemote && !strings.Contains(strings.ToLower(label), "remote") {
		if label == "" {
			return "Remote"
		}
		label += " (remote)"
	}
	return label
}
