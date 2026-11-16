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
