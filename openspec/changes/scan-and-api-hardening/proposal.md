# Proposal

## Why

The 2026-11-26 review found that a single parser regression can silently
ghost real applications (empty-but-`ok` scan), and that the unauthenticated
localhost API accepts cross-site writes and can be exposed on `0.0.0.0`
without warning. Migrations overwrite the user's only database copy without
an automatic backup, redirects bypass the politeness rules, and date-only
timeline events can land on the wrong day. These are correctness and safety
issues in code paths the user relies on every day; none requires a new
feature or dependency.

## What Changes

- **Scan disappearance guard.** A successful fetch that returns zero items
  while the source still has active offers is recorded as `error` and marks
  nothing disappeared. A drop of more than half against a source with at
  least four known offers is also treated as a failed scan. (F-01)
- **Ghost event time.** The automatic `ghosted` event and state transition
  use the offer's last-seen time instead of the scan instant. (F-14)
- **Local API request policy.** Every request goes through middleware that
  rejects non-loopback `Host`, non-loopback `Origin`, and, for writes with a
  body, a non-JSON content type except the multipart attachment upload.
  (F-02a)
- **Bind policy.** `serve --addr` refuses a non-loopback address unless the
  new `--unsafe-bind` flag is passed, and always prints a warning when it is.
  (F-19)
- **Pre-migration backup.** When a database with an existing schema has
  pending migrations, a validated copy is written next to it before the
  first migration runs. Fresh databases are not backed up. (F-09)
- **Date-only event times.** A timeline event entered without a time is sent
  by the UI as local midnight in RFC3339, so the stored UTC instant renders
  as the date the user entered. (F-04)
- **Redirect politeness.** The shared client follows redirects manually
  (at most five hops), re-applying `robots.txt` and the per-host rate limit
  to every hop. (F-11a)

Out of scope: SSRF filtering of user-supplied source/import URLs (F-02b),
database location and export (F-08), the WTTJ browser User-Agent decision
(F-10), retries and source failure counters (F-11b), and duplicate-offer
suggestions (F-13). Each is tracked for a later change.

## Capabilities

### New Capabilities

- `local-api-security`: which hosts, origins and content types the HTTP API
  accepts, and when a non-loopback bind is allowed.
- `database-migrations`: the ordered, transactional migration runner and its
  pre-migration backup obligation.
- `source-fetching`: the shared HTTP client's politeness rules across
  redirect hops (robots, per-host spacing, timeouts).

### Modified Capabilities

- `application-lifecycle`: automatic ghosting only follows a trustworthy
  disappearance, uses the last-seen time as the event time, and date-only
  event times keep their entered date.
- `offer-lifecycle`: a scan that is empty or suspiciously smaller than the
  known set must not mark offers disappeared.

## Impact

- `internal/scan/scan.go`: zero-result and drop guards; no schema change.
- `internal/db/queries.go`: `MarkDisappeared` returns last-seen evidence;
  `GhostDisappearedOffer` takes the event time.
- `internal/api/api.go` + `cmd/jobwatch/serve.go`: request-policy middleware,
  `--unsafe-bind`, host allowlist wiring.
- `internal/db/{db,migrate}.go`: pending-migration detection and backup call.
- `internal/sources/client.go`: manual redirect loop with per-hop checks.
- `web/src/format.ts` (+ `OfferPage.tsx` if needed): date-only local midnight.
- Docs: `docs/API.md` (new rejection codes), `docs/ARCHITECTURE.md`,
  `docs/DATA-MODEL.md` (no new columns), `docs/USAGE.md`/`README.md`
  (`serve --unsafe-bind`), `CHANGELOG.md`, `docs/MILESTONES.md`.
- No new dependency and no new external service.
