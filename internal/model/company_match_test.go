package model

import "testing"

func TestSimilarCompanyNames(t *testing.T) {
	tests := []struct {
		name       string
		a, b       string
		wantScore  int
		wantReason string
	}{
		{"identical", "acme", "acme", 100, "same normalized name"},
		{"empty", "", "acme", 0, ""},
		{"one name extends the other", "qonto", "qonto france", 75, "one name extends the other"},
		{"two extra tokens", "acme", "acme cloud services", 70, "one name extends the other"},
		{"three extra tokens", "acme", "acme cloud services paris", 0, ""},
		{"tokens not nested", "acme paris", "acme lyon", 0, ""},
		{"too short to extend", "ab", "ab cloud", 0, ""},
		{"one edit", "datadog", "datadogg", 90, "similar spelling"},
		{"one edit short name", "qonto", "konto", 90, "similar spelling"},
		{"two edits on a long name", "cloudflare", "cloudflair", 80, "similar spelling"},
		{"too many edits", "datadog", "dog", 0, ""},
		{"unrelated", "gitlab", "spotify", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, reason := SimilarCompanyNames(tt.a, tt.b)
			if score != tt.wantScore || reason != tt.wantReason {
				t.Errorf("SimilarCompanyNames(%q, %q) = %d, %q; want %d, %q",
					tt.a, tt.b, score, reason, tt.wantScore, tt.wantReason)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"qonto", "konto", 1},
		{"é", "e", 1},
	}
	for _, tt := range tests {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
