package model

import "strings"

// CompanyCandidate is a company that may be the same legal entity as
// another one, with a confidence score and a human-readable reason.
// Candidates are only ever proposed; merging always requires an explicit
// user action (SPEC section 5).
type CompanyCandidate struct {
	ID             int64
	Name           string
	NameNormalized string
	Score          int
	Reason         string
}

// MinCompanyCandidateScore is the score below which SimilarCompanyNames
// no longer reports a match.
const MinCompanyCandidateScore = 60

// SimilarCompanyNames compares two normalized company names and returns a
// conservative similarity score (0 = no candidate, 100 = identical) and a
// reason. The rules are intentionally narrow: a candidate is only a
// prompt in the UI, never an automatic merge.
//
//   - token containment: every token of the shorter name appears in the
//     longer one, and the longer name adds at most two tokens
//     ("qonto" / "qonto france");
//   - spelling: edit distance of at most 2 on names of 5+ characters,
//     with at most 20% of the characters changed ("datadog" /
//     "datadogg").
func SimilarCompanyNames(a, b string) (score int, reason string) {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return 0, ""
	}
	if a == b {
		return 100, "same normalized name"
	}

	at, bt := strings.Fields(a), strings.Fields(b)
	short, long := at, bt
	if len(short) > len(long) {
		short, long = long, short
	}
	if len(long) > len(short) && len(long)-len(short) <= 2 &&
		len(strings.Join(short, "")) >= 3 && containsAllTokens(long, short) {
		extra := len(long) - len(short)
		return 80 - 5*extra, "one name extends the other"
	}

	d := levenshtein(a, b)
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if d <= 2 && maxLen >= 5 && d*5 <= maxLen {
		return 100 - d*10, "similar spelling"
	}
	return 0, ""
}

func containsAllTokens(long, short []string) bool {
	set := make(map[string]bool, len(long))
	for _, t := range long {
		set[t] = true
	}
	for _, t := range short {
		if !set[t] {
			return false
		}
	}
	return true
}

// levenshtein returns the edit distance between a and b, measured in
// runes.
func levenshtein(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 {
		return len(br)
	}
	if len(br) == 0 {
		return len(ar)
	}
	prev := make([]int, len(br)+1)
	curr := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		curr[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(br)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
