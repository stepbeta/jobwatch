package sources

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"
)

// robotsRules is the subset of a robots.txt (RFC 9309) that applies to
// one user-agent token.
type robotsRules struct {
	rules      []robotsRule
	crawlDelay time.Duration
}

type robotsRule struct {
	allow   bool
	pattern string
}

// allowAll is used when robots.txt is missing (HTTP 4xx).
var allowAll = &robotsRules{}

// parseRobots parses body and returns the rules for agent (a product
// token such as "JobWatch"). Groups naming the agent take precedence over
// the "*" group; multiple matching groups are merged.
func parseRobots(r io.Reader, agent string) *robotsRules {
	agent = strings.ToLower(agent)
	type group struct {
		agents []string
		rules  []robotsRule
		delay  time.Duration
	}
	var groups []*group
	var cur *group
	lastWasAgent := false

	sc := bufio.NewScanner(io.LimitReader(r, 512*1024))
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		switch key {
		case "user-agent":
			if cur == nil || !lastWasAgent {
				cur = &group{}
				groups = append(groups, cur)
			}
			cur.agents = append(cur.agents, strings.ToLower(val))
			lastWasAgent = true
		case "allow", "disallow":
			lastWasAgent = false
			if cur == nil || val == "" {
				continue
			}
			cur.rules = append(cur.rules, robotsRule{allow: key == "allow", pattern: val})
		case "crawl-delay":
			lastWasAgent = false
			if cur == nil {
				continue
			}
			if f, err := strconv.ParseFloat(val, 64); err == nil && f > 0 {
				cur.delay = time.Duration(f * float64(time.Second))
			}
		default:
			// sitemap and unknown keys do not end a user-agent run per RFC 9309
		}
	}

	collect := func(match func(string) bool) *robotsRules {
		var out *robotsRules
		for _, g := range groups {
			for _, a := range g.agents {
				if match(a) {
					if out == nil {
						out = &robotsRules{}
					}
					out.rules = append(out.rules, g.rules...)
					if g.delay > out.crawlDelay {
						out.crawlDelay = g.delay
					}
					break
				}
			}
		}
		return out
	}
	if rr := collect(func(a string) bool { return a != "*" && strings.HasPrefix(agent, a) }); rr != nil {
		return rr
	}
	if rr := collect(func(a string) bool { return a == "*" }); rr != nil {
		return rr
	}
	return allowAll
}

// allowed reports whether path (path plus optional "?query") may be
// fetched. The longest matching rule wins; on a tie, allow wins.
func (rr *robotsRules) allowed(path string) bool {
	if path == "" {
		path = "/"
	}
	if path == "/robots.txt" {
		return true
	}
	bestLen := -1
	allow := true
	for _, r := range rr.rules {
		if !robotsMatch(r.pattern, path) {
			continue
		}
		l := len(r.pattern)
		if l > bestLen || (l == bestLen && r.allow) {
			bestLen = l
			allow = r.allow
		}
	}
	return allow
}

// robotsMatch matches a robots.txt path pattern supporting "*" (any
// sequence) and a trailing "$" (end anchor) against path.
func robotsMatch(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	parts := strings.Split(pattern, "*")
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}
	pos := len(parts[0])
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if i == len(parts)-1 && anchored {
			return len(path)-pos >= len(p) && strings.HasSuffix(path, p)
		}
		idx := strings.Index(path[pos:], p)
		if idx < 0 {
			return false
		}
		pos += idx + len(p)
	}
	if anchored {
		return pos == len(path)
	}
	return true
}
