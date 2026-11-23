# Spec Delta

## Purpose

Defines how a CSS-selector HTML listing becomes offers, including the optional one-time detail-page description fetch and the polite request budget it must respect.

## ADDED Requirements

### Requirement: Listing selectors are the default surface

The HTML adapter SHALL parse one listing page using `item_selector` and `title_selector` plus optional link, location, company and uid selectors, and SHALL NOT request any offer page unless detail descriptions are configured.

#### Scenario: Minimal listing configuration

- **WHEN** a source sets only `item_selector` and `title_selector`
- **THEN** the adapter requests the listing page once and emits one offer per matching item

#### Scenario: Missing required selectors

- **WHEN** `item_selector` or `title_selector` is absent or blank
- **THEN** the source fails with a configuration error

#### Scenario: No detail selector configured

- **WHEN** the source has no `detail_description_selector`
- **THEN** the adapter makes no request to any offer link

### Requirement: Detail descriptions are opt-in

When `detail_description_selector` is set, the adapter SHALL fetch each eligible item's resolved link through the shared client, select the configured element, convert it to plain text, and emit it as the offer description, taking precedence over the listing description.

#### Scenario: Detail page holds the full text

- **WHEN** the listing description is a short summary and the detail page's selected element contains the full posting
- **THEN** the offer is emitted with the detail page text, not the listing summary

#### Scenario: Detail element is absent

- **WHEN** the detail page loads but contains no element matching the selector
- **THEN** the offer keeps the listing description

#### Scenario: No listing description either

- **WHEN** the listing has no description and the detail page has no matching element
- **THEN** the offer is emitted with an empty description

### Requirement: Stored detail text is reused

The adapter SHALL skip the detail request when the offer has a stored non-empty description that differs from the listing description, and SHALL emit that stored text so the scan neither re-fetches nor overwrites it with the listing text.

#### Scenario: Stored full text and a short listing

- **WHEN** the stored description is the full page text and the listing reports a summary
- **THEN** the adapter makes no detail request and emits the stored text

#### Scenario: Stored text differs only by whitespace

- **WHEN** the stored text and the listing description are equal after collapsing whitespace
- **THEN** the adapter treats them as equal and fetches the detail page

### Requirement: A stored listing summary is upgraded once

The adapter SHALL fetch the detail page when the stored description is empty or equals the listing description after whitespace normalization, so an offer that previously stored only the listing text is upgraded to the detail text exactly once.

#### Scenario: Existing offers hold the listing summary

- **WHEN** a source is given a detail selector while its offers store the listing summary
- **THEN** the next scan fetches each detail page once and replaces the stored summary with the detail text

#### Scenario: Already upgraded

- **WHEN** the stored text is the detail text and the listing still reports its summary
- **THEN** the following scan makes no detail request

### Requirement: Detail lookups require a stable uid

The stored-text comparison SHALL be keyed by the item's external uid. When the adapter emits no uid, it SHALL treat the stored description as unknown and request the detail page on every scan.

#### Scenario: Uid configured

- **WHEN** items carry a `uid_selector` or `uid_attr` value
- **THEN** stored descriptions are matched by that uid and skipped on later scans

#### Scenario: No uid configured

- **WHEN** items carry no uid
- **THEN** every scan requests the detail page for every item

### Requirement: Per-item detail failures do not fail the source

A failed, empty or unmatched detail request SHALL leave the listing description or stored text in place and MUST NOT fail the scan or mark the source `error`.

#### Scenario: One detail page returns a server error

- **WHEN** one item's detail request returns HTTP 500 and other items succeed
- **THEN** the failed item keeps its listing description, the other items get their detail text, and the source remains `ok`

#### Scenario: One detail page has no description

- **WHEN** one item's detail page lacks the selected element and a later item has it
- **THEN** the first item keeps its listing description and the later item is still processed

### Requirement: A blocked detail request stops the detail phase

When a detail request is blocked (robots.txt disallow, HTTP 401, 403 or 429), the adapter SHALL stop issuing detail requests for the rest of the scan and SHALL return the listing offers normally, leaving the source result `ok`.

#### Scenario: Detail pages start refusing access

- **WHEN** the first detail request is blocked
- **THEN** no further detail requests are made in that scan and every listing offer is still returned

#### Scenario: The block clears later

- **WHEN** a later scan reaches a previously blocked item and the detail page now responds
- **THEN** the item is upgraded like any other missing description

### Requirement: Detail requests are polite and sequential

Detail requests SHALL go through the shared client, so robots.txt, the per-source interval (HTML never below 5 seconds) and the identifying User-Agent apply, and the adapter SHALL issue them one at a time.

#### Scenario: Several detail pages

- **WHEN** three items need detail text
- **THEN** the requests are sequential and spaced by the HTML minimum interval

#### Scenario: Robots disallows the detail path

- **WHEN** robots.txt disallows an offer link
- **THEN** the request is treated as blocked and no body is fetched

### Requirement: An invalid detail selector fails the source

An invalid `detail_description_selector` SHALL fail the source with a configuration error instead of being ignored.

#### Scenario: Malformed selector

- **WHEN** `detail_description_selector` cannot be compiled
- **THEN** the source fails with an error naming the selector
