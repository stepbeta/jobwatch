# Proposal

## Why

Milestone 10a verified the `wttj` adapter against one company (Fabriq), whose
bootstrap happens to expose the Algolia credentials in a block that is itself
`kind: "jobs-search"`. A second company page (Partoo) shows that this is not the
general shape: the outer `jobs-search` block is a pure layout container
(`properties: {"columns": 3}`) and the credentials live in a nested `contents[]`
block. Any discovery that keys off the block kind — or takes the first
`jobs-search` block — can miss the credentials and report a permanent "not a
careers page" error.

The query side is also wrong for multi-organization pages. Milestone 10a built
`filters=organization.slug:fabriq` from the discovered `filters` object. Partoo's
discovered filters contain several slugs in one group
(`{"organization.slug": ["localoo", "partoo-italia", "partoo"]}`), which a space
join turns into an AND — no hits. And because the secured key only embeds
`website.reference:wttj_fr`, a request without explicit filters downloads the
entire WTTJ index (~60k hits instead of ~15). The key cannot be relied on to
scope the query.

Finally, a page may list jobs of organizations linked to it (local brands), so a
job URL built from the source page's slug can point at the wrong company page.
The hit's own `organization.slug` is authoritative.

Swile exposes a second upstream inconsistency. Its bootstrap filters use the
public website slug (`swile`), while the Algolia index still stores the
canonical organization slug (`lunchr`), so the otherwise correct facet query
returns zero offers for a page that lists 15. WTTJ's organization record
(`GET /api/v1/organizations/{website slug}`) carries the canonical slug; the
adapter must consult it when an empty first page contradicts the page's own
job count.

## What Changes

- **Discovery** recursively searches the `/api/v1/pages` response for the object
  whose `properties` contain `algolia_app_id`, traversing nested `contents[]`
  arrays. The block `kind` is no longer a gate, so layout-only `jobs-search`
  blocks are harmless. An absent credentials object stays a permanent
  `URL is not a WTTJ careers page` error.
- **Algolia query** replicates the discovery block's `filters` as `facetFilters`
  in the request params: one group per attribute, several values in a group are
  OR-ed (`[["organization.slug:localoo","organization.slug:partoo-italia",
  "organization.slug:partoo"]]`), and the JSON is URL-encoded inside the params
  string. The filter embedded in the secured key (`website.reference`) is never
  repeated in params. `Referer`, the form content type with a raw JSON body, and
  the untouched key are kept as verified in 10a.
- **Job URL construction** uses the hit's `organization.slug`; the discovered
  page slug remains a fallback only for hits that omit it.
- **Stale organization filters** are resolved when the first page's `nbHits`
  differs from the page's `items_count`: the adapter reads the canonical slug
  of every `organization.slug` value from
  `/api/v1/organizations/{slug}`, deduplicates them and retries the first page
  once. This fixes renamed organizations such as Swile (`swile` → `lunchr`)
  and renamed brands on multi-organization pages.
- **Tests** (httptest only): nested-block discovery, multi-org facetFilters with
  URL-encoded params, org-slug-based hit mapping. New trimmed fixtures: the
  Partoo bootstrap (nested block shape) and one Partoo hit (without
  `_highlightResult`/`_snippetResult`/images).
- **Docs**: `SOURCE-DECISIONS.md` gains the new known gaps and the revised flow,
  `USAGE.md` describes the multi-organization behavior, and `MILESTONES.md` /
  `CHANGELOG.md` record the fix-up. Text and Mermaid only.

## Capabilities

### New Capabilities

- `wttj-source`: how the WTTJ adapter discovers Algolia credentials from a
  company careers page, scopes the index query with the page's own filters, and
  addresses each job through its own organization slug.

### Modified Capabilities

None. `source-normalization` describes the adapter contract every source type
fulfils; this change refines the obligations of one adapter, which the new
capability owns.

## Impact

- `internal/sources/wttj.go` (discovery walk, params/facetFilters builder, URL)
- `internal/sources/wttj_test.go`, new fixtures under
  `internal/sources/testdata/` (and its README)
- `docs/SOURCE-DECISIONS.md`, `docs/USAGE.md`, `docs/MILESTONES.md`,
  `CHANGELOG.md`
- No schema, API endpoint, CLI flag or UI change; no new dependency.
