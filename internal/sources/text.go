package sources

import (
	"html"
	"regexp"
	"strings"
)

var (
	reScript   = regexp.MustCompile(`(?is)<script\b.*?</script\s*>`)
	reStyle    = regexp.MustCompile(`(?is)<style\b.*?</style\s*>`)
	reListItem = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	reBreak    = regexp.MustCompile(`(?i)<br\s*/?>|</?(p|div|ul|ol|h[1-6]|tr|table|section|article|blockquote)\b[^>]*>`)
	reTag      = regexp.MustCompile(`<[^>]*>`)
)

// HTMLToText converts an HTML fragment to readable plain text: block
// elements become line breaks, list items become "- " lines, tags are
// dropped, entities decoded, and blank lines collapsed.
func HTMLToText(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reListItem.ReplaceAllString(s, "\n- ")
	s = reBreak.ReplaceAllString(s, "\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")

	var out []string
	blank := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			blank = len(out) > 0
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
