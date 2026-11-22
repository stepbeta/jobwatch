# Proposal

## Why

Welcome to the Jungle's Algolia index does not always carry a description: for some organizations both `profile` and `summary` are empty in every hit (Joko 46/46, Partoo 15/15) while the public job page contains the full `JobPosting.description` in JSON-LD. JobWatch therefore stores 61 offers with `raw_text = NULL`, which weakens reading, classification and remote-policy derivation. Joko also publishes the same 46 roles on Ashby with full descriptions, so its WTTJ source is the wrong source for that company.

## What Changes

- The WTTJ adapter keeps `hit.profile` with `hit.summary` as its first fallback. When both are blank it fetches the public offer page and extracts the JSON-LD `JobPosting.description`, converting the HTML to plain text. It does this during the scan for a description-less hit that has no stored description yet; the scan pipeline passes the known descriptions to the adapter. A detail-page failure leaves the description empty and does not fail the source. Detail requests use the same single browser User-Agent the adapter already sends to WTTJ's API endpoints, because the WAF challenges the identifying JobWatch UA. Partoo uses a 90 s interval to stay under the WAF burst threshold.
- The JSON-LD JobPosting finder that is currently private to `internal/importmeta` moves to a shared helper under `internal/sources`, so manual import and the WTTJ adapter use one parser instead of two.
- Data migration, explicitly requested by the user (one-off, not a scan behavior): back up the database, stop `jobwatch serve`, delete the 46 Joko offers and WTTJ source 87, add the Ashby source `https://jobs.ashbyhq.com/joko` (name `Joko`), link the `ashby:joko` company key to the existing Joko company (id 115) so no duplicate company is created, scan the new source, and restore the bookmark from old offer 2869 to its Ashby homologue (`b83d7f0f-a0d0-4dc6-b305-7d9336e4d389`).
- Rescan the Partoo WTTJ source to backfill its 15 descriptions.
- Tests: a JSON-LD job-page fixture, the empty-profile fallback, a non-empty profile making no page request, a failed detail page keeping the offer, and the existing `importmeta` tests after the helper move.
- Docs: the `wttj-source` capability delta, `docs/SOURCE-DECISIONS.md`, `docs/ARCHITECTURE.md`, `docs/MILESTONES.md` known gaps and `CHANGELOG.md`. No schema or API change.

The deletion of the Joko offers is an explicit user-confirmed one-off migration. It does not change the automated no-delete rule: scans still never delete offers.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `wttj-source`: add a requirement that a blank index description falls back to the public job page's JSON-LD `JobPosting.description`, and update the mapping scenarios.

## Impact

- Code: `internal/sources/wttj.go`, new `internal/sources/jsonld.go`, `internal/sources/wttj_test.go`, `internal/sources/sources_test.go`, `internal/sources/testdata/`, and `internal/importmeta/importmeta.go` (consume the shared helper).
- Data only: source 87 (Joko, WTTJ) and offers 2853–2898 are removed after a backup; a new Ashby source and 46 offers are created; company 115 is kept; the bookmark on the Ashby homologue is restored. No migration file and no schema change.
- Scan duration for a WTTJ source grows by one page request per description-less hit (Partoo is ~30 s; Joko no longer has a WTTJ source after the migration).
