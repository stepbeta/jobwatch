package model

import (
	"fmt"
	"strings"
	"time"
)

// TermAffinity is the offer field a profile term applies to. Description
// terms contribute to location affinity; they never affect the remote
// policy.
type TermAffinity string

const (
	AffinityTitle       TermAffinity = "title"
	AffinityLocation    TermAffinity = "location"
	AffinityDescription TermAffinity = "description"
)

// Valid reports whether a is a known affinity.
func (a TermAffinity) Valid() bool {
	switch a {
	case AffinityTitle, AffinityLocation, AffinityDescription:
		return true
	}
	return false
}

// TermBucket is the display label of a term weight. It is a label, not a
// scoring rule: the scorer adds Weight. A three-bucket profile uses
// positive/neutral/negative; a five-bucket profile uses the strong/mild
// variants.
type TermBucket string

const (
	BucketStrongPositive TermBucket = "strong_positive"
	BucketMildPositive   TermBucket = "mild_positive"
	BucketNeutral        TermBucket = "neutral"
	BucketMildNegative   TermBucket = "mild_negative"
	BucketStrongNegative TermBucket = "strong_negative"

	// BucketPositive and BucketNegative are the three-bucket scale.
	BucketPositive TermBucket = "positive"
	BucketNegative TermBucket = "negative"
)

// Valid reports whether b is a known bucket.
func (b TermBucket) Valid() bool {
	switch b {
	case BucketStrongPositive, BucketMildPositive, BucketNeutral,
		BucketMildNegative, BucketStrongNegative,
		BucketPositive, BucketNegative:
		return true
	}
	return false
}

// BucketScale is the number of weight buckets a profile editor offers.
type BucketScale string

const (
	BucketScaleThree BucketScale = "three"
	BucketScaleFive  BucketScale = "five"
)

// Valid reports whether s is a known scale.
func (s BucketScale) Valid() bool {
	return s == BucketScaleThree || s == BucketScaleFive
}

// Buckets returns the buckets of the scale, strongest positive first.
func (s BucketScale) Buckets() []TermBucket {
	if s == BucketScaleThree {
		return []TermBucket{BucketPositive, BucketNeutral, BucketNegative}
	}
	return []TermBucket{BucketStrongPositive, BucketMildPositive, BucketNeutral,
		BucketMildNegative, BucketStrongNegative}
}

// SearchProfileTerm is one weighted term of a search profile.
type SearchProfileTerm struct {
	ID        int64
	ProfileID int64
	Affinity  TermAffinity
	Bucket    TermBucket
	Term      string
	Weight    int
	SortOrder int
}

// SearchProfileRemote is the acceptance of one remote policy in a
// profile. Remote policy itself is an offer fact; this row is the
// user's membership checkbox.
type SearchProfileRemote struct {
	Policy   RemotePolicy
	Accepted bool
}

// SearchProfile is a user's target role, geography and remote policy
// preference. It is not an offer attribute: affinity is computed from the
// profile at read time.
type SearchProfile struct {
	ID            int64
	Name          string
	TitleLabel    string
	LocationLabel string
	BucketScale   BucketScale
	IsDefault     bool
	Terms         []SearchProfileTerm
	Remote        []SearchProfileRemote
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Accepts reports whether the profile accepts an offer with policy. A
// policy without a row is not accepted.
func (p SearchProfile) Accepts(policy RemotePolicy) bool {
	for _, r := range p.Remote {
		if r.Policy == policy {
			return r.Accepted
		}
	}
	return false
}

// Validate checks the profile input. It is shared by the database layer
// and, later, the API so both reject the same data.
func (p SearchProfile) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	if !p.BucketScale.Valid() {
		return fmt.Errorf("invalid bucket scale %q", p.BucketScale)
	}
	allowed := map[TermBucket]bool{}
	for _, b := range p.BucketScale.Buckets() {
		allowed[b] = true
	}
	seen := map[string]bool{}
	for _, t := range p.Terms {
		term := strings.TrimSpace(t.Term)
		if term == "" {
			return fmt.Errorf("term in %s affinity is empty", t.Affinity)
		}
		if !t.Affinity.Valid() {
			return fmt.Errorf("invalid affinity %q for term %q", t.Affinity, term)
		}
		if !t.Bucket.Valid() || !allowed[t.Bucket] {
			return fmt.Errorf("bucket %q is not part of the %s scale (term %q)", t.Bucket, p.BucketScale, term)
		}
		if t.Bucket == BucketNeutral && t.Weight != 0 {
			return fmt.Errorf("neutral term %q must have weight 0, got %d", term, t.Weight)
		}
		if t.Bucket != BucketNeutral && t.Weight == 0 {
			return fmt.Errorf("term %q has a %s bucket but weight 0", term, t.Bucket)
		}
		key := string(t.Affinity) + "\x1f" + NormalizeKeywords(term)
		if seen[key] {
			return fmt.Errorf("duplicate %s term %q", t.Affinity, term)
		}
		seen[key] = true
	}
	for _, r := range p.Remote {
		if !r.Policy.Valid() {
			return fmt.Errorf("invalid remote policy %q", r.Policy)
		}
	}
	return nil
}
