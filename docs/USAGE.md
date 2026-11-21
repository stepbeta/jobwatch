# Usage

Practical reference for running JobWatch: sources and their configuration,
the web UI, manual import, states, classification, backups and the CLI.
The [README](../README.md) covers what the project is and how to build it;
the endpoint reference is [API.md](API.md) and the product specification is
[SPEC.md](SPEC.md).

## Sources

A fresh database already contains the 71 sources decided in Milestones
5a/5b/6a/10a: 39 enabled ATS/RSS/HTML/WTTJ sources and 32 disabled rows
with a `note` explaining why they are not tracked. Add your own with
`add-source` or register a bookmark list with `add-sources`. Existing
rows are never overwritten by the seed migrations, so local edits
survive upgrades.

The source name is not used as a company fallback. A single-employer
source must declare `company` in its config unless the adapter reports
one (Greenhouse, Workable, WTTJ, or an RSS/HTML `company_field` /
`company_selector`). Migration 0013 sets `company` to the current source
name on the enabled single-employer sources; We Work Remotely lists many
employers and is configured by the RSS/HTML employer extraction instead.

### Supported source types

| Type | What it reads |
|---|---|
| `greenhouse` | Public board API `boards-api.greenhouse.io/v1/boards/{token}/jobs?content=true` |
| `lever` | Public postings API `api.lever.co/v0/postings/{site}?mode=json` (`api.eu.lever.co` when `region` is `eu`) |
| `ashby` | Public posting API `api.ashbyhq.com/posting-api/job-board/{org}` |
| `personio` | XML feed at `{tenant}.jobs.personio.de/xml` (or `.com`) |
| `workable` | Keyless widget API `apply.workable.com/api/v1/widget/accounts/{account}?details=true` |
| `bamboohr` | Public JSON listing `{subdomain}.bamboohr.com/careers/list` |
| `rss` | An RSS 2.0 or Atom feed, with an optional field mapping |
| `html` | A server-rendered listing page, with CSS selectors |
| `wttj` | Welcome to the Jungle company careers page: the page bootstrap is read for the Algolia index and credentials, then the index is queried |
| `manual` | No source. Offers enter through manual import (`source_id` is NULL) |

### Recognized board URLs

`add-source` and `add-sources` detect these URL shapes automatically:

- Greenhouse: `boards.greenhouse.io/<token>`, `job-boards[.eu].greenhouse.io/<token>`
- Lever: `jobs.lever.co/<site>`, `jobs.eu.lever.co/<site>`
- Ashby: `jobs.ashbyhq.com/<org>`
- Personio: `<tenant>.jobs.personio.de`, `<tenant>.jobs.personio.com`
- Workable: `apply.workable.com/<account>`, `<account>.workable.com`
- BambooHR: `<subdomain>.bamboohr.com/careers` (the `/careers/list`
  JSON endpoint is read; the board carries no descriptions)
- Welcome to the Jungle: `www.welcometothejungle.com/<lang>/companies/<slug>/jobs`
  (any path containing `/companies/`; the marketplace pages such as
  `/en/jobs` or `/fr/jobs-matches` are not company pages)

Some public API URLs are recognized too, for example
`boards-api.greenhouse.io/v1/boards/<token>` and
`api.lever.co/v0/postings/<site>`. RSS/Atom and HTML sources need
`--type`; RSS has a default mapping and accepts `--config`, while HTML
requires `--config` with at least `item_selector` and `title_selector`.

### Welcome to the Jungle company pages

Paste a WTTJ company careers page as the source URL and give it the
company name; nothing has to be extracted from the browser:

```sh
./jobwatch add-source https://www.welcometothejungle.com/en/companies/fabriq/jobs --name Fabriq
```

Each scan reads the page bootstrap once to discover the Algolia index,
its filter groups and its secured credentials, then pages through the
index (50 hits per request). The credentials rotate server-side and are
re-read on every scan, so there is never a key to paste or refresh. Two
debug-only config keys, `algolia_app_id` and `algolia_api_key`, override
the discovered credentials; use them only to reproduce a failure.

A page can list jobs from several linked organizations (for example a
local brand and its parent). The scan queries every organization the page
declares and links each offer to the organization that owns it, so an
offer URL may name a different company slug than the source URL.

Organizations renamed on WTTJ keep working: when the page's filter names a
slug the index no longer uses, an empty first page makes the scan read the
canonical slug from the organization record and retry once (the `swile`
page finds the jobs indexed under `lunchr`).

The flow, its rationale and its known gaps are documented in
[SOURCE-DECISIONS.md](SOURCE-DECISIONS.md#welcome-to-the-jungle-milestone-10a).

### Config keys

Optional `--config` keys:

```sh
./jobwatch add-source https://jobs.lever.co/acme \
  --config '{"company":"Acme SAS","rate_limit_ms":5000}'
```

| Key | Meaning |
|---|---|
| `company` | Company name for offers of a single-employer source; required when the adapter reports no company. Migration 0013 sets it on the enabled seeded single-employer sources |
| `rate_limit_ms` | Minimum delay between requests to the host (default 2000; HTML sources never go below 5000) |
| `note` | Explanation shown on the sources page (used by the seeded disabled rows) |
| `board_token` / `site` + `region` / `org` | Greenhouse / Lever (`region: "eu"`) / Ashby board name, if it can't be derived from the URL |
| `tenant` + `language` | Personio tenant (if not in the URL) and optional feed language |
| `account` | Workable account slug (if not in the URL) |
| `subdomain` | BambooHR tenant subdomain (if not in the URL) |
| `url` | Optional careers page override (WTTJ); defaults to the source URL |
| `algolia_app_id`, `algolia_api_key` | WTTJ debug-only overrides of the credentials discovered from the page bootstrap |
| `item`, `title_field`, `link_field`, `description_field`, `location_field`, `company_field`, `uid_field` | RSS/Atom mapping. Paths are child element local names joined with `/` (e.g. `locations/location/name`); `@name` reads an attribute and `#text` reads text. Defaults cover RSS `<item>` and Atom `<entry>` |
| `employer` | Declarative employer rule for RSS/HTML sources that list several employers. `{"separator":":"}` splits `Employer: Role`; `field` (RSS) or `selector` (HTML) reads the employer from a separate element and strips the repeated prefix from the title |
| `item_selector`, `title_selector`, `link_selector`, `link_attr`, `location_selector`, `description_selector`, `company_selector`, `uid_selector`, `uid_attr` | HTML listing page CSS selectors. `item_selector` and `title_selector` are required; only the listing page is fetched, at most every 5 s. Selectors use cascadia's CSS support, including `:has()` and `:haschild()` for item containers |

### Examples

RSS/Atom and HTML sources need `--type`:

```sh
./jobwatch add-source https://weworkremotely.com/remote-jobs.rss \
  --type rss --name "We Work Remotely" \
  --config '{"location_field":"region","employer":{"separator":":"}}'

./jobwatch add-source https://careers.example.com/jobs \
  --type html --name "Example careers" \
  --config '{"item_selector":"li.job","title_selector":"h2 a",
             "location_selector":".location","description_selector":".summary"}'

# Teamtailor feeds nest the location under tt:locations/tt:location;
# paths use local names, so the namespace prefix is omitted.
./jobwatch add-source https://careers.example.com/jobs.rss \
  --type rss --name "Example" \
  --config '{"location_field":"locations/location/name"}'

# Item containers can be selected with :has()/:haschild() when the
# listing has no stable class on the row itself.
./jobwatch add-source https://careers.example.com/jobs \
  --type html --name "Example careers" \
  --config '{"item_selector":"li:haschild(a[href^=\"/jobs/\"])",
             "title_selector":"a[href^=\"/jobs/\"] h2",
             "link_selector":"a[href^=\"/jobs/\"]"}'
```

A fresh database also contains disabled rows for Datadog/Cisco
(Workday), SAP (SuccessFactors) and the WTTJ marketplace pages with an
explanatory `note`; they are never scanned unless you enable them and
configure selectors.

### Bulk source registration

`jobwatch add-sources <file>` reads one URL per line and runs every URL
through the same detection and validation as `add-source`:

```sh
./jobwatch add-sources docs/REMOTE-SOURCES.txt
```

- Blank lines and lines starting with `#` are ignored. A `#` preceded by
  whitespace starts an inline comment (`... # note`); a `#` inside the
  URL is kept as a fragment (`https://example.com/jobs#openings`).
- Duplicate URLs in the file are refused (the first occurrence wins)
  and the remaining lines are still processed.
- A URL that is already registered, a duplicate line, an invalid URL or
  a page without a known ATS adapter never aborts the run. The final
  report has three groups: **added**, **already registered** and **not
  detected**. The not-detected URLs are also printed as a plain list at
  the end.
- `--disabled` registers the new sources disabled, like `add-source`.
- Only URLs of supported ATS boards (Greenhouse, Lever, Ashby, Personio,
  Workable, BambooHR) are detected. Plain career pages need
  `add-source --type html` or `--type rss` plus selectors/mapping.

The command exits successfully as long as the file can be read and the
database does not fail; the report is the result, not an error.

### Source decisions

The 57 URLs that `add-sources` could not detect were investigated one
polite fetch at a time. The outcome is recorded per URL in
[SOURCE-DECISIONS.md](SOURCE-DECISIONS.md): 19 underlying ATS boards, 5
RSS feeds, 5 server-rendered HTML listings, and 28 disabled sources with
an explanatory `note` (JS-only SPAs, Notion, candidate logins,
Workday/SuccessFactors/iCIMS/Pinpoint/Zoho, WTTJ marketplace pages,
robots.txt disallows, aggregators). Disabled sources are never scanned; their offers enter
through manual import. The Workable public search is one of them: its
France+remote query URL is disallowed by robots.txt, and the allowed
category pages are incomplete, so the per-company Workable boards remain
the supported surface.

### Polite fetching and source status

JobWatch honors `robots.txt` and spaces its requests. A source that
refuses access is marked `blocked`; any other failure marks it `error`.
Neither stops the scan of the other sources. A successful scan sets
`last_scan_status = ok`. Sources without an adapter are reported as
`skipped` and keep their previous status.

## Web UI

Built with React + TypeScript (Vite). The pages are:

- **Dashboard** (`#/`) — the Applications section (offers whose
  application state is not `todo`, most recent timeline activity first)
  and the Saved section (bookmarked offers, most recently saved first).
  Applications has a Refresh button that scans only the enabled sources
  behind those applications: sources with only `todo` offers are not
  scanned, and manual imports (`source_id` NULL) contribute no source.
  The button fans out over the existing per-source scan endpoint
  sequentially and reports per-source failures plus an aggregate summary.
  Saved shows every bookmarked offer regardless of application state;
  its "Show all" link opens the offers page with the saved filter preset.
- **Offers** (`#/offers`) — the full table with filters (application/offer
  state, remote policy, saved/unsaved, score minima, a searchable company
  combobox, and debounced full-text search), a column picker, header-click
  sorting, a per-page selector and pagination. Every row has a star
  bookmark toggle, and the header **Add offer** button opens a modal import
  form (URL fetch-and-prefill or manual fields). A successful import closes
  the modal, shows the success toast, refreshes the data and opens the new
  or matched offer's detail page. Score badges show 0–100 title/location
  confidences; hover one to see the matched keywords from the classifier.
- **Incomplete** (`#/incomplete`) — items a source could not normalize,
  with the item count, the source-reported company/title/location, the
  reason and source link. **Repair** opens a form that assigns a company
  and title (and optionally a location) while keeping the provenance,
  snapshots and notes. The top navigation shows the incomplete count and
  refreshes after scans and repairs.
- **Sources** — enable/disable with a row-end toggle switch, scan status,
  filters (enabled/disabled/all, type, scan status, debounced name search),
  a column picker, client-side header sorting, and a per-page selector.
  Selecting rows on the left enables **Delete selected** (one shared
  confirmation listing the names; offers and provenance links are kept and
  their `source_id` is detached) and **Refresh selected** (sequential scans
  through the same batch helper as Refresh all). Per-row Refresh/Edit/Delete
  are icon buttons with accessible labels. A source name links to its
  company page when a company with the same normalized name exists.
  Add/edit uses a modal whose config fields depend on the source type
  (Personio tenant, Workable account, BambooHR subdomain, RSS field
  mapping, HTML CSS selectors). Disabled seeded rows for the known-hard
  Workday/SuccessFactors rows, the WTTJ marketplace rows and the
  not-trackable sources show their explanation; the seeded Fabriq WTTJ
  company page is enabled and scannable.
- **Offer detail** — a two-column header panel: identity (company, title,
  editable application/offer state and location, score badges, bookmark,
  description version count, Merge/Split actions) on the left and an
  always-visible compact provenance list (links, source names, external
  UID, first/last seen) on the right.
  Below it: a vertical timeline with backdating, notes optionally pinned to
  an event, and a single-version description browser (prev/next, version
  dropdown, Scan/Manual origin badge, diff against the previous version)
  whose Add/Edit action uses the shared Edit/View editor.
- **Company detail** — one two-column header block: identity (name,
  website, offer count, created time, attested keys, Edit/Delete company)
  and the dismissible possible-duplicate banner on the left; company notes
  (list, add, delete) on the right. The columns stack at 900px. The offers
  panel below has its own debounced search, header-click sorting
  (server-side `company_id` query), per-page selector and pagination.
  Merging always asks which company row to keep; nothing is merged
  automatically.
- **Interview prep** (`#/interview-prep`) — a personal FAQ of anticipated
  interview questions and prepared answers. Entries are grouped into one
  section per category, sorted alphabetically with "Uncategorized" last; a
  small amber "No answer" badge marks entries still waiting for a prepared
  answer. Drag an entry within its section to reorder it, or onto another
  section to change its category; the new order/category is saved through
  the API and survives a reload. Drag a section header to reorder the
  sections themselves — that order is only a browser preference stored in
  `localStorage` and restored on load, and "Reset to alphabetical" clears
  it. Questions form an accordion; expanding one renders its answer. Add
  and Edit share a modal with a question input, a category dropdown
  (existing names plus "Add new category…"), and one large markdown
  editor with an Edit/View toggle; Delete asks for confirmation. Duplicate
  questions are allowed. Answers are stored as markdown plaintext and
  rendered by a small built-in renderer (headings, bold, italic, inline
  code, fenced code blocks and lists) that never injects HTML, so raw HTML
  is displayed literally.

The table pages keep their view in the hash-route query string.
`#/offers`, `#/sources` and `#/companies/:id` restore active filters,
search, `sort`, `order`, `page` and `per_page` on load and rewrite the
URL as the user changes them. Unknown or invalid values fall back to the
defaults, and clearing filters removes their parameters. Examples:
`#/offers?q=sre&remote_policy=remote&saved=true&sort=title&order=asc&page=2&per_page=20`
and `#/sources?enabled=disabled&status=error&q=board`. The existing
`#/offers?saved=true` preset still works. Page-size defaults are 50 for
the Offers page, 20 for the Sources page and 20 for the company-page
offers panel; the choice is also remembered in `localStorage`
(`offers.perPage`, `sources.perPage`, `companyOffers.perPage`). Column
pickers store the hidden column names in `offers.columns` and
`sources.columns`, so newly added columns stay visible for existing
users, and the last visible column cannot be hidden.

The bookmark is personal interest, not an application state. Saving an
offer never changes its application or offer state, and if a saved offer
later becomes an application it stays in Saved until you un-bookmark it.

Success/error toasts, loading indicators and confirmation dialogs are
shared across the pages; every destructive action asks first. The top bar
only navigates; scan actions live on the Sources page and in the
Dashboard Applications section.

For the two-process development setup, see the Development section of the
[README](../README.md#development).

## Manual import

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
`url` and `description`. If the URL already belongs to an offer, the
import attaches to that offer instead of creating a duplicate; a new URL
creates a new offer even when the company and title match another one.
When a page cannot be read, the API returns the extracted fields to the
UI's manual fallback; the CLI asks for `--company`/`--title` (or
`--no-fetch`).

Aggregator and candidate-only pages are deliberately not scraped:
FlexJobs, RemoteMore, the `my.greenhouse.io` dashboard, The Product
Crew, Welcome to the Jungle matches and Jobgether either require a
candidate account or render matches client-side. When you find an offer
there, copy its public URL and paste it into **Offers → Add offer**
(click **Fetch & import**), or run `jobwatch import <url>`; if the page
is login-gated or has no usable metadata, enter company and title
manually in the same form or with `--company`/`--title`. If the offer
also exists on a company ATS board,
register that board instead so future scans track it automatically. See
[SOURCE-DECISIONS.md](SOURCE-DECISIONS.md#aggregators-milestone-5c).

## Offer and application states

### Offer states

- `active`: seen in the latest scan.
- `disappeared`: no longer listed on any source. Scans never delete offers.

A description edit is not an offer state. Every distinct version is kept
as a snapshot with its scan/manual origin; the detail page browses the
versions and shows the added/removed lines against the previous one.

The full transition table, including reappearance and multi-source
behavior, is in [DATA-MODEL.md](DATA-MODEL.md#offer_state-written-by-scans-and-imports).

### Application states

`todo` is the default. The other values are `applied`, `responded`,
`interview`, `offer`, `negative`, `maybe` and `positive`. Transitions are
user-driven from the UI or `PATCH /api/offers/:id`; no automatic
transition is derived from timeline events. See
[DATA-MODEL.md](DATA-MODEL.md#application_state).

## Classification

Every scan and import scores each offer deterministically (no LLM):
`title_confidence` asks whether the title is a Platform/SRE/DevOps/Infra
role, `location_confidence` whether it is full remote and
France-compatible, and `remote_policy` is derived from the location and
description. The keyword tables live in `internal/classify/tables.go`.
Offer lists can be filtered by both minimum scores. Run
`./jobwatch reclassify` once after upgrading an existing database to fill
older offers; `--all` recomputes after changing the tables.

The scoring rules, caps and reason lists are described in
[ARCHITECTURE.md](ARCHITECTURE.md#classification-internalclassify).

## Re-attribution

Scans never rewrite the company or title of an existing offer, so an
adapter that starts extracting an employer or a canonical title does not
change offers recorded before the fix. `reattribute` corrects them
explicitly:

```sh
./jobwatch reattribute --source 36   # one source by ID
./jobwatch reattribute --all         # every enabled source (the default)
```

It re-fetches the source, matches stored offers by `(source, uid)` and
writes the adapter's current company and title. Events, notes, snapshots,
application/listing states and `last_seen_at` are preserved; the
classifier is re-run for the new title. The command is idempotent, skips
user-repaired offers and offers with no current uid match, and reports
per source how many offers moved, stayed unchanged or were skipped, plus
any company left without offers. An emptied company stays in the
database for you to delete.

## Backups

```sh
./jobwatch backup ~/backups/jobwatch-$(date +%F).db
```

This is safe while `serve` is running. The copy is reopened read-only and
checked (`PRAGMA integrity_check`, `foreign_key_check`, applied
migrations); a failed copy is removed and the command exits non-zero. The
backup is a normal SQLite file; point `--db` at it to use it.

## Command reference

| Command | Description |
|---|---|
| `serve [--port 8080] [--addr 127.0.0.1]` | Start the local server: REST API under `/api` and the SPA at `/` |
| `scan [all\|<source-id>]` | Scan all enabled sources (default) or one source by ID. Exits non-zero if a source failed |
| `add-source <url> [--name N] [--type T] [--config JSON] [--disabled]` | Register a source |
| `add-sources <file> [--disabled]` | Register every recognized URL in a file; reports added / already registered / not detected |
| `list [--offer-state S] [--app-state S] [--limit N] [--sources]` | List offers (most recently seen first) or sources |
| `import <url \| json \| file.json \| ->` | Add an offer seen elsewhere. A bare URL is fetched for JSON-LD/Open Graph/title metadata (`--no-fetch` disables); JSON fields can be supplied manually |
| `reclassify [--all]` | Backfill title/location scores and remote policies with the deterministic classifier; `--all` recomputes every offer |
| `reattribute [--all \| --source <id>]` | Re-fetch a source and move offers matched by `(source, uid)` to the company and title the adapter now reports, preserving history; `--source` accepts a disabled source |
| `backup <dest>` | Write and validate a consistent copy of the database (must not exist) |

All commands accept the global `--db <path>` flag (default
`./jobwatch.db`). Run `./jobwatch <command> --help` for the exact flags.

### Command examples

```sh
# A fresh database already seeds the 71 sources decided in Milestones
# 5a/5b/6a/10a (39 enabled ATS/RSS/HTML/WTTJ, 32 disabled with a note).
# Add extras:
./jobwatch add-source https://boards.greenhouse.io/gitlab --name GitLab
./jobwatch add-source https://jobs.lever.co/spotify --name Spotify
./jobwatch add-source https://jobs.ashbyhq.com/linear --name Linear
./jobwatch add-source https://acme.jobs.personio.de --name Acme
./jobwatch add-source https://apply.workable.com/acme --name Acme
./jobwatch add-source https://acme.bamboohr.com/careers --name Acme
./jobwatch add-source https://www.welcometothejungle.com/en/companies/fabriq/jobs --name Fabriq

# or register every recognized URL from another bookmark list
./jobwatch add-sources docs/REMOTE-SOURCES.txt

# Start the web UI (or scan headlessly with ./jobwatch scan)
./jobwatch serve
# → http://127.0.0.1:8080

# Headless usage still works
./jobwatch scan
./jobwatch list                  # ID, company, title, location, remote, T/L scores, states
./jobwatch list --offer-state disappeared
./jobwatch list --sources        # includes notes on disabled seeded sources
./jobwatch reclassify            # backfill scores of older offers
./jobwatch reclassify --all      # recompute every score after keyword changes
./jobwatch reattribute --source 36   # move old We Work Remotely offers to their employers
./jobwatch reattribute --all         # reattribute every enabled source
./jobwatch backup ~/jobwatch-$(date +%F).db   # validated copy
```

### Command flags

| Command | Flags |
|---|---|
| `serve` | `--port` (default 8080), `--addr` (default `127.0.0.1`) |
| `scan` | none; argument `all` or a numeric source ID |
| `add-source` | `--name`, `--type`, `--config` (JSON object), `--disabled` |
| `add-sources` | `--disabled` |
| `list` | `--offer-state`, `--app-state`, `--limit` (default 100; `0` = no limit), `--sources` |
| `import` | `--company`, `--title`, `--location`, `--description`, `--no-fetch` |
| `reclassify` | `--all` |
| `reattribute` | `--all` or `--source <id>`; no argument |
| `backup` | none; destination must not exist |

Field flags cannot be combined with a JSON array. `scan` exits non-zero
when at least one source failed. `add-sources` exits zero when some URLs
were not detected; file and database errors still fail.

## Smoke test

`go test ./internal/api -run TestSmokeFlows -v` runs the documented flow
(add source → scan a recorded Workable fixture through the real adapter →
classify → set application state → add a backdated timeline event). The
manual UI steps for a live source are in
[SMOKE-TEST.md](SMOKE-TEST.md).

## See also

- [README](../README.md): what JobWatch is, requirements, build and quick start
- [SPEC.md](SPEC.md): product specification and revisions
- [ARCHITECTURE.md](ARCHITECTURE.md): components, adapters, scan pipeline, classifier internals
- [DATA-MODEL.md](DATA-MODEL.md): tables, fingerprints, state machines
- [API.md](API.md): REST endpoints, parameters and errors
- [MILESTONES.md](MILESTONES.md): progress, deviations and known gaps
- [SMOKE-TEST.md](SMOKE-TEST.md): automated and manual smoke-test steps
- [SOURCE-DECISIONS.md](SOURCE-DECISIONS.md): per-URL source decisions
