# REST API

Base path `/api`, JSON in and out. Every endpoint is served by
`jobwatch serve` on localhost. The SPA uses the same API.

For source configuration, the manual-import workflow and CLI usage, see
[USAGE.md](USAGE.md). The product specification is [SPEC.md](SPEC.md) and
the project overview is the [README](../README.md).

## Conventions

- **Errors** always use one shape:

  ```json
  { "error": { "code": "string", "message": "string" } }
  ```

- Unknown method/path under `/api/` returns `404` with code `not_found`.
- IDs are SQLite autoincrement integers, positive.
- Timestamps are RFC3339 strings in UTC (`2026-11-16T09:00:00Z`).
  Nullable timestamps are `null`.
- Enum-like fields are strings; unknown values are rejected with
  `400 validation_failed`.
- Request bodies must be a single JSON object; unknown fields are
  rejected with `400 invalid_json`.
- **Partial updates**: a field that is absent is left unchanged. For
  nullable fields (`location`, `website`, event `description`,
  `pinned_event_id`, interview `display_order`), `null` clears the value.
  The offer detail `description` field is the exception: it accepts a
  string only and rejects `null` (see `PATCH /api/offers/:id`).
- Pagination: `limit` (bounded per endpoint) and `offset` (default 0).
  List responses include `total`, `limit` and `offset`.
- `config` on sources is a JSON object; unknown keys are preserved.

## Enumerations

| Field | Values |
|---|---|
| `source_type` | `greenhouse` `lever` `ashby` `personio` `workable` `bamboohr` `wttj` `rss` `html` `manual` |
| `last_scan_status` | `ok` `error` `blocked`; `null` = never scanned |
| `offer_state` | `active` `disappeared` |
| `application_state` | `todo` `applied` `responded` `interview` `offer` `negative` `maybe` `positive` |
| `remote_policy` | `remote` `hybrid` `onsite` `unknown` |
| event `type` | `cv_sent` `message_received` `message_sent` `interview` `offer` `rejected` `ghosted` `other` |
| snapshot `origin` | `scan` `manual` |
| interview `answer_format` | `markdown` |
| note `entity_type` | `offer` `company` |

Event `message_received` covers both a recruiter's inbound first contact
and a reply to an application; `message_sent` covers user-initiated
pings and follow-ups. Existing `reply_received` rows are renamed to
`message_received` by migration 0006 and the old value is no longer
accepted. Snapshot `origin` is `scan` for the scan pipeline and manual URL
imports, and `manual` for descriptions saved through
`PATCH /api/offers/:id`.

## GET /api/health

Pings the database. `200`:

```json
{ "status": "ok", "version": "dev", "db": "ok" }
```

`503` when the ping fails: `status` = `"degraded"`, `db` = the error.

## Sources

### GET /api/sources

All sources, ordered by ID. A fresh database includes 71 seeded rows:
the 39 sources decided in Milestones 5a/5b/6a/10a (ATS boards including
Semble on BambooHR, the Fabriq WTTJ company page, RSS feeds and HTML
listings) and 32 disabled rows (four known-hard
Workday/SuccessFactors/WTTJ-marketplace rows plus 28 not-trackable
URLs). Every disabled row has a `config.note` explaining why. Their IDs
sit before any source you add; migrations only insert missing URLs and
never overwrite edits.

```json
{ "sources": [ {
  "id": 1, "name": "Acme", "url": "https://boards.greenhouse.io/acme",
  "source_type": "greenhouse", "config": {}, "enabled": true,
  "last_scan_at": "2026-11-16T09:00:00Z", "last_scan_status": "ok",
  "created_at": "2026-11-15T10:00:00Z"
} ] }
```

### POST /api/sources

Body:

```json
{ "name": "Acme", "url": "https://boards.greenhouse.io/acme",
  "source_type": "greenhouse", "config": { "company": "Acme SAS" }, "enabled": true }
```

- `name` required.
- `source_type` optional; when omitted it is detected from the URL
  (Greenhouse, Lever, Ashby board URLs). A URL whose type cannot be
  detected requires an explicit `source_type`.
- `url` must be `http(s)` except for `manual`.
- `enabled` defaults to `true`; `config` defaults to `{}`.
- `201` with `{ "source": {...} }`.
- `409 duplicate_source` when another source has the same URL.
- `400 validation_failed` for invalid fields.

### GET /api/sources/:id

`200 { "source": {...} }`, `404 not_found`.

### PATCH /api/sources/:id

Any of `name`, `url`, `source_type`, `config`, `enabled`. Same
validation as POST, including the duplicate-URL check. Returns
`200 { "source": {...} }`.

### DELETE /api/sources/:id

`204`. The source row is removed; its provenance rows are kept (offers
and links are never deleted) and lose their `source_id` (they become
`null`, like manual imports).

### POST /api/sources/:id/scan

Runs the scan pipeline for one source **synchronously** and returns its
summary. Fetch failures are reported in the body with HTTP `200`; the
source's `last_scan_status` is updated to `error` or `blocked`. A source
type without an adapter is `skipped` and its status is left untouched.

```json
{
  "result": {
    "source_id": 1, "source_name": "Acme", "status": "ok",
    "fetched": 12, "skipped": 0, "new": 2, "seen": 9,
    "changed": 1, "reappeared": 0, "disappeared": 1,
    "error": ""
  },
  "source": { "id": 1, "last_scan_status": "ok", "...": "..." }
}
```

`status` is `ok`, `error`, `blocked` or `skipped`. `error` is omitted
when empty. `404 not_found` for an unknown source.

## Offers

### GET /api/offers

Query parameters (all optional):

| Param | Meaning |
|---|---|
| `q` | FTS5 query over offer title and description. Invalid FTS syntax → `400 invalid_query` |
| `application_state` | one value or comma-separated list |
| `offer_state` | one value or comma-separated list |
| `remote_policy` | one value or comma-separated list |
| `min_title_confidence` | integer 0–100; offers with a NULL score are excluded |
| `min_location_confidence` | integer 0–100; same |
| `company_id` | restrict to one company |
| `has_application` | `true`/`1`: only `application_state != todo` |
| `saved` | `true`/`false`: only bookmarked (`saved_at` set) or only unbookmarked offers. Absent = no filter. Invalid values → `400 validation_failed` |
| `sort` | `last_seen` (default), `first_seen`, `recent_event`, `title`, `company`, `location`, `title_confidence`, `location_confidence`, `saved` |
| `order` | `asc` or `desc`; absent keeps the sort's historical default. Invalid values → `400 validation_failed` |
| `limit` | default 50, maximum 200 |
| `offset` | default 0 |

`recent_event` orders by the latest timeline event, falling back to
`updated_at` when an offer has no events (used by the dashboard's
Applications section). `saved` orders by `saved_at DESC` and is used by
the dashboard's Saved section. Text sorts (`title`, `company`,
`location`) default to ascending; the other sorts default to descending.
`location` sorts by `COALESCE(location, '')`, and the two confidence
sorts by their score (NULL as -1). All sorts append the offer id as a
stable tie-breaker.

```json
{
  "offers": [ {
    "id": 7, "company_id": 2, "company_name": "Acme",
    "title": "Site Reliability Engineer", "location": "Paris, France",
    "remote_policy": "remote", "url": "https://boards.greenhouse.io/acme/jobs/7",
    "offer_state": "active", "application_state": "applied",
    "first_seen_at": "2026-11-10T08:00:00Z",
    "last_seen_at": "2026-11-16T09:00:00Z",
    "updated_at": "2026-11-16T09:00:00Z",
    "title_confidence": 85, "location_confidence": 85,
    "match_reasons": {
      "title": ["strong positive \"site reliability\" (+60)", "weak positive \"reliability\" (+25)"],
      "location": ["description \"remote\" (+25)", "location \"france\" (+35)", "remote policy: remote"]
    },
    "last_event_at": "2026-11-15T17:30:00Z",
    "saved_at": "2026-11-16T08:15:00Z"
  } ],
  "total": 1, "limit": 50, "offset": 0
}
```

`match_reasons` is the classifier's stored JSON: an object with a
`title` and a `location` array of human-readable matched terms (or `null`
until classification runs). Scores are 0–100. Offers inserted before the
classifier existed keep `null` scores until `jobwatch reclassify` or the
next sighting; `min_title_confidence` / `min_location_confidence` exclude
NULL scores.

### POST /api/offers/import

Manually add an offer seen elsewhere. Runs the same dedup path as scans
in one transaction; a matching fingerprint adds a provenance row to the
existing offer instead of creating a duplicate.

Two input modes:

1. **Raw JSON** with the fields supplied. `company` and `title` are
   required in this mode; `url` must be `http(s)` when present.
2. **URL extraction**. If `url` is present and `company` or `title` is
   missing, JobWatch fetches the page politely (robots.txt, rate limit,
   JobWatch User-Agent) and reads JSON-LD `JobPosting`, Open Graph/meta
   tags and the page title. Explicit fields win over extracted values;
   the input URL is always kept as the provenance link. No JavaScript is
   executed.

```json
{ "company": "Qonto", "title": "SRE (H/F)", "location": "Paris",
  "url": "https://example.com/job/1", "description": "optional" }
```

```json
{ "url": "https://example.com/jobs/42" }
```

- `201 { "offer_id": 3, "created": true }` for a new offer.
- `200` with `created: false` when an existing company+title matched.
- `400 validation_failed` for an invalid URL, or when no URL and no
  company/title are supplied.
- `422 metadata_incomplete` when the page was fetched but company or
  title could not be extracted:
  ```json
  {
    "error": { "code": "metadata_incomplete", "message": "…" },
    "metadata": { "company": "", "title": "SRE (H/F)", "location": "Paris",
                  "url": "https://example.com/jobs/42", "description": "…" }
  }
  ```
- `422 metadata_unavailable` when the page could not be fetched (network,
  robots.txt, HTTP 4xx/5xx). `metadata` still carries the input URL. The
  UI uses it to prefill its manual form; the CLI suggests `--company`,
  `--title` or `--no-fetch`.

### GET /api/offers/:id

Full detail: offer, company, every provenance link (with source name),
the timeline, the offer's notes, the description version history, the
latest snapshot and the snapshot count. Optional query parameter
`snapshot_id=N` adds the full `raw_text` of that one version (the other
history entries stay metadata-only).

```json
{
  "offer": { "...": "...", "raw_text": "latest description" },
  "company": { "id": 2, "name": "Acme", "name_normalized": "acme",
               "website": null, "created_at": "...", "offer_count": 3, "note_count": 0 },
  "provenance": [ {
    "id": 1, "source_id": 1, "source_name": "Acme",
    "external_uid": "gh-123", "url": "https://...",
    "first_seen_at": "...", "last_seen_at": "..."
  } ],
  "events": [ { "id": 4, "offer_id": 7, "type": "cv_sent",
                "description": "sent", "occurred_at": "...", "created_at": "..." } ],
  "notes": [ { "id": 2, "entity_type": "offer", "entity_id": 7,
               "body": "...", "pinned_event_id": 4,
               "created_at": "...", "updated_at": "..." } ],
  "snapshots": [
    { "id": 7, "origin": "scan",   "captured_at": "2026-11-10T09:00:00Z", "text_length": 812 },
    { "id": 8, "origin": "scan",   "captured_at": "2026-11-12T09:00:00Z", "text_length": 845 },
    { "id": 9, "origin": "manual", "captured_at": "2026-11-16T09:00:00Z", "text_length": 901 }
  ],
  "latest_snapshot": { "id": 9, "origin": "manual", "raw_text": "...", "captured_at": "..." },
  "snapshot_diff": {
    "from": { "id": 8, "captured_at": "2026-11-12T09:00:00Z" },
    "to": { "id": 9, "captured_at": "2026-11-16T09:00:00Z" },
    "lines": [
      { "op": "equal", "text": "About the team" },
      { "op": "removed", "text": "We use Kubernetes." },
      { "op": "added", "text": "We use Kubernetes and Terraform." }
    ]
  },
  "snapshot_count": 3
}
```

`source_id`/`source_name` are `null` for manual imports. Events are
ordered chronologically. `snapshots` is ordered **oldest → newest** and
version 1 is the oldest row; `text_length` counts Unicode characters.
`raw_text` is omitted for every entry unless `?snapshot_id=N` selects that
snapshot. `latest_snapshot` is `null` when no snapshot exists.
`snapshot_diff` is `null` until two snapshots exist; `op` is `equal`,
`added` or `removed`, and it always compares the two latest snapshots in
chronological order. `404 not_found` for an unknown offer or a
`snapshot_id` that does not belong to it; `400 validation_failed` for a
malformed `snapshot_id`.

### GET /api/offers/:id/snapshot-diff

Returns the line diff between any two snapshots of one offer. Both `from`
and `to` are required positive snapshot IDs and must belong to `:id`.
Arguments may be supplied in either order: the server orders the pair by
`captured_at`, then `id`, and always returns older → newer.

```json
{ "snapshot_diff": {
  "from": { "id": 7, "captured_at": "2026-11-10T09:00:00Z" },
  "to":   { "id": 9, "captured_at": "2026-11-16T09:00:00Z" },
  "lines": [
    { "op": "equal", "text": "About the team" },
    { "op": "removed", "text": "We use Kubernetes." },
    { "op": "added", "text": "We use Kubernetes and Terraform." }
  ]
} }
```

- `400 validation_failed` for a missing/malformed ID or `from == to`.
- `404 not_found` when the offer or either snapshot does not exist on it.

### PATCH /api/offers/:id

Body may contain any of:

```json
{ "application_state": "interview", "offer_state": "active",
  "location": "Remote - France", "description": "manual description",
  "saved": true }
```

- `application_state`, `offer_state`: validated enum values.
- `saved`: `true` sets `saved_at` to the server's current UTC time,
  `false` clears it (NULL). The bookmark is independent of both states;
  the response returns the current `saved_at`.
- `location`: string to set, `null` to clear.
- `description`: string that replaces `raw_text`. The server recomputes
  `text_hash` and appends a `snapshots` row with `origin = "manual"` in
  the same transaction, so a manual edit is versioned like a scan-captured
  description. `null` is rejected with `400 validation_failed`; an empty
  string stores no description (NULL `raw_text`/`text_hash`) and appends
  an empty snapshot. A description edit does not change `offer_state`.
- The update bumps `updated_at`; that is the audit trail.
- Fingerprints are not recomputed on a location correction.
- `200 { "offer": {...} }` (including the current `raw_text` and
  `saved_at`); `400 validation_failed` when no field is given or a value
  is invalid;
  `404 not_found`.
- A later scan is not special-cased: a sighting whose description hash
  differs from the stored (including manual) hash records another
  snapshot and keeps the offer `active`; a sighting without a
  description leaves the stored text untouched.

### POST /api/offers/:id/merge

Explicitly merges offer `:id` **into** `into_id` after user confirmation.

```json
{ "into_id": 12 }
```

In one transaction: every provenance row, event, offer note and snapshot
of `:id` moves to `into_id`; earliest `first_seen_at`/latest
`last_seen_at` are kept; duplicate provenance URLs are collapsed; the
target adopts the source application state only when it is still `todo`;
an active offer stays active, and only two disappeared offers stay
disappeared. The absorbed offer row is removed after its children have
moved.

```json
{
  "offer": { "id": 12, "...": "..." },
  "merged_offer_id": 7,
  "moved_provenance": 2, "moved_events": 3,
  "moved_notes": 1, "moved_snapshots": 4
}
```

- `400 validation_failed` when merging an offer into itself.
- `404 not_found` when either offer does not exist.

### POST /api/offers/:id/split

Creates a second offer in the same company from selected provenance rows.

```json
{ "provenance_ids": [4, 5], "title": "SRE - Payments",
  "location": "Lyon, France" }
```

`provenance_ids` is required and every row must belong to `:id`. `title`
is optional (default: the original title). `location` is optional:
omitted copies the original location, `null` clears it. At least one
provenance row must remain on the original. Events, notes and snapshots
stay on the original; the new offer starts with application state
`todo` and is `active` (or `disappeared` if the original was
disappeared). The original canonical URL is repaired if the moved row
owned it.

`201`:

```json
{ "offer": { "id": 13, "title": "SRE - Payments",
             "location": "Lyon, France", "...": "..." },
  "moved_provenance": 2 }
```

- `400 validation_failed` for no/foreign provenance rows, or when no row
  would remain on the original.
- `409 offer_conflict` when the new company+title+location already
  exists; change the title or location and retry.
- `404 not_found` for an unknown offer.

## Events (offer timeline)

### GET /api/offers/:id/events

`200 { "events": [...] }`, chronological (`occurred_at`, then id).
`404` when the offer does not exist.

### POST /api/offers/:id/events

```json
{ "type": "cv_sent", "description": "optional",
  "occurred_at": "2026-11-01T10:30:00Z" }
```

`type` is required. `occurred_at` is optional (defaults to now) and may
be backdated; it accepts RFC3339, `YYYY-MM-DDTHH:MM` and `YYYY-MM-DD`
(interpreted as UTC). `201 { "event": {...} }`.

### PATCH /api/offers/:id/events/:eventID

Any of `type`, `description`, `occurred_at`. `description: null` clears
it. The event must belong to `:id`, otherwise `404 not_found`.

### DELETE /api/offers/:id/events/:eventID

`204`. Notes pinned to the event keep their text; `pinned_event_id`
becomes `null` (`ON DELETE SET NULL`).

## Notes

Notes are polymorphic: `entity_type` is `offer` or `company` and
`entity_id` points at that entity.

### GET /api/notes?entity_type=offer&entity_id=7

Both parameters are required. `200 { "notes": [...] }`, oldest first.
`404` when the entity does not exist.

### POST /api/notes

```json
{ "entity_type": "offer", "entity_id": 7, "body": "call back Monday",
  "pinned_event_id": 4 }
```

`body` required; `pinned_event_id` optional. The event must belong to
the offer, or (for a company note) to an offer of that company.
`201 { "note": {...} }`.

### GET /api/notes/:id

`200 { "note": {...} }`, `404 not_found`.

### PATCH /api/notes/:id

`body` and/or `pinned_event_id` (`null` removes the pin). Pinning is
validated against the note's entity. `200 { "note": {...} }`.

### DELETE /api/notes/:id

`204`.

## Companies

### GET /api/companies

Parameters: `q` (matches the display or normalized name), `limit`
(default 100, max 500), `offset`.

```json
{ "companies": [ { "id": 2, "name": "Acme", "name_normalized": "acme",
                   "website": null, "created_at": "...",
                   "offer_count": 3, "note_count": 1 } ],
  "total": 1, "limit": 100, "offset": 0 }
```

### POST /api/companies

```json
{ "name": "Acme", "website": "https://acme.example" }
```

`201 { "company": {...} }`. `409 duplicate_company` when the normalized
name already exists.

### GET /api/companies/:id

Company, its offers (up to 200, most recently seen first), its
company-level notes, and conservative possible-duplicate candidates
that the UI may propose for merging:

```json
{
  "company": { "...": "..." },
  "offers": [ { "...": "..." } ],
  "notes": [ { "...": "..." } ],
  "possible_duplicates": [ { "id": 9, "name": "Acme France",
                             "name_normalized": "acme france",
                             "score": 75, "reason": "one name extends the other" } ]
}
```

Candidates are proposals only; nothing is merged automatically.

### PATCH /api/companies/:id

`name` and/or `website` (`null` clears the website). Renaming changes
the normalized name and therefore rewrites the fingerprints of the
company's offers so future scans keep deduplicating correctly.
`200 { "company": {...} }`.

- `409 duplicate_company` when the new normalized name is taken.
- `409 merge_conflict` when a rewritten fingerprint would collide with
  an existing offer; the message lists the offer IDs and nothing is
  written.

### DELETE /api/companies/:id

`204`. Refused with `409 company_has_offers` when the company still has
offers (offers are never deleted; merge the company instead). Company
notes of the deleted company are removed.

### POST /api/companies/:id/merge

Merges company `:id` **into** `into_id`; this is only ever called after
an explicit user confirmation.

```json
{ "into_id": 2 }
```

In one transaction: offers and company-level notes move to the target,
offer fingerprints are rewritten with the target's normalized name
(keeping their location bucket), the target inherits the source website
when it has none, and the emptied company row is deleted.

```json
{ "company": { "id": 2, "...": "..." },
  "merged_company_id": 1, "moved_offers": 4, "moved_notes": 1 }
```

- `400 validation_failed` when merging a company into itself.
- `404 not_found` when either company does not exist.
- `409 merge_conflict` when a moved offer's new fingerprint already
  exists in the target (same company + title + location bucket). The
  merge is rolled back; the user must resolve the offers first.

## Interview prep

A personal FAQ of anticipated interview questions and prepared answers.
Questions are plain text; answers are stored as **markdown plaintext**
(never HTML) and rendered escaped by the UI. Duplicate questions are
allowed: this is a personal notebook, not a deduplicated domain.
`answer_format` is enum-like and currently accepts only `markdown`.

```json
{
  "id": 1,
  "question": "Tell me about a production incident.",
  "answer": "**Situation**\n\n- alert fired\n- rolled back",
  "answer_format": "markdown",
  "category": "Behavioural",
  "display_order": null,
  "created_at": "2026-11-17T09:00:00Z",
  "updated_at": "2026-11-17T09:00:00Z"
}
```

`category` is a free-form display name or `null` (uncategorized). There is
no categories table and no `/categories` endpoint: the set of categories
is derived from the stored rows.

### GET /api/interview-entries

`200 { "interview_entries": [...] }`. Rows are grouped by category.
Categories sort alphabetically (case-insensitive) with uncategorized rows
last. Within each category, rows with an explicit `display_order` come
first in ascending order; rows without one follow by `id`.

### POST /api/interview-entries

```json
{
  "question": "Why this company?",
  "answer": "optional markdown",
  "category": "Motivation"
}
```

- `question` required and non-blank; it is trimmed.
- `answer` optional (defaults to the empty string). It may be empty at
  creation.
- `answer_format` optional, defaults to `markdown`; any other value is
  `400 validation_failed`.
- `category` optional: when supplied it is trimmed and must not be blank;
  omitted or `null` stores `null` (uncategorized). A blank string is
  `400 validation_failed`.
- `display_order` optional integer.
- `201 { "interview_entry": {...} }`.

### GET /api/interview-entries/:id

`200 { "interview_entry": {...} }`, `404 not_found`.

### PATCH /api/interview-entries/:id

Any of `question`, `answer`, `category`, `display_order`.

- `question` must be non-blank when supplied; it is trimmed.
- `answer` replaces the stored markdown; `""` clears it.
- `category` is a string (trimmed, must not be blank) or `null` to clear
  it back to uncategorized. `""` and non-string values are
  `400 validation_failed`.
- `display_order` is an integer, or `null` to remove the manual position.
- `answer_format` is not patchable (sending it is `400 invalid_json`).
- An empty patch is `400 validation_failed`.
- `200 { "interview_entry": {...} }`; `404 not_found`.

### DELETE /api/interview-entries/:id

`204`. Removes the entry permanently.

## Error codes

| HTTP | `code` | When |
|---|---|---|
| 400 | `invalid_json` | body is not one JSON object, or has unknown fields |
| 400 | `invalid_id` | path ID is not a positive integer |
| 400 | `validation_failed` | missing/invalid field, empty patch, self-merge |
| 400 | `invalid_query` | invalid FTS5 search expression |
| 422 | `metadata_incomplete` | URL import fetched the page but needs company/title from the user; response includes `metadata` |
| 422 | `metadata_unavailable` | URL import could not fetch or parse the page; response includes `metadata` |
| 404 | `not_found` | unknown endpoint, entity or child entity |
| 409 | `duplicate_source` | another source uses the URL |
| 409 | `duplicate_company` | normalized company name already exists |
| 409 | `company_has_offers` | deleting a company that still owns offers |
| 409 | `merge_conflict` | offer fingerprints would collide during rename/merge |
| 409 | `offer_conflict` | splitting an offer would reuse an existing company+title+location fingerprint |
| 500 | `internal` | unexpected storage error |
