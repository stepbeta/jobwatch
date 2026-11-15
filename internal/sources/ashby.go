package sources

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

const ashbyBaseURL = "https://api.ashbyhq.com"

// Ashby fetches GET {base}/posting-api/job-board/{org}. Unlisted jobs
// (isListed=false) are skipped.
type Ashby struct {
	Client  *Client
	BaseURL string // empty means the public API
}

type ashbyResponse struct {
	Jobs []struct {
		ID                 string `json:"id"`
		Title              string `json:"title"`
		Location           string `json:"location"`
		SecondaryLocations []struct {
			Location string `json:"location"`
		} `json:"secondaryLocations"`
		IsListed         *bool  `json:"isListed"`
		JobURL           string `json:"jobUrl"`
		DescriptionPlain string `json:"descriptionPlain"`
		DescriptionHTML  string `json:"descriptionHtml"`
	} `json:"jobs"`
}

func (a *Ashby) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	org, err := boardKey(src, func(c Config) string { return c.Org }, model.SourceAshby, "org")
	if err != nil {
		return nil, err
	}
	base := a.BaseURL
	if base == "" {
		base = ashbyBaseURL
	}
	u := fmt.Sprintf("%s/posting-api/job-board/%s", base, url.PathEscape(org))

	var resp ashbyResponse
	if err := a.Client.GetJSON(ctx, src, u, &resp); err != nil {
		return nil, fmt.Errorf("ashby %s: %w", org, err)
	}
	out := make([]RawOffer, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if j.IsListed != nil && !*j.IsListed {
			continue
		}
		if j.Title == "" || j.JobURL == "" {
			continue
		}
		locs := []string{}
		if j.Location != "" {
			locs = append(locs, j.Location)
		}
		for _, s := range j.SecondaryLocations {
			if s.Location != "" {
				locs = append(locs, s.Location)
			}
		}
		desc := strings.TrimSpace(j.DescriptionPlain)
		if desc == "" {
			desc = HTMLToText(j.DescriptionHTML)
		}
		uid := j.ID
		if uid == "" {
			uid = path.Base(strings.TrimSuffix(j.JobURL, "/"))
		}
		out = append(out, RawOffer{
			ExternalUID: uid,
			Title:       j.Title,
			Location:    strings.Join(locs, "; "),
			URL:         j.JobURL,
			Description: desc,
		})
	}
	return out, nil
}
