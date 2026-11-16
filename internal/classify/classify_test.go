package classify

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

func TestTitleConfidence(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  int
	}{
		// Strong positives, including variants and gender markers.
		{"platform engineer", "Platform Engineer", 85}, // strong + weak "platform"
		{"senior platform engineer", "Senior Platform Engineer (H/F)", 85},
		{"site reliability", "Site Reliability Engineer", 85},
		{"sre abbreviation", "SRE (F/H)", 60},
		{"devops", "DevOps Engineer", 60},
		{"devops kubernetes", "DevOps Engineer - Kubernetes", 85},
		{"infrastructure engineer", "Infrastructure Engineer", 85},
		{"cloud engineer", "Cloud Engineer", 85},
		{"cloud infrastructure engineer", "Cloud Infrastructure Engineer", 100},

		// Weak positives alone stay partial.
		{"reliability only", "Reliability Analyst", 25},
		{"kubernetes only", "Kubernetes Consultant", 25},
		{"platform lead", "Platform Lead", 25},
		{"backend infrastructure", "Backend Infrastructure Engineer", 100},

		// Tricky titles: adjacent words must not beat role negatives.
		{"engineering manager platform", "Engineering Manager - Platform", 5},
		{"marketing platform lead", "Marketing Platform Lead", 0},
		{"account executive cloud", "Account Executive, Cloud", 0},
		{"sales engineer infrastructure", "Sales Engineer, Infrastructure", 0},
		{"recruiter platform", "Technical Recruiter - Platform", 0},
		{"internship platform", "Internship - Platform Engineering", 0},
		{"french internship", "Stage - Platform Engineer", 0},
		{"product marketing", "Product Marketing Manager", 0},

		// Unknown and noise.
		{"empty", "", 0},
		{"unrelated", "Office Manager", 0},
		{"token boundary", "Wholesale Operations", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.title, "", "")
			if got.TitleConfidence != tt.want {
				t.Errorf("title %q score = %d, want %d (reasons %v)",
					tt.title, got.TitleConfidence, tt.want, got.Reasons.Title)
			}
			if len(got.Reasons.Title) == 0 {
				t.Errorf("title %q has no reasons", tt.title)
			}
		})
	}
}

func TestTitleTrickyPair(t *testing.T) {
	engineer := Classify("Engineering Manager - Platform", "", "").TitleConfidence
	marketing := Classify("Marketing Platform Lead", "", "").TitleConfidence
	platform := Classify("Platform Engineer", "", "").TitleConfidence
	if !(marketing < engineer && engineer < platform) {
		t.Fatalf("expected marketing %d < manager %d < platform engineer %d", marketing, engineer, platform)
	}
}

func TestTitleReasonsNameMatchedTerms(t *testing.T) {
	got := Classify("Senior SRE - Kubernetes / Terraform", "", "")
	if got.TitleConfidence != 100 {
		t.Errorf("score = %d, want 100", got.TitleConfidence)
	}
	joined := strings.Join(got.Reasons.Title, "\n")
	for _, want := range []string{"sre", "kubernetes", "terraform"} {
		if !strings.Contains(strings.ToLower(joined), want) {
			t.Errorf("reasons %q do not mention %q", joined, want)
		}
	}
	got = Classify("Account Executive, Cloud", "", "")
	joined = strings.Join(got.Reasons.Location, "\n")
	if !strings.Contains(strings.ToLower(joined), "no location") {
		t.Errorf("location reasons = %q", joined)
	}
	if !strings.Contains(strings.ToLower(strings.Join(got.Reasons.Title, "\n")), "account executive") {
		t.Errorf("title reasons do not mention the negative term: %v", got.Reasons.Title)
	}
}

func TestLocationConfidenceAndPolicy(t *testing.T) {
	tests := []struct {
		name        string
		location    string
		description string
		wantScore   int
		wantPolicy  model.RemotePolicy
	}{
		{"remote france", "Remote - France", "", 80, model.RemoteFull},
		{"full remote paris", "Paris, France", "Fully remote within France.", 100, model.RemoteFull},
		{"paris france no remote hint", "Paris, France", "", 60, model.RemoteUnknown},
		{"lyon france", "Lyon, France", "", 60, model.RemoteUnknown},
		{"paris only", "Paris", "", 25, model.RemoteUnknown},
		{"remote emea", "Remote - EMEA", "", 65, model.RemoteFull},
		{"remote eu", "Remote - EU", "", 60, model.RemoteFull},
		{"remote europe cet", "Remote - Europe / CET", "", 70, model.RemoteFull},
		{"remote only", "Remote", "", 45, model.RemoteFull},
		{"hybrid paris", "Hybrid - Paris, France", "", 70, model.RemoteHybrid},
		{"onsite paris", "Onsite - Paris, France", "", 50, model.RemoteOnsite},
		{"onsite description", "Paris, France", "This is an on-site role.", 50, model.RemoteOnsite},
		{"hybrid description", "Paris, France", "Hybrid policy, two days on site.", 65, model.RemoteHybrid},
		{"remote and onsite is hybrid", "Paris, France", "Remote or on-site.", 65, model.RemoteHybrid},
		{"description only", "", "Fully remote, France-based team. CET timezone.", 50, model.RemoteFull},
		{"description france paris", "", "Remote role, our team is in Paris, France.", 60, model.RemoteFull},
		{"new york", "New York, USA", "", 0, model.RemoteUnknown},
		{"remote united states", "Remote, United States", "", 10, model.RemoteFull},
		{"london", "London, UK", "", 0, model.RemoteUnknown},
		{"berlin", "Berlin, Germany", "", 0, model.RemoteUnknown},
		{"remote germany", "Remote - Germany", "", 25, model.RemoteFull},
		{"empty", "", "", 0, model.RemoteUnknown},
		{"worldwide", "Remote - Worldwide", "", 55, model.RemoteFull},
		{"emea timezone", "EMEA", "EU time zones", 25, model.RemoteUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify("Platform Engineer", tt.location, tt.description)
			if got.LocationConfidence != tt.wantScore {
				t.Errorf("location %q / desc %q score = %d, want %d (reasons %v)",
					tt.location, tt.description, got.LocationConfidence, tt.wantScore, got.Reasons.Location)
			}
			if got.RemotePolicy != tt.wantPolicy {
				t.Errorf("location %q / desc %q policy = %q, want %q",
					tt.location, tt.description, got.RemotePolicy, tt.wantPolicy)
			}
			if len(got.Reasons.Location) == 0 {
				t.Errorf("location %q has no reasons", tt.location)
			}
			if !strings.Contains(strings.Join(got.Reasons.Location, "\n"), "remote policy: "+string(tt.wantPolicy)) {
				t.Errorf("reasons %v do not state the policy", got.Reasons.Location)
			}
		})
	}
}

func TestClassifyScoresJSON(t *testing.T) {
	res := Classify("Site Reliability Engineer", "Remote - France", "Fully remote in France. Kubernetes and Terraform.")
	scores := res.Scores()
	if scores.RemotePolicy != model.RemoteFull {
		t.Errorf("policy = %q", scores.RemotePolicy)
	}
	if scores.TitleConfidence < 80 || scores.LocationConfidence < 80 {
		t.Errorf("scores = %d/%d", scores.TitleConfidence, scores.LocationConfidence)
	}
	var reasons Reasons
	if err := json.Unmarshal([]byte(scores.MatchReasonsJSON), &reasons); err != nil {
		t.Fatalf("reasons JSON %q: %v", scores.MatchReasonsJSON, err)
	}
	if len(reasons.Title) == 0 || len(reasons.Location) == 0 {
		t.Errorf("reasons = %+v", reasons)
	}
}

func TestKeywordTablesHaveNoDuplicateNormalizedTerms(t *testing.T) {
	seen := map[string]bool{}
	for _, kw := range titleKeywords {
		if seen[kw.term] {
			t.Errorf("duplicate title keyword %q", kw.term)
		}
		seen[kw.term] = true
	}
}
