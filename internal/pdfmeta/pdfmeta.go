// Package pdfmeta extracts presentation metadata from PDF bytes without an
// external dependency. Every field is best-effort: a missing, unreadable or
// malformed value leaves that field empty, and Extract never returns an
// error that could fail an upload.
package pdfmeta

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/stepbeta/jobwatch/internal/model"
)

// Extract reads the PDF header version, the information dictionary values
// (/Title, /Author, /Producer, /CreationDate, /ModDate) and the number of
// page objects. Unknown values are nil and PageCount is 0; garbage input
// yields an empty result rather than an error.
func Extract(data []byte) model.PDFMetadata {
	if len(data) == 0 {
		return model.PDFMetadata{}
	}
	meta := model.PDFMetadata{Version: pdfVersion(data)}
	meta.Title = infoString(data, "Title")
	meta.Author = infoString(data, "Author")
	meta.Producer = infoString(data, "Producer")
	if raw := infoString(data, "CreationDate"); raw != nil {
		meta.CreationDate = parseDate(*raw)
	}
	if raw := infoString(data, "ModDate"); raw != nil {
		meta.ModificationDate = parseDate(*raw)
	}
	meta.PageCount = countPages(data)
	return meta
}

// pdfVersion parses the version in the %PDF- header, for example "1.7".
func pdfVersion(data []byte) string {
	i := bytes.Index(data, []byte("%PDF-"))
	if i < 0 {
		return ""
	}
	rest := data[i+len("%PDF-"):]
	end := 0
	for end < len(rest) && end < 8 {
		c := rest[end]
		if (c < '0' || c > '9') && c != '.' {
			break
		}
		end++
	}
	v := string(rest[:end])
	if v == "" || !strings.Contains(v, ".") {
		return ""
	}
	return v
}

// infoString returns the decoded string value of /key in the information
// dictionary. It accepts literal strings, hex strings and indirect
// references to a string object.
func infoString(data []byte, key string) *string {
	needle := []byte("/" + key)
	for offset := 0; offset < len(data); {
		i := bytes.Index(data[offset:], needle)
		if i < 0 {
			return nil
		}
		i += offset
		after := i + len(needle)
		// Require a delimiter so /Title never matches /TitleExtra.
		if after < len(data) && !isDelimiter(data[after]) {
			offset = after
			continue
		}
		if s, ok := valueAt(data, after, 0); ok {
			if s = strings.TrimSpace(s); s != "" {
				return &s
			}
		}
		offset = after
	}
	return nil
}

// valueAt decodes the value that starts at or after pos. depth limits
// indirect-reference resolution to one level.
func valueAt(data []byte, pos, depth int) (string, bool) {
	for pos < len(data) && isSpace(data[pos]) {
		pos++
	}
	if pos >= len(data) {
		return "", false
	}
	switch data[pos] {
	case '(':
		raw, ok := literalString(data, pos)
		if !ok {
			return "", false
		}
		return decodeText(raw), true
	case '<':
		if pos+1 < len(data) && data[pos+1] == '<' {
			return "", false // a dictionary, not a string
		}
		raw, ok := hexString(data, pos)
		if !ok {
			return "", false
		}
		return decodeText(raw), true
	}
	if depth > 0 {
		return "", false
	}
	// "/Title 12 0 R" resolves to the string in object "12 0 obj".
	if num, gen, ok := referenceAt(data, pos); ok {
		if obj := findObject(data, num, gen); obj >= 0 {
			return valueAt(data, obj, depth+1)
		}
	}
	return "", false
}

// literalString parses a PDF literal string starting at the '(' at pos,
// handling escapes and balanced parentheses. It returns the raw bytes.
func literalString(data []byte, pos int) ([]byte, bool) {
	if pos >= len(data) || data[pos] != '(' {
		return nil, false
	}
	var out []byte
	depth := 1
	for i := pos + 1; i < len(data); i++ {
		c := data[i]
		switch c {
		case '\\':
			if i+1 >= len(data) {
				return nil, false
			}
			i++
			switch e := data[i]; e {
			case 'n':
				out = append(out, '\n')
			case 'r':
				out = append(out, '\r')
			case 't':
				out = append(out, '\t')
			case 'b':
				out = append(out, '\b')
			case 'f':
				out = append(out, '\f')
			case '(', ')', '\\':
				out = append(out, e)
			case '\r':
				// Line continuation: swallow CR and an optional LF.
				if i+1 < len(data) && data[i+1] == '\n' {
					i++
				}
			case '\n':
				// Line continuation.
			default:
				if e >= '0' && e <= '7' {
					value := int(e - '0')
					for n := 0; n < 2 && i+1 < len(data) && data[i+1] >= '0' && data[i+1] <= '7'; n++ {
						i++
						value = value*8 + int(data[i]-'0')
					}
					out = append(out, byte(value))
				} else {
					out = append(out, e)
				}
			}
		case '(':
			depth++
			out = append(out, c)
		case ')':
			depth--
			if depth == 0 {
				return out, true
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return nil, false
}

// hexString parses a PDF hex string <…> and returns its bytes.
func hexString(data []byte, pos int) ([]byte, bool) {
	if pos >= len(data) || data[pos] != '<' {
		return nil, false
	}
	var digits []byte
	for i := pos + 1; i < len(data); i++ {
		c := data[i]
		if c == '>' {
			if len(digits)%2 == 1 {
				digits = append(digits, '0')
			}
			out := make([]byte, len(digits)/2)
			for n := 0; n < len(out); n++ {
				hi, ok1 := hexValue(digits[n*2])
				lo, ok2 := hexValue(digits[n*2+1])
				if !ok1 || !ok2 {
					return nil, false
				}
				out[n] = hi<<4 | lo
			}
			return out, true
		}
		if isSpace(c) {
			continue
		}
		if !isHex(c) {
			return nil, false
		}
		digits = append(digits, c)
	}
	return nil, false
}

func hexValue(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// decodeText converts PDF string bytes to text. A UTF-16 BOM wins; valid
// UTF-8 is used as-is; everything else falls back to Latin-1, the common
// core of PDFDocEncoding.
func decodeText(raw []byte) string {
	switch {
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		units := make([]uint16, 0, (len(raw)-2)/2)
		for i := 2; i+1 < len(raw); i += 2 {
			units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
		}
		return string(utf16.Decode(units))
	case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
		units := make([]uint16, 0, (len(raw)-2)/2)
		for i := 2; i+1 < len(raw); i += 2 {
			units = append(units, uint16(raw[i+1])<<8|uint16(raw[i]))
		}
		return string(utf16.Decode(units))
	case utf8.Valid(raw):
		return strings.TrimPrefix(string(raw), "\uFEFF")
	}
	runes := make([]rune, len(raw))
	for i, b := range raw {
		runes[i] = rune(b)
	}
	return string(runes)
}

// referenceAt parses "N G R" at pos and returns the object number and
// generation.
func referenceAt(data []byte, pos int) (int, int, bool) {
	i := pos
	num, ok := readInt(data, &i)
	if !ok {
		return 0, 0, false
	}
	skipSpace(data, &i)
	gen, ok := readInt(data, &i)
	if !ok {
		return 0, 0, false
	}
	skipSpace(data, &i)
	if i >= len(data) || data[i] != 'R' {
		return 0, 0, false
	}
	return num, gen, true
}

// findObject returns the offset just after "N G obj", or -1.
func findObject(data []byte, num, gen int) int {
	needle := []byte(strconv.Itoa(num) + " " + strconv.Itoa(gen) + " obj")
	i := bytes.Index(data, needle)
	if i < 0 {
		return -1
	}
	return i + len(needle)
}

func readInt(data []byte, pos *int) (int, bool) {
	start := *pos
	for *pos < len(data) && data[*pos] >= '0' && data[*pos] <= '9' {
		*pos++
	}
	if *pos == start {
		return 0, false
	}
	v, err := strconv.Atoi(string(data[start:*pos]))
	if err != nil {
		return 0, false
	}
	return v, true
}

func skipSpace(data []byte, pos *int) {
	for *pos < len(data) && isSpace(data[*pos]) {
		*pos++
	}
}

// parseDate converts a PDF D:YYYYMMDDHHmmSS±hh'mm' date to UTC. Missing
// parts default to January, day 1 and midnight; an invalid date is nil.
func parseDate(raw string) *time.Time {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "D:")
	if len(s) < 4 {
		return nil
	}
	read := func(from, n int) (int, bool) {
		if len(s) < from+n {
			return 0, false
		}
		v, err := strconv.Atoi(s[from : from+n])
		if err != nil {
			return 0, false
		}
		return v, true
	}
	year, ok := read(0, 4)
	if !ok {
		return nil
	}
	month, day, hour, minute, second := 1, 1, 0, 0, 0
	if v, ok := read(4, 2); ok {
		month = v
		if v, ok := read(6, 2); ok {
			day = v
			if v, ok := read(8, 2); ok {
				hour = v
				if v, ok := read(10, 2); ok {
					minute = v
					if v, ok := read(12, 2); ok {
						second = v
					}
				}
			}
		}
	}
	if month < 1 || month > 12 || day < 1 || day > 31 ||
		hour > 23 || minute > 59 || second > 60 {
		return nil
	}

	// The timezone follows the last parsed numeric field.
	used := 4
	for _, n := range []int{2, 2, 2, 2, 2} {
		if len(s) >= used+n && allDigits(s[used:used+n]) {
			used += n
		} else {
			break
		}
	}
	offset := 0
	rest := s[used:]
	if len(rest) > 0 {
		sign := 1
		switch rest[0] {
		case '+':
		case '-':
			sign = -1
		case 'Z', 'z':
			rest = ""
		}
		if rest != "" && (rest[0] == '+' || rest[0] == '-') {
			rest = rest[1:]
			rest = strings.ReplaceAll(rest, "'", "")
			if len(rest) >= 4 {
				oh, ok1 := readDigits(rest[0:2])
				om, ok2 := readDigits(rest[2:4])
				if ok1 && ok2 && oh <= 23 && om <= 59 {
					offset = sign * (oh*3600 + om*60)
				}
			}
		}
	}

	t := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.FixedZone("", offset))
	// Reject dates time.Date had to normalize (for example 2026-02-30).
	if t.Year() != year || int(t.Month()) != month || t.Day() != day {
		return nil
	}
	utc := t.UTC()
	return &utc
}

func readDigits(s string) (int, bool) {
	if !allDigits(s) {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	return v, err == nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// countPages counts /Type /Page objects. /Type /Pages nodes and the text
// "/Pages" do not match because the name must end at a delimiter.
func countPages(data []byte) int {
	needle := []byte("/Type")
	count := 0
	for offset := 0; offset < len(data); {
		i := bytes.Index(data[offset:], needle)
		if i < 0 {
			return count
		}
		i += offset
		j := i + len(needle)
		for j < len(data) && isSpace(data[j]) {
			j++
		}
		if j+len("/Page") <= len(data) && string(data[j:j+len("/Page")]) == "/Page" {
			end := j + len("/Page")
			if end == len(data) || !isNameChar(data[end]) {
				count++
			}
		}
		offset = j
		if offset <= i {
			offset = i + 1
		}
	}
	return count
}

func isNameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_'
}

func isSpace(c byte) bool {
	switch c {
	case 0, '\t', '\n', '\f', '\r', ' ':
		return true
	}
	return false
}

func isDelimiter(c byte) bool {
	if isSpace(c) {
		return true
	}
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}
