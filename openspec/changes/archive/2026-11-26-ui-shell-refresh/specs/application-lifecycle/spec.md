# Spec Delta

## REMOVED Requirements

### Requirement: Dashboard sections and filters

The dashboard SHALL show `saved` offers in a Saved section and all other
non-`none` offers in an Applications section. It SHALL provide a listing filter
(`all` / `active only`) applying to both sections and an application filter
(`all` / `hide negative`) applying to Applications. The refresh action SHALL
scan only the enabled sources behind the currently visible applications.
Filter choices SHALL persist in the dashboard URL.

#### Scenario: Active-only hides disappeared listings

- **WHEN** the listing filter is `active only`
- **THEN** offers whose listing state is `disappeared` are hidden from both sections

#### Scenario: Hide-negative keeps closed applications

- **WHEN** the application filter is `hide negative`
- **THEN** `negative` offers are hidden while `accepted` and `turned_down` remain visible

#### Scenario: Refresh follows the visible applications

- **WHEN** the user refreshes with a filter hiding negative applications
- **THEN** only sources behind the visible applications are scanned

#### Scenario: Filters survive a reload

- **WHEN** the dashboard URL contains `listing=active` and `application=hide_negative`
- **THEN** the same filters are restored on load

**Reason**: The listing filter duplicated the effect of hide-negative for
applied and interview offers, while offering and saved disappeared offers are
more useful when still visible with their listing state.

**Migration**: No data migration. Existing dashboard URLs with `listing` or
`application` are ignored; the new toggle is `closed=1`.

## ADDED Requirements

### Requirement: Dashboard sections and application toggle

The dashboard SHALL show `saved` offers in a Saved section and all other
non-`none` offers in an Applications section. Both sections SHALL show offers
regardless of listing state. Applications SHALL provide one
`Hide closed applications` toggle, off by default, that hides `negative` offers
while keeping `accepted` and `turned_down` visible. Refresh SHALL scan only the
enabled sources behind the currently visible applications. The toggle SHALL
persist in the dashboard URL as `closed=1`.

#### Scenario: Default shows every application

- **WHEN** the dashboard opens without a filter query
- **THEN** `negative` applications are visible alongside the other states

#### Scenario: Hide closed keeps accepted applications

- **WHEN** the `Hide closed applications` toggle is on
- **THEN** `negative` offers are hidden while `accepted` and `turned_down` remain visible

#### Scenario: Disappeared listings remain visible

- **WHEN** an offer's listing state is `disappeared`
- **THEN** it remains in its dashboard section
- **AND** its listing state is readable from the section table

#### Scenario: Refresh follows the visible applications

- **WHEN** the user refreshes with the hide-closed toggle on
- **THEN** only sources behind the visible applications are scanned

#### Scenario: Toggle survives a reload

- **WHEN** the dashboard URL contains `closed=1`
- **THEN** the hide-closed toggle is restored on load
