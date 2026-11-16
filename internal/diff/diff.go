// Package diff computes a simple, readable line diff between two text
// snapshots. It is deliberately small: the UI only needs to show which
// lines were removed and which were added, not a character-level diff.
package diff

import "strings"

// Op classifies a diff line.
type Op string

const (
	OpEqual   Op = "equal"
	OpAdded   Op = "added"
	OpRemoved Op = "removed"
)

// Line is one line of the diff. Text is the line as it appears in the old
// text (removed/equal) or the new text (added/equal).
type Line struct {
	Op   Op     `json:"op"`
	Text string `json:"text"`
}

// maxCells caps the LCS table. Beyond it the diff degrades to "everything
// removed, everything added" instead of allocating an unbounded matrix.
const maxCells = 4_000_000

// Lines diffs oldText against newText line by line. Line endings may be
// LF or CRLF; trailing whitespace is ignored for matching and output.
func Lines(oldText, newText string) []Line {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	// Trim the common prefix and suffix first; job descriptions usually
	// change in one place, which keeps the LCS table small.
	prefix := 0
	for prefix < len(oldLines) && prefix < len(newLines) && oldLines[prefix] == newLines[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	a := oldLines[prefix : len(oldLines)-suffix]
	b := newLines[prefix : len(newLines)-suffix]

	out := make([]Line, 0, len(oldLines)+len(newLines))
	for _, line := range oldLines[:prefix] {
		out = append(out, Line{Op: OpEqual, Text: line})
	}
	out = append(out, diffMiddle(a, b)...)
	for _, line := range oldLines[len(oldLines)-suffix:] {
		out = append(out, Line{Op: OpEqual, Text: line})
	}
	return out
}

func diffMiddle(a, b []string) []Line {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a) == 0 {
		out := make([]Line, len(b))
		for i, line := range b {
			out[i] = Line{Op: OpAdded, Text: line}
		}
		return out
	}
	if len(b) == 0 {
		out := make([]Line, len(a))
		for i, line := range a {
			out[i] = Line{Op: OpRemoved, Text: line}
		}
		return out
	}
	if (len(a)+1)*(len(b)+1) > maxCells {
		return coarseDiff(a, b)
	}

	// dp[i][j] = LCS length of a[i:] and b[j:].
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	out := make([]Line, 0, n+m)
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, Line{Op: OpEqual, Text: a[i]})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			out = append(out, Line{Op: OpRemoved, Text: a[i]})
			i++
		default:
			out = append(out, Line{Op: OpAdded, Text: b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, Line{Op: OpRemoved, Text: a[i]})
	}
	for ; j < m; j++ {
		out = append(out, Line{Op: OpAdded, Text: b[j]})
	}
	return out
}

func coarseDiff(a, b []string) []Line {
	out := make([]Line, 0, len(a)+len(b))
	for _, line := range a {
		out = append(out, Line{Op: OpRemoved, Text: line})
	}
	for _, line := range b {
		out = append(out, Line{Op: OpAdded, Text: line})
	}
	return out
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return lines
}
