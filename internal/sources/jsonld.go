package sources

import (
	"encoding/json"
	"strings"

	"golang.org/x/net/html"
)

// FindJobPosting returns the first JSON-LD JobPosting object found in doc,
// or nil when the page has none. The walk accepts the shapes boards
// publish: a direct JobPosting object, an array of nodes, an @graph
// wrapper, and the itemListElement/mainEntity containers some CMSes use.
// Field extraction is left to the caller: manual import and the WTTJ
// adapter read different subsets of the object.
func FindJobPosting(doc *html.Node) map[string]any {
	if doc == nil {
		return nil
	}
	var found map[string]any
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && strings.EqualFold(n.Data, "script") &&
			strings.Contains(strings.ToLower(nodeAttr(n, "type")), "ld+json") {
			if raw := strings.TrimSpace(scriptText(n)); raw != "" {
				var v any
				if err := json.Unmarshal([]byte(raw), &v); err == nil {
					found = jobPostingFromJSONLD(v)
				}
			}
		}
		for c := n.FirstChild; c != nil && found == nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// jobPostingFromJSONLD walks decoded JSON-LD (objects, arrays, @graph)
// until a node whose @type contains JobPosting is found.
func jobPostingFromJSONLD(v any) map[string]any {
	switch t := v.(type) {
	case map[string]any:
		if jsonLDTypeContains(t["@type"], "JobPosting") {
			return t
		}
		for _, key := range []string{"@graph", "itemListElement", "mainEntity"} {
			if child, ok := t[key]; ok {
				if found := jobPostingFromJSONLD(child); found != nil {
					return found
				}
			}
		}
	case []any:
		for _, child := range t {
			if found := jobPostingFromJSONLD(child); found != nil {
				return found
			}
		}
	}
	return nil
}

func jsonLDTypeContains(v any, want string) bool {
	switch t := v.(type) {
	case string:
		return strings.EqualFold(t, want)
	case []any:
		for _, x := range t {
			if jsonLDTypeContains(x, want) {
				return true
			}
		}
	}
	return false
}

// scriptText concatenates the raw text of n without collapsing
// whitespace, so a JSON-LD string value keeps its own line breaks.
func scriptText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
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
