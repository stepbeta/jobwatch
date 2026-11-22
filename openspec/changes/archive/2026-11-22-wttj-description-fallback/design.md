# Design

## Context

See `proposal.md` — Why. The current WTTJ adapter is a two-call flow: one bootstrap page-tree read plus one Algolia multi-query per result page. It maps `hit.profile` and falls back to `hit.summary`; it never requests an offer page. `internal/importmeta` already extracts a JSON-LD `JobPosting` from an arbitrary page for manual import, but it imports `internal/sources`, so `sources` cannot import it back.

The affected data lives in the local SQLite file: WTTJ source 87, company 115 (`Joko`), offers 2853–2898 (one bookmark on 2869, no events or notes), and WTTJ source 82 (`Partoo`) with 15 description-less offers. The project's normal rule is that scans never delete offers; this change's Joko migration is an explicit, user-confirmed one-off exception.

## Goals / Non-Goals

**Goals:**

- Backfill descriptions for WTTJ hits whose index text is blank, on every scan, through the existing polite client.
- Reuse one JSON-LD `JobPosting` parser in both manual import and WTTJ.
- Replace the Joko WTTJ source with the richer Ashby board without creating a duplicate Joko company or losing the one bookmark.
- Keep the change reversible for a single SQLite file: backup first, no schema change, no migration file.

**Non-Goals:**

- No generic “fetch every ATS detail page” behavior; only WTTJ needs it because its index omits the text.
- No change to the no-delete rule for scans or to merge semantics.
- No new API endpoint, CLI command, config flag or UI control.
- No attempt to preserve the old Joko offers, snapshots or provenance; only the bookmark is user data and is reattached to the Ashby homologue.

## Decisions

### Detail fallback lives in `WTTJ.Fetch`

When `mapWTTJHit` returns an empty description, `Fetch` calls a new `detailDescription(ctx, src, offerURL)` helper after mapping. The helper fetches the page through `w.Client.fetch` with `text/html`, the WTTJ browser User-Agent already used for the site's API endpoints, parses the page, finds the JSON-LD `JobPosting`, and returns `HTMLToText(description)`.

- Chosen over a separate scan phase or a `Fetcher` interface change: the fallback is specific to this adapter, and the scan pipeline already has the stored descriptions it passes through the context. A separate phase would add cross-package plumbing without changing the result.
- **Stored descriptions are skipped:** the scan pipeline loads the descriptions already stored for the source and passes them through the context (`sources.WithKnownDescriptions`). A hit whose uid has stored text is not fetched again, so the WAF allowance is spent only on genuinely missing descriptions. This is what lets a normal scan reach the tail of the hit order and keeps Partoo scans short once backfilled.
- Consequence: `jobwatch reattribute` also calls `Fetch` and will fetch detail pages for description-less hits. That is accepted; it is rare and bounded by the same interval.
- **Recorded exception:** WTTJ's WAF answers the identifying JobWatch UA on HTML pages with an HTTP 202 challenge. Detail requests therefore use the single browser UA the adapter already sends to WTTJ's API endpoints; there is no per-request UA rotation. Requests still honor robots.txt and the per-source interval.
- Consequence: a WTTJ source with many description-less hits needs a slower `rate_limit_ms` (Partoo uses a conservative interval) so the WAF does not challenge the batch.

### Add a `SiteURL` override for tests

`WTTJ` gains `SiteURL string`, used when building offer URLs and therefore for the detail request. `wttjSearch` carries the resolved site URL so `mapWTTJHit` stays usable in table tests. Empty means the production `https://www.welcometothejungle.com`. Without this, any test with a blank profile would hit the real site.

### Shared JSON-LD helper in `internal/sources`

Move the JSON-LD `JobPosting` finder (`findJobPosting`, `typeContains`) into `internal/sources/jsonld.go`, exported as a function that accepts an `*html.Node` and returns the first `JobPosting` object. `internal/importmeta` keeps its field extraction (`title`, `hiringOrganization`, `jobLocation`, `description`, `url`) but calls the shared finder.

- Chosen over a new `internal/jobpage` package: `sources` already owns HTML parsing and `HTMLToText`, and the move avoids a new package in the documented layout.
- Alternative rejected: keep two parsers. Duplicated parsing would drift when the JSON-LD shape changes.

### Best-effort detail failures

A failed detail request, a non-2xx response, malformed HTML or a missing `JobPosting.description` leaves that offer's description empty and continues. The source is not marked `error`, because the listing itself succeeded and all other offers are valid.

- Alternative rejected: fail the whole source on any detail failure. One stale job URL would then discard the entire listing and mark it disappeared on a later scan.
- Risk accepted: a systemic block of detail pages is not visible in the scan status. The smoke test checks that Partoo descriptions are non-empty after a scan; a future warning field can be added if this becomes common.

### Joko data migration is a one-off SQL operation, not a migration file

The migration is applied with `sqlite3` (or the app's DB handle) inside a transaction, after a backup and with `serve` stopped. It is not added to `migrations/` because it is instance-specific data, not a schema change, and the project's migrations are additive/reversible.

Order:

1. Record the bookmark from offer 2869 (`saved_at = 2026-11-22T17:09:35Z`, title `Platform Software Engineer (senior/staff level)`).
2. Delete the 46 offers through provenance of source 87, then delete source 87:
   ```sql
   PRAGMA foreign_keys = ON;
   BEGIN;
   DELETE FROM job_offers
    WHERE id IN (SELECT offer_id FROM job_offer_provenance WHERE source_id = 87);
   DELETE FROM sources WHERE id = 87;
   COMMIT;
   ```
   The offer delete cascades `events`, `snapshots` and `provenance`; the source delete cascades its scoped company alias. Company 115 and its `wttj:joko-1` key remain.
3. Add the Ashby source: `./jobwatch add-source https://jobs.ashbyhq.com/joko --name Joko --config '{"company":"Joko"}'`.
4. Link the new source to the existing company before the first scan, so `ResolveCompany` finds company 115 by key instead of creating a second `Joko`:
   ```sql
   INSERT INTO company_keys(company_id, key, created_at)
   VALUES (115, 'ashby:joko', <now>);
   ```
   The scan then adds the source-scoped `joko` alias for the new source through `linkCompanyAlias`.
5. Scan the new source.
6. Restore the bookmark by matching the stable Ashby job id
   `b83d7f0f-a0d0-4dc6-b305-7d9336e4d389` (fallback: exact title):
   ```sql
   UPDATE job_offers
      SET saved_at = '2026-11-22T17:09:35Z', updated_at = <now>
    WHERE id = (SELECT offer_id FROM job_offer_provenance
                 WHERE source_id = <new source id>
                   AND external_uid = 'b83d7f0f-a0d0-4dc6-b305-7d9336e4d389');
   ```
7. Scan Partoo (source 82) to exercise the description fallback.

If the Ashby job is no longer listed at implementation time, skip the SQL and leave the bookmark to the user; do not guess a different job.

### Branch and backup are preconditions

Implementation starts with `git fetch origin` and `git switch -c wttj-description-fallback origin/main`. The database is backed up with `jobwatch backup jobwatch.db.<timestamp>.bkp` only after `jobwatch serve` is stopped; if a process still holds the file, stop and ask the user. The backup must pass its integrity check before any delete.

## Risks / Trade-offs

- **Scan cost** — A WTTJ source adds one page request per description-less hit that has no stored text. A fully backfilled source costs none; Partoo needed 15 during the first backfill (~22 min at 90 s spacing under the WAF) and costs only new offers afterwards. Mitigation: the scan passes known descriptions to the adapter, so stored text is never re-fetched.
- **WTTJ page shape** — The fallback depends on a JSON-LD `JobPosting` block. If the page drops it, descriptions stay empty but the scan still succeeds. Mitigation: fixture test and a documented known gap.
- **WAF challenge** — WTTJ's CloudFront WAF answers the identifying JobWatch UA on HTML pages with HTTP 202 (empty body), and challenges even the browser UA after a burst of requests. Mitigation: the recorded single browser-UA exception plus a conservative `rate_limit_ms` on WTTJ sources with many description-less hits; a challenged page leaves the description empty and does not fail the source.
- **Systemic detail failure is silent** — The source remains `ok` with empty descriptions. Mitigation: verify Partoo's `raw_text` after the migration scan; add a warning later if needed.
- **Irreversible Joko delete** — The 46 old offers and their empty snapshots are removed. Mitigation: backup first, verify the bookmark value, and keep company 115. Rollback is restoring the backup while `serve` is stopped.
- **Duplicate company if the key link is missed** — A new `Joko` company would be created. Mitigation: insert `ashby:joko` before the scan and verify one company with `name_normalized = 'joko'`.
- **`reattribute` cost** — It also fetches detail pages for description-less hits, but skips those with a stored description. Accepted for this small dataset; the behavior is documented.

## Migration Plan

1. `git fetch origin`; create `wttj-description-fallback` from `origin/main`.
2. Stop `jobwatch serve` (ask the user if it is running).
3. `jobwatch backup jobwatch.db.<timestamp>.bkp`; confirm “ok” and the integrity check.
4. Implement the adapter fallback, shared JSON-LD helper, tests and fixtures; `go build ./...`, `go vet ./...`, `go test ./...`.
5. Apply the Joko data migration (capture bookmark → delete offers/source → add Ashby source → link company key → scan → restore bookmark).
6. Scan Partoo; verify all 15 descriptions are non-empty. Partoo's `rate_limit_ms` is raised first so the WAF does not challenge the batch.
7. Update docs and `CHANGELOG.md`; commit the branch.
8. Rollback: restore the backup with no `serve` running, then discard the branch.
