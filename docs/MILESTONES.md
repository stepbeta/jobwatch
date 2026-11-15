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

Deviations from SPEC / plan:
- Ashby uses `GET`, not `POST`: Ashby's public posting API documentation
  specifies GET.
- New package `internal/scan` (not in the AGENTS.md layout): the
  pipeline needs both `db` and `sources` and will be shared by CLI and API.
- `remote_policy` is `NOT NULL DEFAULT 'unknown'`, `config_json` is
  `NOT NULL`, `enabled` is INTEGER: avoids NULL handling; same meaning.
- No CHECK constraints on enum columns (validated in Go) so new values
  only need additive migrations.
- Classification is not run during scans yet (score columns NULL,
  `remote_policy` `unknown`): it is outside this milestone's scope.
- Fuzzy "possible duplicate company" detection not implemented: needs
  the merge UI of a later milestone.
- A single failed scan marks a source `error`; there is no consecutive
  failure counter (would need a schema column).
- The rate limiter is keyed by host (using the source's interval) so
  several sources on one ATS do not burst it.
- A `changed` offer stays `changed` on later unchanged sightings, until
  a future UI acknowledgement action exists.
