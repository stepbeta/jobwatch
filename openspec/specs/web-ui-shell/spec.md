# web-ui-shell Specification

## Purpose

Defines the persistent application frame, the identity shown in browser chrome
for each page, and the user's theme selection.

## Requirements

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

### Requirement: Browser title identifies the current page

The document title SHALL identify the current page as
`<page label> — JobWatch`. Offer detail pages SHALL use
`<offer title> — <company> · JobWatch`; company detail pages SHALL use
`<company> — JobWatch`. The application state MUST NOT appear in the title.

#### Scenario: List page title

- **WHEN** the user opens the Offers page
- **THEN** the document title is `Offers — JobWatch`

#### Scenario: Offer detail title

- **WHEN** an offer for Acme titled "Site Reliability Engineer" is loaded
- **THEN** the document title is
  `Site Reliability Engineer — Acme · JobWatch`

#### Scenario: Offer title while loading

- **WHEN** an offer detail page is opened but its data is not loaded yet
- **THEN** the document title is `Offer — JobWatch`
- **AND** it changes once the offer and company are known

#### Scenario: Leaving a detail page restores the page title

- **WHEN** the user navigates from an offer detail page to the Dashboard
- **THEN** the document title is `Dashboard — JobWatch`

#### Scenario: State stays out of the title

- **WHEN** an offer's application state changes
- **THEN** the document title does not gain the state label

### Requirement: Offer favicon reflects application state

The browser favicon SHALL show the brand mark alone for `none` and a coloured
corner badge for every other application state: saved, applied, interview,
offer, accepted, turned_down and negative. The badge SHALL use a distinct
colour and a simple glyph for each state. Navigating away from the offer SHALL
restore the default favicon.

#### Scenario: Applied offer

- **WHEN** an offer in the `applied` state is open
- **THEN** the favicon shows the brand mark with the applied badge

#### Scenario: Closed application

- **WHEN** an offer in the `negative` state is open
- **THEN** the favicon shows the brand mark with the negative badge

#### Scenario: State change updates the favicon

- **WHEN** the user changes the open offer's application state
- **THEN** the favicon updates without a reload

#### Scenario: Leaving the offer restores the default

- **WHEN** the user navigates away from an offer detail page
- **THEN** the favicon shows the default brand mark

### Requirement: Theme selection

The application SHALL offer `system`, `light` and `dark` themes. `system` SHALL
be the default and follow the operating-system preference. The selection SHALL
persist across reloads and take effect before the first paint.

#### Scenario: System follows the operating system

- **WHEN** the theme is `system` and the operating system switches to dark
- **THEN** the application uses the dark palette

#### Scenario: Explicit theme persists

- **WHEN** the user selects the dark theme and reloads the application
- **THEN** the application starts in the dark theme

#### Scenario: Returning to system

- **WHEN** the user selects `system` again
- **THEN** the application follows the current operating-system preference

### Requirement: Brand mark

The header SHALL show a brand mark next to the product name "JobWatch". The
same mark SHALL be the default browser favicon. It MUST remain legible at
16 pixels and MUST work against both the light and dark palettes.

#### Scenario: Default favicon

- **WHEN** no offer state applies
- **THEN** the browser shows the brand mark as the favicon

### Requirement: Hash navigation is authoritative

The application SHALL render the route named by the address-bar hash whenever
that hash changes, including a change that lands while the application is still
starting and a page restored from the back/forward cache. Applying a route
change MUST NOT require a reload or a second click.

#### Scenario: Address-bar hash names the page

- **WHEN** the user opens `#/sources` directly or edits the hash
- **THEN** the Sources page is shown

#### Scenario: Hash change during startup is applied

- **WHEN** the sidebar link is clicked while the application is still starting
- **THEN** the view shows the route named by the hash once startup completes

#### Scenario: Restored page shows the current hash

- **WHEN** the browser restores the page from the back/forward cache
- **THEN** the view matches the address-bar hash

#### Scenario: In-page query rewrite keeps the page

- **WHEN** a filter, sort or page-size control rewrites only the hash query
- **THEN** the same page and its current state stay rendered

### Requirement: API refusal is reported at the top level

The shell SHALL check API reachability when the application starts and when the
page becomes visible again, and SHALL show one banner when the check fails. The
banner SHALL name the cause: a refused host SHALL show the address to open and
the `--allow-host` flag, an unreachable server SHALL say it cannot be reached.
The banner MUST disappear after a check succeeds. Page-level errors SHALL remain
unchanged.

#### Scenario: Refused host explains the address to open

- **WHEN** the API answers `403 forbidden_host`
- **THEN** one banner names the server address to open and the `--allow-host` flag

#### Scenario: Unreachable server is reported

- **WHEN** the health check fails to connect
- **THEN** one banner says the server cannot be reached

#### Scenario: Banner clears after recovery

- **WHEN** a later check succeeds
- **THEN** the banner is removed without a reload

#### Scenario: No banner when the API answers

- **WHEN** the health check succeeds
- **THEN** no connection banner is shown
