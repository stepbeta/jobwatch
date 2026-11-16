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
  source (default ~1 req/2s, configurable per source; enforced per host,
  so sources sharing an ATS host share the limit, and a larger robots
  `Crawl-delay` wins), set a realistic
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
  (a bare URL is fetched for lightweight metadata unless `--no-fetch`)
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
  source_type TEXT NOT NULL,      -- greenhouse | lever | ashby | personio | workable | bamboohr | wttj | rss | html | manual
  config_json TEXT NOT NULL DEFAULT '{}',  -- per-type config: board token, tenant, selectors, etc.
  enabled INTEGER NOT NULL DEFAULT 1,      -- boolean (SQLite has no BOOLEAN type)
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
  remote_policy TEXT NOT NULL DEFAULT 'unknown',  -- remote | hybrid | onsite | unknown
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
  type TEXT NOT NULL,             -- cv_sent | message_received | message_sent | interview | offer | rejected | ghosted | other
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
  origin TEXT NOT NULL DEFAULT 'scan',  -- scan | manual
  raw_text TEXT NOT NULL,
  captured_at TEXT NOT NULL
)
```

Enable SQLite FTS5 over `job_offers.title` and `job_offers.raw_text`,
plus over `notes.body`, for full-text search.

Enum-like TEXT columns are validated in application code, not with
CHECK constraints, so that adding a value never requires a table
rebuild (migrations stay additive). `docs/DATA-MODEL.md` is the
column-level reference for the implemented schema.

## 5. Deduplication & company matching

- Company resolution: normalize name (lowercase, strip punctuation and
  legal suffixes like GmbH/Inc/SAS/Ltd), then lookup in `companies`;
  create if missing. Exact normalized match first; if none, a conservative
  fuzzy match (normalized edit distance threshold) proposes a merge
  candidate but **never merges silently** — the UI shows "possible
  duplicate company" with a user-confirmable merge action.
- Offer identity: `fingerprint = hash(company_normalized + title_normalized)`
  (SHA-256 hex, fields joined with a 0x1F separator).
  On collision (two genuinely different roles with identical titles at the
  same company), disambiguate with a location bucket: a sighting whose
  location bucket differs from the stored offer's (both known) uses
  `hash(company + title + bucket)` instead. The bucket rule is defined in
  `docs/DATA-MODEL.md`. The UI must let the
  user manually merge two offers and manually split a wrongly merged one.
- Offer state transitions during scan:
  - seen again → update `last_seen_at`, provenance `last_seen_at`
  - `text_hash` changed → `offer_state = changed`, add a snapshot, and
    record what changed (diff summary) in the UI
  - not seen in a full scan of its source → `offer_state = disappeared`
    (do not delete; the user tracks the application anyway). An offer
    listed on several sources disappears only when no other source saw
    it in that source's latest successful scan. Failed scans change no
    offer.
  - `disappeared` offer seen again → `offer_state = active`
  - a `changed` offer stays `changed` on later unchanged sightings until
    the user acknowledges it

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
3. `ashby`       — GET `https://api.ashbyhq.com/posting-api/job-board/{org}`
4. `personio`    — XML feed at `{tenant}.jobs.personio.de/xml`
5. `workable`    — public widget API
6. `bamboohr`    — JSON listing at `{subdomain}.bamboohr.com/careers/list`
7. `rss`         — generic RSS/Atom
8. `html`        — last resort, CSS-selector config per source
9. `manual`      — no fetch; user-entered offers only

Notes on known sources:
- Datadog & Cisco run on Workday, SAP on SuccessFactors: skip in v1
  (complex POST APIs); leave the rows disabled with a comment.
- Welcome to the Jungle uses an internal API without public guarantees:
  implement cautiously behind rate limits, expect breakage. v1 leaves a
  disabled source row with an explanation instead of shipping a scraper.
- A source that fails N consecutive times is flagged `error` in the UI
  (visible indicator on the sources list) without blocking other sources.

Scan lifecycle (per source):
1. fetch → parse → RawOffer list
2. resolve company → upsert offer (dedup) → update provenance
3. classify (scores + reasons)
4. capture snapshot if new or changed
5. commit transaction; update `sources.last_scan_at/status`

Scans are manual only: a "Refresh" button per source and a "Refresh all"
on the Sources page, plus the applications-scoped Refresh on the
dashboard. Optionally trigger via `jobwatch scan`.

## 8. REST API (JSON, under /api)

- CRUD `/api/sources`, `POST /api/sources/:id/scan`
- `GET /api/offers?filters...` (state, scores, remote policy, company,
  full-text query), `GET /api/offers/:id` (with snapshot version history;
  `?snapshot_id=N` returns that version's full text)
- `PATCH /api/offers/:id` (states, location edit, description edit,
  manual merge/split)
- `GET /api/offers/:id/snapshot-diff?from=N&to=M` (any two versions,
  ordered older → newer)
- CRUD `/api/offers/:id/events`, CRUD `/api/notes`
- CRUD `/api/companies`, `GET /api/companies/:id` (offers + notes)
- `POST /api/offers/import`, `POST /api/companies/:id/merge`
- Serve SPA from embedded FS for everything else.

## 9. Web UI (SPA, React + TypeScript, English)

Local-only dashboard. Sections:

- **Top bar**: navigation only (Dashboard, Offers, Sources). Scan
  actions live on the Sources page and in the dashboard Applications
  section.
- **Dashboard** (`#/`): the Applications section — all offers with
  `application_state != todo`, sorted by most recent timeline activity
  ("where did I send my CV") — with a Refresh button scoped to the
  enabled sources behind those applications; and the Saved section —
  every offer with `saved_at` set, independent of application state,
  sorted by most recent bookmark, with an un-bookmark toggle.
- **Offers** (`#/offers`): filterable/sortable table. Filters on
  `application_state`, `offer_state`, `remote_policy`, saved/unsaved,
  `title_confidence`, `location_confidence`, full-text search. Each row
  shows small score badges with tooltip explanations (match_reasons) and
  a bookmark star — no review queue.
- **Offer detail page**:
  - header: company, title, states (editable dropdowns), scores
  - provenance block: ALL links where this offer was seen, side by side,
    collapsed by default behind a "Provenance (N links)" disclosure, so
    the user can open them and decide whether they are really the
    same offer (with "split offer" action)
  - left column: vertical **timeline** of events, add/backdate/delete
  - right column: stacked **notes**, each optionally pinned to a
    timeline event
  - description: a single-version browser (never a list), defaulting to
    the latest snapshot; prev/next arrows and a version dropdown, a
    "Version X of N" header with captured time and a Scan/Manual origin
    badge, and an on-demand removed/added diff against the previous
    version (the oldest version shows a hint). An Add/Edit action saves a
    manually pasted description through `PATCH /api/offers/:id`
    (versioned as a new `manual` snapshot and selected after saving).
- **Company detail page**: company-level notes above the list of all its
  offers + possible-duplicate-company merge prompt.
- **Sources page**: list with type, last scan status/time, enable/disable,
  per-source refresh, header "Refresh all", and add/edit in a modal
  (config form adapts to source_type).

Design goals: dense, fast, keyboard-friendly; no framework ceremony —
plain React + a lightweight table component is fine.

## 10. Out of scope (v1)

- Scheduled/automatic scans, notifications
- CV version tracking
- Anti-bot evasion, proxy rotation, headless browsers
- Multi-user, auth, deployment anywhere but localhost
- Auto-merging companies or offers without explicit user confirmation

## 11. Spec revisions

- 2026-11-15 (after Milestone 1): Ashby endpoint corrected from POST to
  GET (per Ashby's public posting API docs); `sources.config_json`,
  `sources.enabled` and `job_offers.remote_policy` made NOT NULL with
  defaults; enum validation moved to application code; fingerprint
  separator, location-bucket collision rule and missing offer-state
  transitions (reappearance, sticky `changed`, multi-source
  disappearance) specified; per-source rate limit clarified as enforced
  per host. Open gaps against this spec are tracked in
  `docs/MILESTONES.md`.
- 2026-11-16 (after Milestone 2): the REST API is implemented with the
  parameters documented in `docs/API.md` (including `has_application`,
  `sort` and `limit`/`offset`, which the spec only described as
  "filters" and "pagination"). Company rename and explicit merge
  rewrite the offers' fingerprints and are rejected with a conflict
  when that would duplicate an existing offer; deleting a source keeps
  its provenance rows but clears their source reference. The SPA is
  embedded only when the binary is built with `-tags ui`; without it a
  placeholder page is served. Milestone 2 needs no schema change.
- 2026-11-16 (after Milestone 3): `match_reasons_json` is stored as
  `{"title":[],"location":[]}` so each score has its own tooltip.
  `remote_policy` is derived by the classifier from location and
  description; location confidence is capped for hybrid (75) and onsite
  (50) roles, and remains ≤ 60 when the policy is unknown. The title
  table also penalizes management titles and French internship words.
  Explicit offer merge moves every provenance/event/note/snapshot row
  and then deletes the absorbed offer row (scans still never delete
  offers). Split requires at least one provenance row to remain and a
  non-colliding company+title+location; events/notes/snapshots stay on
  the original. New endpoints `POST /api/offers/:id/merge` and
  `POST /api/offers/:id/split`; detail responses include `snapshot_diff`
  for the two latest snapshots. `jobwatch reclassify [--all]` backfills
  scores. Milestone 3 needs no schema change.
- 2026-11-16 (after Milestone 4): Personio, Workable, RSS/Atom and
  CSS-selector HTML adapters are implemented; HTML sources have a 5 s
  minimum poll interval and fetch only their listing page. WTTJ,
  Workday and SuccessFactors remain unimplemented and are seeded as
  disabled rows with explanatory notes. Manual import may fetch a URL to
  extract JSON-LD/Open Graph/meta/title metadata; this is the only
  outbound request besides source scanning and it uses the same polite
  client. API errors `metadata_incomplete`/`metadata_unavailable` carry a
  `metadata` object for the UI's manual fallback. `jobwatch backup`
  validates its copy (integrity, foreign keys, applied migrations).
  `jobwatch list` shows both confidence scores. Migration 0002 is
  data-only (seed rows), so the schema is unchanged.
- 2026-11-16 (after Milestone 6a): `bamboohr` is added as a source type.
  Its listing endpoint `GET https://{subdomain}.bamboohr.com/careers/list`
  returns the posting id, title, department and structured location but
  no description; the XML feed variant answers 404. Scans therefore use
  only the JSON listing and leave `RawOffer.Description` empty rather
  than fetching one page per posting. Migration 0005 seeds the Semble
  source (data only, schema unchanged).
- 2026-11-16 (after Milestone 7b): the event vocabulary is `cv_sent`,
  `message_received`, `message_sent`, `interview`, `offer`, `rejected`,
  `ghosted`, `other`. `message_received` covers both a recruiter's
  inbound first contact and a reply to an application; `message_sent`
  covers user-initiated pings and follow-ups. Migration 0006 renames
  existing `reply_received` rows in place (data only, no schema change).
  `PATCH /api/offers/:id` may replace the description; it recomputes the
  text hash and appends a snapshot, and rejects `null`. A manual
  description follows the same changed-detection rules as a scan: a
  later differing source description marks the offer `changed` and
  appends a snapshot, while a sighting without a description never
  clears the stored text.
- 2026-11-16 (after Milestone 7c): `snapshots` gains `origin`
  (`scan` | `manual`) via additive migration 0007. Scan/import inserts
  write `scan`; `PATCH /api/offers/:id` writes `manual`. Pre-0007
  snapshots are backfilled as `scan` because the old schema has no
  reliable manual marker. `GET /api/offers/:id` exposes the version
  history (oldest → newest) as metadata and returns one version's full
  text with `?snapshot_id=N`; `GET /api/offers/:id/snapshot-diff` diffs
  any two versions on demand. The UI shows exactly one version at a time
  with origin badge, navigation and a diff against the previous version.
- 2026-11-16 (after Milestone 7e): the top bar is navigation only; the
  global Refresh all moved to the Sources header. The dashboard shows
  Applications and Saved only; the full offers table and import form
  live on `#/offers` (`#/offers?saved=true` presets the saved filter).
  `job_offers.saved_at` (additive migration 0008) is a personal bookmark
  orthogonal to both state machines: saving never changes
  `offer_state`/`application_state`, and a saved offer that becomes an
  application stays in Saved until un-bookmarked. The Applications
  Refresh scans only the enabled sources behind non-`todo` applications,
  sequentially through the existing per-source endpoint.
