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

`schema_migrations(version INTEGER PK, name TEXT, applied_at TEXT)`.
Each migration runs in its own transaction with its bookkeeping row.
Opening a database that records a version unknown to the binary fails.
Milestone 2 (REST API + web UI) needs no schema change; it uses the
tables, indexes and FTS5 triggers created by 0001.

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
company), `rate_limit_ms` (min interval between requests, default 2000),
`board_token` (greenhouse), `site` + `region` (`""`/`"eu"`, lever),
`org` (ashby). Board keys are derived from the source URL when absent.

### job_offers
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| company_id | INTEGER NOT NULL → companies | |
| title | TEXT NOT NULL | as first seen |
| title_normalized | TEXT NOT NULL | `model.NormalizeTitle`: lowercase, accents folded, gender markers (`H/F`, `m/w/d`…) removed, punctuation → space |
| location | TEXT | free text as first seen |
| remote_policy | TEXT NOT NULL DEFAULT 'unknown' | `remote` `hybrid` `onsite` `unknown` (classifier not yet implemented: always `unknown`) |
| url | TEXT | canonical (first-seen) link |
| fingerprint | TEXT NOT NULL UNIQUE | see Fingerprints |
| first_seen_at, last_seen_at | TEXT NOT NULL | |
| offer_state | TEXT NOT NULL DEFAULT 'active' | `active` `changed` `disappeared` |
| application_state | TEXT NOT NULL DEFAULT 'todo' | `todo` `applied` `responded` `interview` `offer` `negative` `maybe` `positive` |
| raw_text | TEXT | latest plain-text description |
| text_hash | TEXT | SHA-256 hex of whitespace-collapsed `raw_text`; NULL when no description |
| title_confidence | INTEGER | 0..100 (CHECK), NULL until classified |
| location_confidence | INTEGER | 0..100 (CHECK), NULL until classified |
| match_reasons_json | TEXT | NULL until classified |
| created_at, updated_at | TEXT | |

Indexes: company_id, offer_state, application_state, last_seen_at.

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

### snapshots
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| raw_text | TEXT NOT NULL | description at capture time (may be empty for offers created without one) |
| captured_at | TEXT NOT NULL | |

Index: (offer_id, captured_at).

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
- Failed or blocked scans change no offer.
- Offers are never deleted.
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

## Full-text search exposure

`job_offers_fts` backs `GET /api/offers?q=`. `notes_fts` is maintained by
the triggers but is not exposed through a search endpoint yet.
