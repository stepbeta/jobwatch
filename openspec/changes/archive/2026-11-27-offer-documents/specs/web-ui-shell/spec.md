# Spec Delta

## MODIFIED Requirements

### Requirement: Persistent navigation shell

The application SHALL present a full-width header and a left sidebar. The
header SHALL contain the brand mark and right-aligned controls. The sidebar
SHALL link to Dashboard, Offers, Incomplete (with a pending count), Sources,
Documents, Interview prep and Settings. It SHALL mark the section of the current
route, and the Documents route SHALL mark Documents. The sidebar order SHALL
place Documents next to the other record-browsing sections, not among settings.

#### Scenario: Offer detail marks Offers

- **WHEN** the current route is an offer detail page
- **THEN** the sidebar marks Offers as the active section

#### Scenario: Company detail marks Offers

- **WHEN** the current route is a company detail page
- **THEN** the sidebar marks Offers as the active section

#### Scenario: Documents page marks Documents

- **WHEN** the current route is the Documents page
- **THEN** the sidebar marks Documents as the active section

#### Scenario: Incomplete badge shows the pending count

- **WHEN** one or more incomplete offers exist
- **THEN** the Incomplete sidebar link shows their count
- **AND** the count disappears when none remain

#### Scenario: Header stays available

- **WHEN** any page is open
- **THEN** the header shows the brand and its right-aligned controls

#### Scenario: Narrow viewport collapses the sidebar

- **WHEN** the viewport is at most 900px wide
- **THEN** the sidebar collapses and the header controls remain available
