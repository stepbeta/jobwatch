package sources

import (
	"strings"

	"github.com/stepbeta/jobwatch/internal/model"
)

// EmployerRule is the declarative employer extraction used by RSS and
// HTML sources that list several employers. With only Separator set it
// splits "Employer: Role" in the title. Field (RSS) or Selector (HTML)
// reads the employer from a separate element instead; the title prefix is
// then stripped when it repeats that employer.
type EmployerRule struct {
	Separator string `json:"separator,omitempty"`
	Field     string `json:"field,omitempty"`
	Selector  string `json:"selector,omitempty"`
}

func (r *EmployerRule) separator() string {
	if r == nil || strings.TrimSpace(r.Separator) == "" {
		return ":"
	}
	return r.Separator
}

// apply returns the rewritten title and company. employerValue is the
// value read from Field/Selector, empty when the rule reads the title.
func (r *EmployerRule) apply(title, company, employerValue string) (string, string) {
	if r == nil {
		return title, company
	}
	sep := r.separator()
	employer := strings.TrimSpace(employerValue)
	if employer == "" {
		i := strings.Index(title, sep)
		if i <= 0 {
			return title, company
		}
		left := strings.TrimSpace(title[:i])
		right := strings.TrimSpace(title[i+len(sep):])
		if left == "" || right == "" {
			return title, company
		}
		return right, left
	}
	rest := strings.TrimSpace(strings.TrimPrefix(title, employer))
	if strings.HasPrefix(rest, sep) {
		if body := strings.TrimSpace(rest[len(sep):]); body != "" {
			title = body
		}
	}
	return title, employer
}

// companyKey namespaces an adapter token as the company identifier.
func companyKey(typ model.SourceType, token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	return string(typ) + ":" + token
}

// IncompleteReasonFor reports why an item cannot be used. A company key
// is evidence even when the display name is missing.
func IncompleteReasonFor(company, key, title string) model.IncompleteReason {
	missingCompany := strings.TrimSpace(company) == "" && strings.TrimSpace(key) == ""
	missingTitle := strings.TrimSpace(title) == ""
	switch {
	case missingCompany && missingTitle:
		return model.IncompleteMissingCompanyTitle
	case missingCompany:
		return model.IncompleteMissingCompany
	case missingTitle:
		return model.IncompleteMissingTitle
	}
	return ""
}

// finalizeOffer applies the source's declared company fallback and marks
// the item incomplete when it has no usable company or title.
func finalizeOffer(o RawOffer, fallbackCompany string) RawOffer {
	o.Company = firstNonEmpty(o.Company, fallbackCompany)
	o.IncompleteReason = IncompleteReasonFor(o.Company, o.CompanyKey, o.Title)
	return o
}
