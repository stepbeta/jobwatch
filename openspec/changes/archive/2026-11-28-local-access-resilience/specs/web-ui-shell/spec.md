# Spec Delta

## ADDED Requirements

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
