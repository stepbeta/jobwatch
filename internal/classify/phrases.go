package classify

// The phrase lists in this file are fixed and intentionally plain: the
// remote-policy detector is deterministic and hand-tuned. There is no
// user-editable phrase list. Change a phrase only together with the
// detector tests in classify_test.go. Term weights are not here: they
// live in the search profile (search_profile_terms).

// remotePhrases mark a fully remote workplace. Longer phrases come first
// so the reported match is the most specific one.
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
