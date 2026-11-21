# Tasks

## 0. Preliminary database backup

- [x] 0.1 Before any other task, check that no `jobwatch` process holds the database (`pgrep -af jobwatch`); if one is running, ask the user to stop it, then run `jobwatch backup jobwatch.db.<timestamp>.bkp` and verify the backup reported a successful integrity check and the file exists. Repeat at the start of any later session that continues this change

## 1. Discovery

- [x] 1.1 Add the trimmed Partoo bootstrap fixture `internal/sources/testdata/wttj_page_partoo.json` (layout-only outer `jobs-search` block, credentials in nested `contents[]`, three `organization.slug` values, real key tail) and record it in `internal/sources/testdata/README.md`; verify the file is valid JSON and contains `algolia_app_id`
- [x] 1.2 Replace the kind-gated finder with a recursive walk that selects any `properties` object containing `algolia_app_id`, traversing nested `contents[]` arrays in deterministic order; verify `TestParseWTTJPages` covers the Partoo fixture, credentials directly in properties, several blocks, the permanent no-credentials error and malformed JSON
- [x] 1.3 Delete the now-unused kind/`findMapWithKey` helpers and verify `go vet ./internal/sources` plus `go test ./internal/sources -run 'TestParseWTTJPages|TestWTTJErrors'` pass

## 2. Query scoping

- [x] 2.1 Replace the flat filter slice with ordered groups so a multi-valued attribute survives discovery; verify a parse test asserts all three `organization.slug` values from the Partoo fixture
- [x] 2.2 Build `params` as URL-encoded `facetFilters` JSON (one group per attribute, values OR-ed), omit `website.reference` because the secured key embeds it, and keep `hitsPerPage`/`page`; verify `TestWTTJAlgoliaParams` asserts the exact multi-organization, single-value, no-filter and key-filter cases
- [x] 2.3 Update the httptest fetch tests to assert the exact `facetFilters` params on every page and keep asserting the `Referer`, form content type with raw JSON body and credential headers; verify `TestWTTJFetch` and `TestWTTJFetchConfigOverrides` pass
- [x] 2.4 Update `docs/SOURCE-DECISIONS.md` (flow step 2 and the quirks table) to describe `facetFilters`, OR within a group and the key-embedded `website.reference` exclusion; verify no text still claims `params` starts with `filters=`

## 3. Job URL from the hit's organization

- [x] 3.1 Add the trimmed Partoo hit fixture `internal/sources/testdata/wttj_hits_partoo.json` (one hit, `organization.slug` `partoo`; the live query returned 15 hits, all under `partoo`, so no sibling-org hit existed — the precedence case is covered in the unit test) and record it in `internal/sources/testdata/README.md`; verify the file is valid JSON
- [x] 3.2 Extend the hit-mapping test with the Partoo hit: the URL and company key use the hit's organization slug even when discovery reports a different source slug (the table case uses a `partoo-italia` fallback); keep the fallback and dropped-hit cases; verify `TestWTTJMapHit` and `TestWTTJMapPartooHit` pass
- [x] 3.3 Update the `docs/SOURCE-DECISIONS.md` hit-mapping table (organization slug first, discovery slug as fallback), add the two new known gaps (nested `jobs-search` blocks; discovery filters must become `facetFilters`) and update `docs/USAGE.md` to explain multi-organization pages; verify the documented behavior matches the tests

## 4. Verification and milestones

- [x] 4.1 Add the 10a fix-up to `docs/MILESTONES.md` (done criteria, deviations, updated known-gaps table) and an unreleased `CHANGELOG.md` entry; verify both name the Partoo verification and the three fixes
- [x] 4.2 Run `go build ./...`, `go vet ./...` and `go test ./...`; verify all pass and no frontend build is needed
- [x] 4.3 Re-scan the Fabriq source (same 7 offers, 0 new); for Partoo, look for an existing Partoo `wttj` source first (`jobwatch list --sources | grep -i partoo`, or the Sources page): if one exists, scan that row as-is and ask the user before changing its URL or enabled state; only when none exists, add `https://www.welcometothejungle.com/en/companies/partoo/jobs` (or the page's actual locale) and scan it. Use the backup taken in 0.1 (take a fresh one if this runs in a later session); verify the Partoo offer count is the page's own (~15, not ~60k), each URL names the hit's organization slug, and record the live results in `docs/SOURCE-DECISIONS.md` and `docs/MILESTONES.md`

## 5. Stale organization slugs

- [x] 5.1 Add the trimmed Swile fixtures `wttj_page_swile.json` (items_count 15, filters `organization.slug: swile`) and `wttj_hits_swile.json` (organization slug `lunchr`, `website_organization.slug: swile`), record them in `internal/sources/testdata/README.md`, and verify both parse as JSON
- [x] 5.2 Parse `page.path[0].items_count` into discovery as `wttjSearch.ExpectedItems`; verify a parse test asserts 15 for Swile and 7 for Fabriq
- [x] 5.3 When page 0 is empty, `ExpectedItems > 0` and exactly one `organization.slug` filter exists, read the canonical slug from `GET /api/v1/organizations/{website slug}`, replace the filter value and re-query page 0 once; a lookup failure is retryable. Verify httptest tests for the stale, matching, empty-page and failure cases, and that a normal Fabriq fetch makes no organization request
- [x] 5.4 Update `SOURCE-DECISIONS.md` (flow, quirks/known gaps), `USAGE.md`, `ARCHITECTURE.md`, `MILESTONES.md` and `CHANGELOG.md`; verify they describe the alias retry and the multi-organization limitation
- [x] 5.5 Back up the database again, rebuild the binary, re-scan Swile (15 offers, 0 incomplete), then re-scan Fabriq and Partoo (unchanged); record the live results in `docs/SOURCE-DECISIONS.md` and `docs/MILESTONES.md`
- [x] 5.6 Run `go build ./...`, `go vet ./...` and `go test ./...`; verify all pass

## Workflow follow-up

- Archive `harden-wttj-discovery-and-query` after review, sync `wttj-source` to `openspec/specs/`, and replace the WTTJ note in `docs/SPEC.md` §7 with a pointer to that capability plus a spec revision entry.
- Commit the session (no push).
