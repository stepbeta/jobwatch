# Changelog

## Unreleased

### Added
- Go module, project layout, `.gitignore`, placeholders for `web/` and
  `internal/classify`.
- SQLite storage (pure-Go driver, WAL mode) with an embedded, ordered
  migration runner. Migration 0001 creates companies, sources,
  job_offers, job_offer_provenance, events, notes, snapshots, and
  full-text search over offer titles/descriptions and notes.
- Normalization helpers for company names (legal suffixes such as
  GmbH/Inc/SAS/Ltd/SpA/LLC removed), job titles (H/F-style markers
  removed), location buckets, fingerprints and description hashes.
- Source adapters for Greenhouse, Lever and Ashby job boards, with a
  shared client that honors robots.txt, spaces requests (2 s default,
  configurable per source), and identifies itself with a JobWatch
  User-Agent. Board URLs are recognized automatically.
- Scan pipeline: new offers are created with a description snapshot;
  offers seen again are refreshed; changed descriptions mark the offer
  `changed` with a new snapshot; offers no longer listed become
  `disappeared` (never deleted). Each source is written in one
  transaction, and a failing source is marked `error`/`blocked` without
  stopping the others.
- Manual import of offers seen elsewhere, by URL or JSON, matched to
  existing offers when the company and title are the same.
- CLI commands: `serve` (local server with `GET /api/health`), `scan`,
  `add-source`, `list`, `import`, `backup`.
- Documentation: ARCHITECTURE, DATA-MODEL, API, MILESTONES.
- README with build, quick-start and command reference.

### Milestone 2 — REST API and web UI

#### Added
- REST API under `/api`: sources CRUD with a synchronous per-source
  scan endpoint, offers list with enum/score/company/FTS5 filters,
  sorting and pagination, offer detail (provenance, timeline, notes,
  latest snapshot), offer state/location updates, nested event CRUD
  with backdating, polymorphic note CRUD with optional event pinning,
  companies CRUD with detail and an explicit merge endpoint, and manual
  offer import. All errors use `{"error":{"code","message"}}`.
- Conservative possible-duplicate company detection (token containment
  or small edit distance) shown as a merge prompt in the UI. Merging is
  always an explicit user action; it moves offers and notes, rewrites
  offer fingerprints and refuses on collisions.
- React + TypeScript SPA (Vite): dashboard with an Applications section
  sorted by recent activity, filtered/paginated offers table with score
  badge placeholders, sources page with type-dependent add/edit forms
  and scan indicators, offer detail with editable states, provenance
  cards, backdatable timeline and pinned notes, and company detail with
  offers, notes and the merge prompt. English, dense layout, no i18n.
- Integration tests for every API handler group over a temporary
  SQLite database with `httptest`, plus DB tests for offer filtering,
  FTS search, event/note lifecycle, company rename/merge conflicts and
  source deletion.
- The SPA is embedded with `go:embed` when built with `-tags ui`; a
  placeholder page explains the build otherwise. Vite proxies `/api` to
  the Go server in development.

#### Changed
- `serve` now serves the SPA at `/` (embedded or placeholder) instead
  of a plain-text message.
- The API and UI require no schema change; migration 0001 is unchanged.
- Clearing an offer's location stores SQL `NULL`, not an empty string.
- README documents building the UI, the `ui` build tag, the API and the
  development workflow.

#### Fixed
- Deleting a non-existent company now returns `404` instead of `204`.

### Milestone 3 — Classification, dedup UX, changed-offer diff

#### Added
- Deterministic classifier (`internal/classify`): weighted title keyword
  tables (strong/weak positives, strong negatives, management penalty),
  location and description scoring, derived `remote_policy`, and
  `match_reasons_json` with a human-readable rationale list per score.
  Includes French internship terms and whole-token matching so "Marketing
  Platform Lead" cannot outrank "Platform Engineer".
- `jobwatch reclassify [--all]` backfills scores and remote policies for
  offers created before the classifier; `--all` recomputes every offer
  after keyword changes.
- Classifier wired into every scan and manual import; scores and remote
  policy are refreshed on every sighting.
- Offer merge endpoint and detail-page dialog: all provenance rows,
  events, notes and snapshots move to the kept offer, duplicate links are
  collapsed, and the absorbed offer row is removed only afterwards.
- Offer split endpoint and detail-page workflow: select provenance rows,
  adjust the new title/location, and create a second offer in the same
  company. Events/notes/snapshots stay with the tracked original.
- Line diff between the two latest snapshots (`internal/diff`,
  `snapshot_diff` in offer detail) with added/removed lines highlighted
  on `changed` offers.
- Dismissible possible-duplicate-company banner (dismissal remembered in
  localStorage per company) and an explicit keep-which-company merge
  dialog.
- Tests: table-driven classifier cases (including tricky titles and
  remote policies), line-diff cases, DB merge/split and reclassification
  tests, and API tests for offer merge/split and snapshot diff.

#### Changed
- Score badges now show the classifier's matching terms as tooltips;
  title/location confidence filters operate on the new scores.
- `remote_policy` is no longer always `unknown`; it is derived from
  location and description (remote/hybrid/onsite).
- Milestone 3 needs no schema migration; migration 0001 already has the
  confidence columns.
- The data-safety rule now records the single explicit exception: an
  explicit user-confirmed offer merge deletes the absorbed row after all
  child rows have moved; scans still never delete offers.

### Milestone 4 — More sources, import ergonomics, polish

#### Added
- Personio adapter for the public XML feed
  (`https://{tenant}.jobs.personio.de/xml`): office as location, job id
  as external UID, titled description blocks converted to text.
- Workable adapter for the keyless widget API
  (`apply.workable.com/api/v1/widget/accounts/{account}?details=true`):
  multi-location entries are grouped by shortcode, locations joined,
  `telecommuting` marked, and the company name taken from the board.
- Generic RSS 2.0 / Atom adapter with configurable item and field mapping
  (`item`, `title_field`, `link_field`, `description_field`,
  `location_field`, `company_field`, `uid_field`), including Atom
  alternate links and HTML descriptions.
- Last-resort HTML adapter using per-source CSS selectors
  (`item_selector`, `title_selector`, `link_selector`, …); it fetches only
  the listing page and never polls a host faster than every 5 seconds.
- URL detection for Personio (`{tenant}.jobs.personio.de/.com`) and
  Workable (`apply.workable.com/{account}`, `{account}.workable.com`).
- Data-only migration 0002 seeding four disabled sources: Datadog/Cisco
  (Workday), SAP (SuccessFactors) and Welcome to the Jungle, each with an
  explanatory `note` shown in the UI. Workday/SuccessFactors/WTTJ adapters
  are intentionally not implemented.
- `POST /api/offers/import` and `jobwatch import` accept a URL alone and
  extract title/company/location/description from JSON-LD JobPosting,
  Open Graph/meta tags or the page title through the polite client.
  Incomplete pages return `422 metadata_incomplete` /
  `metadata_unavailable` with a `metadata` object so the UI can prefill
  its manual form; `--no-fetch` keeps the old explicit-field behaviour.
- `jobwatch list` now shows remote policy, both confidence scores and the
  offer/application states in a compact table; `list --sources` shows the
  disabled-source note.
- `jobwatch backup` reopens the copy read-only and validates it with
  `PRAGMA integrity_check`, `foreign_key_check` and an applied-migrations
  check; a failed copy is deleted.
- Shared confirmation dialog for every destructive UI action (source,
  event, note, company), auto-dismissing success/error toasts, loading
  indications and empty states.
- Source forms for Personio (tenant/language), Workable (account),
  RSS/Atom field mapping and HTML selectors, with the seeded note shown.
- End-to-end smoke test over the real Workable adapter and fixture
  (add source → scan → classify → application state → timeline event) and
  `docs/SMOKE-TEST.md` with manual UI steps.

#### Changed
- Header/empty-source text and README now describe all supported source
  types and the disabled known-hard rows.
- The polite client applies a per-type interval: HTML sources have a 5 s
  floor; a larger `rate_limit_ms` always wins.
- Manual import and URL metadata extraction share the same polite client
  through the API server, preserving the robots cache and rate limiter.
- `source_type` validation for `add-source` no longer advertises `wttj`.
- Two module dependencies were added: `golang.org/x/net/html` (HTML
  parsing) and `github.com/andybalholm/cascadia` (CSS selectors), because
  the standard library has neither.

#### Fixed
- Backup command now fails (and removes the partial copy) when the copy
  is not a valid JobWatch database.
- API server no longer creates two independent polite clients when both
  scanning and import metadata are used.


### Changed
- SPEC updated to match decisions made in Milestone 1: Ashby is fetched
  with GET, a few schema columns are NOT NULL with defaults, enum values
  are checked by the application, and the fingerprint collision rule and
  missing offer-state transitions are spelled out. Revisions are listed
  in SPEC §11.
- MILESTONES now separates permanent deviations from open known gaps.
- AGENTS.md layout lists `internal/scan` and the migrations embed file;
  its documentation duty now also covers README, SPEC revisions and the
  AGENTS.md layout itself.
