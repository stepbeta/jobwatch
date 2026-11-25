# Design

## Context

See `proposal.md` for motivation. Current constraints that shape the approach:

- `job_offers.application_state` is free-text validated in Go, default `todo`,
  with eight values. The database has 3303 `todo`, 7 `applied`, 4 `negative`
  offers and no `responded`, `maybe` or `positive` rows.
- `events` are independent records; creating one never touches the offer.
- `saved_at` is an independent bookmark toggled by a star in the UI. Six
  offers are saved, all in `todo`.
- `MarkDisappeared` already marks `active -> disappeared` inside the scan
  transaction; it returns the offer IDs that transitioned.
- The migration runner applies each file on a pinned connection with foreign
  keys disabled and runs `PRAGMA foreign_key_check` before commit. Migration
  `0012` is the precedent for rebuilding `job_offers` with its FTS triggers.

## Goals / Non-Goals

**Goals:**

- One stage vocabulary, with `saved` replacing the independent bookmark.
- `cv_sent`, `rejected` and automatic `ghosted` are the only state-changing
  events; manual edits remain free-form.
- Automatic ghosting on a real scan disappearance, atomically.
- Dashboard filters and a refresh scope that follow what is visible.
- A safe in-place migration of the existing SQLite file.

**Non-Goals:**

- No anomaly detection or "N consecutive empty scans" guard for source
  regressions.
- No enforced transition graph for manual state changes.
- No retroactive ghosting of offers already disappeared.
- No change to the event vocabulary or a new rejection event type.
- No LLM or content-based detection of rejection emails.

## Decisions

### State vocabulary and `none` default

The enum becomes `none`, `saved`, `applied`, `interview`, `offer`, `accepted`,
`turned_down`, `negative`. `none` is the default. `todo`, `responded`, `maybe`
and `positive` are removed and become validation errors.

`todo` is not kept as an internal alias with a "None" label: that would leak a
dead concept into the API, filters and tests. Changing the column default means
rebuilding `job_offers`, which the migration runner already supports and which
`0012` has done before.

Manual `PATCH` remains free-form: any valid state is accepted. No transition
graph is enforced, so the user can always correct an application.

### `saved` replaces the star

Saving is `application_state = saved`, not an independent flag. `saved_at`
becomes metadata for that state with the invariant
`saved_at IS NOT NULL` exactly when the state is `saved`:

- Entering `saved` sets `saved_at` to the server's current UTC time.
- Leaving `saved` for any other valid state clears `saved_at`.
- The Saved dashboard section sorts by `saved_at` as before.

The API loses the `saved` boolean on `PATCH /api/offers/:id` and the
`saved=true|false` list filter; clients use `application_state=saved`. The
`sort=saved` value stays and continues to mean `saved_at DESC`. This is a
breaking API change, acceptable for a local single-user application.

Alternatives considered: keeping the star orthogonal (rejected because the
user wants one clean model and every current saved offer is pre-application);
dropping `saved_at` and sorting by `updated_at` (rejected because it loses the
actual save time and the existing Saved ordering).

### Migration `0016`

A rebuild of `job_offers` changes `DEFAULT 'todo'` to `DEFAULT 'none'` and maps
the existing values in the copy:

| Existing value | Migrated value |
|---|---|
| `todo`, `maybe` | `none` |
| `responded` | `applied` |
| `positive` | `accepted` |
| `applied`, `interview`, `offer`, `negative` | unchanged |

A row with `saved_at` set and an old state of `todo` or `maybe` becomes
`saved` and keeps `saved_at`. A row with `saved_at` set and any other state
keeps that state and clears `saved_at`. Current data has only the first case,
but the rule prevents a stage from being lost to a stale bookmark.

The migration drops and recreates the FTS triggers and runs
`INSERT INTO job_offers_fts(job_offers_fts) VALUES('rebuild')`, following
`0012`. It preserves every offer ID and every child row. The mapping is not
reversible in SQL; rollback is the pre-migration backup.

### Event transitions

A pure helper in `internal/model` computes the next state from the current
state and the event type:

- `cv_sent`: `none`, `saved` or `negative` -> `applied`.
- `rejected`: any state -> `negative` (no-op when already negative).
- `ghosted`: `applied` or `interview` -> `negative`.
- Every other event: unchanged.

The helper is table-tested. Event creation applies it inside one transaction
with the event insert, and the response reports the resulting
`application_state` and whether it changed, so the UI can explain the mutation.
Event `PATCH` and `DELETE` never recompute the state. Manual `PATCH` never
creates an event.

A backdated `cv_sent` uses the current state, not `occurred_at`: adding one to a
negative offer reopens it. A later manual correction fixes a mistaken backfill.
Guarding by comparing dates to the latest rejection was considered and deferred
as unnecessary complexity for the current workflow.

### Rejection shortcut

The shortcut is UI-only. When the form's type is `message_received`, a "this is
a rejection" checkbox makes the submit post a single `rejected` event with the
same description and occurrence time. The API already accepts `rejected`, so no
new field or event type is needed, and the timeline does not get a duplicate
`message_received` entry.

### Automatic ghosting

`MarkDisappeared` knows exactly which offers transitioned. It is extended to
return each transitioned offer's current application state (and identity), still
inside the scan transaction. For `applied` or `interview`, the scan inserts one
`ghosted` event and sets `negative` before commit. The event uses the scan's
detection time and a description such as "Posting disappeared; marked ghosted
automatically."

The existing `offer_state != 'disappeared'` predicate makes this idempotent.
Other states are left untouched. No migration backfills ghosted events for
offers that disappeared before this change.

`scan.Result` gains a `Ghosted` count, exposed in the scan summary API and the
CLI output next to `Disappeared`.

The known risk remains: a parser regression that returns an empty-but-`ok` scan
already marks offers disappeared; now it also ghosts applied/interview offers.
The user can restore the state, but the event remains. Adding a
consecutive-empty-scan guard is explicitly out of scope.

### Dashboard filters

- Applications = every non-`none`, non-`saved` offer (`has_application`).
- Saved = `application_state=saved` only.
- Listing filter `all` / `active only` maps to no filter or
  `offer_state=active`, and applies to both sections.
- Application filter `all` / `hide negative` maps to no filter or the explicit
  allowed set `applied,interview,offer,accepted,turned_down`. The API already
  supports a state list, so no "not in" parameter is added.
- Refresh pages through the same filtered query as the visible Applications
  section, so hidden applications are not scanned.
- Filter state lives in the dashboard hash query (`listing=active`,
  `application=hide_negative`) using the existing `currentRouteQuery` /
  `replaceRouteQuery` helpers.

### API surface

- `PATCH /api/offers/:id`: new enum values; `saved` field removed.
- `GET /api/offers`: new enum values; `has_application` excludes `none` and
  `saved`; `saved` filter removed; `sort=saved` remains.
- `POST /api/offers/:id/events`: applies the transition and returns
  `application_state` and `state_changed` alongside the event.
- Scan summary: additive `ghosted` count.

## Risks / Trade-offs

- **`job_offers` rebuild** -> run the first task's backup, verify row/ID counts,
  `PRAGMA foreign_key_check`, FTS search and child row counts on a copy before
  applying to the live file.
- **False-negative scan ghosts real applications** -> only successful scans
  trigger it; the user can restore the state; the ghosted event records what
  happened. A consecutive-miss guard is future work.
- **Backdated `cv_sent` reopens a closed application** -> current-state rule is
  simple and predictable; manual correction is available.
- **`saved_at` for a bookmarked offer in a later stage is cleared** -> the
  migration keeps the stage and sacrifices the stale bookmark; no current row
  is in that situation.
- **Breaking API values and removal of `saved`** -> local single-user
  application; the UI and docs ship in the same change.
- **Hidden applications are not refreshed** -> intended: the refresh scope is
  the visible set.

## Migration Plan

1. Stop the running `jobwatch` process and create a consistent backup with
   `./jobwatch backup jobwatch.db.<timestamp>.bkp`.
2. Apply migration `0016` to a copy of the backup and verify counts, foreign
   keys, FTS and the mapped states.
3. Apply to the live database, then run the backend/frontend test suites and a
   short smoke test (new offer `none`, save, `cv_sent`, `rejected`, scan
   disappearance).
4. Rollback is restoring the pre-migration backup; the state mapping itself is
   not reversed in place.
