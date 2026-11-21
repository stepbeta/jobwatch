package sources

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Personio fetches the public XML feed at
// https://{tenant}.jobs.personio.de/xml. The feed carries one <position>
// per published job; descriptions are optional HTML blocks.
type Personio struct {
	Client  *Client
	BaseURL string // empty means https://{tenant}.jobs.personio.{de,com}
}

type personioFeed struct {
	Positions []personioPosition `xml:"position"`
}

type personioPosition struct {
	ID             string                `xml:"id"`
	Office         string                `xml:"office"`
	Department     string                `xml:"department"`
	Name           string                `xml:"name"`
	JobDescription []personioDescription `xml:"jobDescriptions>jobDescription"`
	EmploymentType string                `xml:"employmentType"`
	Seniority      string                `xml:"seniority"`
	Schedule       string                `xml:"schedule"`
	CreatedAt      string                `xml:"createdAt"`
}

type personioDescription struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

func (p *Personio) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	tenant, suffix, err := personioTenant(src)
	if err != nil {
		return nil, err
	}
	base := p.BaseURL
	if base == "" {
		base = fmt.Sprintf("https://%s.jobs.personio.%s", tenant, suffix)
	}
	base = strings.TrimSuffix(base, "/")
	u := base + "/xml"
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		u += "?language=" + url.QueryEscape(lang)
	}

	body, err := p.Client.Get(ctx, src, u, "application/xml,text/xml")
	if err != nil {
		return nil, fmt.Errorf("personio %s: %w", tenant, err)
	}
	var feed personioFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("personio %s: decode xml: %w", tenant, err)
	}
	out := make([]RawOffer, 0, len(feed.Positions))
	key := companyKey(model.SourcePersonio, tenant)
	for _, pos := range feed.Positions {
		title := strings.TrimSpace(pos.Name)
		if strings.TrimSpace(pos.ID) == "" {
			continue
		}
		out = append(out, finalizeOffer(RawOffer{
			ExternalUID: strings.TrimSpace(pos.ID),
			Title:       title,
			Location:    strings.TrimSpace(pos.Office),
			URL:         fmt.Sprintf("%s/job/%s", base, url.PathEscape(strings.TrimSpace(pos.ID))),
			Description: personioDescriptionText(pos.JobDescription),
			CompanyKey:  key,
		}, cfg.Company))
	}
	return out, nil
}

func personioDescriptionText(parts []personioDescription) string {
	var out []string
	for _, part := range parts {
		body := HTMLToText(part.Value)
		switch {
		case strings.TrimSpace(part.Name) == "":
			if body != "" {
				out = append(out, body)
			}
		case body == "":
			out = append(out, strings.TrimSpace(part.Name))
		default:
			out = append(out, strings.TrimSpace(part.Name)+"\n"+body)
		}
	}
	return strings.Join(out, "\n\n")
}

// personioTenant returns the tenant subdomain and the feed suffix
// ("de" or "com") from config, detection or the source URL.
func personioTenant(src model.Source) (tenant, suffix string, err error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return "", "", err
	}
	if t := strings.TrimSpace(cfg.Tenant); t != "" {
		return t, personioSuffix(src.URL), nil
	}
	if typ, dc, ok := Detect(src.URL); ok && typ == model.SourcePersonio && dc.Tenant != "" {
		return dc.Tenant, personioSuffix(src.URL), nil
	}
	return "", "", fmt.Errorf("source %d: personio tenant not set in config_json and not derivable from url", src.ID)
}

func personioSuffix(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "de"
	}
	host := strings.ToLower(u.Hostname())
	if i := strings.Index(host, ".jobs.personio."); i >= 0 {
		return strings.TrimPrefix(host[i+len(".jobs.personio."):], ".")
	}
	return "de"
}
