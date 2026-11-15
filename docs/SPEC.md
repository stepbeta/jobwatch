# JobWatch — Local Job Offer Tracker for Job Search

## 1. Overview

JobWatch is a local, single-user web application that tracks job offers
(aggregator sites and individual company career pages) for a person
actively looking for a full-remote **Platform Engineer / SRE** permanent
position in **France**.

It periodically (manually triggered) scans configured sources, finds new
offers, detects offers that disappeared or changed, classifies each offer
with a deterministic scoring engine, and lets the user track the full
application lifecycle (timeline, notes, states) per offer and per company.

## 2. Core principles & constraints

- **Single binary, zero external dependencies.** Go backend, SQLite
  database (pure-Go driver preferred, e.g. `modernc.org/sqlite`, for
  trivial cross-compilation; fall back to `mattn/go-sqlite3` if needed),
  frontend assets embedded in the binary via `go:embed`.
- Runs on WSL2 / Ubuntu 24.04. The web UI listens on localhost only
  (bind 127.0.0.1 by default). No authentication: it is a local-only tool.
- Database is a single SQLite file (WAL mode). Include a `backup` CLI
  command that copies the DB file safely.
- The HTTP server serves both the REST API and the embedded SPA.
- Respectful crawling only: honor `robots.txt`, rate-limit requests per
  source (default ~1 req/2s, configurable per source), set a realistic
  User-Agent, never attempt to bypass anti-bot systems. If a source
  blocks or fails repeatedly, mark it `error` and move on.
- Everything (CRUD on sources, offers, states, notes, scanning, import)
  must be achievable from the web UI. The CLI is a convenience layer,
  not a requirement.

## 3. CLI & server

One binary, subcommands:

- `jobwatch serve [--port 8080] [--db ./jobwatch.db]` — starts the server
- `jobwatch scan [all|<source-id>]` — runs scans headlessly
- `jobwatch add-source <url> [--name ...]` — registers a source
- `jobwatch list` — quick terminal listing of offers and states
- `jobwatch import <url-or-json>` — manually add an offer seen elsewhere
- `jobwatch backup <dest>`

Future-proofing: subcommand architecture (e.g. Cobra) so more commands
can be added later without refactoring.

## 4. Data model (SQLite)

```sql
companies (
  id INTEGER PK,
  name TEXT NOT NULL,
  name_normalized TEXT UNIQUE NOT NULL,   -- lowercased, punctuation stripped, legal suffixes removed
  website TEXT,
  created_at TEXT
)

sources (
  id INTEGER PK,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  source_type TEXT NOT NULL,      -- greenhouse | lever | ashby | personio | workable | wttj | rss | html | manual
  config_json TEXT DEFAULT '{}',  -- per-type config: board token, tenant, selectors, etc.
  enabled BOOLEAN DEFAULT true,
  last_scan_at TEXT,
  last_scan_status TEXT,          -- ok | error | blocked
  created_at TEXT
)

job_offers (
  id INTEGER PK,
  company_id INTEGER NOT NULL REFERENCES companies(id),
  title TEXT NOT NULL,
  title_normalized TEXT NOT NULL,
  location TEXT,                  -- free text as parsed
  remote_policy TEXT,             -- remote | hybrid | onsite | unknown
  url TEXT,                       -- canonical (first-seen) link
  fingerprint TEXT UNIQUE NOT NULL,  -- hash(company_normalized + title_normalized [+ location bucket])
  first_seen_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  offer_state TEXT NOT NULL DEFAULT 'active',  -- active | changed | disappeared
  application_state TEXT NOT NULL DEFAULT 'todo',
      -- todo | applied | responded | interview | offer | negative | maybe | positive
  raw_text TEXT,                  -- latest snapshot of the offer description
  text_hash TEXT,                 -- hash to detect changed offers
  title_confidence INTEGER,       -- 0..100, probability it's a platform/SRE role (from classifier)
  location_confidence INTEGER,    -- 0..100, probability it matches "full remote in France"
  match_reasons_json TEXT,        -- human-readable list explaining the scores
  created_at TEXT, updated_at TEXT
)

job_offer_provenance (           -- one row per (source, offer sighting)
  id INTEGER PK,
  offer_id INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
  source_id INTEGER NULL REFERENCES sources(id),  -- NULL for manual imports
  external_uid TEXT,              -- ATS posting id, when available
  url TEXT NOT NULL,              -- exact link where it was seen
  first_seen_at TEXT, last_seen_at TEXT,
  UNIQUE(offer_id, url)
)
-- The user may see the same offer under two different links (company site +
-- aggregator): the UI must surface all provenance rows so the user can open
-- both side by side and manually decide "same offer" vs "split into two".

events (                          -- timeline, per offer
  id INTEGER PK,
  offer_id INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
  type TEXT NOT NULL,             -- cv_sent | reply_received | interview | offer | rejected | ghosted | other
  description TEXT,               -- optional free text
  occurred_at TEXT NOT NULL,      -- user-editable (backdating is normal)
  created_at TEXT
)

notes (                           -- timestamp-less notes, on offers AND companies
  id INTEGER PK,
  entity_type TEXT NOT NULL,      -- offer | company
  entity_id INTEGER NOT NULL,
  body TEXT NOT NULL,
  pinned_event_id INTEGER NULL REFERENCES events(id) ON DELETE SET NULL,
      -- optional anchor to a timeline step ("recruiter mentioned X, review before next step")
  created_at TEXT, updated_at TEXT
)

snapshots (                       -- historical description snapshots
  id INTEGER PK,
  offer_id INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
  raw_text TEXT NOT NULL,
  captured_at TEXT NOT NULL
)
```

Enable SQLite FTS5 over `job_offers.title` and `job_offers.raw_text`,
plus over `notes.body`, for full-text search.

## 5. Deduplication & company matching

- Company resolution: normalize name (lowercase, strip punctuation and
  legal suffixes like GmbH/Inc/SAS/Ltd), then lookup in `companies`;
  create if missing. Exact normalized match first; if none, a conservative
  fuzzy match (normalized edit distance threshold) proposes a merge
  candidate but **never merges silently** — the UI shows "possible
  duplicate company" with a user-confirmable merge action.
- Offer identity: `fingerprint = hash(company_normalized + title_normalized)`.
  On collision (two genuinely different roles with identical titles at the
  same company), disambiguate with a location bucket; the UI must let the
  user manually merge two offers and manually split a wrongly merged one.
- Offer state transitions during scan:
  - seen again → update `last_seen_at`, provenance `last_seen_at`
  - `text_hash` changed → `offer_state = changed`, add a snapshot, and
    record what changed (diff summary) in the UI
  - not seen in a full scan of its source → `offer_state = disappeared`
    (do not delete; the user tracks the application anyway)

## 6. Classification engine (deterministic, hardcoded)

Goal: for every scanned offer, produce two confidence scores (0–100) and
human-readable reasons. No LLM in the scan loop.

- **Title confidence** (is it a Platform Eng / SRE / DevOps / Infra role?):
  weighted keyword sets over the normalized title.
  - strong positives: "platform engineer", "site reliability", "sre",
    "devops", "infrastructure engineer", "cloud engineer"
  - weak positives: "kubernetes", "reliability", "backend infrastructure"
  - strong negatives: "marketing", "sales", "recruiter", "internship",
    "account executive"
  → produce 0–100.
- **Location confidence** (full remote, France-compatible?): parse
  location strings + description keywords ("remote", "France", "Paris",
  "EMEA", "EU", time zones). Map to `remote_policy` and 0–100.
- Store both scores separately so the UI can filter independently on
  them ("show me everything with title ≥ 80 AND location ≥ 80").
- `match_reasons_json` stores the matched keywords/locations so the UI
  can explain any score. Hardcoded config lives in a single Go package
  (`internal/classify`) with keyword tables as constants — easy to tweak.
- No separate "needs review" queue. Scores are shown as small badges;
  filtering happens on the two score fields.

## 7. Sources & fetching architecture

Adapter pattern: one Go interface, one implementation per source type.

```go
type Fetcher interface {
    Fetch(ctx context.Context, src Source) ([]RawOffer, error)
}
type RawOffer struct {
    ExternalUID, Title, Location, URL, Description string
    Company string // from the source, may override the source's own company
}
```

Priority of implementations for v1:

1. `greenhouse`  — GET `https://boards-api.greenhouse.io/v1/boards/{token}/jobs?content=true`
2. `lever`       — GET `https://api.lever.co/v0/postings/{company}?mode=json`
3. `ashby`       — POST `https://api.ashbyhq.com/posting-api/job-board/{org}`
4. `personio`    — XML feed at `{tenant}.jobs.personio.de/xml`
5. `workable`    — public widget API
6. `rss`         — generic RSS/Atom
7. `html`        — last resort, CSS-selector config per source
8. `manual`      — no fetch; user-entered offers only

Notes on known sources:
- Datadog & Cisco run on Workday, SAP on SuccessFactors: skip in v1
  (complex POST APIs); leave the rows disabled with a comment.
- Welcome to the Jungle uses an internal API without public guarantees:
  implement cautiously behind rate limits, expect breakage.
- A source that fails N consecutive times is flagged `error` in the UI
  (visible indicator on the sources list) without blocking other sources.

Scan lifecycle (per source):
1. fetch → parse → RawOffer list
2. resolve company → upsert offer (dedup) → update provenance
3. classify (scores + reasons)
4. capture snapshot if new or changed
5. commit transaction; update `sources.last_scan_at/status`

Scans are manual only: a "Refresh" button per source on the UI, plus a
global "Refresh all". Optionally trigger via `jobwatch scan`.

## 8. REST API (JSON, under /api)

- CRUD `/api/sources`, `POST /api/sources/:id/scan`
- `GET /api/offers?filters...` (state, scores, remote policy, company,
  full-text query), `GET /api/offers/:id`
- `PATCH /api/offers/:id` (states, location edit, manual merge/split)
- CRUD `/api/offers/:id/events`, CRUD `/api/notes`
- CRUD `/api/companies`, `GET /api/companies/:id` (offers + notes)
- `POST /api/offers/import`, `POST /api/companies/:id/merge`
- Serve SPA from embedded FS for everything else.

## 9. Web UI (SPA, React + TypeScript, English)

Local-only dashboard. Sections:

- **Top bar**: global "Refresh all" + per-source refresh on the sources
  list; scan status/error indicators.
- **Applications section (top of dashboard)**: all offers with
  `application_state != todo`, sorted by most recent timeline activity —
  this is the "where did I send my CV" view.
- **Offers list**: filterable/sortable table. Filters on
  `application_state`, `offer_state`, `remote_policy`, `title_confidence`,
  `location_confidence`, full-text search. Each row shows small score
  badges with tooltip explanations (match_reasons) — no review queue.
- **Offer detail page**:
  - header: company, title, states (editable dropdowns), scores
  - provenance block: ALL links where this offer was seen, side by side,
    so the user can open them and decide whether they are really the
    same offer (with "split offer" action)
  - left column: vertical **timeline** of events, add/backdate/delete
  - right column: stacked **notes**, each optionally pinned to a
    timeline event
  - full description text with snapshots/diff view when `changed`
- **Company detail page**: all its offers + company-level notes +
  possible-duplicate-company merge prompt.
- **Sources page**: list with type, last scan status/time, enable/disable,
  add/edit (config form adapts to source_type).

Design goals: dense, fast, keyboard-friendly; no framework ceremony —
plain React + a lightweight table component is fine.

## 10. Out of scope (v1)

- Scheduled/automatic scans, notifications
- CV version tracking
- Anti-bot evasion, proxy rotation, headless browsers
- Multi-user, auth, deployment anywhere but localhost
- Auto-merging companies or offers without explicit user confirmation