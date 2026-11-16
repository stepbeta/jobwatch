package sources

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/stepbeta/jobwatch/internal/model"
)

// HTML is the last-resort adapter: it parses one listing page with CSS
// selectors configured per source. Selectors are documented in Config.
// Only the listing page is fetched (never one request per offer), and the
// shared client enforces the HTML minimum interval of 5 seconds.
type HTML struct {
	Client *Client
}

func (h *HTML) Fetch(ctx context.Context, src model.Source) ([]RawOffer, error) {
	cfg, err := ParseConfig(src)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ItemSelector) == "" {
		return nil, fmt.Errorf("html: item_selector not set in config_json")
	}
	if strings.TrimSpace(cfg.TitleSelector) == "" {
		return nil, fmt.Errorf("html: title_selector not set in config_json")
	}
	itemSel, err := cascadia.Compile(cfg.ItemSelector)
	if err != nil {
		return nil, fmt.Errorf("html: invalid item_selector %q: %w", cfg.ItemSelector, err)
	}
	titleSel, err := cascadia.Compile(cfg.TitleSelector)
	if err != nil {
		return nil, fmt.Errorf("html: invalid title_selector %q: %w", cfg.TitleSelector, err)
	}
	linkSel := titleSel
	if s := strings.TrimSpace(cfg.LinkSelector); s != "" {
		if linkSel, err = cascadia.Compile(s); err != nil {
			return nil, fmt.Errorf("html: invalid link_selector %q: %w", s, err)
		}
	}
	var locSel, descSel, companySel, uidSel cascadia.Selector
	if s := strings.TrimSpace(cfg.LocationSelector); s != "" {
		if locSel, err = cascadia.Compile(s); err != nil {
			return nil, fmt.Errorf("html: invalid location_selector %q: %w", s, err)
		}
	}
	if s := strings.TrimSpace(cfg.DescriptionSelector); s != "" {
		if descSel, err = cascadia.Compile(s); err != nil {
			return nil, fmt.Errorf("html: invalid description_selector %q: %w", s, err)
		}
	}
	if s := strings.TrimSpace(cfg.CompanySelector); s != "" {
		if companySel, err = cascadia.Compile(s); err != nil {
			return nil, fmt.Errorf("html: invalid company_selector %q: %w", s, err)
		}
	}
	if s := strings.TrimSpace(cfg.UIDSelector); s != "" {
		if uidSel, err = cascadia.Compile(s); err != nil {
			return nil, fmt.Errorf("html: invalid uid_selector %q: %w", s, err)
		}
	}

	body, err := h.Client.Get(ctx, src, src.URL, "text/html,application/xhtml+xml")
	if err != nil {
		return nil, fmt.Errorf("html: %w", err)
	}
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("html: parse %s: %w", src.URL, err)
	}
	linkAttr := strings.TrimSpace(cfg.LinkAttr)
	if linkAttr == "" {
		linkAttr = "href"
	}

	nodes := cascadia.QueryAll(doc, itemSel)
	out := make([]RawOffer, 0, len(nodes))
	for _, node := range nodes {
		title := nodeText(queryOne(node, titleSel))
		if title == "" {
			continue
		}
		linkNode := queryOne(node, linkSel)
		rawLink := nodeAttr(linkNode, linkAttr)
		if rawLink == "" && strings.EqualFold(linkAttr, "href") {
			rawLink = firstDescendantAttr(linkNode, "href")
		}
		if rawLink == "" {
			// Some listings put the URL in the link text. Accept it only
			// when it looks like a URL, not an ordinary title.
			text := nodeText(linkNode)
			if isURLText(text) {
				rawLink = text
			}
		}
		rawLink = strings.TrimSpace(rawLink)
		if rawLink == "" {
			continue
		}
		link := resolveURL(src.URL, rawLink)
		if link == "" {
			continue
		}
		uid := ""
		uidAttr := strings.TrimSpace(cfg.UIDAttr)
		if uidSel != nil {
			uidNode := queryOne(node, uidSel)
			if uidAttr != "" {
				uid = nodeAttr(uidNode, uidAttr)
			}
			if uid == "" {
				uid = nodeText(uidNode)
			}
		} else if uidAttr != "" {
			uid = nodeAttr(node, uidAttr)
		}
		out = append(out, RawOffer{
			ExternalUID: uid,
			Title:       title,
			Location:    nodeText(queryOne(node, locSel)),
			URL:         link,
			Description: nodeDescription(queryOne(node, descSel)),
			Company:     nodeText(queryOne(node, companySel)),
		})
	}
	return out, nil
}

// queryOne is cascadia.Query with a nil selector treated as "no match".
func queryOne(root *html.Node, sel cascadia.Selector) *html.Node {
	if sel == nil {
		return nil
	}
	return cascadia.Query(root, sel)
}

// isURLText reports whether s looks like a URL rather than a title.
func isURLText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.Contains(s, "/")
}

// firstDescendantAttr finds the first href-like attribute below n. It
// lets a title selector such as "h2" still find the link inside it.
func firstDescendantAttr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	var found string
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if found != "" {
			return
		}
		if v := nodeAttr(cur, name); v != "" {
			found = v
			return
		}
		for c := cur.FirstChild; c != nil && found == ""; c = c.NextSibling {
			walk(c)
		}
	}
	for c := n.FirstChild; c != nil && found == ""; c = c.NextSibling {
		walk(c)
	}
	return found
}

func nodeAttr(n *html.Node, name string) string {
	if n == nil {
		return ""
	}
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, name) {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}

// nodeText returns the whitespace-collapsed text of a node.
func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
			b.WriteByte(' ')
			return
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// nodeDescription renders the children of n to HTML and then to plain
// text, keeping paragraph and list structure.
func nodeDescription(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&b, c); err != nil {
			return nodeText(n)
		}
	}
	return HTMLToText(b.String())
}
