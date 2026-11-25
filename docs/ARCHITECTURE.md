# Architecture

Single Go binary (`cmd/jobwatch`), SQLite via `modernc.org/sqlite`
(pure Go), CLI via Cobra. No external services.

## Packages

| Package | Role |
|---|---|
| `cmd/jobwatch` | Cobra root + subcommands: `serve`, `scan`, `add-source`, `add-sources`, `list`, `import`, `backup` |
| `migrations` | Numbered `NNNN_name.sql` files embedded with `go:embed` |
| `internal/db` | `Open` (DSN pragmas: WAL, foreign_keys, busy_timeout 5s, `_txlock=immediate`), migration runner, `Backup` (`VACUUM INTO`), queries over a `Querier` (`*sql.DB` or `*sql.Tx`) |
| `internal/model` | Table structs, enum constants, `normalize.go` (company/title keyword normalization, location bucket, fingerprint, text hash), time helpers |
| `internal/sources` | `Fetcher` interface, `RawOffer`, `Config` (config_json), `Registry`, polite HTTP `Client`, robots.txt parser, URL type detection, adapters |
| `internal/scan` | Scan pipeline and manual import; depends on `db`, `classify` and `sources`, shared by CLI and API |
| `internal/importmeta` | Lightweight metadata extraction from a job page (JSON-LD, Open Graph, meta tags, title) for URL imports |
| `internal/classify` | Pure read-time scoring: title affinity, location affinity, fixed remote-policy detector and term reasons, from a search-profile value |
| `internal/diff` | Small line diff used for snapshot version comparisons |
| `internal/api` | REST handlers under `/api`: sources, offers, events, notes, companies, import, health |
| `web/` | React + TypeScript SPA source and its Go embedding (`Handler`) |

## Fetching

```go
type Fetcher interface {
    Fetch(ctx context.Context, src model.Source) ([]RawOffer, error)
}
type RawOffer struct {
    ExternalUID, Title, Location, URL, Description string
    Company    string                  // display-name evidence
    CompanyKey string                  // namespaced attested identifier
    IncompleteReason model.IncompleteReason // set when the item is unusable
}
```

Every adapter emits a stable uid, the canonical title and location, and
company evidence. Typed adapters set `CompanyKey` to their existing token
(`greenhouse:<board_token>`, `lever:<site>`, `ashby:<org>`,
`personio:<tenant>`, `workable:<account>`, `bamboohr:<subdomain>`,
`wttj:<org slug>`); RSS/HTML have no token and rely on the declared
`company` or the employer rule. An item missing a title or company is
returned with `IncompleteReason` (`missing_title`, `missing_company`,
`missing_company_title`) instead of being silently dropped.

Adapters (`sources.NewRegistry`):

| Type | Request | Notes |
|---|---|---|
| greenhouse | `GET https://boards-api.greenhouse.io/v1/boards/{board_token}/jobs?content=true` | `content` is entity-escaped HTML → unescaped → plain text; `company_name` fills `RawOffer.Company` |
| lever | `GET https://api.lever.co/v0/postings/{site}?mode=json` (`api.eu.lever.co` when `region=eu`) | description = `descriptionPlain` + lists + `additionalPlain`; multiple `allLocations` joined with `; ` |
| ashby | `GET https://api.ashbyhq.com/posting-api/job-board/{org}` | `isListed=false` skipped; secondary locations joined with `; ` |
| personio | `GET https://{tenant}.jobs.personio.{de,com}/xml` | `position` elements; office is the location, `id` builds `{tenant}/job/{id}`, `jobDescriptions` joined as titled HTML→text blocks; optional `language` query |
| workable | `GET https://apply.workable.com/api/v1/widget/accounts/{account}?details=true` | keyless widget API; entries with the same `shortcode` are grouped, all `locations` joined, `telecommuting` adds a remote marker; `name` is the company |
| bamboohr | `GET https://{subdomain}.bamboohr.com/careers/list` | public JSON listing; `id` is the external UID and builds `/careers/{id}`, `jobOpeningName` the title, `location`/`atsLocation` fields joined (empty parts and repeated city/state dropped), `isRemote` adds a remote marker; no per-offer description fetch, so `Description` stays empty |
| wttj | `GET https://api.welcometothejungle.com/api/v1/pages?path={careers path}`, then `POST https://{app_id}-dsn.algolia.net/1/indexes/*/queries`, plus one `GET` per description-less offer | credentials, index, filters and page language are discovered from the page tree at every scan; the Algolia call sends a browser `User-Agent`, the fixed `Referer` and the form content type with a JSON body; `reference` is the external UID (`objectID` fallback), `profile` markdown is stored verbatim, offers are paged at 50 hits with the page's own filters sent as `facetFilters`; when the first page's `nbHits` differs from the page's job count, the canonical slug of every filter value is read from `/api/v1/organizations/{slug}` and the page is retried once; when `profile` and `summary` are both blank and the offer has no stored description, the offer page is fetched through the shared client with the fixed WTTJ browser User-Agent and its JSON-LD `JobPosting.description` is stored as plain text (a failed or WAF-challenged page leaves the description empty without failing the source) |
| rss | `GET <source url>` | RSS 2.0 and Atom; item/field mapping in config (`item`, `title_field`, `link_field`, `description_field`, `location_field`, `company_field`, `uid_field`); paths are child local names joined with `/` (e.g. `locations/location/name`) with `@attr` and `#text`; Atom alternate links and HTML descriptions handled; optional `employer` rule splits `Employer: Role` or reads a separate field |
| html | `GET <source url>`, plus one `GET` per eligible offer when `detail_description_selector` is set | last resort: CSS selectors for the listing page (`item_selector`, `title_selector`, …); cascadia supports `:has()` and `:haschild()` for row containers; one listing request per scan; minimum interval 5 s (also for detail pages); optional `employer` rule (`separator` and/or `selector`) rewrites company and title; the optional detail selector fetches an offer page only when the offer has no stored description or its stored text still equals the listing summary (so a first scan or upgrade makes one request per item, later scans none), falls back to the listing text on a per-item miss, and stops the detail phase after a blocked response while the source stays `ok` |

`manual` has no fetcher; manual offers come from the import path.
Disabled seeded rows are `html`/`wttj` sources with a `note` in
`config_json`: the known-hard Workday/SuccessFactors and WTTJ
marketplace rows from migration 0002 and the 28 not-trackable decisions
from 5b. Migration 0003 seeds the 65 sources
registered in 5a/5b with their decided config. Migration 0004 is an
audit-only no-op: the 5c-decided rows (We Work Remotely, Workable search
and the aggregator notes) were already in 0003, so it inserts nothing.
Migration 0005 is a data-only seed for the BambooHR source added in 6a
(Semble, `bamboohr`, `{"subdomain":"semble"}`). Migration 0006 is a
data-only, in-place rename of the old `reply_received` event value to
`message_received`; no schema or source rows change. Migration 0011 is a
data-only seed for the WTTJ source added in 10a (Fabriq,
`wttj`, empty config; the page URL is the only setting).

`sources.Detect(url)` maps public board URLs (`boards.greenhouse.io/x`,
`job-boards[.eu].greenhouse.io/x`, `jobs[.eu].lever.co/x`,
`jobs.ashbyhq.com/x`, `{tenant}.jobs.personio.{de,com}`,
`apply.workable.com/{account}`, `{account}.workable.com`,
`{subdomain}.bamboohr.com/careers[/list|/{id}]`,
`www.welcometothejungle.com/<lang>/companies/<slug>/jobs`, and the API
URLs) to a type + config. Adapters use config keys first, then fall back
to detection on the source URL. The Greenhouse corporate site and the
candidate dashboard (`www.greenhouse.io`, `my.greenhouse.io`) are not
boards and are rejected.

## Source registration (CLI)

`add-source` and `add-sources` share `sourceFromURL` in
`cmd/jobwatch/addsource.go`: HTTP(S) validation, `sources.Detect`, the
`--type` override (single URL only), config merging and the default
name (`board_token` / `site` / `org` / `tenant` / `account`, else the
WTTJ company slug or the hostname). Both reject `manual` and check `sources.url` for duplicates
through `db.FindSourceByURL`.

`add-sources` first parses the file: blank lines and `#`-prefixed lines
are skipped, and a `#` preceded by whitespace starts an inline comment,
so URL fragments survive. It keeps exact-URL duplicates out (first line
wins) and never stops on a bad line. Each line ends up in one of three
buckets: added, already registered (including in-file duplicates) or
not detected (invalid URL or no known adapter). The report and a plain
list of undetected URLs go to stdout.

### Polite client (`sources.Client`)

- User-Agent `Mozilla/5.0 (compatible; JobWatch/0.1; personal job-search tracker; +https://github.com/stepbeta/jobwatch)`; robots token `JobWatch`.
- robots.txt per scheme+host, cached 24 h per process. RFC 9309
  handling: 2xx parsed (specific group over `*`, longest match, `*`/`$`
  wildcards, `Crawl-delay`); 4xx = allow all; 429 = blocked; 5xx/network
  error = error, not cached.
- Rate limit: requests to the same host are spaced by the source's
  `rate_limit_ms` (default 2 s) or the robots `Crawl-delay`, whichever is
  larger. The limiter is keyed by host so several sources on one ATS
  share it. The robots.txt fetch counts as a request. HTML sources never
  run faster than every 5 s, even if `rate_limit_ms` asks for less.
- 30 s timeout, 20 MiB body cap. HTTP 401/403/429 and robots disallow
  wrap `ErrBlocked`.

## Scan pipeline (`scan.Scanner`)

Per source (`ScanSource`), `ScanAll` iterates enabled sources by ID and
never stops on a failing one:

1. `now` = scan start (UTC, seconds); fetch outside any transaction.
2. Fetch error → `last_scan_status = blocked|error`, `last_scan_at = now`, stop.
3. One transaction:
   - for each `RawOffer`: resolve the offer through provenance,
     `(source, external_uid)` first and `(source, url)` second. An item
     without a URL has no provenance identity and is skipped. The
     pipeline recomputes usability from the fields rather than trusting
     the adapter hint: a missing company or title (or both) stores an
     **incomplete offer** instead of skipping the item. The incomplete
     row keeps the known company, its URL/provenance and any snapshot,
     and stores the values the source reported as JSON in
     `incomplete_raw`; `incomplete_reason` records what is missing and
     `Result.Incomplete` is incremented. A rescan of the same identity
     only refreshes `last_seen_at`/provenance.
   - for a usable item, company evidence is `RawOffer.CompanyKey`
     (adapter token) and/or `RawOffer.Company` → `config.company`;
     `ResolveCompany` tries the key, then the source alias, then the
     confirmed global alias, and creates a company only when none
     matches. The source name is not a company fallback. Insert the
     offer (+ snapshot with `origin = scan`). On a hit, update the title
     and a reported location, re-derive the remote policy when the
     location or description changed, apply the offer_state machine
     (DATA-MODEL.md), keep a new description as a snapshot, and
     never rewrite the company or canonical URL from the sighting.
     Classify only the remote policy fact from the location and
     description, store it with the other facts, and upsert provenance by
     `(source, external_uid)` first, then `(offer_id, url)`, with
     `last_seen_at = now`. Title and location affinity are not computed or
     stored during ingestion; they are read-time values. A
     repaired offer is identified by `incomplete_raw` still being set
     after `incomplete_reason` was cleared: scans then keep the user's
     company, title and location instead of the sighting's values.
   - `MarkDisappeared(source, now)` returns the offers it transitioned
     with their application state. For each `applied` or `interview`
     offer, insert one `ghosted` event and set `negative` in the same
     transaction; other states are left untouched. The
     `offer_state != 'disappeared'` predicate makes this idempotent, and
     no backfill runs for offers that disappeared before this behavior
     existed.
   - `last_scan_status = ok`, `last_scan_at = now`; commit.
4. A failure inside the transaction rolls back and marks the source `error`.

Incomplete offers are stored, not displayed: the offers list, dashboard
and search exclude `incomplete_reason IS NOT NULL` by default
(`OfferQuery.IncompleteOnly` lists them for the repair view), and the CLI
`list` hides them. `POST /api/offers/:id/repair` assigns a company and
title, re-derives the remote policy, and clears `incomplete_reason` while
keeping `incomplete_raw` as the repair marker.

## Re-attribution (`jobwatch reattribute`)

`scan.ReattributeSource` re-fetches one source and corrects offers whose
company or title predates an adapter change. It reads the stored
provenance rows for the source with a non-empty uid, matches the
adapter's current `RawOffer` by `(source, uid)`, and for each match:

- resolves the target company with the scan order (attested key, source
  alias, confirmed global alias, create);
- writes only `company_id`, `title`/`title_normalized` and `updated_at`,
  then re-derives the offer's remote policy;
- leaves events, notes, snapshots, `application_state`, `offer_state`,
  `last_seen_at` and provenance untouched.

It skips user-repaired offers (`incomplete_raw` non-null), offers with
no current uid match, and adapter items that are still incomplete. The
pass is idempotent: a second run with the same source output changes
nothing. Per source it reports fetched, re-attributed, unchanged and
skipped counts plus the companies left without offers; an emptied
company stays visible for the user to delete. `ReattributeAll` loops the
enabled sources; the CLI adds `--all` and `--source <id>`.

## Classification (`internal/classify`)

The classifier is deterministic and pure. It holds no term tables and has
no I/O: the term weights are user preferences stored in
`search_profile_terms`, and the caller passes a `model.SearchProfile`.
The same facts and profile always produce the same result. Nothing is
scored by an LLM or in the background.

The model separates three things:

- **Facts** on `job_offers`: title, location, description and
  `remote_policy`.
- **Preferences** in a search profile: title, location and description
  terms with weights, a 3/5 bucket display scale, and the accepted remote
  policies.
- **Scores** computed at read time: title affinity, location affinity and
  remote policy acceptance. No affinity score is written to an offer.

`classify.Score(title, location, description, profile)` returns the three
signals:

- **Title affinity** (`ScoreTitle`): the offer title is normalized with
  `model.NormalizeTitle`, each `title` term of the profile is matched as a
  whole-token phrase, and the matching weights are added and clamped to
  0–100. Negative terms therefore lower the score and can bury an
  otherwise good title, while neutral terms (weight 0) only appear in the
  reasons.
- **Location affinity** (`ScoreLocation`): the location is normalized with
  `model.NormalizeKeywords` and matched against the profile's `location`
  terms; the description is normalized separately and matched against its
  `description` terms. Both sets add into one 0–100 score. Description
  terms add weaker geographical evidence and never influence the remote
  policy.
- **Remote policy** (`DeriveRemotePolicy`): a fixed phrase detector over
  the location and description produces `remote`, `hybrid`, `onsite` or
  `unknown`. A hybrid phrase, or remote and onsite phrases together, is
  hybrid. The user cannot edit the phrase lists. `RemoteAccepted` is a
  profile membership check on the derived policy: remote is a fact plus a
  checkbox preference, never a weighted score.

Each affinity also returns its reasons as structured matches (term,
weight, bucket). The API serializes them; the SPA only displays them.

The removed pieces: the Go title/location/description term tables, the
policy points and caps on the location score, the stored `title_confidence`
/ `location_confidence` / `match_reasons_json` writes, and the
`jobwatch reclassify` command. A profile edit changes the next read with
no reclassification pass.

## Search profiles

User preferences are stored in three additive tables (migration 0015),
separate from the offer facts in `job_offers`:

| Table | Columns | Relationship |
|---|---|---|
| `search_profiles` | `id`, `name`, `title_label`, `location_label`, `bucket_scale`, `is_default`, `created_at`, `updated_at` | root |
| `search_profile_terms` | `id`, `profile_id`, `affinity` (`title`/`location`/`description`), `bucket`, `term`, `weight`, `sort_order` | `profile_id → search_profiles` ON DELETE CASCADE |
| `search_profile_remote` | `profile_id`, `policy`, `accepted` | `profile_id → search_profiles` ON DELETE CASCADE |

A partial unique index enforces at most one `is_default = 1` profile;
`search_profile_terms` is unique on `(profile_id, affinity, term)` and
`search_profile_remote` is keyed on `(profile_id, policy)`. Migration 0015
seeds one default profile, name `Default`, five-bucket scale, with the
exact former classifier term tables: 35 title, 79 location and 15
description terms; it accepts the `remote` and `unknown` policies and
rejects `hybrid` and `onsite`. The `bucket` column is a display label
(three- or five-bucket scale); the scorer adds `weight`.

The legacy `job_offers.title_confidence`, `location_confidence` and
`match_reasons_json` columns are deprecated and unused from 0015 on: the
schema keeps them for rollback, no code path reads or writes them, and no
API response exposes them. `internal/db/search_profiles.go` provides the
default-profile lookup, list, get and the transactional replace used by
the API; validation lives in `model.SearchProfile.Validate` so the API
and the database reject the same input.

## Manual import and URL metadata (`scan.Import`, `importmeta`)

Input: `{company?, title?, location?, url?, description?}`. When a URL is
present and company or title is missing, `importmeta.Extract` fetches the
page through the same polite client (robots.txt, rate limit, UA, timeout)
and reads JSON-LD `JobPosting`, Open Graph/meta tags and the page title.
Explicit fields win; the canonical JSON-LD URL is used only when the
input has no URL. A complete result runs the same sighting logic
(including classification) in one transaction with `source_id = NULL`;
a URL already linked to an offer attaches the import to that offer, while
a new URL creates a new offer even when the company and title match
another one (title and company are attributes, not identity). A manual
import does not rewrite the title or location of an offer it attaches to.
If metadata is incomplete the API returns `422` with the extracted fields
for the UI's manual fallback; the CLI asks for `--company`/`--title` or
`--no-fetch`. Import never scans a source.

## REST API (`internal/api`)

`api.Server{DB, Version, Fetcher, Now}` mounts Go 1.22 `ServeMux`
patterns under `/api/`. The fallback `/api/` pattern answers unknown
methods and paths with `404 not_found`. Bodies are decoded strictly
(unknown fields rejected, 1 MiB cap); responses use snake_case DTOs
defined in `internal/api/dto.go`, independent of the `model` structs.
Errors always use `{"error":{"code","message"}}`.

| Group | Routes |
|---|---|
| health | `GET /api/health` |
| sources | `GET/POST /api/sources`, `GET/PATCH/DELETE /api/sources/{id}`, `POST /api/sources/{id}/scan` |
| offers | `GET /api/offers`, `POST /api/offers/import`, `GET/PATCH /api/offers/{id}`, `GET /api/offers/{id}/snapshot-diff`, `POST /api/offers/{id}/merge`, `POST /api/offers/{id}/split` |
| events | `GET/POST /api/offers/{id}/events`, `PATCH/DELETE /api/offers/{id}/events/{eventID}` |
| notes | `GET/POST /api/notes`, `GET/PATCH/DELETE /api/notes/{id}` |
| companies | `GET/POST /api/companies`, `GET/PATCH/DELETE /api/companies/{id}`, `POST /api/companies/{id}/merge` |

`POST /api/sources/{id}/scan` builds a `scan.Scanner` from the server's
fetcher and runs one source synchronously, returning the `scan.Result`
plus the refreshed source row. A fetch failure is part of the summary
(HTTP 200) and is recorded in `last_scan_status`; an unimplemented
adapter is reported as `skipped` without touching the status. The
default fetcher is one `sources.Registry` with one shared
`sources.Client`, created lazily per server so the robots cache and
per-host rate limiter survive across requests. The same client is used
for import URL metadata, so a user-triggered import also honors
robots.txt and the per-host interval.

Offer listing is `db.QueryOffers`: a shared WHERE builder for the count
and page queries, FTS5 `MATCH` against `job_offers_fts` (validated up
front so a syntax error is a 400), enum lists, score minima,
`has_application`, company filter, five sort orders, `limit`/`offset`.
`recent_event` sorts by `MAX(events.occurred_at)` with `updated_at` as
fallback. Detail responses join provenance with source names, events,
offer notes, the snapshot version history (metadata only), the latest
snapshot, the snapshot count and a line diff between the two latest
snapshots (`snapshot_diff`) when two exist. `?snapshot_id=N` adds the full
text of that version; `GET /api/offers/{id}/snapshot-diff?from=N&to=M`
reuses `internal/diff` for any two versions (ordered older → newer).
`PATCH /api/offers/{id}` applies application/offer state, location and
description changes in one transaction; setting `application_state =
saved` records `saved_at` and any other state clears it. A description
edit recomputes `text_hash`
and appends a snapshot with `origin = manual`. Scan and manual-import snapshots use
`origin = scan`. Event types are validated in Go; `message_received`
replaced `reply_received` (migration 0006) and `message_sent` was added.

## Offer merge and split (`internal/db`)

An explicit offer merge (`POST /api/offers/{id}/merge`) moves every
provenance row, event, offer note and snapshot of the absorbed offer to
the target, keeps the earliest `first_seen_at` and latest `last_seen_at`,
adopts the source application state when the target is still `none`,
promotes a `saved` target to a real application carried by the absorbed
offer, keeps `saved_at` consistent with the resulting state, and keeps
the offer active if either side was active; only two disappeared
offers stay disappeared. Duplicate provenance URLs are
collapsed into one row with the earliest first-seen and latest last-seen.
The absorbed offer row is then deleted after all children have moved;
nothing is lost. Only the UI's explicit merge action calls this.

A split (`POST /api/offers/{id}/split`) takes the selected provenance
rows and creates a second offer in the same company, with an optional
title/location override. Events, notes and snapshots stay on the original
because they belong to the tracked application. At least one link must
remain on the original. The new offer gets a base fingerprint; a
company+title+location match with another offer is allowed because
identity is provenance, not the fingerprint.

## Company matching, rename and merge (`internal/db`)

`model.SimilarCompanyNames` is a conservative pure function used for
the UI's "possible duplicate" prompt: token containment (`qonto` /
`qonto france`, at most two extra tokens) or edit distance ≤ 2 on names
of 5+ characters with ≤ 20% changes. Scores below
`model.MinCompanyCandidateScore` (60) are dropped. Nothing merges
automatically.

`db.ResolveCompany` takes `{Key, Name, SourceID}`. It tries the
namespaced adapter key, then the name alias for that source, then the
confirmed global alias, and creates a company only when none matches.
A name-only claim is recorded for its source, so the same name from two
sources creates two companies until the user merges them; the source
name is never used as a fallback. A new company gets a source alias
(or a global alias for a manual import), and a key/alias that resolves
an existing company is linked to it.

`db.MergeCompanies` moves offers, company notes, keys and aliases to the
target, records the absorbed display name as a confirmed global alias,
copies the website when the target has none, and deletes the emptied
company. A key or alias the target already holds is kept once. Rename
changes `name`/`name_normalized` and records both the old and new name
as global aliases. Neither operation rewrites offer fingerprints: names
are attributes and identity is provenance. Deleting a company is
refused while it owns offers; deleting a source detaches its provenance
rows (`source_id` becomes NULL) instead of deleting offers.

## Web UI (`web/`)

React 19 + TypeScript built by Vite. Source in `web/src`:

| Path | Content |
|---|---|
| `App.tsx` | top bar with navigation (including the incomplete count), source loading, sequential scan helpers (single source and batch), auto-dismissing toast stack, routing |
| `router.ts` | hash routing (`#/`, `#/offers`, `#/incomplete`, `#/sources`, `#/interview-prep`, `#/search-profile`, `#/offers/:id`, `#/companies/:id`) plus helpers to read and `replaceState` the current route's query string |
| `api.ts` | typed fetch wrapper and API error mapping |
| `pages/Dashboard.tsx` | Applications section (`has_application`, `sort=recent_event`) with a Refresh button that fans out over the enabled sources behind those applications, and a Saved section (`application_state=saved`, `sort=saved`) |
| `pages/OffersPage.tsx` | full offers table: URL-restored filters (including remote acceptance, minimum title/location affinity and a searchable company combobox), debounced FTS search, header-click server sorting (including `title_affinity`/`location_affinity`), column picker, per-page selector and pagination, and a header "Add offer" button that opens the manual import dialog (URL fetch-and-prefill plus manual fields) |
| `pages/IncompletePage.tsx` | incomplete offers with the count, the source-reported company/title/location, the reason and source link; a repair modal reuses the manual import layout and company combobox and calls `POST /api/offers/:id/repair` |
| `pages/SourcesPage.tsx` | source list with enabled/type/status/name filters, URL-restored client sorting and pagination, column picker, row selection with bulk refresh/delete, right-aligned enable switch, icon row actions, company links resolved from the local company list, header Refresh all, and an add/edit modal with type-dependent config fields (personio/workable/rss/html included) and explanatory notes on seeded sources |
| `pages/OfferPage.tsx` | two-column header: identity (editable states/location, title/location affinity badges with term reasons, remote policy badge, merge/split actions, description version count) on the left and an always-visible compact provenance list on the right; timeline with backdating, inline event editing and message event labels, notes with event pin, bidirectional note↔event cues (pinned-note badge scrolls to and highlights the event; an event's note-count marker expands its pinned notes inline), and a single-version description browser (prev/next, version dropdown, scan/manual origin badge, on-demand diff against the previous version, Add/Edit manual save through the shared Edit/View editor) |
| `components/icons.tsx` | dependency-free inline SVG event-type and action icons (edit, delete, note, refresh) |
| `pages/CompanyPage.tsx` | one two-column header block: company identity (including its attested keys), actions and possible-duplicate banner on the left, company notes (list, new-note form, add/delete) on the right; a separate offers panel below with URL-restored FTS search, server-side sorting, per-page selector and pagination. The grid collapses at 900px (same breakpoint as `.detail-columns`) |
| `pages/SearchProfilePage.tsx` | edits the default search profile: name and labels, 3/5 bucket scale, title/location term rows (bucket plus numeric weight), an advanced description-terms section, and remote policy checkboxes; explanatory text states what each signal changes. It is a display/edit layer: the API computes affinity |
| `pages/InterviewPrepPage.tsx` | personal interview FAQ grouped into category sections, with an empty-answer cue, native HTML5 drag-and-drop for entries and sections (persisting `display_order`/`category` through PATCH; section order in `localStorage`), per-entry Edit/Delete, and an Add/Edit modal with a category dropdown and the shared Edit/View answer editor |
| `components/Markdown.tsx` | deliberately small markdown renderer for interview answers: headings, bold, italic, inline code, fenced code blocks and lists, emitted as React elements (never HTML) |
| `components/ConfirmDialog.tsx` | shared confirmation dialog used for every destructive action (delete source(s)/event/note/company) |
| `components/Modal.tsx` | shared dialog shell (fixed backdrop, titled head, Escape/backdrop close, wide variant) used by the source add/edit form, the offer import dialog and the incomplete repair dialog |
| `components/OfferTable.tsx` | reusable offers table with optional/selectable columns, optional company/application columns, title/location affinity score badges, a remote policy badge with its profile acceptance, and clickable sortable headers driven by the caller's server sort key |
| `components/TableControls.tsx` | shared column picker with localStorage persistence of hidden columns, `SortHeader`, per-page selector, debounced-value hook and sort toggle helper |
| `components/CompanyCombobox.tsx` | searchable client-side company selector for the offers filter; keeps the `company_id` parameter unchanged |
| `components/EditViewEditor.tsx` | shared Edit/View tabbed editor (textarea plus caller-rendered view), used by interview answers and offer descriptions |

The Search profile page (`#/search-profile`) edits the default profile
through `GET`/`PUT /api/search-profiles`. Choosing a bucket sets a
default numeric weight and the number stays editable; switching the
bucket scale maps existing buckets (strong/mild positives collapse to
`positive`, strong/mild negatives to `negative`). Description terms live
in an advanced disclosure and the copy states that they affect location
affinity only, never remote detection. Saving a profile does not rewrite
offers: the SPA reloads affinities from the API, which computes them
from the current profile.

The dashboard (`#/`) contains only Applications and Saved. The full
filtered/paginated offers table and the manual import dialog live on
`#/offers`; the Saved section's "Show all" link opens
`#/offers?application_state=saved&sort=saved`. The Offers header "Add offer" button opens the
import form in the shared `Modal`. A successful import closes the
dialog, toasts the offer id, refreshes the app data and navigates to the
new or matched `#/offers/:id` detail page, so the id is always
reachable. A `422 metadata_*` response keeps the dialog open and
prefills the extracted fields for manual entry. Saving is the `saved`
application state: `saved_at` is set exactly when the state is `saved`
and cleared otherwise, and a saved offer is not an application.

The Incomplete page (`#/incomplete`) lists the offers a source could not
normalize (`GET /api/offers?incomplete=true`) with the item count, the
reported values, the reason and the source link. Its Repair dialog
reuses the import form layout and the company combobox, calls
`POST /api/offers/:id/repair`, and refreshes the list and the navigation
count. Incomplete offers are absent from the Offers table, dashboard and
search until repaired.

The Offers and Sources tables share `components/TableControls.tsx`.
Column pickers persist the hidden column ids (`offers.columns`,
`sources.columns`) rather than the visible set, so an unknown saved name
is ignored and a column added later is visible by default; the last
visible column cannot be hidden. Header clicks toggle ascending and
descending order. On Offers and the company page the sort is server-side
(`GET /api/offers?sort=...&order=...`); on Sources it is a client-side
sort of the already loaded list. Search inputs are debounced by 300 ms
before they trigger a request or a URL rewrite. Active filters, sort,
order, page and `per_page` live in the hash-route query string and are
restored on load via `replaceState`, so the page state does not depend on
the URL parser in `App.tsx`.

Sources resolve a company link by the effective company name (the
`config.company` override, else the source name) against the local
company list (`GET /api/companies?limit=500`, exact normalized name).
The sources payload does not carry the board-reported company, so a board
whose reported company differs from its configured/source name stays
plain text until an override is set; this avoids adding an endpoint for a
display-only link. Deleting sources (single or bulk) keeps offers and
provenance rows; the provenance URL and external UID are untouched and
`source_id` is detached by the existing delete endpoint. Bulk refresh
reuses `scanBatch` and runs one request at a time.

The company page (`#/companies/:id`) renders identity and notes in one
two-column header panel: company name/website/offer count/created time
and the attested keys, Edit and Delete company, and the
possible-duplicate banner on the left; the unchanged company-note list
and new-note form on the right. The
offers table remains a separate panel below. Both columns stack at
`max-width: 900px`, the breakpoint already used by `.detail-columns`.

The interview-prep page (`#/interview-prep`) is independent of offers and
companies. It groups the personal FAQ into one section per category
(alphabetically, uncategorized last) and shows a small amber "No answer"
badge on entries whose answer is empty. The question is the always-visible
header row with Edit and Delete actions; expanding it renders the stored
markdown answer. Entries and section headers use native HTML5 drag-and-drop
(no dependency). Dropping an entry assigns fresh `display_order` values in
the affected section(s) and PATCHes its `category` when it crosses
sections; dropping a section header reorders the presentation-only section
list, saved as a JSON category array in `localStorage` with a "Reset to
alphabetical" control. Add and Edit share one modal with a question input,
a category dropdown (existing names plus an inline "Add new category…"
input) and a single large answer area with an Edit/View toggle; View uses
the same markdown renderer. The renderer in `components/Markdown.tsx`
supports only headings, bold, italic, inline code, fenced code blocks and
lists, and returns React text nodes instead of HTML, so raw HTML in an
answer is shown literally and never executed. Delete goes through the
shared `ConfirmDialog`.

The Sources header owns "Refresh all"; the top bar only navigates. The
Dashboard's Applications Refresh is a client-side fan-out over
`POST /api/sources/:id/scan`: it reads every offer with
`application_state` not in (`none`, `saved`), resolves their provenance `source_id`s via
the offer detail endpoint, keeps the enabled sources, and scans them
sequentially (one request at a time) so the per-host rate limiter is
respected. Manual provenance rows (`source_id` NULL) contribute no
source; partial failures are reported per source and never abort the
batch.

`vite.config.ts` proxies `/api` to `127.0.0.1:8080` in development.
`vitest.config.ts` runs the component tests in jsdom; they mock `api.ts`
and need no server or network.

In production the built `web/dist` is embedded by `web/embed.go`
(`//go:build ui`, `//go:embed all:dist`). `web/spa.go` serves the files
with immutable caching for `assets/` and an `index.html` fallback for
client-side routes. Without the `ui` build tag, `web/noui.go` serves a
placeholder page explaining how to build the SPA, so plain
`go build ./...` works on a fresh checkout.

## Backup

`db.Backup` uses `VACUUM INTO` (safe with a live WAL database, dest must
not exist), then reopens the copy read-only and runs `PRAGMA
integrity_check`, `PRAGMA foreign_key_check` and a `schema_migrations`
check. A copy that fails validation is removed and the command exits
non-zero.

## Server

`jobwatch serve` binds `127.0.0.1:8080` by default. `/api/` is the REST
API; everything else is the SPA (embedded when built with `-tags ui`,
placeholder otherwise).
