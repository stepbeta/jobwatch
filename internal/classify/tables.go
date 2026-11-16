package classify

// The tables in this file are intentionally plain, commented constants:
// the classifier is deterministic and tuned by hand (SPEC section 6).
// Change a weight only together with the table-driven tests in
// classify_test.go.

// titleKeyword is one weighted term over the normalized title. Matching
// is whole-token (phrase) matching, so "sre" does not match inside
// "pressrelease" and "sales" does not match inside "wholesale".
type titleKeyword struct {
	term   string
	weight int
	kind   string // "strong positive", "weak positive", "negative", "mild negative"
}

// titleKeywords is ordered from strongest role signals to penalties.
// Scores are additive and clamped to 0..100.
var titleKeywords = []titleKeyword{
	// Strong positives: unambiguous Platform/SRE/DevOps/Infra role names.
	// One of these alone makes the title a strong match.
	{"platform engineer", 60, "strong positive"},
	{"platform engineering", 60, "strong positive"},
	{"platform sre", 60, "strong positive"},
	{"site reliability", 60, "strong positive"},
	{"sre", 60, "strong positive"},
	{"devops", 60, "strong positive"},
	{"infrastructure engineer", 60, "strong positive"},
	{"cloud engineer", 60, "strong positive"},

	// Weak positives: technology and partial-role words. They raise an
	// adjacent title ("Engineering Manager - Platform") but cannot make a
	// non-engineering title pass on their own.
	{"kubernetes", 25, "weak positive"},
	{"reliability", 25, "weak positive"},
	{"backend infrastructure", 25, "weak positive"},
	{"platform", 25, "weak positive"},
	{"infrastructure", 25, "weak positive"},
	{"cloud", 25, "weak positive"},
	{"terraform", 20, "weak positive"},
	{"observability", 20, "weak positive"},
	{"distributed systems", 20, "weak positive"},
	{"linux", 15, "weak positive"},

	// Strong negatives: roles whose title may contain adjacent platform
	// words but which are not the target job family.
	{"marketing", -120, "negative"},
	{"sales", -120, "negative"},
	{"recruiter", -120, "negative"},
	{"internship", -120, "negative"},
	{"account executive", -120, "negative"},
	{"talent acquisition", -120, "negative"},
	{"business development", -120, "negative"},
	{"intern", -120, "negative"},
	{"trainee", -120, "negative"},
	{"apprentice", -120, "negative"},
	{"stage", -120, "negative"},      // French internship
	{"alternance", -120, "negative"}, // French apprenticeship

	// Mild negatives: management titles are adjacent to the domain but are
	// not hands-on Platform Engineer / SRE roles.
	{"manager", -20, "mild negative"},
	{"director", -20, "mild negative"},
	{"head of", -20, "mild negative"},
	{"vp", -20, "mild negative"},
	{"chief", -20, "mild negative"},
}

// remotePhrases mark a fully remote workplace. Longer phrases come first
// so the reported reason is the most specific one.
var remotePhrases = []string{
	"fully remote",
	"full remote",
	"remote first",
	"100 remote",
	"work from home",
	"work remotely",
	"remote",
	"teletravail",
	"travail a distance",
	"wfh",
	"anywhere",
}

// hybridPhrases mark a mixed office/remote policy.
var hybridPhrases = []string{
	"partially remote",
	"part remote",
	"hybrid remote",
	"hybrid",
	"hybride",
	"flexible remote",
	"remote friendly",
}

// onsitePhrases mark a workplace that explicitly requires presence.
var onsitePhrases = []string{
	"on site",
	"onsite",
	"in office",
	"office based",
	"full time office",
	"presentiel",
	"sur site",
}

// locationKeyword scores one geographical term in a location string. A
// negative weight marks a place that is unlikely to be France-compatible.
type locationKeyword struct {
	term   string
	weight int
}

// locationKeywords are applied to the location string only. Description
// geography is scored by descriptionKeywords below so a passing mention
// ("our US customers") cannot dominate the decision.
var locationKeywords = []locationKeyword{
	// France and its main tech hubs.
	{"france", 35},
	{"paris", 25},
	{"lyon", 25},
	{"nantes", 25},
	{"bordeaux", 25},
	{"lille", 25},
	{"toulouse", 25},
	{"marseille", 25},
	{"rennes", 25},
	{"grenoble", 25},
	{"montpellier", 25},
	{"strasbourg", 25},
	{"nice", 25},
	{"sophia antipolis", 25},

	// Broadly compatible geographies.
	{"emea", 20},
	{"european union", 15},
	{"europe", 15},
	{"eu", 15},
	{"worldwide", 10},
	{"global", 10},

	// Time-zone hints that usually include France.
	{"central european time", 10},
	{"european time zones", 10},
	{"eu time zones", 10},
	{"paris time", 10},
	{"cet", 10},
	{"cest", 10},
	{"utc 1", 10},
	{"utc 2", 10},

	// Unlikely / incompatible locations.
	{"united states", -35},
	{"usa", -35},
	{"us", -35},
	{"canada", -35},
	{"new york", -35},
	{"san francisco", -35},
	{"seattle", -35},
	{"austin", -35},
	{"boston", -35},
	{"los angeles", -35},
	{"chicago", -35},
	{"denver", -35},
	{"toronto", -35},
	{"vancouver", -35},
	{"montreal", -35},
	{"india", -35},
	{"bangalore", -35},
	{"bengaluru", -35},
	{"australia", -35},
	{"sydney", -35},
	{"melbourne", -35},
	{"singapore", -35},
	{"japan", -35},
	{"tokyo", -35},
	{"china", -35},
	{"brazil", -35},
	{"mexico", -35},
	{"apac", -35},
	{"latam", -35},
	{"south america", -35},
	{"united kingdom", -20},
	{"uk", -20},
	{"london", -20},
	{"germany", -20},
	{"berlin", -20},
	{"munich", -20},
	{"spain", -15},
	{"madrid", -15},
	{"barcelona", -15},
	{"netherlands", -15},
	{"amsterdam", -15},
	{"belgium", -15},
	{"brussels", -15},
	{"italy", -15},
	{"milan", -15},
	{"portugal", -15},
	{"lisbon", -15},
	{"poland", -15},
	{"warsaw", -15},
	{"switzerland", -15},
	{"zurich", -15},
}

// descriptionKeywords add location evidence from the description. They
// carry lower weights than the location field; negatives are deliberately
// absent because descriptions often mention customers or offices abroad.
var descriptionKeywords = []locationKeyword{
	{"france", 20},
	{"paris", 15},
	{"emea", 15},
	{"european union", 10},
	{"europe", 10},
	{"worldwide", 5},
	{"global", 5},
	{"central european time", 5},
	{"european time zones", 5},
	{"eu time zones", 5},
	{"paris time", 5},
	{"cet", 5},
	{"cest", 5},
	{"utc 1", 5},
	{"utc 2", 5},
}

// Weight and policy caps. Full remote France is 100; hybrid and onsite
// roles are not the target, so their scores are capped even in France.
const (
	remoteLocationWeight    = 45
	remoteDescriptionWeight = 25
	hybridLocationWeight    = 10
	hybridDescriptionWeight = 5

	unknownPolicyCap = 60
	hybridPolicyCap  = 75
	onsitePolicyCap  = 50
)
