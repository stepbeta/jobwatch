# jobwatch

JobWatch is a local, single-user tool that tracks job offers from
company job boards (Greenhouse, Lever, Ashby), detects new, changed and
disappeared offers, and keeps everything in a single SQLite file.

Status: Milestone 3 (classification, offer/company dedup UX, changed-offer
diff). The Go server exposes the full API and, when built with the `ui`
tag, embeds the React SPA. See [docs/MILESTONES.md](docs/MILESTONES.md)
for what is done and the known gaps.

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

# 2. Start the web UI (or scan headlessly with ./jobwatch scan)
./jobwatch serve
# → http://127.0.0.1:8080

# 3. Headless usage still works
./jobwatch scan
./jobwatch list
./jobwatch list --offer-state changed
./jobwatch list --sources
./jobwatch reclassify            # backfill scores of older offers
./jobwatch reclassify --all      # recompute every score after keyword changes
```

## Commands

| Command | Description |
|---|---|
| `serve [--port 8080] [--addr 127.0.0.1]` | Start the local server: REST API under `/api` and the SPA at `/` |
| `scan [all\|<source-id>]` | Scan all enabled sources (default) or one source by ID. Exits non-zero if a source failed |
| `add-source <url> [--name N] [--type T] [--config JSON] [--disabled]` | Register a source |
| `list [--offer-state S] [--app-state S] [--limit N] [--sources]` | List offers (most recently seen first) or sources |
| `import <url \| json \| file.json \| ->` | Add an offer seen elsewhere, without fetching anything |
| `reclassify [--all]` | Backfill title/location scores and remote policies with the deterministic classifier; `--all` recomputes every offer |
| `backup <dest>` | Write a consistent copy of the database to `dest` (must not exist) |

Run `./jobwatch <command> --help` for details.

## Web UI

Built with React + TypeScript (Vite). The pages are:

- **Dashboard** — the Applications section on top (offers whose
  application state is not `todo`, most recent timeline activity first),
  then the offers table with filters (application/offer state, remote
  policy, score minima, company, full-text search), sorting and
  pagination. Score badges show 0–100 title/location confidences; hover
  one to see the matched keywords from the classifier.
- **Sources** — enable/disable, scan status, per-source Refresh, a
  global Refresh all in the top bar, and add/edit forms whose config
  fields depend on the source type.
- **Offer detail** — editable application/offer state and location,
  score badges, all provenance links side by side, explicit merge into
  another offer and split (select links for a second offer), a vertical
  timeline with backdating, notes optionally pinned to an event, and the
  latest description plus a removed/added line diff for `changed` offers.
- **Company detail** — offers, company notes, rename/website edit, a
  dismissible possible-duplicate banner and a merge dialog that lets you
  choose which company row to keep. Nothing is merged automatically.

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

The source name is also the company name of its offers, unless the
board itself reports one (Greenhouse) or `company` is set in the config.
Optional `--config` keys:

```sh
./jobwatch add-source https://jobs.lever.co/acme \
  --config '{"company":"Acme SAS","rate_limit_ms":5000}'
```

| Key | Meaning |
|---|---|
| `company` | Company name for offers of this source |
| `rate_limit_ms` | Minimum delay between requests to the host (default 2000) |
| `board_token` / `site` + `region` / `org` | Greenhouse / Lever (`region: "eu"`) / Ashby board name, if it can't be derived from the URL |

Other source types (`personio`, `workable`, `wttj`, `rss`, `html`) can be
registered with `--type`, but `scan` skips them until their adapters exist.

JobWatch honors `robots.txt` and spaces its requests. A source that
refuses access is marked `blocked`; any other failure marks it `error`.
Neither stops the scan of the other sources.

### Manual import

```sh
# URL plus fields
./jobwatch import https://www.linkedin.com/jobs/view/123 \
  --company "Qonto" --title "SRE (H/F)" --location "Paris"

# JSON object or array, inline, from a file, or from stdin
./jobwatch import '{"company":"Qonto","title":"SRE","url":"https://example.com/job"}'
./jobwatch import offers.json
cat offers.json | ./jobwatch import -
```

JSON fields are `company`, `title` (both required), `location`, `url` and
`description`. If the company and title match an existing offer, the URL
is added to that offer instead of creating a duplicate.

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

This is safe while `serve` is running. The backup is a normal SQLite file;
point `--db` at it to use it.

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
- [CHANGELOG.md](CHANGELOG.md)
