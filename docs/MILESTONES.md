# Milestones

Later milestones are defined by the user's milestone prompts and are
added here as they start.

## Milestone 1 — Core: bootstrap, database, sources, scan, CLI — DONE

Done criteria (all met):
- [x] Go module, directory layout, `.gitignore`, `web/` placeholder
- [x] SQLite (modernc) in WAL mode; migration runner with `schema_migrations`
- [x] Migration 0001: all SPEC §4 tables, indexes, FTS5 over
      `job_offers(title, raw_text)` and `notes(body)` with sync triggers
- [x] `internal/model` structs + `normalize.go` (company, title,
      location bucket, fingerprint, text hash) with unit tests
- [x] `Fetcher` interface, polite client (robots.txt, per-host/source
      rate limit, UA, timeout, blocked/error marking), greenhouse /
      lever / ashby adapters, fixture-based tests
- [x] Scan pipeline (company resolution, fingerprint dedup, provenance,
      snapshots, active/changed/disappeared transitions, one transaction
      per source) and manual import
- [x] CLI: `serve` (health endpoint), `scan`, `add-source`, `list`,
      `import`, `backup`
- [x] `go build`, `go vet`, `go test` green

### Deviations (permanent decisions; SPEC updated, see SPEC §11)

| Deviation | Reason |
|---|---|
| Ashby is fetched with `GET`, not `POST` | Ashby's public posting API documentation specifies GET |
| `remote_policy` is `NOT NULL DEFAULT 'unknown'`; `config_json` is `NOT NULL DEFAULT '{}'`; `enabled` is `INTEGER NOT NULL DEFAULT 1` | avoids NULL handling; same meaning; SQLite has no BOOLEAN type |
| No CHECK constraints on enum columns (validated in Go) | adding an enum value then needs no table rebuild, keeping migrations additive |
| Fingerprint fields joined with a 0x1F separator; location bucket defined as in DATA-MODEL.md | avoids ambiguous concatenation; deterministic collision rule |
| Rate limit applied per host, using the source's configured interval (or robots `Crawl-delay` if larger) | several sources on one ATS host must not burst it |
| State rules clarified: `disappeared` → `active` when seen again; `changed` stays `changed` on later unchanged sightings; an offer listed on several sources disappears only when no source still lists it | SPEC did not define these cases |
| New package `internal/scan` (added to the AGENTS.md layout afterwards) | the pipeline needs both `db` and `sources` and will be shared by CLI and API |

## Milestone 2 — REST API + React SPA — DONE

Done criteria (all met):
- [x] REST API under `/api` with the SPEC §8 surface: sources CRUD +
      synchronous per-source scan, offers list (filters, FTS5,
      pagination), offer detail, offer PATCH, nested event CRUD, note
      CRUD, companies CRUD + detail + explicit merge, manual import
- [x] Consistent `{ "error": { "code", "message" } }` error shape
- [x] Integration tests over a temp SQLite DB with `httptest`, grouped
      by handler: sources/scan, offers/import, events, notes, companies
- [x] React + TypeScript SPA (Vite): dashboard with Applications
      section and filtered offers, sources page, offer detail,
      company detail; English, dense layout
- [x] SPA embedded with `go:embed` for production (`-tags ui`) and
      proxied to the Go API in dev (Vite `/api` proxy)
- [x] `go build`, `go vet`, `go test` green; `npm run build` green
- [x] No schema migration needed (API and UI use migration 0001)

### Deviations (permanent decisions; SPEC updated, see SPEC §11)

| Deviation | Reason |
|---|---|
| SPA assets are embedded only with `-tags ui`; without the tag the binary serves a placeholder page | `web/dist` is a build artifact and gitignored; plain `go build ./...` must work on a fresh checkout |
| `POST /api/sources/:id/scan` answers `200` even when the scan failed, with the failure in `result.error` | the request itself succeeded and the UI needs the summary; `last_scan_status` is persisted and returned |
| Company rename and merge rewrite the offers' fingerprints, and are rejected with `409 merge_conflict` on collision | fingerprints embed the normalized company name; without rewriting, later scans would create duplicates |
| Deleting a source detaches its provenance rows (`source_id = NULL`) instead of refusing or deleting links | offers and links are never deleted; a deleted source must not block cleanup |
| Notes are listed per entity (`entity_type` + `entity_id` are required) | there is no global notes view in the UI or SPEC |
| Extra list parameters `has_application`, `sort`, `limit`/`offset`; comma-separated enum filters | SPEC only says "filters" and "pagination"; the Applications section needs them |
| Offer PATCH does not recompute the fingerprint on a location correction | location is a display/collision-bucket field; keeping the identity stable avoids surprise duplicate offers |
| No new Go package for DTOs; API response types live in `internal/api/dto.go` | keeps the JSON contract separate from `model` without another package |

## Milestone 3 — Classification, dedup UX, changed-offer diff — DONE

Done criteria (all met):
- [x] Deterministic `internal/classify` engine: weighted title keyword
      sets, location/description scoring, derived `remote_policy`, and
      `match_reasons_json` with per-score human-readable terms
- [x] Classifier wired into every scan/import sighting; `jobwatch
      reclassify [--all]` backfills existing offers
- [x] Table-driven classifier tests incl. "Engineering Manager -
      Platform" vs "Marketing Platform Lead", locations/remote policies
- [x] Score badges use the matching reason list as tooltip; the
      title/location confidence filters work end to end
- [x] Possible-duplicate-company banner (dismissible) and explicit
      keep-which-company merge dialog
- [x] Offer merge and split endpoints + UI: merge moves all
      events/notes/snapshots/provenance; split creates a second offer and
      assigns selected provenance rows
- [x] `snapshot_diff` line diff between the two latest snapshots shown on
      the changed offer detail page
- [x] `go build`, `go vet`, `go test`, `npm run build` green
- [x] No schema migration needed (0001 already has the score columns)

### Deviations (permanent decisions; SPEC updated, see SPEC §11)

| Deviation | Reason |
|---|---|
| `match_reasons_json` is `{"title":[],"location":[]}`, not one flat array | each score needs its own tooltip; the SPEC only asked for "a list per score" |
| An explicit offer merge deletes the absorbed `job_offers` row after moving every child row | merging must leave one offer; all provenance, events, notes and snapshots are preserved. Scans still never delete offers |
| Splitting requires at least one provenance row to stay and a non-colliding company+title+location | otherwise it is not a split, or it would create a duplicate fingerprint |
| Split leaves events/notes/snapshots on the original | those describe the tracked application, not the link |
| Classifier adds French internship terms (`stage`, `alternance`) and a management penalty | the target market is France and manager titles are not hands-on Platform/SRE roles |
| Score caps by remote policy (onsite 50, hybrid 75, unknown 60) | `location_confidence` means "full remote in France", not just "France" |

## Milestone 4 — More sources, import ergonomics, polish — DONE

Done criteria (all met):
- [x] Personio adapter (`{tenant}.jobs.personio.de/xml`), Workable public
      widget adapter, generic RSS/Atom adapter with configurable item/field
      mapping, and a last-resort HTML adapter with per-source CSS selectors
      and a 5 s minimum poll interval
- [x] Fixture-based tests for every new adapter (recorded feeds/pages, plus
      hand-edited fixture details documented in
      `internal/sources/testdata/README.md`); no test hits the network
- [x] Seeded disabled sources for Datadog/Cisco (Workday), SAP
      (SuccessFactors) and Welcome to the Jungle with explanatory `note`
      values; no Workday/SuccessFactors/WTTJ adapter was added
- [x] `POST /api/offers/import` and `jobwatch import` accept a URL and
      extract lightweight metadata (JSON-LD, Open Graph, meta, title), with
      a manual-form fallback in the API (`422 metadata_*` + `metadata`) and
      the CLI (`--company`/`--title` or `--no-fetch`)
- [x] `jobwatch list` prints a compact table with offer/application states
      and both confidence scores; `list --sources` shows the disabled-source
      note
- [x] `jobwatch backup` validates the copy (read-only reopen, integrity
      check, foreign-key check, applied migrations) and removes a failed copy
- [x] UI polish: empty states (offers/sources/tables), auto-dismissing
      success/error toasts, loading indicators, shared confirm dialogs for
      every destructive action, and type-specific forms for the new adapters
- [x] End-to-end smoke test (`internal/api/e2e_test.go`) covering add source
      → scan through the real Workable adapter → classify → set application
      state → backdated timeline event; manual steps are documented in
      `docs/SMOKE-TEST.md`
- [x] `go build`, `go vet`, `go test`, `npm run build` green; migration 0002

### Deviations (permanent decisions; SPEC updated, see SPEC §11)

| Deviation | Reason |
|---|---|
| WTTJ is left disabled with a note instead of shipping an adapter | WTTJ exposes only an internal API with no public contract; a fragile scraper would break silently and risk blocking. Manual import and RSS/HTML sources cover the use case |
| Workday and SuccessFactors are not implemented; seeded rows are disabled `html`/`wttj` sources | SPEC §7 says to skip them in v1; the rows make the links visible with an explanation |
| The disabled known-hard rows are seeded by data-only migration 0002 | the user asked for the rows to exist; `INSERT … WHERE NOT EXISTS` makes it idempotent and keeps upgrades additive |
| URL metadata extraction performs a user-initiated page fetch | SPEC §2 only allowed source scanning; importing a URL cannot work otherwise. The fetch uses the same robots/rate-limit/UA client and is never automatic |
| HTML sources cannot poll faster than every 5 s | unknown page shapes are the most fragile target; a conservative floor protects the site |
| The HTML adapter fetches only the configured listing page | following per-offer pages would multiply requests; selectors on the listing usually carry title/link/location |
| RSS/Atom field mapping uses child-element local names plus `@attr`/`#text`, not full XPath | keeps the mapper small and dependency-free while covering real RSS/Atom feeds |
| `golang.org/x/net/html` and `github.com/andybalholm/cascadia` are new module dependencies | the standard library has no HTML parser and writing a correct CSS-selector engine in-house would add avoidable risk; both are small, stable, pure-Go libraries |
| `jobwatch backup` validates by reopening the copy read-only | `VACUUM INTO` is consistent, but an integrity/FK/schema check proves the copy is usable and catches truncated destinations |

## Milestone 5a — Bulk source registration — DONE

Done criteria (all met):
- [x] `jobwatch add-sources <file>`: one URL per line; blank lines and
      `#` comments ignored; inline `# comment` suffix stripped; URL
      fragments preserved
- [x] Every line uses the same detection, validation, config and name
      derivation as `add-source`, through a shared `sourceFromURL`
- [x] In-file duplicate URLs are refused; the remaining lines are still
      processed
- [x] Final report groups added / already registered / not detected and
      prints the not-detected URLs as a plain list on stdout
- [x] Table-driven tests over fixture files under
      `cmd/jobwatch/testdata/source-lists/`, plus parser cases for
      comments/fragments/line numbers
- [x] Ran on `docs/REMOTE-SOURCES.txt` (committed): 65 URLs read,
      8 recognized and registered, 0 already registered, 57 not
      detected
- [x] Scanned all 8 new sources: all `ok`, 315 offers (136, 27, 12, 6,
      3, 24, 47, 60 per source)
- [x] `go build`, `go vet`, `go test` green; no schema migration

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| An inline comment starts only at a `#` preceded by whitespace | most bookmark URLs carry a `#fragment` (`.../careers/#openings`); splitting at every `#` would register truncated URLs |
| `www.greenhouse.io` and `my.greenhouse.io` are no longer detected as boards | the corporate site and the candidate dashboard are not job boards; without this, `https://my.greenhouse.io/dashboard` registers a bogus board token `dashboard` |
| Duplicate lines are reported under "already registered" with a `duplicate of line N` reason; a duplicate of an undetected URL goes to "not detected" | the report keeps the requested three groups, and an unrecognized URL was never registered |
| `add-sources` exits 0 when some URLs are not detected | the report is the result; failing the batch would hide the sources that were added. File read and database errors still fail |
| `add-sources` also supports `--disabled` | keeps parity with `add-source`; the bulk list can be staged before enabling |

## Milestone 5b — Undetected source decisions — DONE

Done criteria (all met):
- [x] Every URL in `docs/REMOTE-SOURCES.txt` that `sources.Detect` could
      not classify was investigated with exactly one polite fetch
      through the shared client (robots.txt, rate limit, no retries, no
      anti-bot bypassing)
- [x] `docs/SOURCE-DECISIONS.md` records one action per URL
      (URL | company | action | reason)
- [x] 19 underlying ATS boards found, registered with `add-source`,
      scanned, and confirmed to return offers
- [x] 5 RSS/Atom feeds registered with the mapping they need; fixture
      tests cover the new Teamtailor nested-location and Homerun Atom
      shapes
- [x] 5 server-rendered HTML listings registered with selectors and
      verified with one real scan each
- [x] 28 not-trackable URLs registered with `add-source --disabled` and
      an explanatory `note`
- [x] README, CHANGELOG and MILESTONES updated; no schema migration
- [x] `go build`, `go vet`, `go test` green

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| Toggl is disabled even though `jobs.ashbyhq.com/toggl` exists | the Ashby public posting API returns 404 for that org; only the embed script is available, so the Ashby adapter cannot fetch it |
| Harvest is disabled although its page is server-rendered | the page currently lists no openings, so selectors cannot be verified; it can be revisited when roles appear |
| Murena tracks only the engineering table | the page has a second "Other positions" table with a different column layout; the engineering table is the relevant one and its selectors are stable |
| We Work Remotely keeps the source name as the company | the feed has no company element and titles are "Company: Role"; splitting titles is the existing RSS known gap, not a 5b change |
| The 28 not-trackable rows are `html`/`wttj` sources with a `note` | consistent with the seeded Workday/SuccessFactors/WTTJ rows; they are never scanned and offers enter through manual import |

## Source seeding — migration 0003 — DONE

Done criteria (all met):
- [x] Exported the 69 sources from the working database (4 known-hard
      from 0002 + 65 registered in 5a/5b) instead of guessing any row
- [x] Data-only migration 0003 reproduces all 65 5a/5b rows with their
      name, URL, `source_type`, enabled flag and full `config_json`:
      37 enabled (11 Greenhouse, 11 Ashby, 4 Lever, 1 Personio, 5 RSS,
      5 HTML) and 28 disabled not-trackable rows with their notes
- [x] The four disabled known-hard rows from 0002 are left untouched and
      not duplicated
- [x] Every INSERT uses `WHERE NOT EXISTS` by URL; no row is ever
      updated, so re-applying the file and user-edited sources are safe
- [x] Tests assert the seed count/types/flags/configs, URL uniqueness,
      duplicate-free re-application, restoration of a deleted row and
      preservation of an edited row
- [x] Fresh temp database migrations apply and `jobwatch list --sources`
      shows all 69 rows with the expected type/config/enabled state;
      a real scan of enabled sources still works
- [x] `go build`, `go vet`, `go test` green; no adapter change

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| Seeding is a data-only migration rather than a built-in source registry | migrations already ship with the binary and run on first open; this keeps a single source of truth in SQLite and needs no new code path |
| The seed never updates a row with the same URL | the user may have changed the name, config or enabled flag; the seed must only fill a fresh or partially populated database |
| The 28 not-trackable rows are shipped disabled with their notes | they were decided as manual-import-only in 5b; re-enabling them would make scans fail repeatedly |

## Milestone 5c — Aggregator follow-ups — DONE

Done criteria (all met):
- [x] We Work Remotely uses its public RSS feed
      `https://weworkremotely.com/remote-jobs.rss` as an enabled `rss`
      source with `{"location_field":"region"}`; one real scan returned
      `ok` with 100 feed items and 99 offers (0 new, 0 changed, 0
      disappeared)
- [x] Fixture test for the WWR feed shape (`region`, `Company: Role`
      titles kept verbatim, empty company) using the recorded
      `internal/sources/testdata/rss_feed.xml`
- [x] Workable public search evaluated: the France+remote query URL is
      disallowed by robots.txt and the allowed category pages are
      per-category, capped and incomplete; no targeted adapter or `html`
      source was added and the disabled row stays with its note
- [x] `docs/SOURCE-DECISIONS.md` records the Aggregators review and
      states that offers from FlexJobs, RemoteMore, the
      `my.greenhouse.io` dashboard, The Product Crew, WTTJ matches and
      Jobgether enter JobWatch via `jobwatch import` / the UI import form
- [x] README has one paragraph describing that manual-import workflow
- [x] `go build`, `go vet`, `go test` green; no schema migration and no
      adapter change

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| The WWR row was not created by 5c | 5b already registered and scanned the same feed; 5c re-verified the row against migration 0003 and added the missing fixture test instead of creating a duplicate source |
| No `html` source for the allowed Workable category pages | the sitemap lists 36 France category URLs, each capped at 15 results, overlapping and without a Platform/SRE category; the complete search uses an undocumented API. Per-company `apply.workable.com/<account>` boards are the stable surface |
| Aggregator rows keep `source_type = html`, disabled, with the 5b `note` | they are documentation of why a URL is not tracked; the manual import path already exists and needs no new source type |
| No data migration updates the Workable note | it already states the correct reason; migrations never update existing rows, so changing it would only affect fresh databases |

## Source seeding — migration 0004 (5c audit) — DONE

Done criteria (all met):
- [x] `migrations/0003_seed_remote_sources.sql` already exists and was
      left untouched
- [x] The live database (69 rows) was compared with 0003: the 5c-decided
      rows were already present (We Work Remotely `rss` with
      `{"location_field":"region"}`, Workable search disabled note,
      FlexJobs, Jobgether, WTTJ matches, Greenhouse candidate dashboard,
      The Product Crew and RemoteMore disabled notes)
- [x] `migrations/0004_seed_5c_sources.sql` added as an audit-only,
      data-only no-op; it contains no INSERT or UPDATE and cannot
      duplicate, overwrite or restore a row
- [x] Fresh temp database: migrations 0001–0004 apply in order; all 69
      sources match the live database field for field (name, URL, type,
      enabled, full `config_json`); only migrations 0001–0004 are
      recorded
- [x] Applying `list --sources` twice and re-applying 0004 twice changes
      nothing; the re-apply test leaves a user-edited row and a deleted
      row as the user left them
- [x] One real `jobwatch scan 36` on the fresh database returns `ok`
      (100 feed items, 99 offers)
- [x] `go build`, `go vet`, `go test` green; no adapter change

Final seeded-source count: **69** (4 disabled known-hard rows from 0002
+ 65 rows from 0003). No source is intentionally left out: all 5c
decisions are covered by the existing 0003 rows.

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| 0004 contains no INSERT even though the prompt expected new 5c rows | the audit showed that 5c introduced no new source rows; WWR, Workable and all aggregator rows were already shipped by 0003. Duplicating the URLs in 0004 would break the "only sources introduced by 5c" rule and create two sources of truth |
| 0004 is still created and recorded in `schema_migrations` | it is the durable record that the 5c set was audited; a comment-only migration is accepted by the runner and makes the no-op explicit instead of silently skipping a migration number |
| The test asserts 68 rows after deleting a 5c source and re-applying 0004 | proves the audit migration does not resurrect a row the user deliberately removed, unlike the seed migrations 0002/0003 |

## Known gaps (current)

| Gap | SPEC ref | Status / behavior |
|---|---|---|
| ATS workplace hints discarded | §6 | Open. Lever/Ashby `workplaceType` is not carried in `RawOffer`; Workable's `telecommuting` is used, and `remote_policy` is otherwise derived from location and description text |
| Title change of a posting creates a new offer | §5 | Open. Identity is the fingerprint only (external UID not used) |
| WTTJ adapter missing | §7 | By design. A disabled seeded row exists; scans report `skipped` and leave its status unchanged |
| Workday/SuccessFactors adapters missing | §7 | By design for v1. Disabled seeded rows carry an explanation |
| HTML adapter is listing-only | §7 | Open. It fetches one page per scan; descriptions are whatever the configured selector exposes |
| RSS/Atom item heuristics | §7 | Open. Titles such as “Company: Role” are not split automatically; mapping is configurable per source |
| Import metadata extraction is heuristic | §3 | Open. JSON-LD/Open Graph/meta/title only; pages without them need the manual form |
| No "N consecutive failures" counter | §7 | Open. One failed scan marks the source `error`/`blocked`; the next success resets it. The error text is in the scan summary but not stored |
| `notes_fts` not exposed | §4 | Open. The index is maintained but `GET /api/offers?q=` searches offers only |
| No FTS search over companies in the UI | §9 | Open. `GET /api/companies?q=` matches the name; no full-text company search |
| No snapshot listing endpoint | §9 | Open. Detail returns the latest snapshot, the count and a diff of the two latest only |
| Company notes can be pinned through the API but the UI only pins offer notes | §4, §9 | Open. The company page supports add/delete, not edit/pin |
| robots.txt cache is in memory | §2 | Open. Refetched once per process (24 h TTL within a long-running `serve`) |
| Some career-page bookmarks remain untrackable | §7 | By design after 5b. 19 boards, 5 RSS feeds and 5 HTML listings are tracked; 28 URLs are disabled with a `note` (JS-only SPAs, Notion, candidate logins, Workday/SuccessFactors/iCIMS/Pinpoint/Zoho, WTTJ, robots.txt disallows, aggregators). See `docs/SOURCE-DECISIONS.md` |
| Workable public search is not tracked | §7 | By design after 5c. The France+remote query URL is robots-disallowed; allowed category pages are incomplete. Track the company's `apply.workable.com/<account>` board instead |
| Aggregator offers require manual import | §7 | By design after 5c. FlexJobs, RemoteMore, the `my.greenhouse.io` dashboard, The Product Crew, WTTJ matches and Jobgether are never scraped; use `jobwatch import` or the UI import form |
| Teamtailor offers created before the mapping fix have no location | §7 | Open data artifact. The `locations/location/name` mapping is correct and fixture-tested; scans do not update the location of an existing offer, so only new offers carry it |
| Murena "Other positions" table is not tracked | §7 | By design. The second table has a different column layout; the engineering table is tracked |
| No scheduled scans or notifications | §10 | Out of scope for v1 |
