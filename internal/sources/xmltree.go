package sources

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// xmlNode is a small, namespace-agnostic XML tree used by the generic RSS
// adapter. Element names are stored without their namespace prefix so a
// mapping such as "content:encoded" can be written as "encoded".
type xmlNode struct {
	name     string
	attrs    []xml.Attr
	children []*xmlNode
	text     strings.Builder
}

// parseXMLTree builds a tree from an XML document. Comments, directives
// and processing instructions are ignored.
func parseXMLTree(body []byte) (*xmlNode, error) {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity

	var root *xmlNode
	var stack []*xmlNode
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{name: localName(t.Name.Local), attrs: t.Attr}
			if len(stack) == 0 {
				if root == nil {
					root = n
				}
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text.Write(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("decode xml: empty document")
	}
	return root, nil
}

func localName(name string) string {
	if i := strings.LastIndex(name, ":"); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (n *xmlNode) attr(name string) string {
	for _, a := range n.attrs {
		if strings.EqualFold(localName(a.Name.Local), name) {
			return a.Value
		}
	}
	return ""
}

// childrenNamed returns direct children with the given local name.
func (n *xmlNode) childrenNamed(name string) []*xmlNode {
	var out []*xmlNode
	for _, c := range n.children {
		if strings.EqualFold(c.name, name) {
			out = append(out, c)
		}
	}
	return out
}

// find returns the first direct child with name.
func (n *xmlNode) find(name string) *xmlNode {
	for _, c := range n.children {
		if strings.EqualFold(c.name, name) {
			return c
		}
	}
	return nil
}

// findAll collects every descendant (not the node itself) with name.
func (n *xmlNode) findAll(name string) []*xmlNode {
	var out []*xmlNode
	var walk func(*xmlNode)
	walk = func(cur *xmlNode) {
		for _, c := range cur.children {
			if strings.EqualFold(c.name, name) {
				out = append(out, c)
			}
			walk(c)
		}
	}
	walk(n)
	return out
}

// textContent concatenates the character data of the node and its
// descendants, preserving whitespace for later normalization.
func (n *xmlNode) textContent() string {
	var b strings.Builder
	b.WriteString(n.text.String())
	for _, c := range n.children {
		b.WriteString(c.textContent())
	}
	return b.String()
}

// value evaluates a simple path relative to n. A path is a chain of child
// element local names. The last segment may end in "@attr" to read an
// attribute; a bare "@attr" reads an attribute of n, and "#text" returns
// the node's own text.
func (n *xmlNode) value(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	cur := n
	for _, raw := range strings.Split(path, "/") {
		seg := strings.TrimSpace(raw)
		if seg == "" {
			continue
		}
		if seg == "#text" {
			return strings.TrimSpace(cur.text.String())
		}
		if strings.HasPrefix(seg, "@") {
			return strings.TrimSpace(cur.attr(strings.TrimPrefix(seg, "@")))
		}
		name, attr, hasAttr := strings.Cut(seg, "@")
		cur = cur.find(localName(strings.TrimSpace(name)))
		if cur == nil {
			return ""
		}
		if hasAttr {
			return strings.TrimSpace(cur.attr(strings.TrimSpace(attr)))
		}
	}
	return strings.TrimSpace(cur.textContent())
}
