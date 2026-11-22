package classify

import (
	"testing"

	"github.com/stepbeta/jobwatch/internal/model"
)

// testProfile is a representative subset of the seeded default profile
// (migration 0015): its terms, weights and buckets match the seed, and it
// adds one synthetic neutral term ("contract", weight 0, absent from the
// seed) so the reason-list coverage includes a neutral match. The
// migration tests assert the full seeded table.
func testProfile() model.SearchProfile {
	return model.SearchProfile{
		ID:          1,
		Name:        "test",
		BucketScale: model.BucketScaleFive,
		Terms: []model.SearchProfileTerm{
			{Affinity: model.AffinityTitle, Bucket: model.BucketStrongPositive, Term: "platform engineer", Weight: 60, SortOrder: 0},
			{Affinity: model.AffinityTitle, Bucket: model.BucketStrongPositive, Term: "sre", Weight: 60, SortOrder: 1},
			{Affinity: model.AffinityTitle, Bucket: model.BucketMildPositive, Term: "kubernetes", Weight: 25, SortOrder: 2},
			{Affinity: model.AffinityTitle, Bucket: model.BucketMildPositive, Term: "platform", Weight: 25, SortOrder: 3},
			{Affinity: model.AffinityTitle, Bucket: model.BucketMildPositive, Term: "linux", Weight: 15, SortOrder: 4},
			{Affinity: model.AffinityTitle, Bucket: model.BucketNeutral, Term: "contract", Weight: 0, SortOrder: 5},
			{Affinity: model.AffinityTitle, Bucket: model.BucketMildNegative, Term: "manager", Weight: -20, SortOrder: 6},
			{Affinity: model.AffinityTitle, Bucket: model.BucketStrongNegative, Term: "marketing", Weight: -120, SortOrder: 7},

			{Affinity: model.AffinityLocation, Bucket: model.BucketStrongPositive, Term: "france", Weight: 35, SortOrder: 0},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildPositive, Term: "paris", Weight: 25, SortOrder: 1},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildPositive, Term: "emea", Weight: 20, SortOrder: 2},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildPositive, Term: "europe", Weight: 15, SortOrder: 3},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildPositive, Term: "utc 1", Weight: 10, SortOrder: 4},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildNegative, Term: "united kingdom", Weight: -20, SortOrder: 5},
			{Affinity: model.AffinityLocation, Bucket: model.BucketMildNegative, Term: "spain", Weight: -15, SortOrder: 6},
			{Affinity: model.AffinityLocation, Bucket: model.BucketStrongNegative, Term: "usa", Weight: -35, SortOrder: 7},

			{Affinity: model.AffinityDescription, Bucket: model.BucketMildPositive, Term: "france", Weight: 20, SortOrder: 0},
			{Affinity: model.AffinityDescription, Bucket: model.BucketMildPositive, Term: "paris", Weight: 15, SortOrder: 1},
			{Affinity: model.AffinityDescription, Bucket: model.BucketMildPositive, Term: "cet", Weight: 5, SortOrder: 2},
		},
		Remote: []model.SearchProfileRemote{
			{Policy: model.RemoteFull, Accepted: true},
			{Policy: model.RemoteHybrid, Accepted: false},
			{Policy: model.RemoteOnsite, Accepted: false},
			{Policy: model.RemoteUnknown, Accepted: true},
		},
	}
}

func TestScoreTitle(t *testing.T) {
	p := testProfile()
	tests := []struct {
		name    string
		title   string
		want    int
		matches int
	}{
		{"strong and weak", "Platform Engineer", 85, 2}, // 60 + 25
		{"strong only", "SRE (F/H)", 60, 1},
		{"manager penalty", "Engineering Manager - Platform", 5, 2}, // 25 - 20
		{"strong negative clamps to zero", "Marketing Platform Lead", 0, 2},
		{"token boundary", "Wholesale Operations", 0, 0},
		{"empty", "", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreTitle(tt.title, p)
			if got.Score != tt.want {
				t.Errorf("title %q score = %d, want %d (matches %v)", tt.title, got.Score, tt.want, got.Matches)
			}
			if len(got.Matches) != tt.matches {
				t.Errorf("title %q matches = %d, want %d (%v)", tt.title, len(got.Matches), tt.matches, got.Matches)
			}
		})
	}
}

func TestScoreTitleNeutralTermAppearsInReasons(t *testing.T) {
	got := ScoreTitle("Platform Engineer - Contract", testProfile())
	if got.Score != 85 {
		t.Errorf("score = %d, want 85", got.Score)
	}
	if len(got.Matches) != 3 {
		t.Fatalf("matches = %v, want the neutral term too", got.Matches)
	}
	last := got.Matches[len(got.Matches)-1]
	if last.Term != "contract" || last.Weight != 0 || last.Bucket != model.BucketNeutral {
		t.Errorf("neutral match = %+v", last)
	}
}

func TestScoreTitleReasonsCarryBucketAndWeight(t *testing.T) {
	got := ScoreTitle("Senior SRE - Kubernetes", testProfile())
	if len(got.Matches) != 2 {
		t.Fatalf("matches = %v", got.Matches)
	}
	if got.Matches[0].Term != "sre" || got.Matches[0].Weight != 60 || got.Matches[0].Bucket != model.BucketStrongPositive {
		t.Errorf("first match = %+v", got.Matches[0])
	}
	if got.Matches[1].Term != "kubernetes" || got.Matches[1].Weight != 25 || got.Matches[1].Bucket != model.BucketMildPositive {
		t.Errorf("second match = %+v", got.Matches[1])
	}
}

func TestScoreLocation(t *testing.T) {
	p := testProfile()
	tests := []struct {
		name        string
		location    string
		description string
		want        int
		matches     int
	}{
		{"france and paris", "Paris, France", "", 60, 2},
		{"france only", "Remote - France", "", 35, 1},
		{"emea", "Remote - EMEA", "", 20, 1},
		{"incompatible", "New York, USA", "", 0, 1},
		{"mild negative", "London, United Kingdom", "", 0, 1},
		{"description france and cet", "", "Fully remote, France-based team. CET timezone.", 25, 2},
		{"description paris and france", "", "Remote role, our team is in Paris, France.", 35, 2},
		{"description term not in location", "Berlin", "Our customers are in Paris.", 15, 1},
		{"nothing matches", "", "", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ScoreLocation(tt.location, tt.description, p)
			if got.Score != tt.want {
				t.Errorf("location %q / desc %q score = %d, want %d (matches %v)",
					tt.location, tt.description, got.Score, tt.want, got.Matches)
			}
			if len(got.Matches) != tt.matches {
				t.Errorf("location %q / desc %q matches = %d, want %d (%v)",
					tt.location, tt.description, len(got.Matches), tt.matches, got.Matches)
			}
		})
	}
}

func TestAffinityClamping(t *testing.T) {
	p := model.SearchProfile{
		Name:        "clamp",
		BucketScale: model.BucketScaleFive,
		Terms: []model.SearchProfileTerm{
			{Affinity: model.AffinityTitle, Bucket: model.BucketStrongPositive, Term: "one", Weight: 80},
			{Affinity: model.AffinityTitle, Bucket: model.BucketStrongPositive, Term: "two", Weight: 80},
			{Affinity: model.AffinityLocation, Bucket: model.BucketStrongNegative, Term: "nowhere", Weight: -120},
		},
	}
	if got := ScoreTitle("One Two", p); got.Score != 100 {
		t.Errorf("positive clamp = %d, want 100", got.Score)
	}
	if got := ScoreLocation("Nowhere", "", p); got.Score != 0 {
		t.Errorf("negative clamp = %d, want 0", got.Score)
	}
}

func TestDeriveRemotePolicy(t *testing.T) {
	tests := []struct {
		name        string
		location    string
		description string
		want        model.RemotePolicy
	}{
		{"full remote location", "Remote - France", "", model.RemoteFull},
		{"fully remote description", "Paris", "Fully remote within France.", model.RemoteFull},
		{"hybrid phrase", "Hybrid - Paris", "", model.RemoteHybrid},
		{"remote and onsite is hybrid", "Paris", "Remote or on-site.", model.RemoteHybrid},
		{"onsite location", "Onsite - Paris", "", model.RemoteOnsite},
		{"onsite description", "Paris", "This is an on-site role.", model.RemoteOnsite},
		{"no phrase is unknown", "Paris, France", "", model.RemoteUnknown},
		{"empty is unknown", "", "", model.RemoteUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeriveRemotePolicy(tt.location, tt.description); got != tt.want {
				t.Errorf("policy(%q, %q) = %q, want %q", tt.location, tt.description, got, tt.want)
			}
		})
	}
}

func TestRemoteAcceptedIsMembership(t *testing.T) {
	p := testProfile()
	if !RemoteAccepted(model.RemoteFull, p) || !RemoteAccepted(model.RemoteUnknown, p) {
		t.Error("profile must accept remote and unknown")
	}
	if RemoteAccepted(model.RemoteHybrid, p) || RemoteAccepted(model.RemoteOnsite, p) {
		t.Error("profile must not accept hybrid or onsite")
	}
	strict := p
	strict.Remote = []model.SearchProfileRemote{{Policy: model.RemoteFull, Accepted: true}}
	if RemoteAccepted(model.RemoteUnknown, strict) {
		t.Error("a policy without an accepted row must not be accepted")
	}
}

func TestScoreCombinesTheThreeSignals(t *testing.T) {
	got := Score("Platform Engineer", "Remote - France", "Fully remote in France.", testProfile())
	if got.TitleAffinity.Score != 85 {
		t.Errorf("title affinity = %d, want 85", got.TitleAffinity.Score)
	}
	if got.LocationAffinity.Score != 55 { // france 35 + description france 20
		t.Errorf("location affinity = %d, want 55 (matches %v)", got.LocationAffinity.Score, got.LocationAffinity.Matches)
	}
	if got.RemotePolicy != model.RemoteFull {
		t.Errorf("policy = %q, want remote", got.RemotePolicy)
	}
}

func TestDescriptionTermDoesNotChangeRemotePolicy(t *testing.T) {
	p := testProfile()
	p.Terms = append(p.Terms, model.SearchProfileTerm{
		Affinity: model.AffinityDescription, Bucket: model.BucketMildPositive, Term: "on site", Weight: 20,
	})
	got := Score("Platform Engineer", "Paris, France", "On-site role, offices in Paris.", p)
	if got.RemotePolicy != model.RemoteOnsite {
		t.Errorf("policy = %q, want onsite from the detector", got.RemotePolicy)
	}
	if got.LocationAffinity.Score != 95 { // france 35 + paris 25 + description paris 15 + description "on site" 20
		t.Errorf("location affinity = %d, want 95 (matches %v)", got.LocationAffinity.Score, got.LocationAffinity.Matches)
	}
}

func TestScoreDoesNotUseStoredScores(t *testing.T) {
	p := testProfile()
	titled := ScoreTitle("Platform Engineer", p)
	if titled.Score != 85 {
		t.Fatalf("score = %d", titled.Score)
	}
	// A different profile computes a different score from the same title.
	p.Terms = []model.SearchProfileTerm{
		{Affinity: model.AffinityTitle, Bucket: model.BucketStrongNegative, Term: "platform engineer", Weight: -60},
	}
	if got := ScoreTitle("Platform Engineer", p); got.Score != 0 {
		t.Errorf("score with changed profile = %d, want 0", got.Score)
	}
}
