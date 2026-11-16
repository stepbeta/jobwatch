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
| `internal/scan` | Scan pipeline and manual import; depends on both `db` and `sources`, shared by CLI and API |
| `internal/api` | REST handlers under `/api`: sources, offers, events, notes, companies, import, health |
| `internal/classify` | Placeholder for the scoring engine |
| `web/` | React + TypeScript SPA source and its Go embedding (`Handler`) |

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

## REST API (`internal/api`)

`api.Server{DB, Version, Fetcher, Now}` mounts Go 1.22 `ServeMux`
patterns under `/api/`. The fallback `/api/` pattern answers unknown
methods and paths with `404 not_found`. Bodies are decoded strictly
(unknown fields rejected, 1 MiB cap); responses use snake_case DTOs
defined in `internal/api/dto.go`, independent of the `model` structs.
Errors always use `{"error":{"code","message"}}`.

| Group | Routes |
|---|---|
| health | `GET /api/health` |
| sources | `GET/POST /api/sources`, `GET/PATCH/DELETE /api/sources/{id}`, `POST /api/sources/{id}/scan` |
| offers | `GET /api/offers`, `POST /api/offers/import`, `GET/PATCH /api/offers/{id}` |
| events | `GET/POST /api/offers/{id}/events`, `PATCH/DELETE /api/offers/{id}/events/{eventID}` |
| notes | `GET/POST /api/notes`, `GET/PATCH/DELETE /api/notes/{id}` |
| companies | `GET/POST /api/companies`, `GET/PATCH/DELETE /api/companies/{id}`, `POST /api/companies/{id}/merge` |

`POST /api/sources/{id}/scan` builds a `scan.Scanner` from the server's
fetcher and runs one source synchronously, returning the `scan.Result`
plus the refreshed source row. A fetch failure is part of the summary
(HTTP 200) and is recorded in `last_scan_status`; an unimplemented
adapter is reported as `skipped` without touching the status. The
default fetcher is one `sources.Registry` with one shared
`sources.Client`, created lazily per server so the robots cache and
per-host rate limiter survive across requests.

Offer listing is `db.QueryOffers`: a shared WHERE builder for the count
and page queries, FTS5 `MATCH` against `job_offers_fts` (validated up
front so a syntax error is a 400), enum lists, score minima,
`has_application`, company filter, five sort orders, `limit`/`offset`.
`recent_event` sorts by `MAX(events.occurred_at)` with `updated_at` as
fallback. Detail responses join provenance with source names, events,
offer notes, the latest snapshot and the snapshot count.

## Company matching, rename and merge (`internal/db`)

`model.SimilarCompanyNames` is a conservative pure function used for
the UI's "possible duplicate" prompt: token containment (`qonto` /
`qonto france`, at most two extra tokens) or edit distance ≤ 2 on names
of 5+ characters with ≤ 20% changes. Scores below
`model.MinCompanyCandidateScore` (60) are dropped. Nothing merges
automatically.

`db.MergeCompanies` and `db.UpdateCompany` (rename) share
`planFingerprintUpdates`: for every offer of the source company it
rebuilds the fingerprint with the target normalized name, preserving
whether the offer used the base or the location-bucketed form. If any
new fingerprint already exists outside the company (or twice in the
plan), the whole operation is rejected with `db.ConflictError` and
nothing is written. Merge moves offers and company notes, copies the
website when the target has none, and deletes the emptied company.
Rename changes `name`/`name_normalized` and rewrites fingerprints in
the same transaction. Deleting a company is refused while it owns
offers; deleting a source detaches its provenance rows (`source_id`
becomes NULL) instead of deleting offers.

## Web UI (`web/`)

React 19 + TypeScript built by Vite. Source in `web/src`:

| Path | Content |
|---|---|
| `App.tsx` | top bar, global refresh-all loop, scan notices, routing |
| `router.ts` | hash routing (`#/`, `#/sources`, `#/offers/:id`, `#/companies/:id`) |
| `api.ts` | typed fetch wrapper and API error mapping |
| `pages/Dashboard.tsx` | Applications section (`has_application`, `sort=recent_event`), filtered/paginated offers table, import form |
| `pages/SourcesPage.tsx` | source list, enable toggle, scan status, per-source refresh, add/edit form with type-dependent config fields |
| `pages/OfferPage.tsx` | editable states/location, provenance cards with split placeholder, timeline with backdating, notes with event pin, snapshot block |
| `pages/CompanyPage.tsx` | offers, company notes, rename/website edit, possible-duplicate merge prompt |

`vite.config.ts` proxies `/api` to `127.0.0.1:8080` in development.

In production the built `web/dist` is embedded by `web/embed.go`
(`//go:build ui`, `//go:embed all:dist`). `web/spa.go` serves the files
with immutable caching for `assets/` and an `index.html` fallback for
client-side routes. Without the `ui` build tag, `web/noui.go` serves a
placeholder page explaining how to build the SPA, so plain
`go build ./...` works on a fresh checkout.

## Server

`jobwatch serve` binds `127.0.0.1:8080` by default. `/api/` is the REST
API; everything else is the SPA (embedded when built with `-tags ui`,
placeholder otherwise).
