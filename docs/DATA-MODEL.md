# Data model

SQLite, single file, WAL mode, `foreign_keys=ON`. Schema is created by
numbered migrations in `migrations/` (embedded in the binary) and
tracked in `schema_migrations`. All timestamps are RFC3339 TEXT in UTC,
second precision. Enum-like TEXT columns are validated in Go
(`internal/model`), not with CHECK constraints, so new values do not
require a table rebuild.

## Migrations

| Version | File | Content |
|---|---|---|
| 0001 | `0001_init.sql` | All tables below, indexes, FTS5 tables and triggers |
| 0002 | `0002_known_hard_sources.sql` | Data-only seed: four disabled source rows (Datadog/Cisco Workday, SAP SuccessFactors, WTTJ) with an explanatory `note`. `INSERT … WHERE NOT EXISTS` by URL, so it is safe to re-apply |
| 0003 | `0003_seed_remote_sources.sql` | Data-only seed: the 65 sources decided in Milestones 5a/5b (37 enabled: 11 Greenhouse, 11 Ashby, 4 Lever, 1 Personio, 5 RSS, 5 HTML; 28 disabled not-trackable rows with a `note`). Reproduces each source's name, URL, type, enabled flag and full `config_json`. `INSERT … WHERE NOT EXISTS` by URL; never updates existing rows |
| 0004 | `0004_seed_5c_sources.sql` | Audit-only, data-only no-op: Milestone 5c introduced no new source rows. The We Work Remotely rss row, the Workable search note row and the aggregator rows were already seeded by 0003. Contains no INSERT or UPDATE, so applying it changes nothing and it cannot overwrite or restore user edits |

`schema_migrations(version INTEGER PK, name TEXT, applied_at TEXT)`.
Each migration runs in its own transaction with its bookkeeping row.
Opening a database that records a version unknown to the binary fails.
Milestones 2 and 3 needed no schema change; Milestone 4 adds the
disabled-source rows from 0002 and Milestone 5 ships the 65 registered
sources from 5a/5b in 0003 (both data only, no table or column changes).
Milestone 5c's source decisions were already part of 0003, so 0004 is an
audit-only no-op and the seeded total stays 69 rows.

## Tables

### companies
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| name | TEXT NOT NULL | display name as first seen |
| name_normalized | TEXT NOT NULL UNIQUE | `model.NormalizeCompanyName`: lowercase, accents folded, dots/apostrophes removed, other punctuation → space, trailing legal suffixes removed (gmbh, ag, kg, inc, incorporated, corp, corporation, llc, ltd, limited, plc, sas, sasu, sarl, sa, spa, srl, bv, nv) |
| website | TEXT | nullable |
| created_at | TEXT | |

Companies are resolved by exact `name_normalized` match and created if
missing. `model.SimilarCompanyNames` proposes possible duplicates for
the UI (token containment or small edit distance); nothing is merged
without an explicit `POST /api/companies/:id/merge`. A merge moves
offers and company notes to the target, rewrites offer fingerprints
with the target's normalized name, and deletes the emptied company. A
rename through the API also rewrites fingerprints in the same
transaction. Both operations are rejected (`merge_conflict`) when a new
fingerprint would collide.

### sources
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| name | TEXT NOT NULL | also the default company name of its offers |
| url | TEXT NOT NULL | indexed (not unique; the CLI refuses duplicates) |
| source_type | TEXT NOT NULL | `greenhouse` `lever` `ashby` `personio` `workable` `wttj` `rss` `html` `manual` |
| config_json | TEXT NOT NULL DEFAULT '{}' | see below |
| enabled | INTEGER NOT NULL DEFAULT 1 | boolean |
| last_scan_at | TEXT | start time of the last scan attempt |
| last_scan_status | TEXT | `ok` `error` `blocked`; NULL = never scanned |
| created_at | TEXT | |

`config_json` keys (all optional): `company` (overrides source name as
company), `rate_limit_ms` (min interval between requests, default 2000;
HTML sources are capped at a 5 s minimum), `note` (human-readable
explanation shown in the UI), `board_token` (greenhouse), `site` +
`region` (`""`/`"eu"`, lever), `org` (ashby), `tenant` + `language`
(personio), `account` (workable). Board keys are derived from the source
URL when absent.

RSS mapping keys: `item` (item element; auto-detects `item`/`entry`),
`title_field`, `link_field`, `description_field`, `location_field`,
`company_field`, `uid_field`. A field is a chain of child element local
names; `@name` reads an attribute and `#text` reads the element text.
Defaults: title, link (Atom alternate `href` fallback), description
(`description`/`summary`/`content`), uid (`guid`/`id`).

HTML selector keys: `item_selector` and `title_selector` are required;
`link_selector` (defaults to the title selector), `link_attr` (default
`href`), `location_selector`, `description_selector`,
`company_selector`, `uid_selector`, `uid_attr`. Standard CSS syntax is
supported by cascadia; only the listing page is fetched, once per scan.

Seeded source rows from migration 0002 are disabled and carry a `note`.
They exist so the known-hard Workday/SuccessFactors links and WTTJ are
visible without pretending an adapter can read them. Migration 0003
seeds the 65 sources registered in Milestones 5a/5b with the exact
configuration decided there: the enabled ATS boards and RSS/HTML
listings keep their board tokens, field mappings and selectors, while
the 28 not-trackable rows are disabled and carry their explanatory
`note`. Both seeds only insert missing URLs and never update an existing
row, so re-applying them or keeping a user-edited source is safe.

### job_offers
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| company_id | INTEGER NOT NULL → companies | |
| title | TEXT NOT NULL | as first seen |
| title_normalized | TEXT NOT NULL | `model.NormalizeTitle`: lowercase, accents folded, gender markers (`H/F`, `m/w/d`…) removed, punctuation → space |
| location | TEXT | free text as first seen |
| remote_policy | TEXT NOT NULL DEFAULT 'unknown' | `remote` `hybrid` `onsite` `unknown`; derived by the deterministic classifier from location and description on every sighting (and by `jobwatch reclassify`) |
| url | TEXT | canonical (first-seen) link |
| fingerprint | TEXT NOT NULL UNIQUE | see Fingerprints |
| first_seen_at, last_seen_at | TEXT NOT NULL | |
| offer_state | TEXT NOT NULL DEFAULT 'active' | `active` `changed` `disappeared` |
| application_state | TEXT NOT NULL DEFAULT 'todo' | `todo` `applied` `responded` `interview` `offer` `negative` `maybe` `positive` |
| raw_text | TEXT | latest plain-text description |
| text_hash | TEXT | SHA-256 hex of whitespace-collapsed `raw_text`; NULL when no description |
| title_confidence | INTEGER | 0..100 (CHECK), probability the title is a Platform/SRE/DevOps/Infra role; NULL only until classified |
| location_confidence | INTEGER | 0..100 (CHECK), probability the offer is full remote and France-compatible; NULL only until classified |
| match_reasons_json | TEXT | `{"title":["..."],"location":["..."]}`: human-readable matched terms and weights per score (see Classification below) |
| created_at, updated_at | TEXT | |

Indexes: company_id, offer_state, application_state, last_seen_at.

### Classification columns

- Scores are additive keyword outcomes clamped to 0..100. The title table
  has strong positives (60), weak technology/role terms (15–25), strong
  negatives (marketing, sales, recruiter, internship, account executive,
  French internship/apprenticeship words; −120) and management penalties
  (−20). A strong negative therefore suppresses even a strong positive.
- Location scoring combines remote/hybrid/onsite phrases with France,
  French city, EMEA/EU/Europe and time-zone terms; clearly incompatible
  locations subtract points. Description evidence carries lower weight.
  The derived policy caps the score: onsite ≤ 50, hybrid ≤ 75, unknown
  ≤ 60; only a remote role can reach 100.
- `match_reasons_json` is always valid JSON with two arrays. Each element
  is a readable string such as `strong positive "platform engineer"
  (+60)` or `description "remote" (+25)`. The UI uses the matching array
  as the score-badge tooltip.
- `jobwatch reclassify` fills missing scores; `--all` recomputes every
  offer after a keyword-table change. It does not change `updated_at`.

### job_offer_provenance
One row per (offer, link) sighting. `UNIQUE(offer_id, url)`.

| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| source_id | INTEGER → sources | NULL for manual imports |
| external_uid | TEXT | ATS posting id when available |
| url | TEXT NOT NULL | exact link where seen |
| first_seen_at, last_seen_at | TEXT | `last_seen_at` = start time of the scan that last saw it |

Index: (source_id, external_uid).

### events
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| type | TEXT NOT NULL | `cv_sent` `reply_received` `interview` `offer` `rejected` `ghosted` `other` |
| description | TEXT | |
| occurred_at | TEXT NOT NULL | user-editable |
| created_at | TEXT | |

Index: (offer_id, occurred_at). Written by the events API; `occurred_at`
is user-editable and may be backdated.

### notes
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| entity_type | TEXT NOT NULL | `offer` `company` |
| entity_id | INTEGER NOT NULL | polymorphic, no FK |
| body | TEXT NOT NULL | |
| pinned_event_id | INTEGER → events ON DELETE SET NULL | |
| created_at, updated_at | TEXT | |

Indexes: (entity_type, entity_id), pinned_event_id. Written by the notes
API; a note can be pinned to a timeline event (`ON DELETE SET NULL`
when that event is deleted). The pin must belong to the note's entity
(the offer itself, or an offer of the company).

### Manual import metadata

`POST /api/offers/import` and `jobwatch import` accept a URL without
company/title. The server fetches that page politely and derives
company/title/location/description from JSON-LD `JobPosting`, Open Graph
or `<meta>` tags and the page title before using the normal provenance
path. This is a user-initiated fetch, not a source scan; it stores no
extra metadata beyond the resulting offer/provenance/snapshot rows.

### snapshots
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| raw_text | TEXT NOT NULL | description at capture time (may be empty for offers created without one) |
| captured_at | TEXT NOT NULL | |

Index: (offer_id, captured_at). Every first description and every
text-hash change appends a row. `GET /api/offers/:id` exposes the latest
snapshot and, when at least two exist, a line diff between the two latest
snapshots (oldest → newest) for the changed-offer view.

### Full-text search
- `job_offers_fts` — FTS5, external content `job_offers` (columns
  `title`, `raw_text`), `content_rowid = id`.
- `notes_fts` — FTS5, external content `notes` (column `body`).

Both use `tokenize = 'unicode61 remove_diacritics 2'` and are kept in
sync by AFTER INSERT / DELETE / UPDATE OF triggers
(`job_offers_fts_ai|ad|au`, `notes_fts_ai|ad|au`).

## Fingerprints

`fingerprint = sha256_hex(company_normalized + 0x1F + title_normalized [+ 0x1F + location_bucket])`

- The base fingerprint has no bucket.
- `model.LocationBucket`: first location segment (split on `, ; | / ( )`
  and `" - "`), normalized; if that segment is only a workplace word
  (`remote`, `hybrid`, `onsite`, `anywhere`, `teletravail`, …), the next
  segment is appended (`"Remote, United States"` → `remote united states`).
  Empty = unknown.
- A sighting matches the base-fingerprint offer if either bucket is
  unknown or both are equal. Otherwise the bucketed fingerprint is used
  (looked up, or created for a new offer).

## State machines

### offer_state (written by scans and imports)

| From | Trigger | To |
|---|---|---|
| — | first sighting | `active` (+ snapshot) |
| any | sighting with non-empty description whose hash differs from `text_hash` | `changed` (+ snapshot, `raw_text` replaced) |
| `disappeared` | sighting without text change | `active` |
| `active` / `changed` | sighting without text change | unchanged |
| `active` / `changed` | successful scan of a source that has a provenance row for the offer, no row of that source seen in this scan, and no other source saw it in its own latest `ok` scan | `disappeared` |

- A sighting without description never clears stored text. A first
  description on an offer that had none fills `raw_text` (+ snapshot)
  without becoming `changed`.
- The classifier re-runs on every sighting using the stored title,
  location and latest description; score columns and `remote_policy` are
  updated even when the offer state does not change.
- Failed or blocked scans change no offer.
- Offers are never deleted by scans or imports.
- When several postings of one scan map to the same offer, only the
  first one updates the offer; the others only add provenance.

### application_state
Default `todo`. Transitions are user-driven through
`PATCH /api/offers/:id`; the API validates the enum and bumps
`updated_at` (the audit field). No automatic transition is derived from
timeline events.

### last_scan_status
`ok` after a committed scan; `blocked` when robots.txt disallows the URL
or the server answers 401/403/429; `error` for any other failure.
Sources whose type has no adapter are skipped and keep their status.
`POST /api/sources/:id/scan` returns the summary; the same values are
surfaced on `GET /api/sources`.

## Company merge and rename

- Rename (`PATCH /api/companies/:id`): changes `name` and
  `name_normalized`; every offer of the company gets a new fingerprint
  with the new normalized name. Base fingerprints stay base,
  location-bucketed fingerprints keep their bucket.
- Merge (`POST /api/companies/:id/merge`): offers and company notes of
  the source company move to the target, fingerprints are rewritten the
  same way, the target inherits the source website when it has none,
  and the source company row is deleted. Offers are never deleted.
- Both operations are transactional. If a rewritten fingerprint already
  exists (same company + title [+ location bucket]) the operation is
  rejected with `409 merge_conflict` and nothing changes.
- Deleting a company is refused while it owns offers. Deleting a source
  keeps its provenance rows but sets their `source_id` to NULL.

## Offer merge and split

Both operations are explicit user actions on the offer detail page; scans
never merge or split offers.

- **Merge** (`POST /api/offers/:id/merge` with `into_id`): moves the
  absorbed offer's provenance rows, events, offer notes and snapshots to
  the target, keeps the earliest `first_seen_at` / latest `last_seen_at`,
  collapses duplicate provenance URLs (earliest first-seen, latest
  last-seen), adopts the absorbed application state when the target is
  `todo`, and keeps `changed` when either side was changed. The absorbed
  `job_offers` row is deleted only after all children have moved; no
  events, notes, snapshots or provenance rows are lost.
- **Split** (`POST /api/offers/:id/split` with `provenance_ids`, optional
  `title`, optional `location`): creates a second offer in the same
  company and moves the selected provenance rows to it. Events, notes and
  snapshots stay with the original because they describe the tracked
  application. At least one provenance row must remain on the original.
  The new offer gets `active` (or `disappeared` when the original was
  disappeared), application state `todo`, and a fingerprint computed from
  the company, new title and location; a collision is rejected with
  `409 offer_conflict`. The original canonical `url` is repaired to a
  remaining link when the moved row owned it.

## Full-text search exposure

`job_offers_fts` backs `GET /api/offers?q=`. `notes_fts` is maintained by
the triggers but is not exposed through a search endpoint yet.
