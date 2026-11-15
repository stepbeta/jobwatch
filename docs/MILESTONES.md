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

### Known gaps (open; SPEC unchanged, to be closed in later milestones)

| Gap | SPEC ref | Current behavior |
|---|---|---|
| Classification not run during scans | §6, §7 step 3 | `title_confidence`, `location_confidence`, `match_reasons_json` stay NULL; `remote_policy` stays `unknown` |
| ATS workplace hints discarded | §6 | Lever/Ashby `workplaceType` is not carried in `RawOffer`, so the future classifier only sees location and description text |
| No fuzzy "possible duplicate company" detection | §5 | only exact normalized-name matches; different spellings create separate companies |
| No manual offer merge/split | §5, §8 | collisions are split by location bucket only; same-bucket postings share one offer with several provenance rows |
| No diff summary for `changed` offers | §5 | each version is stored as a snapshot; no diff is computed |
| No "N consecutive failures" counter | §7 | one failed scan marks the source `error`/`blocked`; the next successful scan resets it to `ok`. The error message is printed by the CLI but not stored |
| No way to acknowledge a `changed` offer | §5, §9 | it stays `changed` |
| Title change of a posting creates a new offer | §5 | identity is the fingerprint only (external UID not used); the old offer becomes `disappeared` |
| Adapters for personio, workable, wttj, rss, html missing | §7 | such sources can be registered with `--type` but are skipped by `scan` (status unchanged) |
| REST API limited to `GET /api/health` | §8 | other endpoints not implemented |
| No web UI | §9 | `serve` returns a plain-text placeholder at `/`; nothing embedded from `web/dist` |
| `events` and `notes` tables unused | §4 | schema and FTS exist; no code writes them |
| robots.txt cache is in memory | §2 | refetched once per process (24 h TTL within a long-running `serve`) |
