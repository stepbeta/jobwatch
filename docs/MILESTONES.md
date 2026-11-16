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
| Most career-page bookmarks are not auto-detected | §7 | Open. `add-sources` recognized 8 of 65 URLs in `docs/REMOTE-SOURCES.txt`; the 57 plain career/aggregator pages need the next milestone's handling or a manual `--type html`/`--type rss` source |
| No scheduled scans or notifications | §10 | Out of scope for v1 |
