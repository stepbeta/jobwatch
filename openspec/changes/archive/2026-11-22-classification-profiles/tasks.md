# Tasks

Apply one numbered group at a time. Stop for review after each group before starting the next. The two process prerequisites in group 1 are mandatory for every change.

## 1. Process prerequisites

- [x] 1.1 Always start the change on a new branch cut from an up-to-date `main`: run `git fetch origin`, switch to `main`, update it fast-forward, and create a new branch for this change. Verify `git status --short --branch` shows the new branch with a clean tree and `git merge-base --is-ancestor origin/main HEAD` succeeds. Do not start on a dirty or stale `main`.
- [x] 1.2 Always make a DB backup once it is confirmed the `jobwatch` binary is not running. Ask the user to confirm no `jobwatch` process is running, then run `jobwatch backup jobwatch.db.<timestamp>.bkp`. Verify the backup file exists and the command reported a successful integrity check. Do not copy the database while `serve` holds it. Do not start group 2 until this backup is confirmed.

## 2. Schema and profile storage

- [x] 2.1 Add the additive migration `0015_search_profiles.sql` with `search_profiles`, `search_profile_terms` and `search_profile_remote`, and seed the default profile from the current hardcoded title, location and description tables. Seed the default title/location/description term weights and buckets, the five-bucket scale, and the remote selection accepting `remote` and `unknown` while rejecting `hybrid` and `onsite`. Keep the legacy `title_confidence`, `location_confidence` and `match_reasons_json` columns in place. Verify the migration test applies to a fresh database and to a copy of the current database, the seed term counts and weights match the expected tables, and the legacy columns still exist.
- [x] 2.2 Add the search-profile model types and the database read/update layer: default profile lookup, list, get, update with validation, and transactional term/remote replacement. Verify the profile CRUD and validation tests pass, including multiple profiles in storage, empty terms, invalid weights and the default profile.
- [x] 2.3 Update `docs/DATA-MODEL.md` and `docs/ARCHITECTURE.md` with the three profile tables, their columns and relationships, the seed, the default profile, and the deprecated legacy score columns. Verify the docs name every new table and column and state that the legacy columns are unused.

## 3. Classifier

- [x] 3.1 Refactor `internal/classify` to remove the title, location and description term tables; keep the fixed remote phrase detector; add pure scoring functions for title affinity, location affinity, remote policy derivation, remote acceptance and reasons, taking a profile value. Remove the policy points and caps from the location score. Verify `go test ./internal/classify` passes with the new API.
- [x] 3.2 Rewrite the classifier tests to cover title, location and description term scoring, neutral terms, clamping, remote policy derivation for all four policies, remote acceptance, reasons, and the seeded profile examples. Verify `go test ./internal/classify` passes and no test relies on stored scores or policy caps.
- [x] 3.3 Update `docs/ARCHITECTURE.md` with the facts/preferences/scores model, the read-time scoring flow, the fixed remote detector and the removal of the term tables. Verify the document describes the title, location and remote signals and does not present the old confidence columns as active.

## 4. Scan and ingestion

- [x] 4.1 Update the scan, manual import, repair, split and merge paths to store offer facts only and derive `remote_policy`; remove all title and location score writes. Remove `internal/scan/reclassify.go`, `cmd/jobwatch/reclassify.go`, the command registration and their tests. Verify `go test ./internal/scan ./internal/db ./cmd/jobwatch` passes and `go run ./cmd/jobwatch --help` no longer lists `reclassify`.
- [x] 4.2 Update the scan and ingestion tests for facts-only writes, remote policy derivation on new sightings and on location/description edits, repair, split, merge and incomplete offers. Verify the tests pass and assert that no title or location score is written.
- [x] 4.3 Update `docs/ARCHITECTURE.md` and `docs/MILESTONES.md` with the facts-only ingestion path and mark the known gap about stale scores after manual edits as resolved. Verify the documents describe remote policy derivation and read-time affinity.

## 5. API

- [x] 5.1 Add the search-profile API endpoints: list profiles, read one profile and replace one profile. Validate names, terms, weights and remote policies; keep the profile unchanged on validation failure. Verify the API tests pass, including default profile selection, multiple profiles, invalid input and transactional updates.
- [x] 5.2 Update the offers list and detail endpoints to compute title and location affinity at read time from the selected or default profile, return `title_affinity`, `location_affinity`, `remote_policy`, `remote_policy_accepted` and title/location reasons, and remove the legacy score fields. Add `profile_id`, `min_title_affinity`, `min_location_affinity`, `remote_accepted`, `title_affinity` and `location_affinity` filters/sorts. Verify the API tests pass for affinity values, reasons, filters, sorts, pagination, total counts, profile selection, remote acceptance and the absence of legacy fields.
- [x] 5.3 Update `docs/API.md` and `docs/ARCHITECTURE.md` with the profile endpoints, the changed offers fields and filters/sorts, the default-profile behavior and the breaking changes. Verify the documented examples match the tested responses and no endpoint still documents the legacy score fields.

## 6. SPA

- [x] 6.1 Add the Search profile page with profile name, title terms, location terms, advanced description terms, 3/5 bucket selection, optional numeric weights, remote policy checkboxes and concise explanatory text for each section. Verify the page tests pass for editing, validation, bucket switching, advanced weights, remote selection and the explanatory copy.
- [x] 6.2 Update the SPA types, score badges, offer table, offers list and offer detail to show title affinity, location affinity and the remote policy badge/acceptance instead of the old scores. Add affinity filters and sorts and show the term reasons. Verify the SPA tests pass for the updated list, detail, filters, sorts and empty/legacy-field cases.
- [x] 6.3 Update `docs/ARCHITECTURE.md` with the profile page and the three-signal presentation. Verify the document describes the SPA as a display layer and does not claim the SPA computes affinity.

## 7. CLI and project docs

- [x] 7.1 Update the `list` command to use the default profile and print title affinity, location affinity, remote policy and remote acceptance instead of the old scores. Verify the CLI tests pass and the output contains no legacy score columns.
- [x] 7.2 Remove `reclassify` from the command reference and all documentation. Verify `go test ./...` passes and `grep -R "reclassify"` returns no active command or documentation references.
- [x] 7.3 Update `README.md`, `CHANGELOG.md` and `docs/MILESTONES.md` with the profile page, the changed commands and API, the removed `reclassify` command, the migration and the resolved known gap. Verify the README commands run as written and the changelog has an Unreleased entry with Added, Changed, Fixed and Removed bullets.

## 8. Integration verification

- [x] 8.1 Run `go build ./...`, `go vet ./...` and `go test ./...`. Verify all three commands pass.
- [x] 8.2 Run the SPA test suite and production build. Verify both pass.
- [x] 8.3 Run the end-to-end checks: apply the migration to a copy of the database, update a profile and confirm the offers list affinity changes, edit an offer's location/description and confirm the remote policy is re-derived, and confirm offer responses omit the legacy score fields. Verify each check against the running API and the test suite.
- [x] 8.4 Run `openspec validate "classification-profiles" --strict` and review the updated docs. Verify validation passes with no warnings and the docs are consistent with the implemented behavior.

## Workflow follow-up

- Archive the change after the project review requirements are satisfied. Run `openspec archive "classification-profiles"` and follow the CLI prompts. Verify the archived specs at `openspec/specs/classification/spec.md` and `openspec/specs/search-profiles/spec.md`.
- Replace the `docs/SPEC.md` classification section with a one-line pointer to `openspec/specs/classification/spec.md` and add the revision to the Spec revisions section. Verify the pointer resolves and no duplicate behavior remains.
- Commit the completed change with a conventional commit message. Do not push.
- Do not archive until every tracked task is ticked and the integration checks pass.
