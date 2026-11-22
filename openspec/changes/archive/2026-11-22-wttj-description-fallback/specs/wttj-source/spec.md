# Spec Delta

## ADDED Requirements

### Requirement: Blank index descriptions fall back to the public job page

When an Algolia hit carries neither `profile` nor `summary`, the adapter SHALL fetch the offer's public WTTJ page and store the JSON-LD `JobPosting.description`, converted from HTML to plain text, unless the offer already has a stored description. A non-blank `profile` or `summary` MUST keep the current mapping and MUST NOT trigger a page fetch.

#### Scenario: Both index description fields are blank

- **WHEN** a hit has an empty `profile` and an empty `summary` and the offer has no stored description
- **THEN** the adapter requests the offer URL once and returns the page's `JobPosting.description` as the offer description

#### Scenario: The offer already has a stored description

- **WHEN** a hit has an empty `profile` and an empty `summary` but the scan supplies a stored description for that offer
- **THEN** the adapter does not request the offer page and the scan keeps the stored text

#### Scenario: Profile is present

- **WHEN** a hit has a non-blank `profile`
- **THEN** the adapter stores the profile verbatim and does not request the offer page

#### Scenario: Only summary is present

- **WHEN** a hit has a blank `profile` but a non-blank `summary`
- **THEN** the adapter stores the summary and does not request the offer page

#### Scenario: The page has no JobPosting description

- **WHEN** the offer page loads but contains no JSON-LD `JobPosting.description`
- **THEN** the offer is returned with an empty description and the scan continues

### Requirement: A detail-page failure does not fail the source

A failure to fetch or parse one offer page SHALL leave that offer's description empty and SHALL NOT abort the scan or mark the source `error`. The listing result MUST still be applied.

#### Scenario: One offer page is unavailable

- **WHEN** the detail request for one description-less hit returns an HTTP or network error
- **THEN** that offer is returned without a description, the other offers are unchanged, and the source status remains `ok`

### Requirement: Detail-page requests use the shared polite client

The adapter SHALL fetch detail pages through the shared HTTP client, so robots.txt and the per-source interval apply to every page request. Detail requests SHALL carry the same single browser User-Agent the adapter already sends to WTTJ's API endpoints, because the site's WAF answers the identifying JobWatch UA on HTML pages with a challenge; this is a recorded exception to the identifying-UA rule, not per-request rotation.

#### Scenario: Detail requests are rate-limited

- **WHEN** several description-less hits are fetched in one scan
- **THEN** each page request goes through the shared client, carries the WTTJ browser User-Agent, and is spaced by the source interval
