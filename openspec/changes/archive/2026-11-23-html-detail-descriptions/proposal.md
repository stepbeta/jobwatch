# Proposal

## Why

The HTML adapter stores only what the listing page exposes. Two server-rendered sources keep their full job text on a linked detail page instead: the Wazuh careers page has no description in the listing, and Murena's listing stores a summary followed by a "Read full job description." link, so its offers currently hold that summary rather than the actual posting. Both detail pages are plain server-rendered HTML, so the information is reachable without a new ATS adapter. Re-fetching every detail page on every scan is not acceptable: the WTTJ fallback already records that AWS WAF budgets force it to skip stored descriptions, and HTML sources should not assume otherwise.

## What Changes

- Add an opt-in `detail_description_selector` to the HTML source config. When set, the adapter fetches an item's resolved link through the shared polite client, extracts the selected element as plain text, and stores it as the offer description.
- Fetch a detail page only when the offer has no stored description, or when the stored text still equals the listing description (the Murena upgrade case). Otherwise reuse the stored text and make no detail request. This keeps routine scans at one listing request.
- Stop detail fetching for the rest of a scan after a blocked response (`ErrBlocked`: robots disallow, 401, 403, 429), leaving the listing and stored texts in place and the source `ok`.
- Treat a per-item detail failure or a selector miss as "no detail text": fall back to the listing description and continue.
- Update Murena's seeded HTML source config (data-only migration) with `detail_description_selector: "section.entry-content"`, so the five existing offers are upgraded from the summary to the full text on the next scan.
- Register `https://wazuh.com/careers/` as a local HTML source tracking only the "Open positions" cards (the "Upcoming positions" table is not open for applications), with `detail_description_selector: ".job-content-description"`.
- Document the new config key, its request budget, the fail-fast rule and the Wazuh/Murena decisions.

Out of scope: a new source type, changes to the manual import path, fixing the invalid Wazuh JSON-LD (its `JobPosting` script cannot be decoded because of raw newlines; the selector path does not need it), and detecting description edits after the first capture.

## Capabilities

### New Capabilities

- `html-source`: how a CSS-selector HTML listing becomes offers, including the optional detail-page description fetch, its stored-text skip, per-item failure isolation and polite request budget.

### Modified Capabilities

None. The adapter contract (normalized values, stable uid, per-item failures) in `source-normalization` is unchanged; this capability defines the HTML-specific surface.

## Impact

- `internal/sources/fetcher.go` — new `Config` field.
- `internal/sources/html.go` — selector compilation, conditional detail fetch, stored-text comparison, fail-fast.
- `internal/sources/sources_test.go` and testdata — table/httptest coverage for the new path.
- `migrations/0016_*.sql` — Murena seeded config update.
- `docs/USAGE.md`, `docs/ARCHITECTURE.md`, `docs/SOURCE-DECISIONS.md`, `docs/MILESTONES.md`, `docs/SPEC.md`, `CHANGELOG.md`.
- The user database gains one local Wazuh source row; no schema, API or dependency changes.
