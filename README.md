# jobwatch

JobWatch is a local, single-user tool that tracks job offers from
company job boards (Greenhouse, Lever, Ashby), detects new, changed and
disappeared offers, and keeps everything in a single SQLite file.

Status: Milestone 1 (core + CLI). There is no web UI yet; `serve` only
exposes a health endpoint. See [docs/MILESTONES.md](docs/MILESTONES.md)
for what is done and the known gaps.

## Requirements

- Go 1.27 or newer. No other dependencies: SQLite is a pure-Go driver
  compiled into the binary.
- Linux (developed on WSL2 / Ubuntu 24.04); other platforms should work.

## Build

```sh
go build -o jobwatch ./cmd/jobwatch
# optional: embed a version string
go build -ldflags "-X main.version=0.1.0" -o jobwatch ./cmd/jobwatch
```

## Quick start

The database defaults to `./jobwatch.db` and is created on first use.
Use the global `--db <path>` flag to choose another file.

```sh
# 1. Register sources (type and board name are detected from the URL)
./jobwatch add-source https://boards.greenhouse.io/gitlab --name GitLab
./jobwatch add-source https://jobs.lever.co/spotify --name Spotify
./jobwatch add-source https://jobs.ashbyhq.com/linear --name Linear

# 2. Scan all enabled sources (or one: ./jobwatch scan 2)
./jobwatch scan

# 3. Look at the results
./jobwatch list
./jobwatch list --offer-state changed
./jobwatch list --sources
```

## Commands

| Command | Description |
|---|---|
| `serve [--port 8080] [--addr 127.0.0.1]` | Start the local server. Only `GET /api/health` exists so far |
| `scan [all\|<source-id>]` | Scan all enabled sources (default) or one source by ID. Exits non-zero if a source failed |
| `add-source <url> [--name N] [--type T] [--config JSON] [--disabled]` | Register a source |
| `list [--offer-state S] [--app-state S] [--limit N] [--sources]` | List offers (most recently seen first) or sources |
| `import <url \| json \| file.json \| ->` | Add an offer seen elsewhere, without fetching anything |
| `backup <dest>` | Write a consistent copy of the database to `dest` (must not exist) |

Run `./jobwatch <command> --help` for details.

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
- `changed`: the description changed (each version is kept as a snapshot).
- `disappeared`: no longer listed on any source. Offers are never deleted.

### Backups

```sh
./jobwatch backup ~/backups/jobwatch-$(date +%F).db
```

This is safe while `serve` is running. The backup is a normal SQLite file;
point `--db` at it to use it.

## Development

```sh
go build ./... && go vet ./... && go test ./...
```

Tests use recorded fixtures (`internal/sources/testdata`) and never hit
the network.

## Documentation

- [docs/SPEC.md](docs/SPEC.md): product specification (source of truth)
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): components and data flow
- [docs/DATA-MODEL.md](docs/DATA-MODEL.md): tables, fingerprints, state machines
- [docs/API.md](docs/API.md): REST endpoints
- [docs/MILESTONES.md](docs/MILESTONES.md): progress, deviations, known gaps
- [CHANGELOG.md](CHANGELOG.md)
