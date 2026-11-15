package model

import (
	"fmt"
	"time"
)

// FormatTime renders t as RFC3339 in UTC, the storage format for all
// timestamp columns.
func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// ParseTime parses a stored RFC3339 timestamp.
func ParseTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse timestamp %q: %w", s, err)
	}
	return t.UTC(), nil
}
