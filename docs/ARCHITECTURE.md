# Architecture

Single Go binary (`cmd/jobwatch`), SQLite via `modernc.org/sqlite`
(pure Go), CLI via Cobra. No external services.

## Packages

| Package | Role |
|---|---|
| `cmd/jobwatch` | Cobra root + subcommands: `serve`, `scan`, `add-source`, `list`, `import`, `backup` |
| `migrations` | Numbered `NNNN_name.sql` files embedded with `go:embed` |
| `internal/db` | `Open` (DSN pragmas: WAL, foreign_keys, busy_timeout 5s, `_txlock=immediate`), migration runner, `Backup` (`VACUUM INTO`), queries over a `Querier` (`*sql.DB` or `*sql.Tx`) |
| `internal/model` | Table structs, enum constants, `normalize.go` (company/title normalization, location bucket, fingerprint, text hash), time helpers |
| `internal/sources` | `Fetcher` interface, `RawOffer`, `Config` (config_json), `Registry`, polite HTTP `Client`, robots.txt parser, URL type detection, adapters |
| `internal/scan` | Scan pipeline and manual import; depends on both `db` and `sources`, shared by CLI and (later) API |
| `internal/api` | REST handlers under `/api` (only `GET /api/health` so far) |
| `internal/classify` | Placeholder for the scoring engine |
| `web/` | Placeholder for the SPA |

## Fetching

```go
type Fetcher interface {
    Fetch(ctx context.Context, src model.Source) ([]RawOffer, error)
}
type RawOffer struct {
    ExternalUID, Title, Location, URL, Description string
    Company string
}
```

Adapters (`sources.NewRegistry`):

| Type | Request | Notes |
|---|---|---|
| greenhouse | `GET https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs?content=true` | `content` is entity-escaped HTML → unescaped → plain text; `company_name` fills `RawOffer.Company` |
| lever | `GET https://api.lever.co/v0/postings/{site}?mode=json` (`api.eu.lever.co` when `region=eu`) | description = `descriptionPlain` + lists + `additionalPlain`; multiple `allLocations` joined with `; ` |
| ashby | `GET https://api.ashbyhq.com/posting-api/job-board/{org}` | `isListed=false` skipped; secondary locations joined with `; ` |

Other types return `ErrNotImplemented`; the scan skips them.

`sources.Detect(url)` maps public board URLs (`boards.greenhouse.io/x`,
`job-boards[.eu].greenhouse.io/x`, `jobs[.eu].lever.co/x`,
`jobs.ashbyhq.com/x`, and the API URLs) to a type + config. Adapters use
config keys first, then fall back to detection on the source URL.

### Polite client (`sources.Client`)

- User-Agent `Mozilla/5.0 (compatible; JobWatch/0.1; personal job-search tracker; +https://github.com/stepbeta/jobwatch)`; robots token `JobWatch`.
- robots.txt per scheme+host, cached 24 h per process. RFC 9309
  handling: 2xx parsed (specific group over `*`, longest match, `*`/`$`
  wildcards, `Crawl-delay`); 4xx = allow all; 429 = blocked; 5xx/network
  error = error, not cached.
- Rate limit: requests to the same host are spaced by the source's
  `rate_limit_ms` (default 2 s) or the robots `Crawl-delay`, whichever is
  larger. The limiter is keyed by host so several sources on one ATS
  share it. The robots.txt fetch counts as a request.
- 30 s timeout, 20 MiB body cap. HTTP 401/403/429 and robots disallow
  wrap `ErrBlocked`.

## Scan pipeline (`scan.Scanner`)

Per source (`ScanSource`), `ScanAll` iterates enabled sources by ID and
never stops on a failing one:

1. `now` = scan start (UTC, seconds); fetch outside any transaction.
2. Fetch error → `last_scan_status = blocked|error`, `last_scan_at = now`, stop.
3. One transaction:
   - for each `RawOffer` (skipped if no title or URL): company =
     `RawOffer.Company` → `config.company` → source name;
     `ResolveCompany` (exact normalized match or insert); `resolveOffer`
     by fingerprint with location-bucket fallback; insert (+ snapshot) or
     update per the offer_state machine (DATA-MODEL.md); upsert
     provenance `(offer_id, url)` with `source_id`, `external_uid`,
     `last_seen_at = now`.
   - `MarkDisappeared(source, now)`.
   - `last_scan_status = ok`, `last_scan_at = now`; commit.
4. A failure inside the transaction rolls back and marks the source `error`.

Classification (SPEC step 3) is not wired in yet; score columns stay NULL.

## Manual import (`scan.Import`)

Input: `{company, title, location?, url?, description?}`. Runs the same
sighting logic in one transaction with `source_id = NULL`; a matching
fingerprint adds a provenance row to the existing offer. Never fetches.

## Server

`jobwatch serve` binds `127.0.0.1:8080` by default and mounts the API
under `/api/`; `/` returns a plain-text placeholder until the SPA exists.
