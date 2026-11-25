# Tasks

## 1. Preparation

- [x] 1.1 Confirm `main` is up to date and cut `feat/simplify-application-lifecycle` from it; verify `git status` is clean and the branch is not `main`. (Done: `origin/main` was 0 behind / 0 ahead and the branch was created.)
- [x] 1.2 Stop the running `jobwatch` process that holds `jobwatch.db` (currently PID 1223; ask the user to stop it), then create a consistent backup with `./jobwatch backup jobwatch.db.<timestamp>.bkp`; verify the file exists and the backup reports a successful integrity check. Do not edit the database or run migration 0016 before this passes. (Done: no process held the DB; backup `jobwatch.db.20261125T105502Z.bkp` written, integrity check ok.)

## 2. State vocabulary, saved state, and migration

- [x] 2.1 Write migration `0017_application_lifecycle.sql` (renumbered from 0016 because `0016_murena_detail_description.sql` already exists): rebuild `job_offers` with `DEFAULT 'none'`, map `todo`/`maybe` -> `none`, `responded` -> `applied`, `positive` -> `accepted`, turn bookmarked `todo`/`maybe` rows into `saved`, clear `saved_at` on other bookmarked rows, recreate indexes and FTS triggers, and rebuild the FTS index. Verify on a copy of the backup: row and ID counts, child row counts, `PRAGMA foreign_key_check`, FTS search, and the mapped state distribution. (Done on a copy: 3314 offers, IDs and all child rows preserved, `none` 3297 + `saved` 6, `applied` 7, `negative` 4, FTS search ok, default `none`, `saved_at` invariant holds.)
- [x] 2.2 Update the application state constants and `Valid` in `internal/model`, set the default to `none`, and add table-driven tests for all new values and rejection of `todo`, `responded`, `maybe` and `positive`. (Done: `internal/model/application_state_test.go`.)
- [x] 2.3 Update `internal/db`: make `has_application` exclude `none` and `saved`, use `none` for merge/split defaults, make `application_state=saved` set `saved_at` and any other state clear it, and remove the independent `Saved` update path. Verify with db tests covering the timestamp invariant and merge/split behavior. (Done: `internal/db/application_state_test.go`.)
- [x] 2.4 Update the API offer filters, PATCH validation and DTOs; remove the `saved` PATCH field and `saved=true|false` list filter while keeping `sort=saved`; update API tests for the new enum, the removed filter, and `has_application` excluding `none`/`saved`. (Done: `TestOfferSavedState`, e2e default `none`.)
- [x] 2.5 Update the frontend application state types, labels, orders and pill CSS; remove the star/bookmark controls from the offer table, offer detail and dashboard; use `application_state=saved` and the existing `sort=saved` for the Saved section. Verify with `npm test` and `npm run build`. (Done: 63 Vitest tests pass, production build ok.)
- [x] 2.6 Update `docs/DATA-MODEL.md`, `docs/API.md`, `docs/SPEC.md`, `docs/USAGE.md`, `docs/SMOKE-TEST.md` and `CHANGELOG.md` for the new vocabulary, the `saved` state, the removed star and migration 0017. (Also updated `docs/ARCHITECTURE.md` and `README.md`.)

## 3. Event-driven transitions and rejection shortcut

- [x] 3.1 Implement the pure application-state transition helper for `cv_sent`, `rejected` and `ghosted`, with a table-driven test matrix covering every from/to pair and every no-op case. (Done: `model.NextApplicationState` + transition matrix test.)
- [x] 3.2 Make event creation apply the transition atomically with the event insert and return `application_state` and `state_changed`; add API tests for `cv_sent` from `none`/`saved`/`negative`, `rejected` from every state, no downgrade, and no state recomputation on event PATCH or DELETE. (Done: `db.InsertEventWithTransition` + `TestEventApplicationTransitions`.)
- [x] 3.3 Show a state-change notice after adding an event and add the "this is a rejection" shortcut on the `message_received` form; verify with Vitest tests for the submitted `rejected` payload and the notice. (Done: 65 Vitest tests pass, build ok.)
- [x] 3.4 Update `docs/API.md`, `docs/USAGE.md` and `CHANGELOG.md` for event-driven transitions, the event response fields and the rejection shortcut. (Also updated `docs/DATA-MODEL.md`.)

## 4. Automatic ghosting on disappearance

- [x] 4.1 Extend the disappearance path to return the transitioned offers and their current application states, then insert one `ghosted` event and set `negative` inside the scan transaction for `applied` and `interview` only. Add scan tests for `applied`, `interview`, `offer`, `none`, `saved`, an already-disappeared offer, a repeat scan, and an offer still listed by another source. (Done: `db.DisappearedOffer`, `db.GhostDisappearedOffer`, three scan tests.)
- [x] 4.2 Add a `Ghosted` count to `scan.Result`, the scan summary API and the CLI output; verify with scan/API tests that the count matches the number of created ghosted events and does not double-count plain disappearances. (Done: `ghosted` JSON field, GHOSTED CLI column, dashboard summary, `TestSourceScanGhostedCount`.)
- [x] 4.3 Verify and document that existing disappeared offers are not backfilled and that a ghosted event is not recreated on later scans; update `docs/ARCHITECTURE.md`, `docs/DATA-MODEL.md`, `docs/API.md` and `CHANGELOG.md`. (Repeat-scan test covers idempotency; migration 0017 inserts no events. Also updated `docs/USAGE.md`.)

## 5. Dashboard filters and refresh scope

- [x] 5.1 Add the listing `all`/`active only` and application `all`/`hide negative` filters to the Applications section and the listing filter to the Saved section; persist them in the dashboard hash query and verify with Vitest tests for filtering, URL round-trip and reload. (Done: URL-restored filters, 69 Vitest tests pass.)
- [x] 5.2 Make the refresh action page through and scan only the sources behind the currently visible applications; verify through tests that a hidden negative application's source is not scanned and that the empty state and notice match the filter. (Done: `fetchAllApplications(filters)` + filter-aware empty state/notice.)
- [x] 5.3 Update `docs/USAGE.md`, `docs/SPEC.md`, dashboard copy and `CHANGELOG.md` for the filters and refresh scope. (Also updated `docs/ARCHITECTURE.md`.)

## 6. Integration verification

- [ ] 6.1 Run `go build ./...`, `go vet ./...`, `go test ./...`, `npm test` and `npm run build`; verify all pass.
- [ ] 6.2 Apply migration 0017 to a copy of the backup and smoke-test: new offer is `none`, save sets `saved_at`, `cv_sent` opens, `rejected` closes, a scan ghosts an applied offer, and the dashboard filters and refresh scope behave; record the results.
- [ ] 6.3 Run `openspec validate --change simplify-application-lifecycle`, verify every task is ticked, and confirm the docs listed in `AGENTS.md` are synchronized with the implemented behavior.

## Workflow follow-up

- Archive the change after review; sync `application-lifecycle` to `openspec/specs/` and replace the matching `docs/SPEC.md` section with a pointer.
- Update `docs/MILESTONES.md` current state and known gaps if the implementation exposes any deviation.
