# Design

## Context

See `proposal.md` - Why.

- `internal/sources/html.go` fetches one listing page and extracts item fields with cascadia selectors; `scan.go` already attaches `WithKnownDescriptions` for every source, keyed by `job_offer_provenance.external_uid`.
- The WTTJ adapter skips detail fetches for offers with a stored description because AWS WAF budgets a small number of page requests; this change treats HTML detail pages with the same caution.
- Wazuh: `careers/` exposes `#careers-open` cards and a `#careers-upcoming` table. Upcoming pages state they are "not open for applications", so only the open cards are tracked. The full text is in `.job-content-description`; the page's `JobPosting` JSON-LD is invalid JSON (raw newlines in the string) and cannot be used.
- Murena: the five tracked rows store the listing summary plus `Read full job description.`; the link target holds the full text in `section.entry-content`. Wazuh is fronted by CloudFront, Murena by plain nginx; neither was probed beyond a handful of 200 responses, so their rate limits are unknown.
- HTML sources already have a 5 s minimum interval and a single identifying User-Agent; no new dependency is needed.

## Goals / Non-Goals

**Goals:**

- One optional config key that turns full detail text on for a source, with no new source type.
- Routine scans make zero detail requests; the first scan after enabling (or after an upgrade) makes one per eligible item.
- Murena's existing summary rows become the full text once, without a destructive data migration.
- A blocked detail page stops the detail phase instead of hammering the host.

**Non-Goals:**

- Re-fetching detail text on every scan or detecting later description edits.
- Fixing the invalid Wazuh JSON-LD, or changing manual import.
- Parallel detail requests, a new database column, or UI changes.
- A generic JSON-LD detail extractor; the selector covers both current sources.

## Decisions

### `detail_description_selector` is a plain CSS selector

Add one `Config` field, compiled with cascadia like the listing selectors. It points at the element whose children become the description, using the existing `nodeDescription` HTML→text conversion. A selector was chosen over a `fetch_detail` boolean plus JSON-LD because both sites expose the text as HTML and Wazuh's JSON-LD is not decodable. An invalid selector fails the source, matching the existing selector handling.

### The fetch condition is based on the stored text

For each item, with `stored := known[uid]` and `listing := description from the listing selectors`:

```text
needDetail = (stored == "") || (normalized(stored) == normalized(listing))
```

- If `needDetail`, fetch the detail page and use its text when non-empty.
- Otherwise skip the request and emit `stored`.

`normalized` trims and collapses whitespace. This makes the first Wazuh scan fetch (no stored text), and makes the first Murena scan fetch once because the stored summary equals the listing summary; after that the stored full text differs from the summary and is reused. Emitting `stored` on the skip path is essential: returning the listing summary instead would let the scan overwrite the full text.

A stable uid is required for the lookup. Sources without a `uid_selector`/`uid_attr` cannot skip and are documented as fetching every scan.

Alternative considered: always fetch. Rejected because it re-spends the WAF budget on every scan; the WTTJ precedent exists precisely because that budget can run out. Alternative: a one-off backfill command or nulling Murena's `raw_text` in the migration. Rejected as destructive and unnecessary once the equality rule exists.

### Blocked detail requests end the detail phase

Any `ErrBlocked` (robots disallow, 401, 403, 429) stops further detail requests for that scan; the listing offers are still returned and the source stays `ok`, because the listing itself succeeded. Other errors (network, 5xx, selector miss, empty text) are per-item misses and do not stop the loop.

A 2xx WAF challenge page that parses as ordinary HTML is indistinguishable from a selector miss; it is treated as a miss and retried next scan. Combined with the skip rule, this can at worst cause a few requests per scan, never a burst. If a challenge is observed in practice, the recorded remedy is raising `rate_limit_ms` or, like WTTJ, a browser User-Agent exception - not automatic case-by-case spoofing.

### Detail requests stay sequential on the shared client

The adapter uses `h.Client.Get`, so robots.txt, the source interval (5 s HTML minimum) and the identifying UA apply unchanged; requests are issued inside the existing item loop. No concurrency is introduced, which keeps the request pattern predictable for a WAF.

### Murena is upgraded by the data rule, not a data migration

Migration 0016 only adds `detail_description_selector: "section.entry-content"` to Murena's seeded config. The equality rule then fetches the five detail pages on the next scan and the scan stores the full text, keeping a snapshot for the change. Wazuh is registered locally with `detail_description_selector: ".job-content-description"` and only the `#careers-open` card selectors; no seed migration, per the user's choice.

### Keep the identifying User-Agent for now

Both sites returned 200 to the JobWatch UA for the listing and a detail page. No browser-UA override is added; if either site starts challenging, that becomes a recorded exception like WTTJ's, not a default.

## Risks / Trade-offs

- [A detail page starts challenging] → The skip rule keeps routine scans at one listing request; a blocked response stops the detail phase, and the interval is configurable.
- [The listing summary changed before the first upgrade] → The equality check no longer matches, so the old summary is kept and no detail fetch happens. Low impact (one stale summary); a future explicit refresh can re-trigger it.
- [Description edits after the first capture are invisible] → Same trade-off as WTTJ. Deliberately out of scope; an opt-in refresh mode can be added later without changing this contract.
- [Selector drift on a redesign] → Per-item misses fall back to the listing text; the source stays `ok`, so the failure is silent until noticed. The verification scan and fixture tests cover the current layouts.
- [Large HTML sources opt in] → Each eligible item costs one request at the interval; the key is opt-in and documented with its cost. The two configured sources are small (5 and 1-2 items).
- [Murena detail text includes the meta block] → Accepted: Team/Location/Contract are useful and come before the prose.
- [An offer link points back at the listing] → The selector may capture listing text; documented as a configuration mistake, not guarded.

## Migration Plan

1. Preparation: cut `feat/html-detail-descriptions` from an up-to-date `main` (the repository's default branch); if `jobwatch serve` is running, ask the user to stop it; then back up the database with `jobwatch backup`.
2. Ship the adapter change, migration 0016 and docs; build and restart `serve` (or open the DB with the new CLI, which applies the data-only migration).
3. Scan Murena: five detail requests, descriptions change from summary to full text, snapshots recorded.
4. Insert the local Wazuh source and scan it: one or two open offers, full descriptions; later scans reuse them.
5. Rollback: remove `detail_description_selector` from the affected source configs (the code path is inert without it); migration 0016 only adds a JSON key, so no schema rollback is needed.

## Open Questions

None. A description refresh mode and a fix for the invalid JSON-LD are explicitly deferred.
