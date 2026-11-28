# Design

## Context

See `proposal.md` for motivation. The relevant current shapes:

- `Scanner.ScanSource` fetches, calls `apply` in one transaction, then
  `MarkDisappeared` and `GhostDisappearedOffer`.
- `sources.Client` owns robots, rate limiting and the HTTP client; adapters
  use `Get`/`GetJSON`/`PostJSON`.
- `api.Server.Handler` builds a `ServeMux`; `decodeJSON` is the only body
  entry point; the attachment upload is the only multipart handler.
- `db.Open` opens the file and immediately runs `Migrate`; `db.Backup`
  already produces a validated `VACUUM INTO` copy.
- The web UI sends offset RFC3339 for events that have a time, and a bare
  date when it does not.

## Goals / Non-Goals

**Goals:**

- One failed parse can no longer ghost a real application.
- A random web page cannot write to or read the local API.
- No user database is migrated without a restorable copy beside it.
- Redirects cannot escape robots or rate limits.
- Date-only events land on the day the user typed.

**Non-Goals:**

- No SSRF filtering, retries, failure counters or source error text; these
  need their own design (F-02b, F-11b).
- No new columns, so no schema migration and no change to existing data.
- No changes to the WTTJ adapter or its User-Agent decision.

## Decisions

### Scan guard before the transaction

The guard compares `len(raws)` after dropping items without a URL against
the source's known active offer count, read from `job_offer_provenance`
joined to active offers. It runs before `apply`, so a rejected observation
never opens a write transaction:

- fetched `0` and known `> 0` → `error`;
- known `>= 4` and fetched `< known/2` → `error`;
- otherwise apply normally.

Rationale: `MarkDisappeared` runs inside the scan transaction, so the only
way to keep listing states untouched is to not run it. Counting after the
empty-URL filter catches a regression that returns placeholder rows.

Alternative considered: mark disappeared but never ghost. Rejected because
the dashboard and re-appearance logic would still flap; guarding the whole
transition is simpler and matches the "trustworthy observation" rule.
Alternative considered: N consecutive misses before ghosting. Deferred: it
needs per-provenance state (a migration) and the guard removes the dominant
failure mode.

The error is a sentinel (`ErrUntrustworthyScan`) so the CLI prints a useful
reason while the UI keeps the existing `error` status.

### Ghost event time from the offer

`MarkDisappeared` already returns each transitioned offer. Extend
`DisappearedOffer` with `LastSeenAt` from `job_offers.last_seen_at` (the
latest sighting across sources) and pass it as the event time;
`updated_at` still uses the scan instant. Using the offer timestamp keeps
the timeline truthful without adding per-source provenance state.

### Request policy as middleware in front of the mux

`api.Server.Handler` wraps its mux in a policy handler that checks, in
order: `Host`, `Origin` (only for non-GET/HEAD/OPTIONS), then routes. The
handler receives the explicit bind address so `--unsafe-bind` can extend
the allowlist; with the default bind only loopback hostnames and any port
are accepted. This keeps the Vite proxy (`Host: localhost:5173`) working
without proxy configuration changes.

`decodeJSON` gains the `Content-Type: application/json` check. That is
naturally scoped: the multipart handler never calls `decodeJSON`, bodyless
writes never call it, and the check happens before decoding. A route that
does not decode a body (the synchronous source scan) is unaffected.

Alternatives considered: a third-party CSRF library (new dependency, not
needed); checking `Content-Type` only in middleware (would need a path
exception for the upload); a per-handler wrapper (easy to forget on new
routes). The mux-wide wrapper plus the decoder check is the smallest
surface that cannot be bypassed by adding a route.

### Bind validation at command start

`serve` validates `--addr` before opening the database: an address that is
not loopback requires `--unsafe-bind`; with the flag it prints a warning to
stderr. Loopback includes `localhost`, `127.0.0.0/8` and `::1`.

### Pre-migration backup with an opt-in open path

`db.Open` keeps its current behavior for tests and embedded callers. Add
`db.OpenWithOptions(ctx, path, db.Options{BackupBeforeMigrate: true})`,
used by the CLI. Migrate exposes `pendingMigrations`; the option path
detects whether `schema_migrations` already exists and has rows, and if
there are pending versions writes `<path>.<UTC timestamp>.pre-<version>.bkp`
with `db.Backup`. The timestamp avoids the `VACUUM INTO` "destination
exists" failure when a failed migration is retried. A fresh file has no
schema history and is skipped.

Rationale: keeping the backup in the CLI layer means test databases stay
fast, and `:memory:` or DSN paths are never backed up.

### Redirects handled in `fetch`

Set `CheckRedirect` to `http.ErrUseLastResponse` so every hop is visible,
then loop in `fetch`:

- follow only `GET`; a redirect on `POST` becomes an `HTTPError`;
- resolve relative `Location`, allow only `http`/`https`;
- run `robotsFor` and `wait` for the hop's host before requesting it;
- stop after five hops, and treat a repeated URL as a loop.

Go's default redirect policy cannot be used here because `CheckRedirect`
runs inside `Do`, where re-entering the robots fetch would recurse into the
same client. Handling redirects in the caller keeps the politeness checks
in one place.

### Date-only events

`eventTimestamp` sends `new Date(`${date}T00:00`).toISOString()` when the
time field is empty, which is local midnight with the current offset. The
server continues to accept bare dates as UTC midnight for compatibility.
A `format.test.ts` asserts the local calendar date round-trips, so the test
is timezone-independent.

## Risks / Trade-offs

- [A board genuinely closes every role] → the scan is reported as `error`
  and the offers stay visible; the user can re-scan later. Losing a real
  application to a parser glitch is the worse failure.
- [The source sits just above the guard threshold] → a drop from 4 to 1 is
  rejected. Mitigation: the threshold only applies at four or more, and the
  error is visible; N-consecutive-misses is the follow-up.
- [A non-browser client breaks on the new Origin/Content-Type rule] →
  those clients are local; the error codes are documented, and `Host`
  remains loopback-only by default.
- [`--unsafe-bind` weakens the rebinding defense] → it is opt-in, warns
  loudly, and still enforces Origin and Content-Type.
- [Pre-migration backup fails on a read-only directory] → the migration
  aborts before writing. That is intentional; the user can pass a writable
  `--db` path.
- [POST redirects now fail where Go silently followed them] → no adapter
  relies on that, and retrying a POST is not something a polite client
  should do implicitly.

## Migration Plan

There is no schema migration. Ship the binary; the next start of `serve`,
`scan`, `add-source` or any command that opens the database will create a
pre-migration backup only if a migration is pending. Rollback is reverting
the binary; the database is untouched by this change.
