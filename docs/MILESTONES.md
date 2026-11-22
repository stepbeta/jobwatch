# Milestones

Later milestones are defined by the user's milestone prompts and are
added here as they start.

## Current state

Milestones 1–11 are complete. The binary scans the 71 seeded sources
plus any sources the user adds, detects new and disappeared offers,
keeps every distinct description as a version, classifies offers
deterministically, and serves the React UI and REST API from one process.
The OpenSpec change `classification-profiles` (task groups 1-8) is
implemented: offer facts are separated from the search profile, title and
location affinity are computed at read time, and the profile is editable
in the SPA.
The OpenSpec change `offer-identity-from-source-uid` is implemented
through task group 8 and archived; its four capabilities now live under
`openspec/specs/` and the matching `docs/SPEC.md` sections are pointers.
The OpenSpec change `harden-wttj-discovery-and-query` is implemented and
archived; its `wttj-source` capability lives under `openspec/specs/` and
the WTTJ note in `docs/SPEC.md` is a pointer.
End to end, a user can add sources, scan, filter and bookmark offers, set
application states, keep a timeline and notes, browse description
versions, merge or split records after explicit confirmation, and build a
personal interview-prep FAQ with categorized markdown answers and
drag-and-drop ordering.
Milestone 8 reorganizes the README and adds `docs/USAGE.md`; it changes no
behavior, schema or API. Milestone 9 adds the standalone interview-prep
table, REST resource and page. Milestone 10 adds entry categories,
section/entry drag-and-drop, the redesigned answer dialog and a slightly
larger base font. Milestone 10a adds the Welcome to the Jungle company
page adapter and seeds the Fabriq source. Milestone 11 adds shared table
controls, the sources bulk workflow, the two-column offer header and the
root Makefile. Open items and deliberate limitations remain in the
known-gaps table below.

## Classification profiles — DONE

OpenSpec change `classification-profiles`, implemented in task groups 1-8.

Done criteria (all met):
- [x] Migration 0015 creates `search_profiles`, `search_profile_terms`
  and `search_profile_remote` (additive; legacy score columns kept and
  unused) and seeds one default profile with the former 35/79/15 term
  tables and their exact weights.
- [x] `internal/classify` is a pure read-time scorer taking a profile:
  title affinity, location affinity (location + description terms),
  the fixed remote detector and a membership check, with structured
  reasons. The hardcoded term tables, policy points and caps are gone.
- [x] Scan, manual import, repair, split, merge and location/description
  edits store facts and re-derive only `remote_policy`; no affinity is
  written. `reclassify` and `scan.Reclassify` are removed.
- [x] Profile API (`GET` list/detail, `PUT` replace) and offer affinity
  filters/sorts with a Go-side score pass over SQL fact filters.
- [x] Search profile page and affinity/remote presentation in the offers
  table and detail; SPA tests and production build pass.
- [x] `go build ./...`, `go vet ./...`, `go test ./...`, `npm test` and
  `npm run build` pass; docs and changelog updated.
- [x] Known gap about stale scores after manual edits is closed.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| The SPA edits only the default profile; the API and schema support several | the change explicitly defers a profile selector; a second profile can be selected with `profile_id` in API/URL tests |
| Affinity filters and sorts run in Go after the SQL fact filters | scores are read-time values with no stored column; the personal dataset is small and this keeps profile edits immediately effective |
| `floor`/`ceil` weights are not used: bucket selection sets a default numeric weight that stays editable | the bucket is a display label; the numeric weight is the value the scorer uses, as the data model states |
| Offer responses use the stored `remote_policy` fact and compute acceptance from the profile | the detector runs at ingestion/edits; reads must not silently re-detect a fact |
| `docs/SPEC.md` §6 is updated now and becomes a pointer when the change is archived | ownership moves to `openspec/specs/classification` only at archive time |

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
| ~~Company rename and merge rewrite the offers' fingerprints, and are rejected with `409 merge_conflict` on collision~~ — superseded by OpenSpec change `offer-identity-from-source-uid`: rename/merge move company keys and aliases and never rewrite offer fingerprints | fingerprints stopped being offer identity; same-title offers are legitimate, so a fingerprint conflict is no longer an error |
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
| ~~We Work Remotely keeps the source name as the company~~ — closed by OpenSpec change `offer-identity-from-source-uid`: the RSS `employer` rule splits `Company: Role` into company and title, and `jobwatch reattribute` moves already-stored offers to the extracted employer | the feed has no company element; the live remediation moved 33 of 289 offers to 26 employers, and the other 256 have no current uid match so they stay under the feed as history |
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

## Milestone 6a — BambooHR adapter — DONE

Done criteria (all met):
- [x] Endpoint investigation with one polite fetch per candidate through
      the shared client: `https://semble.bamboohr.com/careers/list`
      returns the public JSON listing (9 postings, `meta.totalCount` 9);
      `https://semble.bamboohr.com/careers/jobs_feed.xml` returns
      HTTP 404. The JSON listing is implemented.
- [x] `bamboohr` added to the source types (model constant + validation,
      API enum, UI type), without touching existing rows.
- [x] Adapter reads `{subdomain}.bamboohr.com/careers/list`: posting `id`
      → `ExternalUID`, detail URL `/careers/{id}`, `jobOpeningName` →
      title, `location`/`atsLocation` parts joined into the location
      (empty and repeated parts dropped), `isRemote` adds a remote
      marker; the company stays empty so the source name applies.
- [x] Missing/null location, department, `atsLocation` or `isRemote`
      never fails a scan; entries without an id or title are skipped.
      No per-offer request is made, so `Description` is empty.
- [x] Fixture `internal/sources/testdata/bamboohr_careers.json` recorded
      from the Semble board (no hand edits) and documented in the
      testdata README. No test hits the network.
- [x] Table-driven tests cover the fixture happy path, empty and null
      listings, missing optional fields, the remote flag and malformed
      JSON (error wraps a decode failure; the source is marked `error`
      by the scan pipeline without crashing).
- [x] URL detection for `{sub}.bamboohr.com/careers` (and `/list` or
      `/{id}`), `subdomain` config key, `add-source` help/example, and a
      Subdomain field in the sources-page form.
- [x] One real scan of source 70 (Semble): `ok`, 9 fetched, 9 new,
      0 changed, 0 disappeared; all offers belong to company `Semble`.
      Samples: “Senior Platform Engineer” (Paris, France) title score
      85; “Senior Platform Engineer” (London, United Kingdom) title
      score 85; “Senior Product Designer I” (Paris, ile de France,
      France) location score 60.
- [x] Data-only migration `0005_seed_bamboohr_sources.sql` inserts the
      Semble row (`bamboohr`, `{"subdomain":"semble"}`, enabled 1) with
      `INSERT … WHERE NOT EXISTS` by URL; no other source is added and
      existing rows are never updated.
- [x] Fresh temp database: migrations 0001–0005 apply in order;
      `jobwatch list --sources` shows Semble as row 70 with the right
      type/config/enabled state. `TestSeedBambooHRSource` re-applies the
      migration twice (no duplicate, user edit preserved) and restores a
      deleted seed row.
- [x] `go build`, `go vet`, `go test` green; no schema change.

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| The JSON `/careers/list` endpoint is implemented; the XML `jobs_feed.xml` variant is not | the XML endpoint answers 404 for the Semble tenant (and BambooHR's public careers pages use the JSON listing), so an XML adapter would have no working target |
| Descriptions stay empty and no per-offer detail page is fetched | the listing has no description; fetching one page per posting would multiply requests and break the one-listing-fetch-per-scan rule |
| `departmentLabel` is parsed but not stored | `RawOffer` has no department field, and mixing it into the description would pollute `raw_text`; the value is available if a later milestone adds a column |
| Location parts are deduplicated with a substring rule | BambooHR can return city `London` plus state `London, City of`; joining both verbatim reads badly. The country is kept even when a province name contains it |
| The Semble source ships in data-only migration 0005 | follows the 0002/0003 seed pattern; migrations are immutable, so 0005 is the next free number and a fresh database gets the verified source without `add-source` |

## Milestone 7b — Event vocabulary, manual description editing, collapsible provenance — DONE

Done criteria (all met):
- [x] Data-only migration 0006 renames `reply_received` to
      `message_received` in place (`UPDATE … WHERE type = 'reply_received'`);
      no CHECK constraint or table rebuild is involved and re-applying the
      file matches no rows.
- [x] Event enum in `internal/model`, API validation and the API/docs enum
      tables now list `cv_sent` `message_received` `message_sent`
      `interview` `offer` `rejected` `ghosted` `other`; the old
      `reply_received` value is rejected. `message_received` means either a
      recruiter's inbound first contact or a reply to an application;
      `message_sent` means a user-initiated ping/follow-up.
- [x] UI event dropdown/labels use "Message received" and "Message sent";
      the old label disappears.
- [x] `PATCH /api/offers/:id` accepts an optional `description` string. In
      one transaction it replaces `raw_text`, recomputes `text_hash`,
      appends a snapshot and bumps `updated_at`. `description: null` is
      rejected with `400 validation_failed`; the response still returns
      the refreshed offer.
- [x] The offer detail Description block has an Add/Edit action (textarea +
      Save/Cancel) for empty and non-empty descriptions and shows a success
      toast after saving.
- [x] Provenance is a disclosure after the header, collapsed by default,
      labelled "Provenance (N links)"; the expanded state persists per
      offer in `localStorage` (`jobwatch:provenance-open:<offer id>`). The
      explicit "Merge into another offer…" and split actions remain inside
      the expanded content.
- [x] Tests: fresh upgrade DB with a pre-0006 `reply_received` event reads
      `message_received` and keeps its other columns; 0006 is idempotent;
      API PATCH description appends snapshots and updates the hash; null,
      numeric and empty-patch requests are rejected; event create/patch
      accepts both message types and keeps occurred_at ordering; the
      dropped old value is rejected; a scan test proves that a sighting
      without a description keeps manual text while a differing source
      description marks the offer `changed` and appends a snapshot.
- [x] `go build`, `go vet`, `go test` and `npm run build` green; no note,
      application-state, classifier or adapter change.

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| An empty `description` string stores no description (NULL `raw_text`/`text_hash`) and appends an empty snapshot | the prompt only forbids `null`; an empty string is a legitimate edit and follows the existing scan/import convention for offers without text |
| ~~A manual description edit does not re-run the classifier~~ — superseded by `classification-profiles`: a manual location/description edit re-derives the remote policy, and title/location affinity is computed at read time, so no refresh command exists | the original 7b prompt kept the classifier unchanged; the later change moved scoring out of ingestion |
| Provenance expansion persists per offer in `localStorage` instead of per session | the user usually revisits the same offer; remembering the choice per offer is the least noisy option and survives reloads |
| The description editor lives inside the existing Description `<details>` block rather than a separate panel | keeps the diff, snapshot count and manual edit in one place without changing the established detail layout |

## Milestone 7c — Snapshot origins and version browsing — DONE

Done criteria (all met):
- [x] Additive migration `0007_snapshot_origin.sql` adds
      `snapshots.origin TEXT NOT NULL DEFAULT 'scan'`; existing rows keep
      their text and timestamps and read as `scan`.
- [x] Scan pipeline and manual URL imports write `scan`; description
      saves through `PATCH /api/offers/:id` write `manual`. The default
      keeps existing `InsertSnapshot` callers working as `scan`.
- [x] `GET /api/offers/:id` returns a `snapshots` version history ordered
      oldest → newest with `id`, `origin`, `captured_at`, `text_length`
      (Unicode characters) and no full text by default. `?snapshot_id=N`
      adds the full `raw_text` of that one version; unknown/foreign IDs
      return `404 not_found`.
- [x] `GET /api/offers/:id/snapshot-diff?from=N&to=M` diffs any two
      versions on demand through `internal/diff`, validates ownership of
      both IDs, and orders the pair by `captured_at` then `id`
      (older → newer). Missing/malformed IDs and `from == to` are `400`.
- [x] `latest_snapshot` also carries its `origin`.
- [x] UI Description block is a single-version viewer (never a list),
      defaulting to the latest version: prev/next arrows disabled at the
      ends, a version dropdown, "Version X of N", captured time and a
      Scan/Manual origin badge. The removed/added diff against the
      previous version is fetched from the snapshot-diff endpoint on
      demand; the oldest version shows a "first captured version" hint.
- [x] Add/Edit still appends a `manual` snapshot; after saving, the viewer
      selects the new latest version.
- [x] Tests: pre-0007 upgrade keeps every snapshot and marks it `scan`;
      the snapshots array/table metadata and conditional full text;
      arbitrary and reversed diffs; foreign snapshot rejection; origin
      recorded by both a real scan path and PATCH. Existing snapshot tests
      keep passing through the default `scan`.
- [x] `go build`, `go vet`, `go test` and `npm run build` green; no
      classifier, adapter, event-type or note change.

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| Every pre-0007 snapshot is backfilled as `scan` | the old schema has no reliable marker: scan and manual inserts both set `captured_at` at write time, and scans also bump `updated_at` in the same transaction. The only partial signal identifies at most the latest manual snapshot per offer, not older ones, so a heuristic would be misleading |
| Snapshot history is metadata-only; full text is returned for one version through `?snapshot_id=N` | a long ATS description can be tens of kilobytes and an offer can have many versions, so returning every full text would inflate the common detail payload |
| Versions are ordered oldest → newest with `id` as the tie-break within the same second | matches the "Version 1 = first captured" UI wording and gives a stable order when `captured_at` has second precision |
| `snapshot-diff` rejects `from == to` | a self-diff is not a version comparison and is almost certainly a UI bug |
| Older versions are fetched by reusing `GET /api/offers/:id?snapshot_id=N` instead of adding a separate snapshot endpoint | keeps the API surface small; the detail response is local and bounded, and only one extra fetch happens when the user opens an older version |

## Milestone 7d — Editable events and note↔event linking — DONE

Done criteria (all met):
- [x] Every timeline event row has Edit and Delete icon actions. Edit
      turns the row into an inline form with type, description and a
      required date + optional time; Save calls
      `PATCH /api/offers/:id/events/:eventID` with all three fields and
      Cancel sends no request.
- [x] Backdating accepts a date alone in both the edit and create forms;
      the API value is `YYYY-MM-DD` when no time is given.
- [x] Delete keeps the shared ConfirmDialog. Deleting a pinned event
      reloads the offer, so the note keeps its text and loses its pin
      badge (`ON DELETE SET NULL`).
- [x] Direction A: a pinned note card has a coloured left border and a
      badge with the event type icon and date; clicking it scrolls to and
      briefly highlights the event row.
- [x] Direction B: an event with pinned notes shows a note icon and
      count; clicking it expands the pinned notes inline under the event
      row (chosen interaction).
- [x] Both directions are built client-side from `notes.pinned_event_id`
      in the offer detail response; no API or schema change.
- [x] Component tests (Vitest + jsdom + Testing Library, mocked API)
      cover the backdated save, cancel-without-request, the pinned-note
      cue, the note-count marker and pin clearing after deletion.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      green; no classifier, adapter, migration, event-enum,
      note-pinning-contract or application-state change.

### Deviations (permanent decisions)

| Deviation | Reason |
|---|---|
| The create form was changed to the same date + optional time inputs as the edit form | the prompt refers to a Milestone 7a input format that is not present in this repository; the existing create form used a single `datetime-local` field, which cannot express a date-only backdate. Aligning both forms satisfies the date-only requirement and keeps the two flows identical |
| The PATCH sends all three editable fields, not only the changed ones | the inline form always shows the complete event, so sending the full state avoids diffing and makes clearing the description explicit (`null`) |
| Direction B expands the pinned notes inline under the event row | keeps the note text next to its event without moving the notes column or adding a second scroll target |
| Frontend tests add Vitest, jsdom and Testing Library as dev dependencies | the repository had no browser test runner and the milestone requires UI-level tests; these are dev-only and do not affect the single-binary build |

## Milestone 7e — Dashboard/layout restructuring and saved offers — DONE

Done criteria (all met):
- [x] Add source opens a modal dialog (shared `Modal`, same `.modal-*`
      styling) instead of an inline panel. The list stays mounted, all
      type-dependent config fields, validation and submit behavior are
      unchanged. Edit source reuses the same modal.
- [x] Migration 0008 adds nullable `job_offers.saved_at`; old databases
      open with every offer unsaved. `PATCH /api/offers/:id` accepts
      `saved`, and `GET /api/offers` accepts `saved=true|false`.
- [x] Bookmark toggles on offers rows and the offer detail header; saved
      rows are visually marked.
- [x] Dashboard Saved section lists all bookmarked offers
      (`saved_at DESC`, independent of application state) with an
      un-bookmark toggle and a "Show all" link to
      `#/offers?saved=true`.
- [x] The full offers table moved to a dedicated `#/offers` route; `#/`
      is Applications + Saved only. Hash routing stays in `router.ts`.
- [x] Refresh all moved from the top bar to the Sources header; the top
      bar is navigation only. Per-source Refresh remains.
- [x] Company notes render above the offers list with unchanged
      add/delete, rename/website edit and duplicate-banner behavior.
- [x] Applications Refresh scans only enabled sources that own at least
      one `application_state != todo` offer, sequentially through the
      existing scan endpoint. Sources with only `todo` offers are not
      scanned; manual provenance contributes nothing and is reported.
- [x] Tests: migration upgrade, saved PATCH/filter, add-source modal,
      Dashboard sections and un-bookmark, applications refresh fan-out,
      and the new routes.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      green; no classifier, adapter, scan-pipeline, notes-pinning or
      event-type change.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| Edit source reuses the add-source modal | one form, one validation path and one visual language; the modal receives the existing source as its initial state |
| The manual import form moved from the dashboard to `#/offers` | the dashboard is specified as Applications + Saved only, and import belongs with the full offers table |
| Dedicated `#/offers` route rather than a collapsible dashboard section | keeps the dashboard short and gives the table a stable, linkable URL; `#/offers?saved=true` carries the Saved filter preset |
| The top-bar Refresh-all loop was removed, not kept dormant | the same sequential batch helper now serves the Sources header and the Dashboard Applications refresh, so there is no dead code |
| Applications refresh resolves provenance with one detail request per application | `GET /api/offers?has_application=true` has no provenance field and the milestone forbids a new endpoint; the detail payload already returns `source_id`s |
| Applications refresh includes every enabled source behind an application, including `manual`/`wttj` types | the scope is defined by enabled + application ownership; those adapters report `skipped` through the existing scan endpoint |
| `sort=saved` was added to the offers API | the Saved section needs a server-side `saved_at DESC` order that is complete before the UI cap |

## Milestone 7f — Company two-column header and modal manual import — DONE

Done criteria (all met):
- [x] Company detail header and notes form one unified block:
      identity (name, website link, offer count, created_at, Edit/Delete
      company, possible-duplicate banner) on the left and company notes
      (list, new-note textarea, Add note) on the right.
- [x] Company notes keep add/delete only; edit/pin is still not offered
      for company notes.
- [x] The block collapses to vertical stacking (identity first, notes
      below) at `max-width: 900px`, reusing the existing
      `.detail-columns` breakpoint.
- [x] The company offers list remains a separate panel below the block,
      unchanged.
- [x] The inline import panel is removed from `#/offers`; an **Add offer**
      button in the Offers header opens the import form in the shared
      `Modal` from 7e.
- [x] Import behavior parity: bare URL fetch-and-prefill, explicit
      fields, inline validation, error toasts, and the
      `422 metadata_*` prefill fallback.
- [x] On success the modal closes, the existing success toast appears,
      app data refreshes, and the UI navigates to the new or matched
      `#/offers/:id` detail page so the returned id is reachable.
- [x] Tests: Offers import modal open/submit/422 fallback/close+navigate;
      Company two-column DOM structure and the 900px collapse contract.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      green; no schema, API, classifier, adapter, scan-pipeline, notes or
      event change.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| The Add offer button lives on `#/offers`, not the Dashboard | the prompt described the pre-7e layout; 7e had already moved manual import from the Dashboard to `#/offers`. The user explicitly chose "Offers page header only" when asked |
| Post-import navigation opens the offer detail page instead of adding a link to the toast | the toast component is text-only; navigation makes the returned id immediately reachable and shows the imported offer without extending the toast contract |
| The possible-duplicate banner moved into the header's left column | requested by the prompt; the merge dialog and its explicit keep-which-company confirmation are untouched |
| The shared `Modal` component was reused; no second modal abstraction was added | 7e already extracted the modal shell, and the import form is the only new consumer |
| `vitest.config.ts` enables CSS processing | the Company layout test reads `styles.css?raw` to assert the 900px breakpoint contract; dev-only config, no runtime effect |

## Milestone 8 — Documentation overhaul — DONE

Documentation-only milestone: no Go, SQL, API or UI change.

Done criteria (all met):
- [x] `README.md` rewritten visitor-first: title and factual intro,
      Requirements and Quick start immediately after it, a Core workflow
      section with a Mermaid flowchart, a trimmed command table, short
      page summaries and one-sentence reference teasers with links.
- [x] The milestone-status paragraph moved from the second line to a
      bottom "Status" section; `docs/MILESTONES.md` gained a "Current
      state" summary at the top.
- [x] New `docs/USAGE.md` holds the moved reference material: source
      types and config keys, recognized URLs, bulk registration, source
      decisions, polite fetching, web UI details, manual import, offer and
      application states, classification, backups, the full CLI reference
      and the smoke test.
- [x] Cross-links are relative and updated between README, USAGE, API,
      SPEC, ARCHITECTURE, DATA-MODEL, MILESTONES, SMOKE-TEST and
      SOURCE-DECISIONS.
- [x] README examples verified against the built CLI: build commands,
      quick-start sequence, `--help` flags, `add-source`/`add-sources`,
      `scan`, `list`, `import`, `reclassify` and `backup`.
- [x] Mermaid block uses only basic `flowchart TD` syntax and reflects the
      documented scan pipeline, dedup, classification, offer states,
      review, bookmark, application tracking and manual-import branch.
- [x] No information was dropped: moved sections are preserved in
      `docs/USAGE.md`; no feature or behavior was invented.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      remain green (documentation-only change).

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| The README keeps the eight-command table with one-line descriptions | commands are the main CLI surface; the full flag reference and long examples live in USAGE so the landing page stays scannable |
| Web UI page details moved to USAGE, README keeps five one-line bullets | a visitor needs to know what the pages are, not every modal and disclosure behavior |
| Source config tables, bulk-file rules and per-URL decisions moved to USAGE; SOURCE-DECISIONS.md remains the decision record | avoids two copies of the same table while preserving the detailed reference |
| API details stay in API.md; README links to it instead of repeating endpoint shapes | API.md is already the endpoint reference required by AGENTS.md |
| No screenshots or images; the workflow is text plus one Mermaid diagram | the repository is text-first and the prompt forbids images |

## Milestone 9 — Interview prep section — DONE

Done criteria (all met):
- [x] Additive migration 0009 creates `interview_entries` with question,
      markdown answer (`answer` TEXT NOT NULL DEFAULT ''), enum-like
      `answer_format` defaulting to `markdown`, nullable `display_order`
      for manual ordering, and created/updated timestamps. No applied
      migration is edited and no existing table or row changes.
- [x] CRUD under `/api/interview-entries`: list (explicit
      `display_order` first, then id), create (question required, answer
      may be empty), get one, partial PATCH (`question`, `answer`,
      `display_order`; unknown fields rejected; `null` clears the manual
      position) and DELETE. Duplicate questions are allowed. Errors use
      the shared `{ "error": { "code", "message" } }` shape.
- [x] Dedicated `#/interview-prep` route and top-bar "Interview prep"
      entry; the dashboard is untouched.
- [x] Accordion list: the question is the header row, expanding shows the
      rendered answer. Per-entry Edit and Delete actions sit on the header
      row; Delete uses the shared `ConfirmDialog`.
- [x] Add opens a modal (shared `Modal`); Edit reuses the same modal.
      Both have a single-line question input, a markdown textarea and a
      live preview pane. No WYSIWYG engine or formatting toolbar.
- [x] Markdown rendering covers headings, bold, italic, inline code,
      fenced code blocks and lists. It is a small in-house React renderer
      that never injects HTML, so `<script>` and other raw HTML in an
      answer are displayed literally and cannot execute. No dependency
      was added.
- [x] Tests: API integration tests for full CRUD, ordering, duplicate
      questions and validation errors; a migration-upgrade test; and
      Vitest component tests for expand/collapse, the add modal, escaped
      markdown, edit and delete.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      green; offers/events/notes/companies, the classifier, adapters,
      scan pipeline and existing migrations are unchanged.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| The REST resource is `/api/interview-entries` with `interview_entries` / `interview_entry` response keys | follows the existing plural-noun resource naming (`/api/sources`, `/api/notes`) and avoids overloading `/api/interview` as both section and resource |
| Add and Edit share one modal; editing is not inline | one form, one validation path and one live-preview implementation, matching the 7e source modal pattern |
| A hand-rolled markdown renderer is used instead of a parser dependency | the supported subset is small and fixed; emitting React text nodes makes HTML injection impossible without a sanitizer dependency, and no new runtime package is needed |
| `display_order` is supported by the API but there are no up/down reordering controls yet | the prompt marks reordering optional and forbids drag-and-drop; adding reliable mixed-NULL ordering controls was not cheap enough for this milestone |
| Rows with an explicit `display_order` sort before rows without one | SQLite's default would put NULLs first, which makes a partially ordered list confusing; the fallback to id stays defined |

## Milestone 10 — Interview prep categories, reordering and dialog UX — DONE

Done criteria (all met):
- [x] Additive migration 0010 adds nullable `interview_entries.category`;
      existing entries keep every column and read as uncategorized. No
      categories table, no `/categories` endpoint: the category set is
      derived from the rows.
- [x] `category` is returned by list/single responses and accepted by
      POST (trimmed, non-blank; omitted/`null` means uncategorized) and
      PATCH (string replaces, `null` clears, `""`/non-string rejected).
      Unknown fields are still rejected.
- [x] List ordering is per section: categories alphabetically
      (case-insensitive), uncategorized last, and within each section the
      existing explicit-`display_order`-then-id rule.
- [x] The interview-prep page groups entries into visible category
      sections, flags empty answers with an amber cue, and supports native
      HTML5 drag-and-drop: entries reorder within a section, cross-section
      drops change the category, and fresh `display_order` values plus the
      moved category are persisted through PATCH.
- [x] Section headers are draggable; the presentation-only section order
      is stored in `localStorage`, restored on load, ignores stale names,
      appends new categories alphabetically and keeps "Uncategorized"
      last. "Reset to alphabetical" restores the default.
- [x] Add/Edit modal now has a question input, a category dropdown with an
      inline "Add new category…" option, and one large markdown area with
      an Edit/View toggle (no side-by-side panes). Save/Cancel,
      validation, errors and the shared Modal shell are unchanged.
- [x] Base font size raised from 14px to 15px for body and inherited form
      controls, keeping tables, badges and dialogs dense.
- [x] Tests: migration-upgrade test for 0010, API tests for category
      round-trips/validation and per-section ordering, and Vitest tests
      for section grouping, the empty-answer cue, both drag-and-drop
      payloads, the Edit/View toggle and new-category creation.
- [x] `go build`, `go vet`, `go test`, `npm run build` and `npm test`
      green; offers/events/notes/companies, the classifier, adapters,
      scan pipeline and migrations 0001–0009 are unchanged.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| Categories are free-form row values, not a table | the prompt forbids a categories table and a `/categories` endpoint; deriving the set keeps the schema additive and avoids orphan cleanup |
| Native HTML5 drag-and-drop, no dependency | fits the zero-dependency UI constraint; the drag state is held in React rather than relying on `DataTransfer`, which jsdom does not fully implement |
| Entry drops always rewrite `display_order` for the affected section(s) | guarantees a deterministic order that survives reload even when some rows previously had no explicit position |
| Section order is browser-local only | it is a presentation preference; keeping it out of the API/database avoids inventing a second persistence model |
| The "No answer" cue is the literal empty-string check (after trim) | whitespace-only answers are just as unprepared, and the badge is only a scan aid |

## Milestone 10a — Welcome to the Jungle (WTTJ) source adapter — DONE

Done criteria (all met):
- [x] `wttj` source type registered in model validation, the adapter
      registry, the API enum and the UI type list; `add-source` detects
      any `welcometothejungle.com` URL whose path contains `/companies/`
      (the marketplace paths stay unrecognized) and names the source
      after the company slug.
- [x] Two-call adapter: one bootstrap `GET
      https://api.welcometothejungle.com/api/v1/pages?path=<careers path>`
      per scan, then `POST
      https://{app_id}-dsn.algolia.net/1/indexes/*/queries` per result
      page. Both requests go through the shared polite client, so
      robots.txt and the per-host rate limit apply to each host; the
      browser User-Agent is used only for these two endpoints.
- [x] Algolia request reproduces the verified quirks: mandatory
      `Referer`, form content type with a JSON body, `params` built as
      `filters=<discovered filters>&hitsPerPage=50&page=N` (filters
      joined with spaces, key never decoded), pagination until
      `page >= nbPages`.
- [x] Mapping: `reference` → `ExternalUID` (`objectID` as a string when
      absent), `name` → title, `organization.name` → company,
      `offices[]`/`office` → location, `profile` markdown verbatim
      (`summary` fallback), URL from the discovery language, company
      slug and `hit.slug`; salary/remote/contract_type are ignored (no
      offer column, no schema change).
- [x] Discovery failure modes: a missing `jobs-search` block returns the
      permanent `URL is not a WTTJ careers page`; HTTP/network errors and
      undecodable JSON are retryable and surface in the scan result.
- [x] Fixtures: `wttj_page_fabriq.json` (trimmed live bootstrap),
      `wttj_hits_page0.json` (reference-UUID shape) and
      `wttj_hits_page1.json` (`FABRI_wqJ1xND`, numeric `objectID`, salary
      and contract fields) under `internal/sources/testdata`, documented
      in its README. No test touches the network (httptest only).
- [x] Tests: hit mapping (UID and `objectID` fallback, offices/office,
      empty profile → summary, several offices, slug fallback, language
      fallback, dropped hits, both timestamps, null extras), bootstrap
      parsing (live shape, direct properties, missing block, malformed
      JSON, bad filters), params/body builders, the full fetch over
      httptest (exact headers, form content type with JSON body, page 0
      and 1 only), config credential overrides and the error matrix.
- [x] Data-only migration `0011_seed_wttj_sources.sql` inserts Fabriq
      (`wttj`, `{}`, enabled) with `INSERT … WHERE NOT EXISTS` by URL.
      The 0002 WTTJ marketplace row keeps its own URL and is untouched;
      `TestSeedWTTJSource` proves there is no collision, that re-applying
      preserves user edits and restores a deleted row.
- [x] Live verification (2026-11-18): discovery returned app id
      `CSEKHVMS53`, index `wk_cms_jobs_production`, language `en`,
      filters `organization.slug:fabriq`; Algolia returned `nbHits` 7;
      the first scan of source 76 created 7 offers with the expected
      titles, locations and URLs and a `profile`-filled description; the
      constructed URL
      `.../en/companies/fabriq/jobs/senior-full-stack-engineer-front-end-focus_paris`
      answers HTTP 200; a second scan reported 0 new, 7 seen, 0 changed,
      0 disappeared, 7 provenance rows and 7 snapshots.
- [x] `go build`, `go vet`, `go test ./...`, `npm run build` green; no
      schema change.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| The parser searches the whole `jobs-search` block for the `algolia_*` keys instead of reading `block.properties` directly | on the live response the credentials are in `block.contents[*].properties` while `block.properties` only holds layout keys; searching the subtree accepts both that shape and the documented one |
| The Algolia `params` string is prefixed with `filters=` | verified live: without the prefix Algolia replies `400 Unknown parameter: organizationslugfabriq`; the prefix is what the site's client sends |
| The offer URL uses the discovered page language, not `hit.language` | follows the verified construction (and the expected Fabriq URL); French-language hits still resolve under `/en/`, and a per-hit language would make the URL depend on a mutable hit attribute. The hit language is the fallback when discovery has none |
| The company slug falls back to the discovered `organization.slug` filter (and `website_organization.slug`) | one observed index shape omits `organization.slug` from the hits, and the URL cannot be built without it |
| `published_at` is decoded but not stored | `job_offers` has no publication-date column and 10a adds no schema change; `first_seen_at` remains the scan time |
| Salary, `remote` and `contract_type` are ignored | no offer column exists for them and mapping them into an unrelated field (description, location) would corrupt data |
| The discovery response is not cached beyond the scan | the prompt asks for in-memory, per-run caching only; one extra GET per scan is acceptable and credentials can rotate between runs |
| `algolia_app_id`/`algolia_api_key` config overrides still trigger discovery | the index name, filters and language are only available from the bootstrap; the overrides are documented as a debug tool |

## Milestone 10a fix-up — WTTJ discovery and query hardening — DONE

Done criteria (all met):
- [x] Discovery walks the whole bootstrap tree for the first `properties`
      object carrying `algolia_app_id`, including nested `contents[]`
      blocks. Block `kind` no longer gates the search, so a layout-only
      outer `jobs-search` block with credentials in its contents is
      handled; a response with no credentials object stays the permanent
      `URL is not a WTTJ careers page`.
- [x] The discovered filters are sent as URL-encoded `facetFilters` (one
      JSON group per attribute, several values OR-ed) instead of a
      space-joined `filters=`. The `website.reference` filter embedded in
      the secured key is omitted, and the key is never decoded. `Referer`,
      browser User-Agent, form content type with raw JSON body and
      50-hit paging are unchanged.
- [x] Job URLs and company keys use the hit's `organization.slug`; the
      discovered page slug is only a fallback for hits that omit it.
      Multi-organization pages therefore link each offer to its owner.
- [x] A stale organization filter is resolved: when page 0's `nbHits`
      differs from `items_count` (above zero), the canonical slug of every
      `organization.slug` value is read from
      `/api/v1/organizations/{slug}` and page 0 is retried once if any
      value changed (Swile: `swile` → `lunchr`, 15 offers; also covers a
      renamed brand on a multi-organization page). A page reporting no
      jobs skips the lookup, a mismatch without a rename returns the
      received hits, and a lookup failure is retryable.
- [x] Fixtures: `wttj_page_partoo.json` (nested layout-only block, three
      organization slugs) and `wttj_hits_partoo.json` (one real hit),
      documented in `internal/sources/testdata/README.md`. No test touches
      the network (httptest only).
- [x] Tests: nested/direct/generic-container discovery, the permanent and
      retryable discovery errors, multi-organization and key-filter
      `facetFilters` params, exact per-page params in the full fetch,
      org-slug precedence in the table and fixture mapping, plus the
      existing error matrix.
- [x] Live verification (2026-11-21): Fabriq re-scan fetched 7 offers
      (0 new, 7 seen, status ok). The pre-existing Partoo `wttj` source
      (id 82, `/fr/companies/partoo/jobs`, previously `error`) scanned
      `ok` with 15 fetched and 15 new, 0 incomplete; all 15 offers belong
      to company Partoo and their URLs use
      `/en/companies/partoo/jobs/...` (the discovered page language), and
      a spot-checked URL answered HTTP 200. No duplicate source was
      created.
- [x] Live verification (2026-11-21, stale slug): the `/swile/jobs`
      source previously fetched 0 offers because its page filter said
      `swile` while the index still used `lunchr`. The organization record
      returned `lunchr`, the retry fetched 15 offers (0 incomplete), and
      the URLs use `/companies/lunchr/jobs/...` and resolve with HTTP 200.
- [x] `go build ./...`, `go vet ./...` and `go test ./...` green; no
      schema, API, CLI or UI change.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| Discovery selects the first `properties` object with `algolia_app_id` anywhere in the tree and ignores block `kind` | live pages nest a layout-only `jobs-search` block around the credentialed block; the property is the real contract, and `kind` is a CMS detail |
| Multi-value filters become `facetFilters` JSON arrays with URL-encoded params | a space-joined `filters=` ANDs the values and returns no hits; the secured key embeds only `website.reference`, so the page's filters must be sent explicitly or the query returns the whole site |
| `website.reference` is dropped when building params | the key already enforces it; repeating it is redundant and the adapter deliberately never decodes keys |
| The page-slug fallback prefers `website_organization.slug`, then a single filter value, then the careers path | on a multi-organization page the first filter value is a sibling (`localoo`), not the page itself |
| The Partoo hit fixture keeps `organization.slug: partoo` | all 15 live hits belonged to `partoo` that day; the sibling-slug precedence is covered by a table case (`partoo-italia` fallback) and the fixture test (`localoo` fallback) instead of fabricating a fixture |
| The organization lookup runs only when `nbHits` differs from `items_count` | keeps the two-call flow for healthy pages; the CMS filter is normally correct and only lags after a rename, and a count mismatch without a rename returns the received hits |

The change is archived as OpenSpec change
`harden-wttj-discovery-and-query`; its `wttj-source` capability is synced
to `openspec/specs/wttj-source/spec.md` and the WTTJ note in
`docs/SPEC.md` section 7 points there.

## Milestone 11 — table controls, sources upgrade, detail layout, Makefile — DONE

Done criteria (all met):
- [x] Offers and Sources both have a persisted column picker, header-click
      sorting, debounced search-as-you-type, URL query-string state and a
      10/20/50 per-page selector. The picker stores hidden ids, ignores
      unknown saved names, appends newly added columns and refuses to hide
      the last visible one. Offers sorting is server-side; Sources sorting
      is client-side.
- [x] `GET /api/offers` supports `sort=location`, `sort=title_confidence`,
      `sort=location_confidence` and `order=asc|desc`; all previous sort
      values and their default directions still work. Invalid values are
      rejected with `400 validation_failed`.
- [x] Sources filters (enabled/disabled/all, type present in the list,
      status ok/error/blocked/never, debounced name), right-aligned enable
      switch, left selection checkbox, select-all-visible, sequential bulk
      delete with one confirmation and sequential bulk refresh through the
      shared batch helper. Row actions are icon-only with accessible labels.
- [x] Source names link to `#/companies/:id` when the effective name
      (`config.company` override, else source name) matches a company;
      otherwise they render as plain text. No new endpoint.
- [x] Offers company filter is a searchable combobox; the `company_id`
      parameter is unchanged.
- [x] Offer detail has a two-column header (identity + always-visible
      compact provenance) that stacks at `max-width: 900px`, with merge and
      split still reachable. The description Add/Edit flow uses the shared
      `EditViewEditor`; the save payload, manual snapshot selection and
      version browser are unchanged.
- [x] The company-page offers panel has debounced FTS search, header-click
      server sorting, pagination, a per-page selector and URL state.
- [x] Root `Makefile` provides `ui`, `build` and `serve`, documented in the
      README.
- [x] `go build ./...`, `go vet ./...`, `go test ./...`, `npm test` and
      `npm run build` green; no schema change.

### Deviations and implementation choices

| Choice | Reason |
|---|---|
| `order` is an explicit `GET /api/offers` parameter alongside the new sort values | server-side header toggling needs a direction; defaults reproduce the old ordering exactly when it is absent, and the URL example in the prompt includes `order` |
| Sources enabled filter defaults to `all` | preserves the current "every source visible" behavior and avoids hiding disabled seeded rows that carry explanations |
| Sources name link resolves against the local company list by exact normalized name | the sources payload has no board-reported company, and adding an endpoint for a display-only link was out of scope; a mismatch leaves plain text |
| The offer provenance disclosure was removed, not kept in the two-column header | the right column already gives the list permanent space; an always-visible compact list keeps merge/split easier to reach and avoids a per-offer localStorage key |
| Offers score cells are split into Title score and Location score | each new confidence sort needs its own clickable header; a combined "Scores" header could only sort one of the two |
| Company-page offers default to 20 rows; Offers keeps 50 and Sources uses 20 | the company panel previously showed every offer and had no page size; 20 keeps it compact, while Offers preserves the existing default |
| Company-page offers reuse the `offers.perPage` row size only through its own `companyOffers.perPage` key | the company panel is a different table and its per-page choice should not silently change the full Offers page |

## Known gaps (current)

| Gap | SPEC ref | Status / behavior |
|---|---|---|
| ATS workplace hints discarded | §6 | Open. Lever/Ashby `workplaceType` is not carried in `RawOffer`; Workable's `telecommuting` and BambooHR's `isRemote` are used, and `remote_policy` is otherwise derived from location and description text |
| BambooHR offers have no description | §7 | Open. The public listing has no description and per-offer pages are not fetched; `raw_text` stays empty unless the offer is imported from its detail URL |
| ~~Title change of a posting creates a new offer~~ — closed by OpenSpec change `offer-identity-from-source-uid`: identity is provenance `(source, uid)` then `(source, url)`, so a title edit updates the existing offer | §5 | the fingerprint is no longer read for matching |
| ~~RSS/Atom item heuristics~~ — closed by OpenSpec change `offer-identity-from-source-uid`: the declarative `employer` rule splits `Company: Role` or reads a separate field/selector, and it is configured for We Work Remotely | §7 | remaining feed-specific mappings are still configurable per source |
| Offers with no current sighting keep their adapter-era company | §5 | Open data artifact. `jobwatch reattribute` matches by `(source, uid)`; an offer the source no longer lists has no current sighting, so it stays as history until explicitly merged or deleted |
| Workday/SuccessFactors adapters missing | §7 | By design for v1. Disabled seeded rows carry an explanation |
| HTML adapter is listing-only | §7 | Open. It fetches one page per scan; descriptions are whatever the configured selector exposes |
| WTTJ adapter has undocumented dependencies | §7 | Open. Credentials are found by walking the bootstrap page tree for a `properties` object with the `algolia_*` keys; layout-only `jobs-search` blocks and nested `contents[]` are handled, but a CMS rename breaks discovery (permanent scan error). The query is scoped by copying the page's `filters` as `facetFilters`; on a count mismatch every `organization.slug` is canonicalized from the organization record and page 0 is retried once, while a mismatch without a rename returns the received hits. The secured key embeds `website.reference` and is referrer-restricted, and two index shapes are known. See `docs/SOURCE-DECISIONS.md` |
| WTTJ descriptions depend on the offer page's JSON-LD | §7 | Open. When the index has neither `profile` nor `summary` and the offer has no stored text, the adapter fetches the offer page with the fixed WTTJ browser User-Agent (the WAF challenges the JobWatch UA) and stores `JobPosting.description` as plain text; stored descriptions are skipped on later scans. A missing block, a failed page or a WAF challenge leaves `raw_text` empty while the source stays `ok`, so a systemic failure is silent. Joko was moved to its Ashby board instead (46 roles, full descriptions); Partoo uses the fallback with a 90 s interval. See `docs/SOURCE-DECISIONS.md` |
| Import metadata extraction is heuristic | §3 | Open. JSON-LD/Open Graph/meta/title only; pages without them need the manual form |
| No "N consecutive failures" counter | §7 | Open. One failed scan marks the source `error`/`blocked`; the next success resets it. The error text is in the scan summary but not stored |
| `notes_fts` not exposed | §4 | Open. The index is maintained but `GET /api/offers?q=` searches offers only |
| No FTS search over companies in the UI | §9 | Open. `GET /api/companies?q=` matches the name; no full-text company search |
| Pre-7c snapshot origins are all `scan` | §9 | By design after 7c. The old schema had no reliable manual marker; only snapshots written from 7c onward carry an exact origin |
| ~~Manual description edits do not refresh classifier scores~~ — closed by OpenSpec change `classification-profiles`: affinity is computed at read time from the current search profile, and the remote policy is re-derived on every location/description edit | §6 | the stored score columns are deprecated and unused; a profile edit applies on the next read |
| Offer merge does not carry the absorbed offer's bookmark | §9 | Open. The kept offer keeps its own `saved_at`; the absorbed bookmark is not copied. Merge semantics were left unchanged in 7e |
| Frontend component tests cover the offer timeline/notes, add-source modal, Dashboard sections/refresh, offers import modal, offers table controls (columns, sort requests, URL round-trip, debounced search, page size, company combobox), incomplete list/repair, sources filters/bulk/switch/icons/company links, company two-column layout, company keys and offers-table controls, offer header/provenance layout and description Edit/View save, hash routing and interview prep (sections, cue, drag-and-drop payloads, Edit/View, new category, accordion, add/edit/delete, escaped markdown) | §9 | Open. Vitest + jsdom + Testing Library run with a mocked API (`npm test`); real-browser drag-and-drop, company note mutations and the company merge flow still rely on `tsc`/`vite build` and backend tests |
| Source names link by override/source name only, not the board-reported company | §9 | Open. The sources payload has no board-reported company and Milestone 11 avoided a display-only endpoint; a source whose offers use a different company stays unlinked unless `config.company` is set |
| Company notes can be pinned through the API but the UI only pins offer notes | §4, §9 | Open. The company page supports add/delete, not edit/pin |
| robots.txt cache is in memory | §2 | Open. Refetched once per process (24 h TTL within a long-running `serve`) |
| Some career-page bookmarks remain untrackable | §7 | By design after 5b. 19 boards, 5 RSS feeds and 5 HTML listings are tracked, plus the Fabriq (10a) and Partoo (10a fix-up) WTTJ company pages; 28 URLs are disabled with a `note` (JS-only SPAs, Notion, candidate logins, Workday/SuccessFactors/iCIMS/Pinpoint/Zoho, WTTJ marketplace pages, robots.txt disallows, aggregators). See `docs/SOURCE-DECISIONS.md` |
| Workable public search is not tracked | §7 | By design after 5c. The France+remote query URL is robots-disallowed; allowed category pages are incomplete. Track the company's `apply.workable.com/<account>` board instead |
| Aggregator offers require manual import | §7 | By design after 5c. FlexJobs, RemoteMore, the `my.greenhouse.io` dashboard, The Product Crew, WTTJ marketplace pages and Jobgether are never scraped; use `jobwatch import` or the UI import form |
| Teamtailor offers created before the mapping fix have no location | §7 | Open data artifact. The `locations/location/name` mapping is correct and fixture-tested; scans do not update the location of an existing offer, so only new offers carry it |
| Murena "Other positions" table is not tracked | §7 | By design. The second table has a different column layout; the engineering table is tracked |
| No scheduled scans or notifications | §10 | Out of scope for v1 |
