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

## Known gaps (current)

| Gap | SPEC ref | Status / behavior |
|---|---|---|
| Classification not run during scans | §6, §7 step 3 | Open. `title_confidence`, `location_confidence`, `match_reasons_json` stay NULL; `remote_policy` stays `unknown`. The UI shows score badges as `—` placeholders |
| ATS workplace hints discarded | §6 | Open. Lever/Ashby `workplaceType` is not carried in `RawOffer` |
| No offer merge/split | §5, §8 | Open. Company merge exists; the offer detail page shows a disabled "Split offer" placeholder. Same-bucket postings share one offer with several provenance rows |
| No diff summary for `changed` offers | §5 | Open. Snapshots are stored; the UI shows the latest text and the snapshot count, no diff |
| No "N consecutive failures" counter | §7 | Open. One failed scan marks the source `error`/`blocked`; the next success resets it. The error text is in the scan summary but not stored |
| Title change of a posting creates a new offer | §5 | Open. Identity is the fingerprint only (external UID not used) |
| Adapters for personio, workable, wttj, rss, html missing | §7 | Open. Such sources can be registered; scan reports `skipped` and leaves their status unchanged |
| `notes_fts` not exposed | §4 | Open. The index is maintained but `GET /api/offers?q=` searches offers only |
| No FTS search over companies in the UI | §9 | Open. `GET /api/companies?q=` matches the name; no full-text company search |
| Import by URL alone is not implemented | §3 | Open. Both CLI and API require `company` and `title`; a bare URL cannot be fetched for metadata |
| No snapshot listing endpoint | §9 | Open. Detail returns the latest snapshot and the count only |
| Company notes can be pinned through the API but the UI only pins offer notes | §4, §9 | Open. The company page supports add/delete, not edit/pin |
| robots.txt cache is in memory | §2 | Open. Refetched once per process (24 h TTL within a long-running `serve`) |
| No scheduled scans or notifications | §10 | Out of scope for v1 |
