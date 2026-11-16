# jobwatch

JobWatch is a local, single-user tool that tracks job offers from
company job boards (Greenhouse, Lever, Ashby, Personio, Workable), RSS /
Atom feeds and CSS-selector career pages. It detects new, changed and
disappeared offers and keeps everything in a single SQLite file.

Status: Milestone 4 (more source adapters, URL import metadata, backup
validation, UI polish). The Go server exposes the full API and, when
built with the `ui` tag, embeds the React SPA. See
[docs/MILESTONES.md](docs/MILESTONES.md) for what is done and the known
gaps.

## Requirements

- Go 1.27 or newer. No other backend dependencies: SQLite is a pure-Go
  driver compiled into the binary.
- Node.js 20+ (with npm) only to build the web UI.
- Linux (developed on WSL2 / Ubuntu 24.04); other platforms should work.

## Build

```sh
# Backend only: the binary serves the API and a placeholder page.
go build -o jobwatch ./cmd/jobwatch

# With the embedded web UI.
cd web
npm ci
npm run build
cd ..
go build -tags ui -o jobwatch ./cmd/jobwatch

# optional: embed a version string
go build -tags ui -ldflags "-X main.version=0.2.0" -o jobwatch ./cmd/jobwatch
```

`web/dist` is a build artifact (gitignored). Plain `go build ./...`
works without it; `-tags ui` requires `npm run build` first.

## Quick start

The database defaults to `./jobwatch.db` and is created on first use.
Use the global `--db <path>` flag to choose another file.

```sh
# 1. Register sources (type and board name are detected from the URL)
./jobwatch add-source https://boards.greenhouse.io/gitlab --name GitLab
./jobwatch add-source https://jobs.lever.co/spotify --name Spotify
./jobwatch add-source https://jobs.ashbyhq.com/linear --name Linear
./jobwatch add-source https://acme.jobs.personio.de --name Acme
./jobwatch add-source https://apply.workable.com/acme --name Acme

# 2. Start the web UI (or scan headlessly with ./jobwatch scan)
./jobwatch serve
# → http://127.0.0.1:8080

# 3. Headless usage still works
./jobwatch scan
./jobwatch list                  # ID, company, title, location, remote, T/L scores, states
./jobwatch list --offer-state changed
./jobwatch list --sources        # includes the note on disabled known-hard sources
./jobwatch reclassify            # backfill scores of older offers
./jobwatch reclassify --all      # recompute every score after keyword changes
./jobwatch backup ~/jobwatch-$(date +%F).db   # validated copy
```

## Commands

| Command | Description |
|---|---|
| `serve [--port 8080] [--addr 127.0.0.1]` | Start the local server: REST API under `/api` and the SPA at `/` |
| `scan [all\|<source-id>]` | Scan all enabled sources (default) or one source by ID. Exits non-zero if a source failed |
| `add-source <url> [--name N] [--type T] [--config JSON] [--disabled]` | Register a source |
| `list [--offer-state S] [--app-state S] [--limit N] [--sources]` | List offers (most recently seen first) or sources |
| `import <url \| json \| file.json \| ->` | Add an offer seen elsewhere. A bare URL is fetched for JSON-LD/Open Graph/title metadata (`--no-fetch` disables); JSON fields can be supplied manually |
| `reclassify [--all]` | Backfill title/location scores and remote policies with the deterministic classifier; `--all` recomputes every offer |
| `backup <dest>` | Write and validate a consistent copy of the database (must not exist) |

Run `./jobwatch <command> --help` for details.

## Web UI

Built with React + TypeScript (Vite). The pages are:

- **Dashboard** — the Applications section on top (offers whose
  application state is not `todo`, most recent timeline activity first),
  then the offers table with filters (application/offer state, remote
  policy, score minima, company, full-text search), sorting and
  pagination. Score badges show 0–100 title/location confidences; hover
  one to see the matched keywords from the classifier. The import form
  accepts a URL and pre-fills the manual fields when extraction is
  incomplete.
- **Sources** — enable/disable, scan status, per-source Refresh, a
  global Refresh all in the top bar, and add/edit forms whose config
  fields depend on the source type (Personio tenant, Workable account,
  RSS field mapping, HTML CSS selectors). Disabled seeded rows for the
  known-hard Workday/SuccessFactors/WTTJ links show their explanation.
- **Offer detail** — editable application/offer state and location,
  score badges, all provenance links side by side, explicit merge into
  another offer and split (select links for a second offer), a vertical
  timeline with backdating, notes optionally pinned to an event, and the
  latest description plus a removed/added line diff for `changed` offers.
- **Company detail** — offers, company notes, rename/website edit, a
  dismissible possible-duplicate banner and a merge dialog that lets you
  choose which company row to keep. Nothing is merged automatically.

Success/error toasts, loading indicators and confirmation dialogs are
shared across the pages; every destructive action asks first.

Development (two processes):

```sh
./jobwatch serve                 # API on 127.0.0.1:8080
cd web && npm install && npm run dev
# open http://127.0.0.1:5173 — Vite proxies /api to the Go server
```

The production build (`npm run build`) writes `web/dist`, which is
embedded by `go build -tags ui`.

## REST API

All endpoints are under `/api` and documented in
[docs/API.md](docs/API.md). Errors use
`{"error":{"code":"...","message":"..."}}`. `GET /api/health` is the
liveness check.

### Sources

Recognized URLs:

- Greenhouse: `boards.greenhouse.io/<token>`, `job-boards[.eu].greenhouse.io/<token>`
- Lever: `jobs.lever.co/<site>`, `jobs.eu.lever.co/<site>`
- Ashby: `jobs.ashbyhq.com/<org>`
- Personio: `<tenant>.jobs.personio.de`, `<tenant>.jobs.personio.com`
- Workable: `apply.workable.com/<account>`, `<account>.workable.com`

The source name is also the company name of its offers, unless the
board itself reports one (Greenhouse, Workable) or `company` is set in
the config.
Optional `--config` keys:

```sh
./jobwatch add-source https://jobs.lever.co/acme \
  --config '{"company":"Acme SAS","rate_limit_ms":5000}'
```

| Key | Meaning |
|---|---|
| `company` | Company name for offers of this source |
| `rate_limit_ms` | Minimum delay between requests to the host (default 2000; HTML sources never go below 5000) |
| `note` | Explanation shown on the sources page (used by the seeded disabled rows) |
| `board_token` / `site` + `region` / `org` | Greenhouse / Lever (`region: "eu"`) / Ashby board name, if it can't be derived from the URL |
| `tenant` + `language` | Personio tenant (if not in the URL) and optional feed language |
| `account` | Workable account slug (if not in the URL) |
| `item`, `title_field`, `link_field`, `description_field`, `location_field`, `company_field`, `uid_field` | RSS/Atom mapping. Paths are child element local names; `@name` reads an attribute and `#text` reads text. Defaults cover RSS `<item>` and Atom `<entry>` |
| `item_selector`, `title_selector`, `link_selector`, `link_attr`, `location_selector`, `description_selector`, `company_selector`, `uid_selector`, `uid_attr` | HTML listing page CSS selectors. `item_selector` and `title_selector` are required; only the listing page is fetched, at most every 5 s |

RSS/Atom and HTML sources need `--type`:

```sh
./jobwatch add-source https://weworkremotely.com/remote-jobs.rss \
  --type rss --name "We Work Remotely" \
  --config '{"location_field":"region","company_field":"category"}'

./jobwatch add-source https://careers.example.com/jobs \
  --type html --name "Example careers" \
  --config '{"item_selector":"li.job","title_selector":"h2 a",
             "location_selector":".location","description_selector":".summary"}'
```

`wttj` has no adapter. A fresh database also contains disabled rows for
Datadog/Cisco (Workday), SAP (SuccessFactors) and WTTJ with an
explanatory `note`; they are never scanned unless you enable them and
configure selectors.

JobWatch honors `robots.txt` and spaces its requests. A source that
refuses access is marked `blocked`; any other failure marks it `error`.
Neither stops the scan of the other sources.

### Manual import

```sh
# A bare URL: title/company/location/description are extracted from
# JSON-LD, Open Graph/meta tags or the page title.
./jobwatch import https://www.linkedin.com/jobs/view/123

# URL plus explicit fields (no page fetch), or --no-fetch to force this
./jobwatch import https://www.linkedin.com/jobs/view/123 \
  --company "Qonto" --title "SRE (H/F)" --location "Paris"

# JSON object or array, inline, from a file, or from stdin
./jobwatch import '{"company":"Qonto","title":"SRE","url":"https://example.com/job"}'
./jobwatch import offers.json
cat offers.json | ./jobwatch import -
```

JSON fields are `company` and `title` (required for raw JSON), `location`,
`url` and `description`. If the company and title match an existing
offer, the URL is added to that offer instead of creating a duplicate.
When a page cannot be read, the API returns the extracted fields to the
UI's manual fallback; the CLI asks for `--company`/`--title` (or
`--no-fetch`).

### Offer states

- `active`: seen in the latest scan.
- `changed`: the description changed (each version is kept as a snapshot;
  the detail page shows the added/removed lines between the two latest).
- `disappeared`: no longer listed on any source. Scans never delete offers.

### Classification

Every scan and import scores each offer deterministically (no LLM):
`title_confidence` asks whether the title is a Platform/SRE/DevOps/Infra
role, `location_confidence` whether it is full remote and
France-compatible, and `remote_policy` is derived from the location and
description. The keyword tables live in `internal/classify/tables.go`.
Offer lists can be filtered by both minimum scores. Run
`./jobwatch reclassify` once after upgrading an existing database to fill
older offers; `--all` recomputes after changing the tables.

### Backups

```sh
./jobwatch backup ~/backups/jobwatch-$(date +%F).db
```

This is safe while `serve` is running. The copy is reopened read-only and
checked (`PRAGMA integrity_check`, `foreign_key_check`, applied
migrations); a failed copy is removed and the command exits non-zero. The
backup is a normal SQLite file; point `--db` at it to use it.

### Smoke test

`go test ./internal/api -run TestSmokeFlows -v` runs the documented flow
(add source → scan a recorded Workable fixture through the real adapter →
classify → set application state → add a backdated timeline event). The
manual UI steps for a live source are in
[docs/SMOKE-TEST.md](docs/SMOKE-TEST.md).

## Development

```sh
go build ./... && go vet ./... && go test ./...
cd web && npm run build        # type-check + production bundle
```

Tests use recorded fixtures (`internal/sources/testdata`) and never hit
the network. The API integration tests use a temporary SQLite file per
handler group.

## Documentation

- [docs/SPEC.md](docs/SPEC.md): product specification (source of truth)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): components and data flow
- [docs/DATA-MODEL.md](docs/DATA-MODEL.md): tables, fingerprints, state machines
- [docs/API.md](docs/API.md): REST endpoints
- [docs/MILESTONES.md](docs/MILESTONES.md): progress, deviations, known gaps
- [docs/SMOKE-TEST.md](docs/SMOKE-TEST.md): automated and manual smoke-test steps
- [CHANGELOG.md](CHANGELOG.md)
