package model

import (
	"regexp"
	"testing"
)

func TestNormalizeCompanyName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Datadog", "datadog"},
		{"Datadog, Inc.", "datadog"},
		{"DATADOG INC", "datadog"},
		{"Acme GmbH", "acme"},
		{"Société Générale S.A.S.", "societe generale"},
		{"Qonto SAS", "qonto"},
		{"Foo Bar Ltd.", "foo bar"},
		{"Example Ltd", "example"},
		{"Esempio S.p.A.", "esempio"},
		{"Widgets LLC", "widgets"},
		{"Stacked Holdings SAS Ltd", "stacked holdings"},
		{"Booking.com", "bookingcom"},
		{"O'Reilly Media", "oreilly media"},
		{"  Mistral   AI  ", "mistral ai"},
		{"Back Market", "back market"},
		{"Café-Crème & Co", "cafe creme co"},
		// a suffix alone is never stripped to an empty name
		{"SAS", "sas"},
		// suffix words are only stripped at the end
		{"Inc Labs", "inc labs"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeCompanyName(tt.in); got != tt.want {
				t.Errorf("NormalizeCompanyName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Senior Site Reliability Engineer", "senior site reliability engineer"},
		{"Ingénieur DevOps (H/F)", "ingenieur devops"},
		{"Platform Engineer F/H", "platform engineer"},
		{"Platform Engineer (m/w/d)", "platform engineer"},
		{"SRE - H/F/X - Remote", "sre remote"},
		{"DevOps/SRE Engineer", "devops sre engineer"},
		{"Senior / Staff  Fullstack Engineer", "senior staff fullstack engineer"},
		{"Backend Engineer, GitLab Orbit", "backend engineer gitlab orbit"},
		{"  Cloud   Engineer\t", "cloud engineer"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeTitle(tt.in); got != tt.want {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestLocationBucket(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"Paris", "paris"},
		{"Paris, France", "paris"},
		{"Paris, Île-de-France, France", "paris"},
		{"Aix-en-Provence", "aix en provence"},
		{"Remote", "remote"},
		{"Remote, United States", "remote united states"},
		{"Remote - France", "remote france"},
		{"Remote, Canada; Remote, United States", "remote canada"},
		{"France (Remote)", "france"},
		{"Hybrid - Lyon", "hybrid lyon"},
		{"New York, NY", "new york"},
		{"Europe", "europe"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := LocationBucket(tt.in); got != tt.want {
				t.Errorf("LocationBucket(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

var hexSHA256 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestFingerprint(t *testing.T) {
	base := Fingerprint("datadog", "site reliability engineer", "")
	if !hexSHA256.MatchString(base) {
		t.Fatalf("fingerprint %q is not SHA-256 hex", base)
	}
	tests := []struct {
		name               string
		company, title, lb string
		wantEqualBase      bool
	}{
		{"same inputs", "datadog", "site reliability engineer", "", true},
		{"different title", "datadog", "platform engineer", "", false},
		{"different company", "qonto", "site reliability engineer", "", false},
		{"with location bucket", "datadog", "site reliability engineer", "paris", false},
		// the separator prevents "ab"+"c" == "a"+"bc"
		{"ambiguous concatenation", "datadogsite", " reliability engineer", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Fingerprint(tt.company, tt.title, tt.lb)
			if (got == base) != tt.wantEqualBase {
				t.Errorf("Fingerprint(%q,%q,%q) equal-to-base = %v, want %v", tt.company, tt.title, tt.lb, got == base, tt.wantEqualBase)
			}
		})
	}
}

func TestFingerprintFromRawNames(t *testing.T) {
	// Variants of the same company/title must produce the same fingerprint
	// once normalized.
	a := Fingerprint(NormalizeCompanyName("Datadog, Inc."), NormalizeTitle("Site Reliability Engineer (H/F)"), "")
	b := Fingerprint(NormalizeCompanyName("DATADOG"), NormalizeTitle("site  reliability engineer"), "")
	if a != b {
		t.Errorf("normalized variants differ: %s vs %s", a, b)
	}
	// Different buckets of the same role differ.
	p := Fingerprint("datadog", "sre", LocationBucket("Paris, France"))
	r := Fingerprint("datadog", "sre", LocationBucket("Remote - France"))
	if p == r {
		t.Error("different location buckets produced the same fingerprint")
	}
}

func TestTextHash(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{"identical", "hello world", "hello world", true},
		{"whitespace only", "hello\n\n  world ", " hello world", true},
		{"content change", "hello world", "hello there", false},
		{"case matters", "Hello", "hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TextHash(tt.a) == TextHash(tt.b); got != tt.equal {
				t.Errorf("TextHash equality = %v, want %v", got, tt.equal)
			}
		})
	}
}
