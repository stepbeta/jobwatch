package model

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"
)

// legalSuffixes are company-name tokens stripped from the END of a
// normalized company name (repeatedly, e.g. "foo sas ltd" -> "foo").
// Matching happens after dots are removed, so "S.A.S." matches "sas"
// and "S.p.A." matches "spa".
var legalSuffixes = map[string]bool{
	"gmbh":         true, // Germany/Austria
	"ag":           true, // Germany/Switzerland
	"kg":           true, // Germany
	"inc":          true, // US
	"incorporated": true,
	"corp":         true,
	"corporation":  true,
	"llc":          true, // US
	"ltd":          true, // UK
	"limited":      true,
	"plc":          true, // UK
	"sas":          true, // France
	"sasu":         true, // France
	"sarl":         true, // France
	"sa":           true, // France/Switzerland/Spain
	"spa":          true, // Italy
	"srl":          true, // Italy/Romania
	"bv":           true, // Netherlands
	"nv":           true, // Netherlands/Belgium
}

// workplaceWords are location segments that only describe the workplace
// type; LocationBucket keeps the following segment to stay specific.
var workplaceWords = map[string]bool{
	"remote":         true,
	"full remote":    true,
	"fully remote":   true,
	"remote first":   true,
	"hybrid":         true,
	"onsite":         true,
	"on site":        true,
	"anywhere":       true,
	"teletravail":    true,
	"en teletravail": true,
}

// genderMarker matches inclusive-hiring markers common in FR/DE titles:
// "(H/F)", "F/H", "(m/w/d)", "H/F/X"...
var genderMarker = regexp.MustCompile(`\(?\b[hfmwdx](?:\s*/\s*[hfmwdx]){1,3}\b\)?`)

var accentFolder = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"ç", "c",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ñ", "n",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o", "ø", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y",
	"œ", "oe", "æ", "ae", "ß", "ss",
)

// foldLower lowercases s and folds common Latin diacritics to ASCII.
func foldLower(s string) string {
	return accentFolder.Replace(strings.ToLower(s))
}

// alnumWords replaces every non letter/digit rune with a space and
// collapses runs of whitespace.
func alnumWords(s string) string {
	mapped := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, s)
	return strings.Join(strings.Fields(mapped), " ")
}

// NormalizeCompanyName lowercases, folds accents, strips punctuation and
// trailing legal suffixes. A suffix is never stripped if it is the only
// remaining token.
func NormalizeCompanyName(name string) string {
	s := foldLower(name)
	s = strings.NewReplacer(".", "", "'", "", "’", "").Replace(s)
	tokens := strings.Fields(alnumWords(s))
	for len(tokens) > 1 && legalSuffixes[tokens[len(tokens)-1]] {
		tokens = tokens[:len(tokens)-1]
	}
	return strings.Join(tokens, " ")
}

// NormalizeTitle lowercases, folds accents, removes gender markers and
// punctuation, and collapses whitespace.
func NormalizeTitle(title string) string {
	s := foldLower(title)
	s = genderMarker.ReplaceAllString(s, " ")
	return alnumWords(s)
}

// NormalizeKeywords lowercases, folds accents, replaces punctuation with
// spaces and collapses whitespace. It is the shared normalization for
// free-text keyword matching (classifier locations and descriptions).
func NormalizeKeywords(s string) string {
	return alnumWords(foldLower(s))
}

// LocationBucket reduces a free-text location to a coarse bucket used to
// disambiguate fingerprint collisions. It keeps the first segment (split
// on , ; | / ( and " - "); if that segment only names a workplace type
// ("Remote", "Hybrid"...), the next segment is appended so that
// "Remote, United States" and "Remote - France" stay distinct.
// An empty result means "unknown location".
func LocationBucket(location string) string {
	s := foldLower(location)
	s = strings.ReplaceAll(s, " - ", ",")
	segs := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', ';', '|', '/', '(', ')':
			return true
		}
		return false
	})
	var parts []string
	for _, seg := range segs {
		if w := alnumWords(seg); w != "" {
			parts = append(parts, w)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if workplaceWords[parts[0]] && len(parts) > 1 {
		return parts[0] + " " + parts[1]
	}
	return parts[0]
}

// Fingerprint returns the SHA-256 hex identity of an offer. Fields are
// joined with a unit separator (0x1f) to avoid ambiguous concatenation.
// bucket is empty for the base fingerprint; a non-empty bucket is only
// used when the base fingerprint collides with a different-location offer.
func Fingerprint(companyNormalized, titleNormalized, bucket string) string {
	key := companyNormalized + "\x1f" + titleNormalized
	if bucket != "" {
		key += "\x1f" + bucket
	}
	return sha256Hex(key)
}

// NormalizeText collapses whitespace (including newlines) so cosmetic
// formatting changes do not count as content changes.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TextHash returns the SHA-256 hex of the normalized description.
func TextHash(s string) string {
	return sha256Hex(NormalizeText(s))
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
