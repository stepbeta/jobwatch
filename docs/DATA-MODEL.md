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
| 0005 | `0005_seed_bamboohr_sources.sql` | Data-only seed: the Semble BambooHR source verified in Milestone 6a (`bamboohr`, `{"subdomain":"semble"}`, enabled). `INSERT … WHERE NOT EXISTS` by URL; never updates existing rows |
| 0006 | `0006_event_message_types.sql` | Data-only rename for Milestone 7b: `UPDATE events SET type = 'message_received' WHERE type = 'reply_received'`. No table or column change; the WHERE clause makes re-application match no rows |
| 0007 | `0007_snapshot_origin.sql` | Additive schema change for Milestone 7c: `ALTER TABLE snapshots ADD COLUMN origin TEXT NOT NULL DEFAULT 'scan'`. Existing snapshots keep their text/timestamps and read as `scan`; new inserts record `scan` or `manual`. No row is deleted or rewritten |
| 0008 | `0008_offer_saved_at.sql` | Additive schema change for Milestone 7e: `ALTER TABLE job_offers ADD COLUMN saved_at TEXT`. NULL means not bookmarked; a value is the UTC RFC3339 bookmark time. Existing offers keep every column and read as not saved. No row is deleted or rewritten |
| 0009 | `0009_interview_entries.sql` | Additive schema change for Milestone 9: creates `interview_entries` for the personal interview-prep FAQ. No existing table or row changes |
| 0010 | `0010_interview_entry_category.sql` | Additive schema change for Milestone 10: `ALTER TABLE interview_entries ADD COLUMN category TEXT`. NULL means uncategorized; existing entries keep every column and read as uncategorized. No row is deleted or rewritten |
| 0011 | `0011_seed_wttj_sources.sql` | Data-only seed: the Fabriq Welcome to the Jungle careers page verified in Milestone 10a (`wttj`, `{}` config, enabled). The 0002 WTTJ marketplace row has a different URL and is left untouched. `INSERT … WHERE NOT EXISTS` by URL; never updates existing rows |
| 0012 | `0012_company_identity.sql` | OpenSpec change `offer-identity-from-source-uid`: creates `company_keys` and `company_aliases`; backfills one confirmed alias per company and keys from typed adapter config tokens; rebuilds `job_offers` so `company_id` is nullable, `fingerprint` is no longer UNIQUE and `incomplete_reason`/`incomplete_raw` exist; converts `offer_state = 'changed'` to `active`; squashes duplicate `(source_id, external_uid)` provenance rows onto the surviving offer and adds a partial unique index on that pair |
| 0013 | `0013_company_resolution.sql` | OpenSpec change `offer-identity-from-source-uid`: rebuilds `companies` so `name_normalized` is no longer UNIQUE (display names are attributes, not identity) and adds a non-unique lookup index; backfills a global alias for any company without one; sets `config.company` to the current display name on enabled single-employer sources (typed ATS plus RSS/HTML except We Work Remotely) so the scan pipeline can stop using `sources.name` as a company fallback |
| 0014 | `0014_wwr_employer_rule.sql` | Data-only: adds `{"employer":{"separator":":"}}` to the registered We Work Remotely RSS config when no employer rule is present, so the adapter splits `Employer: Role` into company and title. Every other config key is kept; re-applying matches no row |
| 0015 | `0015_search_profiles.sql` | OpenSpec change `classification-profiles`: creates `search_profiles`, `search_profile_terms` and `search_profile_remote` and seeds one default profile from the former hardcoded classifier tables (35 title, 79 location, 15 description terms with their exact weights; `remote` and `unknown` accepted). Additive: no `job_offers` row or column is changed; the legacy score columns stay unused |
| 0016 | `0016_murena_detail_description.sql` | Data-only, idempotent: adds `detail_description_selector` to the seeded Murena HTML source when the key is absent. Every other config key is preserved and a user-edited selector is never overwritten |
| 0017 | `0017_application_lifecycle.sql` | OpenSpec change `simplify-application-lifecycle`: rebuilds `job_offers` with `application_state DEFAULT 'none'`; maps `todo`/`maybe` → `none`, `responded` → `applied`, `positive` → `accepted`; turns bookmarked `todo`/`maybe` rows into `saved` and clears `saved_at` on other bookmarked rows; recreates indexes and FTS triggers and rebuilds the FTS index. Every offer ID and every child row is preserved |

Migration 0012 runs on a pinned connection with `foreign_keys=OFF` so it
can rebuild `job_offers` without `DROP TABLE` cascading into events,
notes and snapshots; the runner verifies the result with
`PRAGMA foreign_key_check` before committing and restores enforcement on
the connection. The rebuild preserves every offer id and child row.
The one lossy step is the provenance squash: for each duplicate
`(source_id, external_uid)` the link on the surviving offer (active, then
changed, then most recently seen) inherits the earliest `first_seen_at`
and latest `last_seen_at`; the other links are removed. The duplicate
offers themselves are not deleted and keep their events, notes and
snapshots, but may have no provenance row afterwards. Rolling back
therefore requires the pre-migration backup; `jobwatch backup` before
migrating is the supported safety net. The `changed` -> `active` update is
recoverable by the next scan, and the two new tables plus the two new
columns are additive.

Migration 0013 rebuilds `companies` with the same runner and preserves
every row, id and child reference; it removes the UNIQUE constraint on
`name_normalized` because two employers may share a display name. The
`config.company` update only fills a missing key on enabled
single-employer sources, so it is additive and preserves every other
config value. Re-adding the UNIQUE constraint is not possible while
duplicate names exist; rolling back requires the pre-migration backup.

Migration 0014 is data-only and additive: it adds the employer rule to
the We Work Remotely config when the key is absent and leaves every other
config value untouched. Removing the `employer` key from that source's
config reverts it; no table or row is rebuilt.

Migration 0015 is additive: it creates the three profile tables and
seeds one default profile. It does not rebuild or rewrite `job_offers`,
so the legacy `title_confidence`, `location_confidence` and
`match_reasons_json` columns keep their values and every offer id, child
row and fingerprint is untouched. The previous binary ignores the new
tables; rolling back only needs the pre-migration backup if the user also
wants the seeded profile gone. Re-running the migration is impossible
because `schema_migrations` records version 15.

`schema_migrations(version INTEGER PK, name TEXT, applied_at TEXT)`.
Each migration runs in its own transaction with its bookkeeping row.
Opening a database that records a version unknown to the binary fails.
Milestones 2 and 3 needed no schema change; Milestone 4 adds the
disabled-source rows from 0002 and Milestone 5 ships the 65 registered
sources from 5a/5b in 0003 (both data only, no table or column changes).
Milestone 5c's source decisions were already part of 0003, so 0004 is an
audit-only no-op. Milestone 6a adds one enabled BambooHR row in 0005 and
Milestone 10a one enabled WTTJ row in 0011, so the seeded total is 71
rows (39 enabled, 32 disabled). Milestone 7b renames the old
`reply_received` event value in place in 0006; it changes no source row
and no schema. Milestone 7c adds the snapshot `origin`
column in 0007 (additive, default `scan`) so every future snapshot records
whether it came from a scan/import or a manual description edit.
Milestone 7e adds the nullable `job_offers.saved_at` column in 0008;
since 0017 it is metadata for the `saved` application state, not an
independent bookmark.
Milestone 9 adds the standalone `interview_entries` table in 0009; it is
not linked to offers, companies or the application lifecycle. Milestone
10 adds the nullable `interview_entries.category` column in 0010; there is
no categories table, so the category set is derived from the rows.

## Tables

### companies
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| name | TEXT NOT NULL | display name as first seen |
| name_normalized | TEXT NOT NULL | `model.NormalizeCompanyName`: lowercase, accents folded, dots/apostrophes removed, other punctuation → space, trailing legal suffixes removed (gmbh, ag, kg, inc, incorporated, corp, corporation, llc, ltd, limited, plc, sas, sasu, sarl, sa, spa, srl, bv, nv). Not unique: identity is carried by `company_keys` and `company_aliases` |
| website | TEXT | nullable |
| created_at | TEXT | |

Companies are resolved by attested key, then by a name claim for the
sighting's source, then by a name confirmed for all sources, and created
only when none matches. A bare name is never matched across sources.
`model.SimilarCompanyNames` proposes possible duplicates for the UI
(token containment or small edit distance); nothing is merged without an
explicit `POST /api/companies/:id/merge`. A merge moves offers, company
notes, keys and aliases to the target, records the absorbed display name
as a confirmed alias, and deletes the emptied company; a rename records
both the old and the new name as aliases. Neither operation rewrites
offer fingerprints.

### company_keys
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| company_id | INTEGER NOT NULL → companies ON DELETE CASCADE | |
| key | TEXT NOT NULL UNIQUE | attested identifier, namespaced by provider: `greenhouse:<board_token>`, `lever:<site>`, `ashby:<org>`, `personio:<tenant>`, `workable:<account>`, `bamboohr:<subdomain>` |
| created_at | TEXT | |

One company may hold several keys (for example a Greenhouse board and a
Lever site). Keys are globally unique because of the provider prefix.
Migration 0012 backfills a key only for a typed source whose offers all
belong to one company. A sighting that resolves by a key records the
reported name for its source; a merge moves the absorbed keys to the
winner and drops a duplicate key the winner already holds.

### company_aliases
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| company_id | INTEGER NOT NULL → companies ON DELETE CASCADE | |
| name_normalized | TEXT NOT NULL | `model.NormalizeCompanyName` |
| source_id | INTEGER → sources ON DELETE CASCADE | NULL = confirmed for every source |
| created_at | TEXT | |

Two partial unique indexes make the claim unambiguous: one row per
`(source_id, name_normalized)` for a specific source, and one row per
`name_normalized` for the global (NULL) scope. Migration 0012 backfills
one global alias per existing company; 0013 covers companies created
since.

Resolution order is key, then the alias for the sighting's source, then
the confirmed global alias, then create. A name-only claim is recorded
for that source, so the same name from two sources creates two companies
until the user merges them. An explicit `POST /api/companies` creates a
global alias when the name is free. A merge moves the absorbed aliases
to the winner, drops conflicts, and records the absorbed display name as
a confirmed global alias.

### sources
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| name | TEXT NOT NULL | display name of the source; no longer used as a company fallback |
| url | TEXT NOT NULL | indexed (not unique; the CLI refuses duplicates) |
| source_type | TEXT NOT NULL | `greenhouse` `lever` `ashby` `personio` `workable` `bamboohr` `wttj` `rss` `html` `manual` |
| config_json | TEXT NOT NULL DEFAULT '{}' | see below |
| enabled | INTEGER NOT NULL DEFAULT 1 | boolean |
| last_scan_at | TEXT | start time of the last scan attempt |
| last_scan_status | TEXT | `ok` `error` `blocked`; NULL = never scanned |
| created_at | TEXT | |

`config_json` keys (all optional): `company` (declares the company for a
single-employer source; required when the adapter supplies no company),
`employer` (declarative employer rule for RSS/HTML sources that list
several employers: `separator`, and optionally `field` for RSS or
`selector` for HTML),
`rate_limit_ms` (min interval between requests, default 2000;
HTML sources are capped at a 5 s minimum), `note` (human-readable
explanation shown in the UI), `board_token` (greenhouse), `site` +
`region` (`""`/`"eu"`, lever), `org` (ashby), `tenant` + `language`
(personio), `account` (workable), `subdomain` (bamboohr), `url`
(optional wttj careers page override; defaults to the source URL) and
`algolia_app_id` + `algolia_api_key` (wttj debug-only overrides of the
credentials discovered from the page bootstrap). Board keys are derived
from the source URL when absent.

RSS mapping keys: `item` (item element; auto-detects `item`/`entry`),
`title_field`, `link_field`, `description_field`, `location_field`,
`company_field`, `uid_field`. A field is a chain of child element local
names; `@name` reads an attribute and `#text` reads the element text.
Defaults: title, link (Atom alternate `href` fallback), description
(`description`/`summary`/`content`), uid (`guid`/`id`). The optional
`employer` rule splits `Employer: Role` (or reads `field`) and rewrites
both company and title.

HTML selector keys: `item_selector` and `title_selector` are required;
`link_selector` (defaults to the title selector), `link_attr` (default
`href`), `location_selector`, `description_selector`,
`company_selector`, `uid_selector`, `uid_attr`. The optional `employer`
rule uses `separator` and/or `selector` to rewrite company and title.
Standard CSS syntax is supported by cascadia; the listing page is fetched
once per scan. `detail_description_selector` optionally names the
element holding the full text on an offer page: the adapter fetches that
page only when the offer has no stored description or the stored text
still equals the listing description, emits the stored text otherwise,
and stops detail fetches for the scan after a blocked response.

Seeded source rows from migration 0002 are disabled and carry a `note`.
They exist so the known-hard Workday/SuccessFactors links and the WTTJ
marketplace pages are visible without pretending an adapter can read
them. Migration 0003
seeds the 65 sources registered in Milestones 5a/5b with the exact
configuration decided there: the enabled ATS boards and RSS/HTML
listings keep their board tokens, field mappings and selectors, while
the 28 not-trackable rows are disabled and carry their explanatory
`note`. Migration 0005 adds the enabled Semble BambooHR source and
migration 0011 the enabled Fabriq WTTJ company page. All seeds only
insert missing URLs and never update an existing row, so re-applying
them or keeping a user-edited source is safe.

### job_offers
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| company_id | INTEGER → companies | nullable since 0012: an incomplete item may have no company yet |
| title | TEXT NOT NULL | as first seen; empty only for an incomplete item whose source reported no title |
| title_normalized | TEXT NOT NULL | `model.NormalizeTitle`: lowercase, accents folded, gender markers (`H/F`, `m/w/d`…) removed, punctuation → space; empty when the title is missing |
| location | TEXT | free text as first seen |
| remote_policy | TEXT NOT NULL DEFAULT 'unknown' | `remote` `hybrid` `onsite` `unknown`; an offer fact derived by the fixed remote detector from location and description on every sighting and on location/description edits |
| url | TEXT | canonical (first-seen) link |
| fingerprint | TEXT NOT NULL DEFAULT '' | no longer UNIQUE since 0012; see Fingerprints. Written on creation for compatibility with the merge/rename code; never used for resolution |
| first_seen_at, last_seen_at | TEXT NOT NULL | |
| offer_state | TEXT NOT NULL DEFAULT 'active' | `active` `disappeared` |
| application_state | TEXT NOT NULL DEFAULT 'none' | `none` `saved` `applied` `interview` `offer` `accepted` `turned_down` `negative` |
| raw_text | TEXT | latest plain-text description |
| text_hash | TEXT | SHA-256 hex of whitespace-collapsed `raw_text`; NULL when no description |
| title_confidence | INTEGER | **Deprecated since 0015, unused.** Legacy stored title score. Kept only for rollback; no code reads or writes it, and it is absent from API responses and filters |
| location_confidence | INTEGER | **Deprecated since 0015, unused.** Legacy stored location score. Same status as `title_confidence` |
| match_reasons_json | TEXT | **Deprecated since 0015, unused.** Legacy stored reason strings; reasons are computed from the current profile at read time |
| saved_at | TEXT | NULL unless `application_state = 'saved'`; otherwise the UTC RFC3339 time the offer entered the saved state. Set when the state becomes `saved`, cleared when it leaves `saved` |
| incomplete_reason | TEXT | NULL for a usable offer; otherwise `missing_company`, `missing_title` or `missing_company_title` (validated in Go) |
| incomplete_raw | TEXT | JSON object with the values the source reported for an incomplete item. Kept after a repair as the durable marker that protects the user's title and location from later scans |
| created_at, updated_at | TEXT | |

Indexes: company_id, offer_state, application_state, last_seen_at.

An incomplete offer keeps its provenance, snapshots and notes; it is
excluded from the offers list, dashboard and search until the user
repairs it. A repair (`POST /api/offers/:id/repair`) assigns a company
and title, clears `incomplete_reason` and re-runs the classifier, but
leaves `incomplete_raw` in place. A later scan resolves the same identity
to the offer, refreshes `last_seen_at`/provenance and never overwrites the
user's company, title or location. An incomplete offer with a known
company keeps that company; only a missing company leaves `company_id`
NULL.

`saved` is an application state, and `saved_at` is its metadata. The
invariant is `saved_at IS NOT NULL` exactly when
`application_state = 'saved'`. Entering `saved` records the server time;
leaving it for any other state clears the column. A saved offer is not
an application: `has_application` excludes both `none` and `saved`.

### Deprecated classification columns

The 0015 migration moved classification decisions out of the offer row.
`title_confidence`, `location_confidence` and `match_reasons_json` remain
in `job_offers` but are **unused**: no code reads or writes them, no API
response exposes them, and no filter or sort uses them. They are kept for
rollback only. Title affinity, location affinity and the score reasons
are computed from the current search profile at read time (see
`ARCHITECTURE.md`); a profile edit takes effect on the next read with no
reclassification command.

### search_profiles

One user search profile: the target role, geography and remote policy
selection. The schema supports several profiles; the UI edits one.

| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| name | TEXT NOT NULL | display name; validated non-empty in Go |
| title_label | TEXT NOT NULL DEFAULT '' | optional heading for the title term list |
| location_label | TEXT NOT NULL DEFAULT '' | optional heading for the location term list |
| bucket_scale | TEXT NOT NULL DEFAULT 'five' | `three` (`positive` `neutral` `negative`) or `five` (`strong_positive` `mild_positive` `neutral` `mild_negative` `strong_negative`). A display choice: the scorer always adds `search_profile_terms.weight` |
| is_default | INTEGER NOT NULL DEFAULT 0 | 0/1; a partial unique index `idx_search_profiles_default` allows at most one default |
| created_at, updated_at | TEXT NOT NULL | |

The default profile is the one with `is_default = 1`; it is used for
offer scoring when a request does not name a profile. The migration seeds
exactly one: name `Default`, title label `Platform Engineer / SRE`,
location label `France / full remote`, five-bucket scale.

### search_profile_terms

| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| profile_id | INTEGER NOT NULL → search_profiles ON DELETE CASCADE | |
| affinity | TEXT NOT NULL | `title`, `location` or `description` (validated in Go). Description terms add location evidence only |
| bucket | TEXT NOT NULL | display label; must belong to the profile's `bucket_scale` |
| term | TEXT NOT NULL | term as entered (outer whitespace trimmed). Matching normalizes with `model.NormalizeKeywords` |
| weight | INTEGER NOT NULL | score added when the term matches. Neutral terms must be 0; a non-neutral bucket needs a non-zero weight |
| sort_order | INTEGER NOT NULL DEFAULT 0 | position inside the profile editor |

`UNIQUE(profile_id, affinity, term)`; index
`idx_search_profile_terms_profile(profile_id, affinity, sort_order)`.
Validation rejects empty terms, duplicate terms after normalization,
buckets outside the scale and neutral/non-neutral weight mismatches. A
profile update replaces these rows in one transaction.

The seed copies the former hardcoded tables exactly: 35 title terms
(strong positive 60, weak technology/role terms 15–25, management −20,
strong negatives −120), 79 location terms (France and cities, EMEA/EU/
Europe and time zones positive; UK/Germany −20, other countries −15,
clearly incompatible countries −35) and 15 description terms (5–20).

### search_profile_remote

| Column | Type | Notes |
|---|---|---|
| profile_id | INTEGER NOT NULL → search_profiles ON DELETE CASCADE | |
| policy | TEXT NOT NULL | `remote` `hybrid` `onsite` `unknown`; `remote` is labelled "Full remote" in the UI |
| accepted | INTEGER NOT NULL DEFAULT 0 | 0/1 membership checkbox; missing row = not accepted |

`PRIMARY KEY(profile_id, policy)`. Remote is a fact plus a membership
check, never a weighted score: `job_offers.remote_policy` is detected from
the offer text and `accepted` says whether the current profile wants it.
The seed accepts `remote` and `unknown` and rejects `hybrid` and `onsite`.

### Manual description edits

`PATCH /api/offers/:id` accepts `description` (string). It replaces
`raw_text`, recomputes `text_hash` from the whitespace-collapsed text and
appends a `snapshots` row with `origin = 'manual'` in the same
transaction; `updated_at` is bumped.
`null` is rejected (`400 validation_failed`), so the version chain cannot
be cleared through the API. An empty string is valid: it stores no
description (`raw_text` and `text_hash` NULL) and appends an empty
snapshot. A manual edit never changes `offer_state`.

A manual description is not special to the scan diff/merge logic. A later
sighting whose description hash differs from the stored (manual) hash
records another snapshot and keeps the offer active; a sighting without a
description never clears the stored text. Manual and scan-captured text
therefore share one snapshot chain, and the detail diff compares the two
latest snapshots regardless of their origin.

### job_offer_provenance
One row per (offer, link) sighting. `UNIQUE(offer_id, url)`, plus a
partial unique index on `(source_id, external_uid)` for rows whose
`external_uid` is non-empty (migration 0012).

| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| source_id | INTEGER → sources | NULL for manual imports |
| external_uid | TEXT | ATS posting id when available; identity carrier together with `source_id` |
| url | TEXT NOT NULL | exact link where seen |
| first_seen_at, last_seen_at | TEXT | `last_seen_at` = start time of the scan that last saw it |

Index: unique partial `(source_id, external_uid)` where `external_uid` is
non-empty.

Before 0012 the same `(source_id, external_uid)` could point at several
offers because the fingerprint identity changed with a title or company
name. Migration 0012 squashes those rows onto the surviving offer; the
duplicate offers remain (never deleted) but may have no provenance row.
New sightings resolve by provenance, so they cannot recreate the
duplicate.

A manual import has no source of its own (`source_id` NULL). Its URL is
matched against every provenance row, so importing a link a scan already
tracks attaches to that offer; a new URL creates a new offer. Title and
company are attributes, not identity.

`jobwatch reattribute` is the explicit remediation path for offers whose
company or title predates an adapter change. It matches stored offers by
`(source_id, external_uid)` against the adapter's current output and
updates only `company_id`, `title`, `title_normalized`, the classifier
columns and `updated_at`; events, notes, snapshots, both states,
`last_seen_at` and provenance stay as they are. A repaired offer
(`incomplete_raw` non-null) and an offer with no current uid match are
left untouched, so the command is safe to re-run.

### events
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | |
| offer_id | INTEGER NOT NULL → job_offers ON DELETE CASCADE | |
| type | TEXT NOT NULL | `cv_sent` `message_received` `message_sent` `interview` `offer` `rejected` `ghosted` `other`. `message_received` covers a recruiter's inbound first contact and a reply to an application; `message_sent` covers user-initiated pings and follow-ups |
| description | TEXT | |
| occurred_at | TEXT NOT NULL | user-editable |
| created_at | TEXT | |

Index: (offer_id, occurred_at). Written by the events API; `occurred_at`
is user-editable and may be backdated. The type is validated in Go only
(no CHECK constraint), so migration 0006 renamed existing
`reply_received` rows to `message_received` in place and the new
`message_sent` value needed no data change.

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

### interview_entries
| Column | Type | Notes |
|---|---|---|
| id | INTEGER PK | autoincrement |
| question | TEXT NOT NULL | plain-text anticipated question; trimmed, must not be blank |
| answer | TEXT NOT NULL DEFAULT '' | prepared answer as **markdown plaintext**, never HTML; may be empty at creation |
| answer_format | TEXT NOT NULL DEFAULT 'markdown' | enum-like, validated in Go; `markdown` is the only value for now |
| category | TEXT | free-form category name or NULL (uncategorized); trimmed, non-blank when supplied. No categories table: the set is derived from the rows |
| display_order | INTEGER | optional manual position; NULL falls back to id order |
| created_at, updated_at | TEXT | |

Written by the interview-prep API (`/api/interview-entries`). Duplicate
questions are allowed. Listing groups rows by category: categories sort
alphabetically (case-insensitive), uncategorized rows last, and within each
category explicit `display_order` rows come first in ascending order, then
the remaining rows by id. The web UI renders the answer with a small
in-house markdown renderer that emits React text nodes (no
`dangerouslySetInnerHTML`), so raw HTML in an answer is displayed literally
and cannot execute. The chosen section order is a presentation-only
preference stored in the browser's `localStorage`, never in the database.

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
| origin | TEXT NOT NULL DEFAULT 'scan' | `scan` (scan pipeline or manual URL import) or `manual` (description saved through `PATCH /api/offers/:id`); validated in Go |
| raw_text | TEXT NOT NULL | description at capture time (may be empty for offers created without one) |
| captured_at | TEXT NOT NULL | |

Index: (offer_id, captured_at). Every first description, every
text-hash change and every manual description save appends a row.
Snapshots are ordered by `captured_at`, then `id` for versions captured in
the same second. `GET /api/offers/:id` exposes the version history from
oldest to newest, the latest snapshot (with full text), the count and the
diff between the two latest snapshots; `GET /api/offers/:id?snapshot_id=N`
adds the full text of one version and `GET /api/offers/:id/snapshot-diff`
computes the diff between any two versions on demand.

Migration 0007 adds `origin` with a default of `scan`. Historical manual
snapshots are not distinguishable from scan snapshots: both used
`captured_at` as their capture time, and scans also bump
`job_offers.updated_at` in the same transaction. The only partial signal
(a latest snapshot whose `captured_at` equals `updated_at` but differs
from `last_seen_at`) cannot cover older manual versions, so every
pre-0007 snapshot is backfilled as `scan`; origins are exact from 7c on.
Merging offers moves snapshot rows unchanged, so their origin is
preserved.

### Full-text search
- `job_offers_fts` — FTS5, external content `job_offers` (columns
  `title`, `raw_text`), `content_rowid = id`.
- `notes_fts` — FTS5, external content `notes` (column `body`).

Both use `tokenize = 'unicode61 remove_diacritics 2'` and are kept in
sync by AFTER INSERT / DELETE / UPDATE OF triggers
(`job_offers_fts_ai|ad|au`, `notes_fts_ai|ad|au`).

## Fingerprints

`fingerprint = sha256_hex(company_normalized + 0x1F + title_normalized [+ 0x1F + location_bucket])`

Migration 0012 removes the UNIQUE constraint and the scan pipeline
resolves sightings through `job_offer_provenance(source_id, external_uid)`
first, then `(source_id, url)`. The fingerprint is computed for new
offers and splits so the legacy column stays populated, but it is never
an identity carrier, it is not used for matching, and two offers may
share one. Do not add new dependencies on its uniqueness.

- The base fingerprint has no bucket.
- `model.LocationBucket`: first location segment (split on `, ; | / ( )`
  and `" - "`), normalized; if that segment is only a workplace word
  (`remote`, `hybrid`, `onsite`, `anywhere`, `teletravail`, …), the next
  segment is appended (`"Remote, United States"` → `remote united states`).
  Empty = unknown.

## State machines

### offer_state (written by scans and imports)

An offer has exactly one of two listing states: `active` or
`disappeared`. A description edit is not a state change; it is recorded
as a snapshot.

| From | Trigger | To |
|---|---|---|
| — | first sighting | `active` (+ snapshot) |
| any | sighting with a non-empty description whose hash differs from `text_hash` | unchanged (+ snapshot, `raw_text` replaced) |
| `disappeared` | sighting without text change | `active` |
| `active` | sighting without text change | unchanged |
| `active` | successful scan of a source that has a provenance row for the offer, no row of that source seen in this scan, and no other source saw it in its own latest `ok` scan | `disappeared` |

- Migration 0012 converts every existing `changed` row to `active`; the
  value is no longer part of the Go enum or the API filter.
- A sighting without description never clears stored text. A first
  description on an offer that had none fills `raw_text` (+ snapshot)
  without changing the state.
- A manual description edit through `PATCH /api/offers/:id` does not
  change the state: it replaces `raw_text`, recomputes `text_hash` and
  appends a snapshot. Later scans compare against that manual hash
  exactly like any other stored text (see Manual description edits).
- The classifier re-runs on every sighting using the stored title,
  location and latest description; score columns and `remote_policy` are
  updated even when the offer state does not change.
- Failed or blocked scans change no offer.
- Offers are never deleted by scans or imports.
- When several postings of one scan map to the same offer, only the
  first one updates the offer; the others only add provenance.

### application_state
Default `none`. The vocabulary is `none`, `saved`, `applied`,
`interview`, `offer`, `accepted`, `turned_down`, `negative`. Manual
changes go through `PATCH /api/offers/:id`; the API validates the enum
and bumps `updated_at` (the audit field). Manual changes are free-form
and create no timeline event. `saved` maintains `saved_at` as described
above; `none` and `saved` are not applications.

### last_scan_status
`ok` after a committed scan; `blocked` when robots.txt disallows the URL
or the server answers 401/403/429; `error` for any other failure.
Sources whose type has no adapter are skipped and keep their status.
`POST /api/sources/:id/scan` returns the summary; the same values are
surfaced on `GET /api/sources`.

## Company merge and rename

- Rename (`PATCH /api/companies/:id`): changes `name` and
  `name_normalized`; both the old and the new normalized name are
  recorded as global aliases. Offers keep their company, identity and
  fingerprint. Renaming to a name another company already uses is
  allowed; the existing alias stays with its company.
- Merge (`POST /api/companies/:id/merge`): offers, company notes, keys
  and aliases of the source company move to the target; the absorbed
  display name becomes a confirmed global alias of the target; the
  target inherits the source website when it has none, and the source
  company row is deleted. Offers are never deleted. A key or alias the
  target already holds is kept once and the duplicate is dropped.
- Both operations are transactional. There is no fingerprint conflict:
  offers may share a company, title and location because identity is
  provenance.
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
  `none` (and promotes a `saved` target to a real application carried by
  the absorbed offer), keeps `saved_at` consistent with the resulting
  state, and keeps the offer active if either side was active; only two
  disappeared offers stay disappeared. The absorbed
  `job_offers` row is deleted only after all children have moved; no
  events, notes, snapshots or provenance rows are lost.
- **Split** (`POST /api/offers/:id/split` with `provenance_ids`, optional
  `title`, optional `location`): creates a second offer in the same
  company and moves the selected provenance rows to it. Events, notes and
  snapshots stay with the original because they describe the tracked
  application. At least one provenance row must remain on the original.
  The new offer gets `active` (or `disappeared` when the original was
  disappeared), application state `none`, and a base fingerprint. A
  company/title/location match with another offer is allowed because
  identity is provenance; the fingerprint is not checked for collisions.
  The original canonical `url` is repaired to a remaining link when the
  moved row owned it.

## Full-text search exposure

`job_offers_fts` backs `GET /api/offers?q=`. `notes_fts` is maintained by
the triggers but is not exposed through a search endpoint yet.
