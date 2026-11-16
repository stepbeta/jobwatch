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

### Milestone 5a — Bulk source registration

#### Added
- `jobwatch add-sources <file>`: registers every recognized ATS board
  URL in a file of bookmarks, one per line. Blank lines and `#` comments
  are ignored; an inline `# comment` (after whitespace) is stripped and
  URL fragments are kept. Each URL uses the same detection, validation
  and name derivation as `add-source` (shared `sourceFromURL`).
- The command never stops on a bad line: in-file duplicates are refused,
  unknown or invalid URLs are skipped, and the final report lists the
  added sources, the already registered ones (with the source ID and a
  reason) and the not-detected URLs, which are also printed as a plain
  list for follow-up. `--disabled` stages the batch without scanning it.
- Table-driven tests over fixture files cover comments, inline comments,
  fragments, line numbers, duplicates, already-registered URLs,
  undetected URLs and the `--disabled` flag.
- `docs/REMOTE-SOURCES.txt` (65 bookmark URLs) is committed; running the
  command on it registered 8 ATS boards (Grafana Labs, Malt, Mural,
  Overstory, Cycloid, Everbridge, 360Learning, n8n), all scanned `ok`
  for 315 offers.

#### Changed
- Greenhouse URL detection rejects `www.greenhouse.io` and
  `my.greenhouse.io` (corporate site / candidate dashboard): they are
  not job boards and previously produced a bogus board token from
  `/dashboard`.
- README documents the bulk file format, duplicate handling and the
  three-group report; ARCHITECTURE describes the shared registration
  path and the parser rules.

#### Fixed
- `add-source` and `add-sources` now share one validation path, so their
  accepted URLs and derived names cannot drift apart.

### Milestone 5b — Undetected source decisions

#### Added
- `docs/SOURCE-DECISIONS.md`: one row per undetected URL from
  `docs/REMOTE-SOURCES.txt` with the company, the single action taken
  and the reason. Totals: 19 underlying ATS boards, 5 RSS feeds, 5 HTML
  listings and 28 disabled not-trackable sources.
- 19 underlying boards registered and scanned: Nearform, GitLab,
  Wikimedia Foundation, Zapier, Percona, Automattic, Algolia, Camunda,
  GitBook, BlaBlaCar, Linear, ESL FACEIT Group, Docker, CircleCI,
  Canonical, Pulumi, Knack, Ashby and Supabase.
- 5 RSS sources registered and scanned: Ghost (Homerun Atom), Epi
  Company, Tiime and Sekoia (Teamtailor), and We Work Remotely.
- 5 server-rendered HTML sources registered and verified with a real
  scan: Buffer, Tiger Data, Shopify, ElevenLabs and Murena.
- 28 disabled sources with an explanatory `note` for JS-only SPAs,
  Notion, candidate logins, Workday/SuccessFactors/iCIMS/Pinpoint/Zoho,
  WTTJ, robots.txt disallows and aggregators.
- Fixture tests for the Teamtailor nested-location mapping
  (`locations/location/name`) and the Homerun Atom `<description>`
  shape, with trimmed feeds under `internal/sources/testdata/`.

#### Changed
- README documents nested RSS paths, `:has()`/`:haschild()` HTML
  selectors, the new source examples and the per-URL decision table.
- ARCHITECTURE notes the nested RSS path and selector support.
- MILESTONES records the 5b criteria and updates the known-gaps table.
- HTML sources can use `:haschild()` to match the immediate row
  container without matching every ancestor; the Murena source uses it
  to skip the separate "Other positions" table.

### Seed remote sources with the binary

#### Added
- Data-only migration 0003 seeds the 65 sources decided in Milestones
  5a/5b: 37 enabled sources (11 Greenhouse, 11 Ashby, 4 Lever, 1
  Personio, 5 RSS and 5 HTML listings) and the 28 disabled
  not-trackable rows, each with its exact `config_json` (board tokens,
  feed mappings, HTML selectors and explanatory notes). The four
  disabled known-hard rows from 0002 are not repeated.
- Tests cover the seeded count, types, enabled flags, representative
  configs, URL uniqueness, re-application without duplicates, restoration
  of a deleted seed row and preservation of user edits.

#### Changed
- A fresh database, or a deleted `jobwatch.db`, now contains every remote
  source without re-running `add-sources` or repeating the 5b
  investigation. Existing rows are never updated, so user edits survive
  upgrades.
- README, API, DATA-MODEL and SMOKE-TEST document the seeded source set
  and migration 0003.

### Milestone 5c — Aggregator follow-ups

#### Added
- Fixture test for the We Work Remotely RSS shape
  (`internal/sources/testdata/rss_feed.xml`): the `region` element maps
  to the location, `Company: Role` titles are kept verbatim and the
  company stays empty. A real scan of the live feed returned
  100 items → 99 offers, all already tracked.
- A documented manual-import route for aggregator and candidate-only
  pages (FlexJobs, RemoteMore, `my.greenhouse.io` dashboard, The Product
  Crew, Welcome to the Jungle matches, Jobgether): paste the public offer
  URL into the UI import form or run `jobwatch import`.

#### Changed
- `docs/SOURCE-DECISIONS.md` now records the Milestone 5c Aggregators
  review: the WWR feed mapping, why the Workable public search is not
  worth an adapter or HTML source (robots-disallowed query URL,
  incomplete category pages, undocumented search API), and the
  manual-import workflow for account/JS-only aggregators.
- README describes that workflow and the Workable decision; the status
  line and MILESTONES known-gaps table are up to date.
- No schema migration, adapter or API change; the Workable and
  aggregator rows stay disabled with their explanatory notes.

### Seed 5c source decisions (migration 0004)

#### Added
- `0004_seed_5c_sources.sql`, an audit-only data migration recorded in
  `schema_migrations`. It documents the 5c source decisions but contains
  no INSERT: We Work Remotely, the Workable search row and all
  account-required/JS-only aggregator rows were already seeded by 0003.
  Re-declaring them would duplicate source URLs across seed migrations.
- `TestSeed5cAudit` checks the exact 5c row set (WWR `rss` mapping,
  Workable and aggregator disabled notes) and verifies that re-applying
  0004 changes nothing, does not overwrite a user edit and does not
  restore a deliberately deleted source.

#### Changed
- `docs/DATA-MODEL.md` migration table and `docs/ARCHITECTURE.md` record
  0004; MILESTONES notes the audit and the unchanged seeded total of 69.
- Verified on a fresh database: all four migrations apply in order,
  `jobwatch list --sources` matches the live 69-row set field for field,
  running the migrations twice changes nothing, and a real scan of the
  We Work Remotely source returns `ok` (100 items, 99 offers).

### Milestone 6a — BambooHR adapter

#### Added
- `bamboohr` source type and adapter for the public JSON listing
  `https://{subdomain}.bamboohr.com/careers/list`: posting id becomes the
  external UID and `https://{subdomain}.bamboohr.com/careers/{id}` the
  detail link, `jobOpeningName` is the title, and the populated
  `location`/`atsLocation` fields are joined (repeated city/state parts
  dropped, `isRemote` adds a remote marker). The XML feed variant
  `careers/jobs_feed.xml` returns 404, so the JSON listing is the
  implemented surface. The listing has no description, so scans make one
  request per source and leave descriptions empty.
- URL detection for `{subdomain}.bamboohr.com/careers` (also `/list` and
  `/{id}`), a `subdomain` config key, the `add-source` type help and
  example, and the sources-page form field.
- Recorded fixture `internal/sources/testdata/bamboohr_careers.json`
  (Semble board, 9 postings) and table-driven tests for the happy path,
  empty/null listings, missing id/title/location/department, the remote
  flag and malformed JSON.
- Data-only migration `0005_seed_bamboohr_sources.sql` seeds the Semble
  source (`bamboohr`, `{"subdomain":"semble"}`, enabled) with an
  idempotent `INSERT … WHERE NOT EXISTS` by URL. `TestSeedBambooHRSource`
  covers the row, double application, user edits and restoration of a
  deleted row.

#### Changed
- The seeded source total is now 70 (38 enabled, 32 disabled); README,
  ARCHITECTURE, DATA-MODEL, API, SMOKE-TEST and MILESTONES are updated.
- SPEC records `bamboohr` in the source types and the 6a endpoint choice;
  no schema or table change.

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

### Milestone 7b — Event vocabulary, manual description editing, collapsible provenance

#### Added
- `message_sent` timeline event type for user-initiated pings and
  follow-ups. `message_received` now covers both a recruiter's inbound
  first contact and a reply to an application, replacing the narrower
  `reply_received` label. The offer-page dropdown shows "Message
  received" and "Message sent".
- Manual description editing on the offer detail page: an Add/Edit
  action opens a textarea with Save/Cancel, works for empty and non-empty
  descriptions, and shows a success toast. Saving calls
  `PATCH /api/offers/:id` with a `description` string.
- `PATCH /api/offers/:id` accepts `description`: it replaces `raw_text`,
  recomputes `text_hash` and appends a snapshot in the same transaction,
  so manually pasted text is versioned exactly like scan-captured text.
  `null` is rejected with `400 validation_failed`.
- The Provenance block is now a disclosure after the header, collapsed by
  default as "Provenance (N links)". Its expanded state persists per
  offer in `localStorage`; the explicit merge and split actions live
  inside the expanded content.

#### Changed
- Data-only migration `0006_event_message_types.sql` renames existing
  `reply_received` rows to `message_received` in place. It has no INSERT
  and no schema change, and its `WHERE` clause makes re-application
  harmless. The old event value is no longer accepted by the API.
- A later source sighting with a differing description still marks the
  offer `changed` and appends a snapshot, even when the stored text came
  from a manual edit; a sighting without a description still never clears
  the text. Documented in DATA-MODEL and API.
- Tests cover the upgrade migration (old event renamed, other columns and
  occurred_at preserved, idempotent re-apply), PATCH description
  snapshot/hash/updated_at behavior, null rejection, both message event
  types, ordering and rejection of the old value. A scan test proves that
  a no-description sighting keeps manual text and a differing source
  description marks the offer `changed` with another snapshot.
- ARCHITECTURE, API, DATA-MODEL, SPEC, MILESTONES and this changelog are
  updated; no command, adapter, classifier or application-state change.

### Milestone 7c — Snapshot origins and version browsing

#### Added
- Additive migration `0007_snapshot_origin.sql` adds
  `snapshots.origin` (`scan` or `manual`, default `scan`). Scan and manual
  URL import snapshots are written as `scan`; description saves through
  `PATCH /api/offers/:id` are written as `manual`.
- `GET /api/offers/:id` now returns a `snapshots` version history (oldest
  → newest) with id, origin, captured time and text length. The full text
  of one version is included only with `?snapshot_id=N`, so the common
  detail payload stays small.
- `GET /api/offers/:id/snapshot-diff?from=N&to=M` returns the line diff
  between any two versions of the offer. Both must belong to the offer;
  the server orders the pair older → newer regardless of argument order.
- The offer detail Description block is now a single-version browser:
  prev/next arrows, a version dropdown, "Version X of N", captured time,
  a Scan/Manual origin badge, and an on-demand removed/added diff against
  the previous version. The oldest version shows a "first captured
  version" hint instead of a diff. Saving a manual description selects
  the new latest version.

#### Changed
- Historical snapshots are all backfilled as `scan`: the pre-0007 schema
  has no reliable marker that distinguishes older manual edits from scans.
  Origins are exact for snapshots written from this migration onward.
  This is recorded in DATA-MODEL and MILESTONES.
- The fixed "diff of the two latest snapshots" in the UI is replaced by
  the selected version's diff. The API keeps `snapshot_diff` and
  `latest_snapshot` for compatibility; `latest_snapshot` now includes its
  origin.
- Tests cover the pre-0007 upgrade with no data loss, snapshot history and
  conditional text payload, arbitrary/reversed/foreign diffs, and origins
  recorded through a real scan and through PATCH. Existing snapshot tests
  keep passing through the default `scan` origin.
- API, DATA-MODEL, ARCHITECTURE, SPEC, MILESTONES, README and this
  changelog are updated; no classifier, adapter, event-type or note change.

### Milestone 7d — Editable events and note↔event linking

#### Added
- Every timeline event has inline Edit and Delete icon actions. Edit
  opens a form with the same fields as creation (type, description,
  required date plus optional time) and calls
  `PATCH /api/offers/:id/events/:eventID` with all three fields; Cancel
  discards the draft without a request. Backdating accepts a date alone,
  exactly like the create form.
- The create form now uses the same required date + optional time inputs,
  so date-only backdating works there too.
- Bidirectional note↔event cues, both derived from the offer detail
  payload: a pinned note card shows a coloured left border and a badge
  with the event's type icon and date; clicking it scrolls to and briefly
  highlights the event row. An event with pinned notes shows a note icon
  with the count; clicking it expands the pinned notes inline under the
  event row.
- Deleting a pinned event refreshes the notes, so the pin badge
  disappears while the note text stays (the API clears
  `pinned_event_id`).
- Dependency-free inline SVG icons for event types and the edit, delete
  and note actions.

#### Changed
- The frontend now has component tests: Vitest + jsdom +
  Testing Library run against a mocked API. They cover saving a
  backdated event, cancelling without a request, the pinned-note cue,
  the event note-count marker and the pin clearing after event deletion.
  `npm test` runs them.
- No backend, schema, classifier, adapter, event-enum or note-contract
  change was needed; the existing PATCH/DELETE endpoints already support
  the flow.
- ARCHITECTURE, MILESTONES, SMOKE-TEST, README and this changelog are
  updated.
