package sources

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strconv"

	"github.com/stepbeta/jobwatch/internal/model"
)

const greenhouseBaseURL = "https://boards-api.greenhouse.io"

// Greenhouse fetches GET {base}/v1/boards/{board_token}/jobs?content=true.
type Greenhouse struct {
	Client  *Client
	BaseURL string // empty means the public API
}

type greenhouseResponse struct {
	Jobs []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		AbsoluteURL string `json:"absolute_url"`
		CompanyName string `json:"company_name"`
		Content     string `json:"content"` // HTML, entity-escaped once more
		Location    struct {
			Name string `json:"name"`
		} `json:"location"`
	} `json:"jobs"`
}

func (g *Greenhouse) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	token, err := boardKey(src, func(c Config) string { return c.BoardToken }, model.SourceGreenhouse, "board_token")
	if err != nil {
		return nil, err
	}
	base := g.BaseURL
	if base == "" {
		base = greenhouseBaseURL
	}
	u := fmt.Sprintf("%s/v1/boards/%s/jobs?content=true", base, url.PathEscape(token))

	var resp greenhouseResponse
	if err := g.Client.GetJSON(ctx, src, u, &resp); err != nil {
		return nil, fmt.Errorf("greenhouse %s: %w", token, err)
	}
	out := make([]RawOffer, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if j.Title == "" || j.AbsoluteURL == "" {
			continue
		}
		out = append(out, RawOffer{
			ExternalUID: strconv.FormatInt(j.ID, 10),
			Title:       j.Title,
			Location:    j.Location.Name,
			URL:         j.AbsoluteURL,
			Description: HTMLToText(html.UnescapeString(j.Content)),
			Company:     j.CompanyName,
		})
	}
	return out, nil
}

// boardKey returns the board identifier from config, falling back to
// detecting it from the source URL.
func boardKey(src model.Source, get func(Config) string, typ model.SourceType, field string) (string, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return "", err
	}
	if k := get(cfg); k != "" {
		return k, nil
	}
	if t, dc, ok := Detect(src.URL); ok && t == typ {
		if k := get(dc); k != "" {
			return k, nil
		}
	}
	return "", fmt.Errorf("source %d: %s %q not set in config_json and not derivable from url", src.ID, typ, field)
}
