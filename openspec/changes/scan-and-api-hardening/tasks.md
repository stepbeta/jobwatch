# Tasks

## 1. Scan trust guard and ghost event time

- [x] 1.1 Add a database query that counts a source's active offers and verify it with a table test covering a source with no offers, one with active offers, and one whose offers are all disappeared
- [x] 1.2 Reject untrustworthy scan observations in `ScanSource` before the write transaction: zero items with known offers, and fewer than half of four or more known offers; verify with tests that the source is recorded as `error`, every listing and application state is unchanged, and no `ghosted` event exists
- [x] 1.3 Prove the guard does not over-trigger: a source with two known offers returning one still follows the normal disappearance rules, and a first scan of an empty source is applied normally
- [x] 1.4 Return `last_seen_at` with each disappeared offer and use it as the `ghosted` event time; verify the event occurrence equals the last-seen time and `updated_at` still reflects the scan
- [x] 1.5 Update `docs/ARCHITECTURE.md` (scan pipeline) and `CHANGELOG.md` for the guard and ghost timing, and verify the documented flow matches the code

## 2. Local API request policy and bind safety

- [x] 2.1 Add the request-policy middleware in front of the API mux: reject non-loopback `Host` with `403 forbidden_host` and foreign non-empty `Origin` on state-changing methods with `403 forbidden_origin`; verify with tests for a rebound host, a public origin, a missing origin, and `localhost:5173` as the development proxy host
- [x] 2.2 Require `Content-Type: application/json` for JSON bodies in `decodeJSON` and return `415 unsupported_media_type` before decoding; verify a `text/plain` write is rejected and the multipart attachment upload and bodyless `POST /api/sources/{id}/scan` still work
- [x] 2.3 Validate `serve --addr`: refuse a non-loopback address without `--unsafe-bind`, accept it with the flag, and print a warning to stderr; verify with unit tests over `localhost`, `127.0.0.1`, `::1`, `0.0.0.0`, and a LAN address
- [x] 2.4 Document the new rejection codes in `docs/API.md`, the `--unsafe-bind` flag in `README.md` and `docs/USAGE.md`, the policy in `docs/ARCHITECTURE.md`, and the behavior in `CHANGELOG.md`; verify each documented example matches the implemented response

## 3. Pre-migration backup

- [ ] 3.1 Expose pending-migration detection and add `db.OpenWithOptions` with a `BackupBeforeMigrate` option; verify a database with recorded schema history and a pending migration gets a validated `<db>.<timestamp>.pre-<version>.bkp` before the migration runs
- [ ] 3.2 Verify a fresh database is not backed up, a failed migration is retried under a second backup name without overwriting the first, and a backup failure aborts without applying the migration
- [ ] 3.3 Wire the CLI's `opts.open` to `OpenWithOptions`; verify a CLI command on an old-schema test database leaves the backup file next to it
- [ ] 3.4 Update `docs/ARCHITECTURE.md`, `docs/USAGE.md` and `CHANGELOG.md` with the automatic backup and its skip rules; verify the documented filenames match the code

## 4. Date-only event times

- [ ] 4.1 Make `eventTimestamp` send local midnight for a date with no time, and verify with a timezone-independent `web/src/format.test.ts` that the local calendar date round-trips
- [ ] 4.2 Keep the existing date-time path covered and verify an event with a time still stores the entered local instant
- [ ] 4.3 Update the code comment and `CHANGELOG.md`, and verify the documented date-only behavior matches the test

## 5. Redirect politeness

- [ ] 5.1 Disable automatic redirects on the shared client and follow `GET` redirects in `fetch` with at most five hops, rejecting non-HTTP schemes and repeated URLs; verify each failure with a test
- [ ] 5.2 Re-apply robots and the per-host interval to every hop; verify a redirect to a disallowed path is blocked without contacting it and a redirect to a second host waits for that host's interval
- [ ] 5.3 Verify a `POST` that receives a redirect fails with an HTTP error instead of being replayed
- [ ] 5.4 Update `docs/ARCHITECTURE.md` and `CHANGELOG.md` for the redirect rule; verify the documented hop limit matches the code

## 6. Integration and review bookkeeping

- [ ] 6.1 Run `go build ./...`, `go vet ./...`, `go test -count=1 ./...`, and `cd web && npm test`; verify all pass
- [ ] 6.2 In the untracked `REVIEW.md`, mark F-01, F-04, F-09, F-14 and F-19 as fixed, and F-02/F-11 as partially fixed naming the deferred SSRF/retry parts; verify no deferred item is marked fixed
- [ ] 6.3 Add the change's done criteria and any deviations to `docs/MILESTONES.md`; verify the known-gaps table no longer claims the fixed items are open

## Workflow follow-up

- Create `fix/scan-and-api-hardening` off `main` before the first edit; the tree is currently on `main`.
- Apply one numbered group at a time and stop for review before the next group, per `AGENTS.md`.
- Commit each completed group with a conventional-commit message; never stage `REVIEW.md`.
- After review, archive the change and replace the corresponding `docs/SPEC.md` sections with pointers to the new capability specs.
