package sources

import (
	"context"
	"fmt"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// RSS fetches a generic RSS 2.0 or Atom feed. Item detection and field
// mapping are configurable through config_json (see Config):
//
//	{"item":"item","title_field":"title","link_field":"link",
//	 "description_field":"description","location_field":"category",
//	 "company_field":"author","uid_field":"guid"}
//
// A field path is a chain of child element local names; "@name" reads an
// attribute and "#text" reads the element's own text. Defaults cover the
// common RSS and Atom shapes.
type RSS struct {
	Client *Client
}

func (r *RSS) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	body, err := r.Client.Get(ctx, src, src.URL, "application/rss+xml,application/atom+xml,application/xml,text/xml")
	if err != nil {
		return nil, fmt.Errorf("rss: %w", err)
	}
	root, err := parseXMLTree(body)
	if err != nil {
		return nil, fmt.Errorf("rss: %w", err)
	}
	items := rssItems(root, cfg)
	if len(items) == 0 {
		return []RawOffer{}, nil
	}
	base := src.URL
	out := make([]RawOffer, 0, len(items))
	for _, item := range items {
		offer, ok := rssOffer(item, cfg, base)
		if ok {
			out = append(out, offer)
		}
	}
	return out, nil
}

func rssItems(root *xmlNode, cfg Config) []*xmlNode {
	if item := strings.TrimSpace(cfg.Item); item != "" {
		return root.findAll(localName(item))
	}
	if items := root.findAll("item"); len(items) > 0 {
		return items
	}
	return root.findAll("entry")
}

func rssOffer(item *xmlNode, cfg Config, base string) (RawOffer, bool) {
	title := rssField(item, cfg.TitleField, "title")
	title = HTMLToText(title)
	if title == "" {
		return RawOffer{}, false
	}
	link := rssField(item, cfg.LinkField, "link")
	if link == "" {
		link = rssLinkHref(item)
	}
	if link == "" {
		link = rssField(item, cfg.UIDField, "guid")
	}
	link = strings.TrimSpace(link)
	if link == "" {
		return RawOffer{}, false
	}
	link = resolveURL(base, link)
	if link == "" {
		return RawOffer{}, false
	}
	desc := rssField(item, cfg.DescriptionField, "description")
	if desc == "" {
		desc = rssField(item, "", "summary")
	}
	if desc == "" {
		desc = rssField(item, "", "content")
	}
	uid := rssField(item, cfg.UIDField, "guid")
	if uid == "" {
		uid = rssField(item, "", "id")
	}
	if uid == "" {
		uid = link
	}
	return RawOffer{
		ExternalUID: uid,
		Title:       title,
		Location:    HTMLToText(rssField(item, cfg.LocationField, "")),
		URL:         link,
		Description: HTMLToText(desc),
		Company:     HTMLToText(rssField(item, cfg.CompanyField, "")),
	}, true
}

// rssField evaluates a configured path, or def when the config is empty.
func rssField(item *xmlNode, configured, def string) string {
	path := strings.TrimSpace(configured)
	if path == "" {
		path = def
	}
	if path == "" {
		return ""
	}
	return strings.TrimSpace(item.value(path))
}

// rssLinkHref handles Atom's <link rel="alternate" href="..."/> form and
// plain attribute-only link elements.
func rssLinkHref(item *xmlNode) string {
	for _, link := range item.childrenNamed("link") {
		href := strings.TrimSpace(link.attr("href"))
		if href == "" {
			continue
		}
		rel := strings.ToLower(strings.TrimSpace(link.attr("rel")))
		if rel == "" || rel == "alternate" {
			return href
		}
	}
	return ""
}
