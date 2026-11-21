package sources

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

const (
	leverBaseURL   = "https://api.lever.co"
	leverEUBaseURL = "https://api.eu.lever.co"
)

// Lever fetches GET {base}/v0/postings/{site}?mode=json.
type Lever struct {
	Client  *Client
	BaseURL string // empty means the public API for the configured region
}

type leverPosting struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	HostedURL  string `json:"hostedUrl"`
	Categories struct {
		Location     string   `json:"location"`
		AllLocations []string `json:"allLocations"`
	} `json:"categories"`
	DescriptionPlain string `json:"descriptionPlain"`
	Lists            []struct {
		Text    string `json:"text"`
		Content string `json:"content"` // HTML
	} `json:"lists"`
	AdditionalPlain string `json:"additionalPlain"`
}

func (l *Lever) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	site, err := boardKey(src, func(c Config) string { return c.Site }, model.SourceLever, "site")
	if err != nil {
		return nil, err
	}
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	base := l.BaseURL
	if base == "" {
		base = leverBaseURL
		region := cfg.Region
		if region == "" {
			_, dc, _ := Detect(src.URL)
			region = dc.Region
		}
		if region == "eu" {
			base = leverEUBaseURL
		}
	}
	u := fmt.Sprintf("%s/v0/postings/%s?mode=json", base, url.PathEscape(site))

	var postings []leverPosting
	if err := l.Client.GetJSON(ctx, src, u, &postings); err != nil {
		return nil, fmt.Errorf("lever %s: %w", site, err)
	}
	key := companyKey(model.SourceLever, site)
	out := make([]RawOffer, 0, len(postings))
	for _, p := range postings {
		if p.HostedURL == "" {
			continue
		}
		loc := p.Categories.Location
		if len(p.Categories.AllLocations) > 1 {
			loc = strings.Join(p.Categories.AllLocations, "; ")
		}
		out = append(out, finalizeOffer(RawOffer{
			ExternalUID: p.ID,
			Title:       p.Text,
			Location:    loc,
			URL:         p.HostedURL,
			Description: leverDescription(p),
			CompanyKey:  key,
		}, cfg.Company))
	}
	return out, nil
}

func leverDescription(p leverPosting) string {
	var parts []string
	if d := strings.TrimSpace(p.DescriptionPlain); d != "" {
		parts = append(parts, d)
	}
	for _, l := range p.Lists {
		body := HTMLToText(l.Content)
		if l.Text == "" && body == "" {
			continue
		}
		parts = append(parts, strings.TrimSpace(l.Text+"\n"+body))
	}
	if a := strings.TrimSpace(p.AdditionalPlain); a != "" {
		parts = append(parts, a)
	}
	return strings.Join(parts, "\n\n")
}
