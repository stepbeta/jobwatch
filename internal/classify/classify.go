// Package classify is JobWatch's deterministic scoring engine. It turns
// offer facts (title, location, description) into title affinity,
// location affinity and a remote policy, using a user search profile for
// the preferences. There is no LLM and no I/O: the same input and profile
// always produce the same output.
//
// The term tables are not in Go: they are stored per profile in
// search_profile_terms (migration 0015). This package only normalizes
// text, matches whole-token phrases, adds weights and clamps the result.
// The remote phrase lists are fixed; the user selects which derived
// policies are acceptable.
package classify

import (
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Match is one profile term that matched an offer text. Neutral terms are
// reported with weight 0: they explain a match without changing the score.
type Match struct {
	Term   string
	Weight int
	Bucket model.TermBucket
}

// Affinity is a computed score (0..100) plus the terms that produced it.
type Affinity struct {
	Score   int
	Matches []Match
}

// Result is the read-time classification of one offer against one
// profile.
type Result struct {
	TitleAffinity    Affinity
	LocationAffinity Affinity
	RemotePolicy     model.RemotePolicy
}

// Score computes the three signals for one offer. location and
// description may be empty; the description contributes to location
// affinity and to remote detection, never to title affinity.
func Score(title, location, description string, p model.SearchProfile) Result {
	return Result{
		TitleAffinity:    ScoreTitle(title, p),
		LocationAffinity: ScoreLocation(location, description, p),
		RemotePolicy:     DeriveRemotePolicy(location, description),
	}
}

// ScoreTitle adds the weights of the profile's title terms that match the
// title, using whole-token phrase matching after model.NormalizeTitle.
func ScoreTitle(title string, p model.SearchProfile) Affinity {
	norm := model.NormalizeTitle(title)
	matches := make([]Match, 0, 4)
	for _, t := range p.Terms {
		if t.Affinity != model.AffinityTitle {
			continue
		}
		if containsPhrase(norm, t.Term) {
			matches = append(matches, Match{Term: t.Term, Weight: t.Weight, Bucket: t.Bucket})
		}
	}
	return newAffinity(matches)
}

// ScoreLocation adds the weights of the profile's location terms that
// match the location and of its description terms that match the
// description. Description terms affect geography only; they never change
// the remote policy.
func ScoreLocation(location, description string, p model.SearchProfile) Affinity {
	loc := model.NormalizeKeywords(location)
	desc := model.NormalizeKeywords(description)
	matches := make([]Match, 0, 4)
	for _, t := range p.Terms {
		switch t.Affinity {
		case model.AffinityLocation:
			if containsPhrase(loc, t.Term) {
				matches = append(matches, Match{Term: t.Term, Weight: t.Weight, Bucket: t.Bucket})
			}
		case model.AffinityDescription:
			if containsPhrase(desc, t.Term) {
				matches = append(matches, Match{Term: t.Term, Weight: t.Weight, Bucket: t.Bucket})
			}
		}
	}
	return newAffinity(matches)
}

// DeriveRemotePolicy is the fixed detector: it produces the offer fact
// from the location and description text. The user cannot edit the phrase
// lists; only the acceptance of the resulting policy is a preference.
func DeriveRemotePolicy(location, description string) model.RemotePolicy {
	loc := model.NormalizeKeywords(location)
	desc := model.NormalizeKeywords(description)

	remote := matchAny(loc, remotePhrases) != "" || matchAny(desc, remotePhrases) != ""
	hybrid := matchAny(loc, hybridPhrases) != "" || matchAny(desc, hybridPhrases) != ""
	onsite := matchAny(loc, onsitePhrases) != "" || matchAny(desc, onsitePhrases) != ""

	switch {
	case hybrid || (remote && onsite):
		return model.RemoteHybrid
	case onsite:
		return model.RemoteOnsite
	case remote:
		return model.RemoteFull
	default:
		return model.RemoteUnknown
	}
}

// RemoteAccepted reports whether the profile accepts a derived policy. It
// is a membership check, never a weighted score.
func RemoteAccepted(policy model.RemotePolicy, p model.SearchProfile) bool {
	return p.Accepts(policy)
}

// newAffinity sums the matched weights and clamps the total to 0..100.
func newAffinity(matches []Match) Affinity {
	score := 0
	for _, m := range matches {
		score += m.Weight
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return Affinity{Score: score, Matches: matches}
}

// containsPhrase reports whether the normalized phrase occurs in the
// normalized text as a whole token sequence. Both strings must already be
// normalized with model.NormalizeTitle or model.NormalizeKeywords.
func containsPhrase(normalized, phrase string) bool {
	p := model.NormalizeKeywords(phrase)
	if p == "" {
		return false
	}
	return strings.Contains(" "+normalized+" ", " "+p+" ")
}

// matchAny returns the first phrase contained in normalized text, or "".
func matchAny(normalized string, phrases []string) string {
	for _, p := range phrases {
		if containsPhrase(normalized, p) {
			return p
		}
	}
	return ""
}
