package pdfmeta

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func derefTime(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.UTC().Format(time.RFC3339)
}

// Extract opens its fixtures and compares the decoded metadata field by
// field. Extract has no error return, so no malformed input can fail an
// upload; the garbage cases below prove it also never panics.
func TestExtract(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		title    string
		author   string
		producer string
		created  string
		modified string
		pages    int
		version  string
	}{
		{
			name:     "normal information dictionary",
			file:     "normal.pdf",
			title:    "Acme CV",
			author:   "Alice Martin",
			producer: "Test Producer 1.0",
			created:  "2026-01-02T15:30:00Z",
			modified: "2026-01-03T10:00:00Z",
			pages:    2,
			version:  "1.7",
		},
		{
			name:    "hex and UTF-16BE strings",
			file:    "hex-utf16.pdf",
			title:   "Hello",
			author:  "Alice",
			created: "2026-02-01T00:00:00Z",
			pages:   1,
			version: "1.4",
		},
		{
			name:    "literal escapes",
			file:    "literal-escapes.pdf",
			title:   "Résumé (CV) draft",
			author:  "Alice \\ Smith",
			pages:   1,
			version: "1.5",
		},
		{
			name:     "missing dates",
			file:     "no-date.pdf",
			title:    "Undated CV",
			author:   "Bob",
			producer: "No Dates 2.0",
			pages:    1,
			version:  "1.6",
		},
		{
			name:    "missing information dictionary",
			file:    "no-info.pdf",
			pages:   3,
			version: "1.4",
		},
		{
			name:    "truncated file",
			file:    "truncated.pdf",
			pages:   1,
			version: "1.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Extract(readFixture(t, tt.file))
			if derefString(got.Title) != tt.title {
				t.Errorf("title = %q, want %q", derefString(got.Title), tt.title)
			}
			if derefString(got.Author) != tt.author {
				t.Errorf("author = %q, want %q", derefString(got.Author), tt.author)
			}
			if derefString(got.Producer) != tt.producer {
				t.Errorf("producer = %q, want %q", derefString(got.Producer), tt.producer)
			}
			if derefTime(got.CreationDate) != tt.created {
				t.Errorf("creation date = %q, want %q", derefTime(got.CreationDate), tt.created)
			}
			if derefTime(got.ModificationDate) != tt.modified {
				t.Errorf("modification date = %q, want %q", derefTime(got.ModificationDate), tt.modified)
			}
			if got.PageCount != tt.pages {
				t.Errorf("page count = %d, want %d", got.PageCount, tt.pages)
			}
			if got.Version != tt.version {
				t.Errorf("version = %q, want %q", got.Version, tt.version)
			}
		})
	}
}

func TestExtractGarbageReturnsEmptyMetadata(t *testing.T) {
	garbage := [][]byte{
		nil,
		{},
		[]byte("not a pdf at all"),
		bytes.Repeat([]byte{0x00, 0xFF}, 64),
		[]byte("%PDF-"),
		[]byte("%PDF-1.7\n<< /Title <zz> /Author (unterminated"),
	}
	for i, data := range garbage {
		meta := Extract(data)
		if meta.Title != nil || meta.Author != nil || meta.Producer != nil ||
			meta.CreationDate != nil || meta.ModificationDate != nil || meta.PageCount != 0 {
			t.Errorf("garbage %d produced metadata %+v", i, meta)
		}
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"D:20260102153000Z", "2026-01-02T15:30:00Z"},
		{"D:20260103120000+02'00'", "2026-01-03T10:00:00Z"},
		{"D:20260103120000-0500", "2026-01-03T17:00:00Z"},
		{"D:20260201", "2026-02-01T00:00:00Z"},
		{"20260102", "2026-01-02T00:00:00Z"},
		{"D:20260230120000Z", ""},
		{"D:20261301120000Z", ""},
		{"D:not-a-date", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := parseDate(tt.in)
		if derefTime(got) != tt.want {
			t.Errorf("parseDate(%q) = %q, want %q", tt.in, derefTime(got), tt.want)
		}
	}
}

func TestVersion(t *testing.T) {
	for in, want := range map[string]string{
		"%PDF-1.7\n":         "1.7",
		"%PDF-2.0":           "2.0",
		"junk %PDF-1.4 junk": "1.4",
		"%PDF-":              "",
		"no header":          "",
	} {
		if got := pdfVersion([]byte(in)); got != want {
			t.Errorf("pdfVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// Compile-time guard that Extract returns one value: there is no error path
// an upload could trip on.
var _ model.PDFMetadata = Extract(nil)
