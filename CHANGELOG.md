# Changelog

## Unreleased

### Offer identity from source uid (OpenSpec change, task group 1)

#### Added
- Migration 0012 (`company_keys`, `company_aliases`): companies gain
  attested identifiers (namespaced ATS board/site/org/tenant/account/
  subdomain tokens) and per-source or global name aliases. Existing
  companies get one confirmed alias each, and typed single-employer
  sources get a key backfilled from their config token.
- `job_offers.incomplete_reason` and `job_offers.incomplete_raw`, and a
  nullable `job_offers.company_id`, so an item a source cannot normalize
  can be stored instead of dropped.
- A partial unique index on `job_offer_provenance(source_id,
  external_uid)` for non-empty uids: a posting seen again on the same
  source can no longer point at two offers.
- Table-driven migration test covering the rebuild, the provenance
  squash, key/alias backfill, the `changed` conversion, nullable
  companies, duplicate fingerprints, FTS triggers, foreign keys and a
  second no-op run.

#### Changed
- `offer_state` is now `active | disappeared`. Existing `changed` rows are
  migrated to `active`; a description edit records a snapshot and keeps
  the offer active. `changed` is removed from the Go enum, the API filter
  and the UI state filter.
- The migration runner runs each migration on a pinned connection with
  foreign keys disabled (required for the `job_offers` rebuild) and
  verifies `PRAGMA foreign_key_check` before committing.
- `job_offers.fingerprint` is no longer UNIQUE. The current scan code
  still writes and resolves by it until the offer-identity task group
  lands.

#### Fixed
- 80 duplicate `(source, external_uid)` pairs (81 extra provenance rows)
  from the old fingerprint identity are squashed onto the surviving
  offer. No offer, event, note or snapshot is deleted; the old duplicate
  offers remain, but without a source link. The live database went from
  2465 to 2384 provenance rows.

### Offer identity from source uid (OpenSpec change, task group 2)

#### Added
- Scans resolve a sighting through provenance: `(source, external_uid)`
  first, then `(source, url)`. A company is resolved and an offer created
  only on a miss, so the same posting keeps one offer through title,
  location, company-name and description changes.
- Title and a reported location follow the source on later sightings; the
  offer's company and canonical URL are not rewritten by a sighting.
- Manual imports use the URL as identity: a URL already linked to an offer
  attaches to it, while a new URL creates a new offer even when the
  company and title match another one.

#### Changed
- The scan summary no longer reports `changed`: a description edit keeps
  the offer `active` and only records a snapshot. `Result.Changed`,
  `OutcomeChanged`, the API `changed` field and the CLI/UI scan summaries
  are gone.
- Same-title siblings with different uids are separate offers; they are no
  longer collapsed by the fingerprint. A source reporting a new company
  name for the same uid no longer duplicates or moves the offer.
- `UpsertProvenance` updates the row by `(source, uid)` before
  `(offer, url)`, so a new URL under a known uid cannot create a second
  provenance row. The link keeps the first-seen URL.

#### Fixed
- The `TestScanDedup` and Lever fixture expectations now assert distinct
  uids and offers instead of distinct fingerprints.

### Offer identity from source uid (OpenSpec change, task group 3)

#### Added
- Migration 0013: `companies.name_normalized` is no longer UNIQUE (display
  names are attributes, not identity), with a non-unique lookup index.
  Enabled single-employer sources get `config.company` set to their
  current display name so the scan pipeline can stop using the source
  name as a company fallback. We Work Remotely is left for the RSS/HTML
  employer extraction task.
- Company resolution is key → source alias → confirmed global alias →
  create. A name-only claim is recorded for its source; a new company gets
  a source alias (or a global alias for a manual import). A key that
  resolves an existing company is linked to it.
- Company merge moves offers, notes, keys and aliases to the target and
  records the absorbed display name as a confirmed global alias. A rename
  records both the old and the new name as aliases. Neither rewrites offer
  fingerprints.

#### Changed
- The scan pipeline no longer uses `sources.name` as a company fallback.
  An item with no adapter company evidence and no `config.company` is
  skipped (and counted in `skipped`) until the incomplete-listing task
  stores it with its raw values.
- `POST /api/companies` and `PATCH /api/companies/:id` allow duplicate
  normalized names; `409 duplicate_company` is gone. Company rename and
  merge no longer return `409 merge_conflict`, and offer split no longer
  returns `409 offer_conflict`; same-title offers are legitimate because
  identity is provenance.
- Offer split computes a base fingerprint and does not check collisions.

#### Fixed
- Merging two companies with same-title offers now succeeds instead of
  reporting a fingerprint conflict, and a later scan cannot recreate the
  absorbed company because its keys and names resolve to the target.

### Offer identity from source uid (OpenSpec change, task group 4)

#### Added
- `RawOffer` carries `CompanyKey` (the namespaced identifier the adapter
  already holds) and `IncompleteReason` (`missing_company`,
  `missing_title`, `missing_company_title`). Every typed adapter emits its
  token: `greenhouse:<board_token>`, `lever:<site>`, `ashby:<org>`,
  `personio:<tenant>`, `workable:<account>`, `bamboohr:<subdomain>`,
  `wttj:<org slug>`.
- A declarative `employer` rule for RSS and HTML sources. With
  `{"separator":":"}` it splits `Employer: Role` and rewrites both the
  company and the title; `field` (RSS) or `selector` (HTML) reads the
  employer from a separate element and strips the repeated prefix.
- Migration 0014 configures the rule for We Work Remotely. A recorded
  169-employer fixture (`wwr_feed.xml`) scans twice and asserts 169
  distinct employers and stable uids.
- Fixture tests fetch every adapter twice and compare the external uids:
  Greenhouse, Lever, Ashby, Personio, Workable, BambooHR, WTTJ, RSS and
  HTML.

#### Changed
- Adapters no longer silently drop an item with an empty title or no
  company: they return it with an `IncompleteReason`. The scan pipeline
  still counts such items as `skipped` until the incomplete-listing group
  stores them.
- RSS/HTML titles are rewritten by the employer rule, so a multi-employer
  feed no longer attributes its offers to the feed itself.

#### Fixed
- We Work Remotely titles such as `Hometap: Underwriter` now yield
  company `Hometap` and title `Underwriter` instead of a verbatim title
  and an empty company. Offers recorded before the rule keep their old
  company until explicitly reattributed.

### Offer identity from source uid (OpenSpec change, task group 5)

#### Added
- Items a source cannot normalize are stored as incomplete offers
  instead of dropped. A missing company leaves `company_id` NULL; a
  missing title leaves `title` empty. The values the source reported are
  kept as JSON in `incomplete_raw`, and `incomplete_reason` records
  `missing_company`, `missing_title` or `missing_company_title`. The
  offer keeps its provenance, snapshots and notes, and a rescan of the
  same identity only refreshes `last_seen_at`.
- `POST /api/offers/:id/repair` assigns a company and title to an
  incomplete offer, optionally sets the location, clears
  `incomplete_reason` and re-runs the classifier.
- The scan result and the CLI/UI summaries report an `incomplete` count
  per source.

#### Changed
- The offers list, dashboard, search and CLI `list` exclude incomplete
  offers by default; `OfferQuery.IncompleteOnly` (used by the repair
  view) lists them. Incomplete offers with no company come back with an
  empty company name and `company_id: null`.
- A repaired offer keeps `incomplete_raw` after `incomplete_reason` is
  cleared. Later scans use that marker to preserve the user's company,
  title and location, even when the source later reports a usable item.
- The scan pipeline recomputes incompleteness from the item fields; the
  adapter's `IncompleteReason` is only a hint. An item without a URL is
  still skipped because it has no provenance identity.

#### Fixed
- The company-name fallback test now asserts an incomplete offer instead
  of a skipped item, and the dedup test keeps a URL-less item skipped
  while a title-less item is stored incomplete.

### Offer identity from source uid (OpenSpec change, task group 6)

#### Added
- `jobwatch reattribute [--all | --source <id>]` re-fetches a source and
  moves the offers linked by `(source, uid)` to the company and title the
  adapter now reports. Events, notes, snapshots, application state and
  listing state are preserved, and the classifier is re-run for the new
  title. The target company uses the scan resolution order (key, source
  alias, confirmed global alias, create).
- Per-source output reports fetched, re-attributed, unchanged, skipped
  offers and the companies the pass emptied. An emptied company stays in
  the database for the user to delete.
- The pass is idempotent and skips offers without a current `(source,
  uid)` sighting, offers with no uid link, and user-repaired offers.

#### Changed
- Live remediation of We Work Remotely moved 33 of its 289 offers from
  the feed company to 26 extracted employers; the total offer count was
  unchanged and a second run moved nothing. The other 256 offers predate
  the current 98-item feed and have no uid match, so they remain under
  the feed as history (task 6.6).

### Offer identity from source uid (OpenSpec change, task group 7)

#### Added
- `GET /api/offers?incomplete=true` lists incomplete offers; the
  default list still excludes them. The `incomplete` value is validated
  like `saved`.
- An **Incomplete** page (`#/incomplete`) with the item count, the
  values each source reported, the reason, and a repair form. The form
  reuses the manual import layout and company combobox; saving calls
  `POST /api/offers/:id/repair` and the item leaves the view.
- The app navigation shows the incomplete count, refreshed after scans
  and repairs.
- `GET /api/companies/:id` returns the company's attested keys, and the
  company page shows them so same-named companies are distinguishable.

#### Changed
- `changed` is gone from the offer-state type, filter and badge. The
  offer page shows the description version count in the header and in
  the snapshot summary, and no longer opens a changed-description
  banner.
- Offer rows with no company render a dash instead of a broken company
  link; the offer detail page handles a null company.

### Offer identity from source uid (OpenSpec change, task group 8)

#### Changed
- Documentation synchronized with the implemented change:
  `DATA-MODEL.md` covers company keys/aliases, provenance identity, the
  incomplete marker and the two-state offer machine;
  `ARCHITECTURE.md` documents the adapter contract, company resolution,
  the incomplete scan path and the re-attribution command; `API.md`
  documents the `incomplete` filter, the repair endpoint, the offer
  state/reason enums and the company `keys` response; `README.md` and
  `USAGE.md` add the `reattribute` command and the Incomplete page;
  `SPEC.md` records groups 1–8; and `MILESTONES.md` closes the
  fingerprint-identity, RSS title-splitting and We Work Remotely gaps.

### Offer identity from source uid (OpenSpec change, archive)

#### Changed
- The change is archived. Its four capabilities were synced to
  `openspec/specs/` (offer-lifecycle, company-resolution,
  source-normalization, incomplete-listings), and the matching
  `docs/SPEC.md` sections now point there instead of restating the
  behavior. `docs/MILESTONES.md` records the archive as done.

### WTTJ discovery and query hardening (OpenSpec change `harden-wttj-discovery-and-query`)

#### Fixed
- Discovery finds the Algolia credentials by walking the whole bootstrap
  page tree for the first `properties` object carrying `algolia_app_id`,
  including nested `contents[]` blocks, instead of requiring a
  `kind: "jobs-search"` block. Pages whose outer `jobs-search` block is a
  layout container with no credentials (Partoo) now resolve; block `kind`
  no longer decides anything.
- The Algolia query sends the page's filters as URL-encoded
  `facetFilters`, one group per attribute with several values OR-ed,
  instead of a space-joined `filters=`. The secured key only embeds
  `website.reference:wttj_fr`, so an unscoped query returned the whole
  site (~60k hits). The embedded filter is not repeated in `params`, and
  the key is never decoded.
- Offer URLs and company keys follow the hit's own `organization.slug`,
  not the source page's slug; the page slug is only a fallback when a hit
  omits its organization. Pages that list linked organizations therefore
  link each offer to its owner.

#### Added
- Partoo fixtures under `internal/sources/testdata`: the nested bootstrap
  block (layout-only outer block, three `organization.slug` values) and
  one hit, trimmed and documented in the fixtures README.
- httptest-only tests for nested-block discovery, multi-organization
  `facetFilters` construction (including key-filter exclusion), and
  org-slug-based hit mapping.

#### Changed
- The WTTJ filter model is now ordered groups rather than flat
  attribute/value pairs; `website.reference` is omitted from the query
  params.
- Live verification 2026-11-21: Fabriq re-scan fetched 7 offers (0 new,
  7 seen); the pre-existing Partoo `wttj` source (id 82,
  `/fr/companies/partoo/jobs`) recovered from its previous error and
  fetched 15 offers (15 new, 0 incomplete), all under company Partoo; one
  constructed URL answered HTTP 200.

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

### Milestone 7e — Dashboard/layout restructuring and saved offers

#### Added
- Persistent per-offer bookmark: migration 0008 adds nullable
  `job_offers.saved_at` (NULL = not saved). `PATCH /api/offers/:id`
  accepts `saved: true|false`, setting the current UTC time or clearing
  it, and the offer payload returns `saved_at`. `GET /api/offers` accepts
  `saved=true|false` and the `saved` sort (`saved_at DESC`).
- Star bookmark toggles on every offers-table row and in the offer detail
  header. Saved rows get a subtle highlight and a filled star.
- Dashboard Saved section: every bookmarked offer regardless of
  application state, most recently saved first, with an un-bookmark
  toggle and a "Show all" link to `#/offers?saved=true`.
- Dedicated Offers page (`#/offers`) holding the full filters, sorting,
  pagination and manual import form. The dashboard (`#/`) now contains
  only Applications and Saved.
- Dashboard Applications Refresh: collects offers with
  `application_state != todo`, resolves their provenance sources, and
  scans the enabled ones sequentially through the existing per-source
  endpoint. Partial failures are reported per source; manual links are
  counted in the summary.
- Sources header owns Refresh all; per-source Refresh is unchanged.

#### Changed
- The add/edit source form is now a modal dialog (shared `Modal` shell)
  instead of an inline panel. All type-dependent fields, validation and
  submit behavior are unchanged; the list state stays mounted.
- Company notes render above the offers list; add/delete, rename/website
  edit and the possible-duplicate banner are unchanged.
- The top bar is navigation only. The global Refresh-all loop moved from
  `App.tsx`'s top bar into a shared sequential batch helper used by the
  Sources header and the Dashboard Applications section.
- Component tests cover the add-source modal, Dashboard
  Applications+Saved, un-bookmarking, the applications refresh fan-out
  and the new hash routes. Backend tests cover the pre-0008 upgrade,
  `saved` PATCH and the `saved` list filter.
- ARCHITECTURE, DATA-MODEL, API, MILESTONES, README and this changelog
  are updated. No classifier, adapter, scan-pipeline, notes-pinning or
  event-type change.

### Milestone 7f — Company two-column header and modal manual import

#### Added
- The Offers page header has an **Add offer** button that opens the manual
  import form in the shared modal shell. The URL fetch-and-prefill flow,
  the manual company/title/location/URL/description fields, validation,
  inline errors and the `422 metadata_incomplete`/`metadata_unavailable`
  prefill fallback are unchanged.
- A successful import closes the modal, shows the existing success toast,
  refreshes the app data and opens the new or matched offer's
  `#/offers/:id` detail page, so the returned offer id is always
  reachable.
- Company detail now renders identity and notes as one two-column header
  panel: name, website, offer count, created time, Edit/Delete company and
  the possible-duplicate banner on the left; the company-note list and
  new-note form on the right. The offers list stays below as its own
  panel. The grid collapses to a vertical stack at `max-width: 900px`,
  the breakpoint already used by the offer detail columns.

#### Changed
- The inline import panel at the bottom of `#/offers` is gone; the import
  form is modal-only. The Dashboard is unchanged (it still has no import
  UI, per 7e). `jobwatch import` is untouched.
- The possible-duplicate banner and its merge dialog behavior are
  unchanged; the banner moved into the company header's identity column.
- Component tests cover the Offers import modal (open, URL submit,
  422-prefill fallback, close/navigate) and the Company two-column
  structure plus its 900px breakpoint contract. `vitest.config.ts`
  processes CSS so the layout test can read `styles.css?raw`.
- ARCHITECTURE, MILESTONES, README and this changelog are updated. No
  schema, API, classifier, adapter, scan-pipeline, notes or event change.

### Milestone 8 — Documentation overhaul

#### Added
- `docs/USAGE.md`: the practical reference that used to sit in the README.
  It covers supported source types and recognized board URLs, every
  `--config` key, RSS/HTML examples, bulk source registration, the per-URL
  source decisions, polite fetching and scan status, web UI page behavior,
  manual import, offer and application states, classification, backups,
  the full command and flag reference, and the smoke test.
- A Mermaid flowchart in the README showing the real workflow: sources →
  polite scan → adapter parse → company/fingerprint dedup → deterministic
  classification → offer states → review/bookmark/apply → timeline and
  notes, with manual import entering the same dedup path.
- A "Current state" summary at the top of `docs/MILESTONES.md` and a
  Milestone 8 entry recording this documentation-only pass.

#### Changed
- `README.md` is now visitor-first: factual intro, Requirements and Quick
  start at the top, Core workflow next, then a trimmed command table, five
  one-line UI summaries, reference teasers with links, development notes
  and a bottom Status section. The milestone-status paragraph no longer
  appears near the top.
- Source configuration tables, bulk-registration rules, source decisions,
  manual-import details, state definitions, classification notes, backup
  validation and the long CLI examples moved to `docs/USAGE.md`; the
  README keeps one-sentence summaries and relative links.
- Cross-links were refreshed between README, USAGE, API, SPEC,
  ARCHITECTURE, DATA-MODEL, MILESTONES, SMOKE-TEST and SOURCE-DECISIONS.
  `docs/API.md` and `docs/SPEC.md` now point to USAGE for source and CLI
  configuration.

#### Fixed
- README no longer reads as an internal reference dump, and the command
  examples, flags and build steps were re-checked against the CLI
  implementation and the build.

No code, schema, API or behavior changed.

### Milestone 9 — Interview prep FAQ

#### Added
- Additive migration 0009 creates the standalone `interview_entries`
  table: question, markdown answer, enum-like `answer_format`, optional
  `display_order` and created/updated timestamps. Existing tables and rows
  are untouched.
- Interview-prep CRUD under `/api/interview-entries` (`GET` list, `POST`,
  `GET` one, `PATCH`, `DELETE`) with the usual validation, error shape and
  partial-update semantics. Duplicate questions are allowed. Answers are
  stored as markdown plaintext, never HTML.
- A dedicated **Interview prep** page at `#/interview-prep` with a top-bar
  entry: an accordion of questions where expanding shows the rendered
  answer, Edit/Delete actions on each header row, and an Add button that
  opens a modal with a question input, a markdown textarea and a live
  preview pane. Edit reuses the same modal. Delete uses the shared
  confirmation dialog.
- A small in-house markdown renderer (`components/Markdown.tsx`) for
  answers: headings, bold, italic, inline code, fenced code blocks and
  lists. It emits React text nodes and never uses
  `dangerouslySetInnerHTML`, so raw HTML in an answer is shown literally
  and cannot execute. No markdown dependency was added.
- API integration tests for interview-entry CRUD, ordering, duplicate
  questions and validation errors; a migration-upgrade test; and component
  tests for expand/collapse, the add modal, escaped markdown, edit and
  delete flows.

#### Changed
- `docs/API.md`, `docs/DATA-MODEL.md`, `docs/ARCHITECTURE.md`,
  `docs/USAGE.md`, `README.md`, `docs/MILESTONES.md` and this changelog
  document the new API, table, route and page.

No offer, event, note, company, classifier, adapter or scan-pipeline
behavior changed.

### Milestone 10 — Interview prep categories, reordering and dialog UX

#### Added
- Additive migration 0010 adds a nullable `interview_entries.category`.
  There is no categories table: the category set is derived from the rows,
  and `NULL` means "Uncategorized". Existing entries keep every value and
  read as uncategorized.
- `category` in the interview-prep API: returned by list/single responses;
  accepted by POST (trimmed, non-blank; omitted/`null` stores `NULL`) and
  PATCH (string replaces, `null` clears, `""` rejected). Listing now sorts
  categories alphabetically (case-insensitive) with uncategorized last,
  applying the existing `display_order`-then-id rule inside each section.
- Category sections on the interview-prep page, an amber "No answer"
  badge for entries with an empty answer, and native HTML5 drag-and-drop:
  entries reorder within a section or move to another section (persisting
  fresh `display_order` values and a category change through PATCH), while
  section headers reorder the presentation-only section list.
- The section order is saved as a JSON category array in `localStorage`,
  restored on load, ignores stale names, appends new categories
  alphabetically and always keeps "Uncategorized" last. A "Reset to
  alphabetical" control clears the preference.
- The Add/Edit dialog now has a question input, a category dropdown with
  an inline "Add new category…" option, and one large markdown area with
  an Edit/View toggle instead of the cramped side-by-side textarea and
  preview.
- The application base font size is raised from 14px to 15px (inherited
  by form controls), keeping tables, badges and dialogs dense.
- Tests: a 0010 migration-upgrade test (old rows survive, category NULL,
  re-apply is a no-op), API tests for category trimming/blank/null and
  per-section ordering, and component tests for section grouping, the
  empty-answer cue, both drag-and-drop PATCH payloads, the Edit/View
  toggle and new-category creation.

#### Changed
- `docs/API.md`, `docs/DATA-MODEL.md`, `docs/ARCHITECTURE.md`,
  `docs/USAGE.md`, `docs/MILESTONES.md`, `docs/SPEC.md` and this
  changelog document the category column, ordering rules, page behavior
  and dialog change. The Milestone 9 "no reordering controls" gap is
  closed.

No offer, event, note, company, classifier, adapter, scan-pipeline or
migration 0001–0009 behavior changed.

### Milestone 10a — Welcome to the Jungle (WTTJ) source adapter

#### Added
- `wttj` source adapter for Welcome to the Jungle company careers pages.
  Each scan calls the page bootstrap once to discover the Algolia index,
  its filters, the page language and the secured credentials, then pages
  through `https://{app_id}-dsn.algolia.net/1/indexes/*/queries` (50 hits
  per request) until `nbPages` is reached. Both calls use the shared
  polite client, so robots.txt and the per-host rate limit apply to
  `api.welcometothejungle.com` and `*.algolia.net` as well.
- The Algolia request reproduces the verified quirks: a mandatory
  `Referer`, the form content type with a JSON body, and a `params`
  string built from the discovered filters as
  `filters=organization.slug:fabriq&hitsPerPage=50&page=N`. The secured
  key is sent verbatim; it is never decoded and its embedded restrictions
  are never repeated in the filters.
- Hit mapping tolerates the two index shapes seen in the wild:
  `reference` (falling back to `objectID` as a string) as the external
  UID, `name` as the title, `organization.name` as the company,
  `offices[]` (falling back to `office`) as the location, and `profile`
  markdown stored verbatim (falling back to `summary`). Offer URLs are
  built from the discovered language, company slug and `hit.slug`.
  Salary, remote and contract type have no offer column and are ignored.
- A missing `jobs-search` block reports the permanent error
  "URL is not a WTTJ careers page"; network, HTTP and JSON decoding
  failures stay retryable errors, and an Algolia 403 marks the source
  `blocked`.
- URL detection for `www.welcometothejungle.com/<lang>/companies/<slug>/jobs`
  (the marketplace paths are not company pages), the `www`-less host, the
  `add-source` help/example, the source name defaulting to the company
  slug, and a form hint on the sources page. `wttj` sources can now be
  scanned from the UI.
- Data-only migration `0011_seed_wttj_sources.sql` seeds the Fabriq
  careers page (`wttj`, empty config, enabled). The disabled 0002 WTTJ
  marketplace row is untouched; the URLs do not collide.
- Fixtures `wttj_page_fabriq.json`, `wttj_hits_page0.json` and
  `wttj_hits_page1.json`, and tests for mapping, bootstrap parsing,
  params/body builders, the full fetch (headers, content type, pagination
  stop), credential overrides, URL detection and the seed migration. No
  test hits the network.

#### Changed
- The shared client can now issue polite POST requests with explicit
  headers and a per-request User-Agent; GET behavior is unchanged.
- `docs/SOURCE-DECISIONS.md` documents the two-call flow (with a Mermaid
  sequence diagram), why credentials are discovered at runtime, the
  debug-only credential overrides and the three known gaps. `docs/USAGE.md`,
  `docs/ARCHITECTURE.md`, `docs/DATA-MODEL.md`, `docs/API.md`,
  `docs/MILESTONES.md`, `docs/SPEC.md` and `README.md` follow.

Live check: source 76 (Fabriq) returned `nbHits` 7, created 7 offers
(`ok`, 7 new), the constructed job URL answers HTTP 200, and a second
scan reported 0 new, 7 seen, 0 changed, 0 disappeared.

No schema change. No offer, event, note, company, classifier or
scan-pipeline behavior changed.

### Milestone 11 — table controls, sources upgrade, detail layout, Makefile

#### Added
- Shared table controls for the Offers list and Sources list: a column
  picker persisted per page in `localStorage` (hidden-column storage, so
  new columns stay visible for existing users and the last visible column
  cannot be hidden), header-click sorting with a directional indicator, a
  10/20/50 per-page selector, and URL query-string state for filters,
  sort, order, page and page size. The Offers page uses server-side
  sorting through the new `location`, `title_confidence` and
  `location_confidence` sort values plus an `order=asc|desc` parameter;
  the Sources list sorts client-side.
- Sources list filters (enabled/disabled/all, type, scan status, debounced
  name search), a right-aligned enable switch, left-edge row selection
  with a select-all-visible checkbox, and bulk **Delete selected** /
  **Refresh selected** actions. Bulk refresh reuses the existing
  sequential batch helper and aggregate toast; bulk delete uses one
  confirmation that lists the names and explains that offers and
  provenance rows are kept with `source_id` detached.
- Source row actions are now icon-only (refresh/edit/delete) with
  `aria-label` and `title`, and the source name links to the company page
  when a company with the same normalized name exists.
- A searchable company combobox on the Offers page and a searchable,
  sortable, paginated offers panel on the company page.
- The offer detail header is now two-column: identity and merge/split
  actions on the left, an always-visible compact provenance list on the
  right. It stacks at the same 900px breakpoint as the company header.
- A shared `EditViewEditor` component, extracted from the interview-prep
  modal, now also backs the offer description Add/Edit flow.
- A root `Makefile` with `ui`, `build` and `serve` targets, documented in
  the README development section.

#### Changed
- Offers full-text search is now search-as-you-type with a 300 ms
  debounce; no Enter or Search button is required. The offers table splits
  the combined score column into separate sortable Title score and
  Location score columns.
- The offer provenance block no longer uses a per-offer collapsed
  disclosure; `localStorage` provenance-open keys are no longer read or
  written.
- `GET /api/offers` accepts `order=asc|desc` and the three new sort
  values; existing sorts and their historical default directions are
  unchanged when `order` is absent. No schema change.

#### Fixed
- Source name/company links no longer depend on the offers payload; the
  lookup failure leaves the name as plain text instead of breaking the
  list.

#### Tests
- Backend: table-driven DB tests for the new sort values, explicit
  directions, defaults and NULL/empty values; API tests for `order`
  validation and descending title ordering.
- Frontend (Vitest + jsdom): column-picker persistence and the
  last-column guard, header-click sort requests, URL parameter round-trip
  and invalid-value fallbacks, debounced search, page-size persistence,
  searchable company selection, source filters, client sorting, switch
  PATCH, icon actions, company links, bulk delete payloads and bulk
  refresh, company-page table controls, offer header/provenance layout,
  and the shared description Edit/View save payload.
