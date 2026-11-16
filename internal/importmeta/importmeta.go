// Package importmeta extracts lightweight job metadata (title, company,
// location, description) from an arbitrary job page for the manual import
// flow. It uses meta tags, JSON-LD JobPosting blocks and the page title,
// and never runs JavaScript.
package importmeta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"

	xhtml "golang.org/x/net/html"

	"github.com/stepbeta/jobwatch/internal/model"
	"github.com/stepbeta/jobwatch/internal/sources"
)

// maxDescription caps the extracted text stored on an offer.
const maxDescription = 200_000

// Metadata is what could be extracted from a page. Missing fields are
// left empty so the caller can fall back to a manual form.
type Metadata struct {
	Company     string `json:"company"`
	Title       string `json:"title"`
	Location    string `json:"location"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// Extract fetches rawURL politely and extracts job metadata. The returned
// Metadata is always populated with the input URL; other fields may be
// empty. An error means the page could not be fetched or parsed, in which
// case the caller should offer the manual form.
func Extract(ctx context.Context, client *sources.Client, rawURL string) (Metadata, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Metadata{}, fmt.Errorf("import metadata: %q is not an http(s) URL", rawURL)
	}
	out := Metadata{URL: rawURL}
	body, err := client.Get(ctx, model.Source{}, rawURL, "text/html,application/xhtml+xml")
	if err != nil {
		return out, fmt.Errorf("import metadata: %w", err)
	}
	doc, err := xhtml.Parse(bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("import metadata: parse page: %w", err)
	}
	meta := extractHTML(doc)
	if meta.URL == "" {
		meta.URL = rawURL
	} else if abs := absoluteURL(rawURL, meta.URL); abs != "" {
		meta.URL = abs
	}
	if meta.Title == "" && meta.Company == "" && meta.Description == "" {
		// Keep the fetch successful: the UI only needs to know that it
		// must ask the user for the missing fields.
		return meta, nil
	}
	if meta.URL == "" {
		meta.URL = rawURL
	}
	return meta, nil
}

// extractHTML is separated from Extract so it can be tested directly.
func extractHTML(doc *xhtml.Node) Metadata {
	var out Metadata
	applyJSONLD(doc, &out)
	og := map[string]string{}
	for _, n := range allNodes(doc) {
		if n.Type != xhtml.ElementNode || !strings.EqualFold(n.Data, "meta") {
			continue
		}
		key := strings.ToLower(attr(n, "property"))
		if key == "" {
			key = strings.ToLower(attr(n, "name"))
		}
		if v := strings.TrimSpace(attr(n, "content")); key != "" && v != "" {
			og[key] = v
		}
	}
	if out.Title == "" {
		out.Title = firstNonEmpty(og["og:title"], nodeText(findFirst(doc, "title")), nodeText(findFirst(doc, "h1")))
	}
	if out.Company == "" {
		out.Company = og["og:site_name"]
	}
	if out.Description == "" {
		out.Description = firstNonEmpty(og["og:description"], og["description"])
	}
	if out.URL == "" {
		out.URL = og["og:url"]
	}
	out.Title = cleanText(out.Title)
	out.Company = cleanText(out.Company)
	out.Location = cleanText(out.Location)
	out.Description = truncate(cleanDescription(out.Description), maxDescription)
	return out
}

func applyJSONLD(doc *xhtml.Node, out *Metadata) {
	for _, n := range allNodes(doc) {
		if n.Type != xhtml.ElementNode || !strings.EqualFold(n.Data, "script") {
			continue
		}
		if !strings.Contains(strings.ToLower(attr(n, "type")), "ld+json") {
			continue
		}
		raw := strings.TrimSpace(textContent(n))
		if raw == "" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			continue
		}
		var posting map[string]any
		findJobPosting(v, &posting)
		if posting == nil {
			continue
		}
		if out.Title == "" {
			out.Title = asString(posting["title"])
		}
		if out.Company == "" {
			out.Company = organizationName(posting["hiringOrganization"])
		}
		if out.Location == "" {
			out.Location = jobLocation(posting["jobLocation"])
		}
		if out.Description == "" {
			out.Description = sources.HTMLToText(asString(posting["description"]))
		}
		if out.URL == "" {
			out.URL = asString(posting["url"])
		}
	}
}

// findJobPosting walks decoded JSON-LD (objects, arrays, @graph) until a
// node whose @type contains JobPosting is found.
func findJobPosting(v any, out *map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		if typeContains(t["@type"], "JobPosting") {
			*out = t
			return
		}
		for _, key := range []string{"@graph", "itemListElement", "mainEntity"} {
			if child, ok := t[key]; ok {
				findJobPosting(child, out)
				if *out != nil {
					return
				}
			}
		}
	case []any:
		for _, child := range t {
			findJobPosting(child, out)
			if *out != nil {
				return
			}
		}
	}
}

func typeContains(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, want)
	case []any:
		for _, x := range t {
			if typeContains(x, want) {
				return true
			}
		}
	}
	return false
}

func organizationName(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return asString(t["name"])
	case []any:
		if len(t) > 0 {
			return organizationName(t[0])
		}
	}
	return ""
}

func jobLocation(v any) string {
	var parts []string
	switch t := v.(type) {
	case string:
		parts = append(parts, t)
	case []any:
		for _, x := range t {
			if s := jobLocation(x); s != "" {
				parts = append(parts, s)
			}
		}
	case map[string]any:
		if addr, ok := t["address"]; ok {
			switch a := addr.(type) {
			case string:
				parts = append(parts, a)
			case map[string]any:
				for _, key := range []string{"addressLocality", "addressRegion", "addressCountry"} {
					if s := asString(a[key]); s != "" {
						parts = append(parts, s)
					}
				}
			}
		}
		if len(parts) == 0 {
			for _, key := range []string{"name", "addressLocality", "addressRegion"} {
				if s := asString(t[key]); s != "" {
					parts = append(parts, s)
				}
			}
		}
	}
	// Deduplicate adjacent repeated segments.
	seen := map[string]bool{}
	var out []string
	for _, p := range parts {
		p = cleanText(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return asString(t["name"])
	case []any:
		if len(t) > 0 {
			return asString(t[0])
		}
	}
	return ""
}

func absoluteURL(base, ref string) string {
	b, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return ""
	}
	r, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ""
	}
	u := b.ResolveReference(r)
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func allNodes(root *xhtml.Node) []*xhtml.Node {
	var out []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		out = append(out, n)
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func findFirst(root *xhtml.Node, name string) *xhtml.Node {
	for _, n := range allNodes(root) {
		if n.Type == xhtml.ElementNode && strings.EqualFold(n.Data, name) {
			return n
		}
	}
	return nil
}

func attr(n *xhtml.Node, name string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func textContent(n *xhtml.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(cur *xhtml.Node) {
		if cur.Type == xhtml.TextNode {
			b.WriteString(cur.Data)
			return
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func nodeText(n *xhtml.Node) string {
	return cleanText(textContent(n))
}

// cleanText decodes entities and collapses whitespace.
func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

// cleanDescription trims lines and collapses blank-line runs while
// keeping paragraph structure for stored descriptions and diffs.
func cleanDescription(s string) string {
	s = strings.ReplaceAll(html.UnescapeString(s), "\r\n", "\n")
	var out []string
	blank := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if len(out) > 0 {
				blank = true
			}
			continue
		}
		if blank {
			out = append(out, "")
			blank = false
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	// Cut at a rune boundary to keep valid UTF-8.
	cut := max
	for cut > 0 && (s[cut]&0xC0) == 0x80 {
		cut--
	}
	return s[:cut]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
