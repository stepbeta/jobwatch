// Package classify is JobWatch's deterministic scoring engine (SPEC
// section 6). It turns a title, location and description into a title
// confidence (is this a Platform/SRE/DevOps role?), a location confidence
// (is this full remote and France-compatible?), a derived remote policy
// and a human-readable list of reasons. There is no LLM and no I/O: the
// same input always produces the same output.
package classify

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Reasons is the JSON shape stored in job_offers.match_reasons_json. Each
// list explains one score in human-readable terms; the UI shows them as
// badge tooltips.
type Reasons struct {
	Title    []string `json:"title"`
	Location []string `json:"location"`
}

// Result is the classifier output for one offer.
type Result struct {
	TitleConfidence    int
	LocationConfidence int
	RemotePolicy       model.RemotePolicy
	Reasons            Reasons
}

// Scores converts the result into the persisted column values.
func (r Result) Scores() model.OfferScores {
	return model.OfferScores{
		RemotePolicy:       r.RemotePolicy,
		TitleConfidence:    r.TitleConfidence,
		LocationConfidence: r.LocationConfidence,
		MatchReasonsJSON:   r.ReasonsJSON(),
	}
}

// ReasonsJSON marshals the reasons. Encoding a struct of strings cannot
// fail in practice, but the fallback keeps the column valid JSON.
func (r Result) ReasonsJSON() string {
	b, err := json.Marshal(r.Reasons)
	if err != nil {
		return `{"title":[],"location":[]}`
	}
	return string(b)
}

// Classify scores one offer. location and description may be empty; the
// description is used for remote and geography keywords only, never as a
// substitute for the title.
func Classify(title, location, description string) Result {
	titleScore, titleReasons := scoreTitle(title)
	locationScore, policy, locationReasons := scoreLocation(location, description)
	return Result{
		TitleConfidence:    titleScore,
		LocationConfidence: locationScore,
		RemotePolicy:       policy,
		Reasons: Reasons{
			Title:    titleReasons,
			Location: locationReasons,
		},
	}
}

func scoreTitle(title string) (int, []string) {
	norm := model.NormalizeTitle(title)
	score := 0
	reasons := make([]string, 0, 4)
	for _, kw := range titleKeywords {
		if !containsPhrase(norm, kw.term) {
			continue
		}
		score += kw.weight
		reasons = append(reasons, fmt.Sprintf("%s %q (%+d)", kw.kind, kw.term, kw.weight))
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no title keywords matched")
	}
	return clamp(score), reasons
}

func scoreLocation(location, description string) (int, model.RemotePolicy, []string) {
	loc := model.NormalizeKeywords(location)
	desc := model.NormalizeKeywords(description)
	reasons := make([]string, 0, 8)

	locRemoteMatch := matchAny(loc, remotePhrases)
	descRemoteMatch := matchAny(desc, remotePhrases)
	locHybridMatch := matchAny(loc, hybridPhrases)
	descHybridMatch := matchAny(desc, hybridPhrases)
	locOnsiteMatch := matchAny(loc, onsitePhrases)
	descOnsiteMatch := matchAny(desc, onsitePhrases)

	remote := locRemoteMatch != "" || descRemoteMatch != ""
	hybrid := locHybridMatch != "" || descHybridMatch != ""
	onsite := locOnsiteMatch != "" || descOnsiteMatch != ""

	policy := model.RemoteUnknown
	switch {
	case hybrid || (remote && onsite):
		policy = model.RemoteHybrid
	case onsite:
		policy = model.RemoteOnsite
	case remote:
		policy = model.RemoteFull
	}

	score := 0
	switch policy {
	case model.RemoteFull:
		if locRemoteMatch != "" {
			score += remoteLocationWeight
			reasons = append(reasons, fmt.Sprintf("location %q (%+d)", locRemoteMatch, remoteLocationWeight))
		}
		if descRemoteMatch != "" {
			score += remoteDescriptionWeight
			reasons = append(reasons, fmt.Sprintf("description %q (%+d)", descRemoteMatch, remoteDescriptionWeight))
		}
	case model.RemoteHybrid:
		hook := locRemoteMatch
		if locHybridMatch != "" {
			hook = locHybridMatch
		}
		if hook != "" {
			score += hybridLocationWeight
			reasons = append(reasons, fmt.Sprintf("location %q (%+d)", hook, hybridLocationWeight))
		}
		hook = descRemoteMatch
		if descHybridMatch != "" {
			hook = descHybridMatch
		}
		if hook != "" {
			score += hybridDescriptionWeight
			reasons = append(reasons, fmt.Sprintf("description %q (%+d)", hook, hybridDescriptionWeight))
		}
	case model.RemoteOnsite:
		hook := locOnsiteMatch
		if hook == "" {
			hook = descOnsiteMatch
		}
		reasons = append(reasons, fmt.Sprintf("workplace %q (onsite, full remote not offered)", hook))
	}

	for _, kw := range locationKeywords {
		if !containsPhrase(loc, kw.term) {
			continue
		}
		score += kw.weight
		reasons = append(reasons, fmt.Sprintf("location %q (%+d)", kw.term, kw.weight))
	}
	for _, kw := range descriptionKeywords {
		if !containsPhrase(desc, kw.term) {
			continue
		}
		score += kw.weight
		reasons = append(reasons, fmt.Sprintf("description %q (%+d)", kw.term, kw.weight))
	}

	score = clamp(score)
	switch policy {
	case model.RemoteOnsite:
		score = min(score, onsitePolicyCap)
	case model.RemoteHybrid:
		score = min(score, hybridPolicyCap)
	case model.RemoteUnknown:
		score = min(score, unknownPolicyCap)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "no location or remote keywords matched")
	}
	reasons = append(reasons, "remote policy: "+string(policy))
	return score, policy, reasons
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

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
