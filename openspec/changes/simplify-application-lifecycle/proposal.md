# Proposal

## Why

The application lifecycle carries unused states (`todo`, `responded`, `maybe`,
`positive`), and the default `todo` is really a placeholder for "not applied"
that the independent star bookmark currently approximates. The state does not
react to the two events that mark real progress (`cv_sent`, `rejected`), and an
`applied` or `interview` offer whose listing disappears stays open instead of
being ghosted. This makes the dashboard noisy and forces manual state
bookkeeping.

## What Changes

- Replace the application state vocabulary with `none`, `saved`, `applied`,
  `interview`, `offer`, `accepted`, `turned_down`, `negative`. `todo`,
  `responded`, `maybe` and `positive` are removed. **BREAKING** for API clients
  and saved filters.
- Make `saved` an application state and remove the independent star/bookmark.
  `saved_at` becomes the timestamp of entering the saved state, maintained by
  the server.
- Apply state transitions when events are created:
  - `cv_sent`: `none`, `saved` or `negative` -> `applied`.
  - `rejected`: any state -> `negative`.
  - `ghosted`: `applied` or `interview` -> `negative` (automatic).
- Add an inbound-message "this is a rejection" shortcut that records a
  `rejected` event.
- On a successful scan, when an `applied` or `interview` offer transitions to
  `disappeared`, record a `ghosted` event and set `negative` in the same
  transaction.
- Add dashboard filters: listing `all` / `active only`, application `all` /
  `hide negative`; the refresh action follows the visible applications.
- Migrate existing values: `todo` / `maybe` -> `none` (or `saved` when
  bookmarked), `responded` -> `applied`, `positive` -> `accepted`, and change
  the column default to `none`.

## Capabilities

### New Capabilities

- `application-lifecycle`: application state vocabulary, the saved state,
  manual and event-driven transitions, automatic ghosting, dashboard filters,
  and migration of existing state values.

### Modified Capabilities

None. The `offer-lifecycle` requirements for listing identity and
`active` / `disappeared` transitions are unchanged; this change adds an
application-level consequence to a disappearance.

## Impact

- `internal/model`: application state enum and default.
- `internal/db`: offer filters, merge/split defaults, saved timestamp handling.
- `migrations/`: new migration `0016` rebuilding `job_offers`.
- `internal/api`: offer PATCH validation, `has_application`, removal of the
  `saved` filter, event-create response, scan summary.
- `internal/scan`: disappearance path and ghosted count.
- `web/`: state types, labels and pills, star removal, event rejection
  shortcut, dashboard filters and refresh scope.
- Docs: `DATA-MODEL.md`, `API.md`, `ARCHITECTURE.md`, `USAGE.md`, `SPEC.md`,
  `SMOKE-TEST.md`, `CHANGELOG.md`, `MILESTONES.md`.
- Existing SQLite data is migrated in place; a consistent backup is required
  before migration `0016`.
