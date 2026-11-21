# Design

## Context

See `proposal.md - Why`. The current adapter (`internal/sources/wttj.go`) has
three behaviors that the Partoo page exposes as assumptions rather than rules:

- `findJobSearchProperties` only descends into subtrees whose block carries
  `kind: "jobs-search"`; credentials living under a generic container are never
  seen.
- `wttjFilters` flattens a filter object into `attribute:value` pairs joined with
  spaces, so a multi-valued `organization.slug` group becomes an AND under
  `filters=`.
- `mapWTTJHit` already prefers `hit.organization.slug`, but the behavior has no
  test that separates a hit's organization from the source page slug.

The two-call flow, credentials, headers, content type and pagination were verified
live in Milestone 10a and are documented in `docs/SOURCE-DECISIONS.md`. That
document also carries the request quirks; this design only changes what 10a got
wrong.

## Goals / Non-Goals

**Goals:**

- Discovery succeeds on any bootstrap shape that publishes the `algolia_*`
  properties somewhere in the page tree.
- A scan returns only the jobs the page's own filters select, including pages
  that aggregate several linked organizations.
- Each offer URL and company identifier follows the organization that owns the
  hit.
- The behavior is covered by httptest-only tests with fixtures from two
  companies.

**Non-Goals:**

- Decoding or reproducing the secured key's embedded filter.
- Persisting discovery (credentials, filters, index) beyond one scan.
- New source types, schema changes, API changes or UI work.
- Expanding hit mapping (salary, remote, contract type stay unmapped).

## Decisions

### Discovery searches the tree for the credentials object, ignoring `kind`

Replace `findJobSearchProperties` + `findMapWithKey` with one recursive walk:

- a map is a candidate when its `properties` value is a map containing
  `algolia_app_id`;
- otherwise recurse into object values in sorted key order and into array
  elements in order, so the first match is deterministic;
- no code path reads `kind` anymore.

Alternatives considered: keep the kind check but also search any container
(`kind == "container"`). Rejected — it encodes another CMS vocabulary fact that
can change again; the credentials property is the actual contract. The cost is
that a future response containing two credentials objects would pick the first in
sorted/array order; WTTJ publishes one per page.

`ErrNotCareersPage` keeps its meaning: a decodable response with no credentials
object. Malformed JSON and HTTP errors stay retryable, as today.

### Filter groups are preserved, then rendered as `facetFilters`

Model the discovered filters as ordered groups instead of a flat slice:

```go
type wttjFilterGroup struct {
    Attribute string
    Values    []string
}
```

`wttjFilters` reads the object in sorted attribute order and keeps array order
inside a group, rejecting non-string values as it does today.
`website_organization.slug` seeds the URL fallback slug; a single
`organization.slug` filter stands in for it when absent. A multi-organization
group does not: its first value is a sibling brand, so those pages fall back to
the careers path slug in `discover`.

`wttjAlgoliaParams` renders each group as one array of `attribute:value`
strings, marshals the array of groups with `encoding/json`, URL-encodes the JSON
with `url.QueryEscape`, and prepends `facetFilters=` to the paging parameters:

```
facetFilters=%5B%5B%22organization.slug%3Alocaloo%22%2C...%5D%5D&hitsPerPage=50&page=0
```

Several values in one group are OR (one JSON inner array); several attributes
would be AND (several inner arrays). This is Algolia's documented query-string
form. Alternatives considered: keep `filters=` and join with ` OR `
(`organization.slug:localoo OR organization.slug:partoo`). Rejected — it requires
quoting/escaping inside Algolia's filter grammar and is farther from what the
page's own client sends; `facetFilters` is the intended facet-scoping parameter.

The `website.reference` group is dropped when building params: the secured key
already embeds `filters=website.reference:wttj_fr`, and the adapter neither
decodes the key nor mirrors that restriction. If the key ever stops embedding it,
the gap is visible as a wider `nbHits` and is recorded in
`docs/SOURCE-DECISIONS.md`.

### Transport contract is unchanged and asserted in one test

The query still sends the discovered app id and the unmodified key in
`x-algolia-application-id` / `x-algolia-api-key`, `Referer:
https://www.welcometothejungle.com/`, the browser UA, and
`Content-Type: application/x-www-form-urlencoded` with a raw JSON body
(`SetEscapeHTML(false)`, so `&` stays literal). The test server asserts these on
every page request and asserts the exact `facetFilters` string in the body, so a
future change to params cannot silently drop the scoping.

### Job URL and company key follow the hit's organization slug

`mapWTTJHit` keeps `firstNonEmpty(hit.Organization.Slug, search.OrgSlug)` and
uses it both for `wttjOfferURL` and for `companyKey(model.SourceWTTJ, org)`.
This is the existing behavior, now covered by the recorded Partoo hit (whose
`organization.slug` is `partoo`) mapped with a sibling fallback (`localoo`),
and by a table case with a `partoo-italia` fallback: the hit's own slug always
wins. A hit without its own slug still resolves through the discovered slug;
when neither exists the hit is dropped, as today.

### Fixtures are trimmed bootstrap and hit copies from two companies

- `wttj_page_partoo.json`: the Partoo bootstrap reduced to the nested block
  shape — outer `jobs-search` block with layout-only `properties`
  (`{"columns": 3}`), credentials in a nested `contents[]` block, filters
  `{"organization.slug":["localoo","partoo-italia","partoo"]}`. The key keeps its
  real trailing `...%3Aweb...` form so the "never repeat the embedded filter"
  test is meaningful.
- `wttj_hits_partoo.json`: one Partoo hit (recorded 2026-11-21). All 15
  live hits were under `organization.slug: partoo`; the fixture keeps that
  real value and is trimmed of `_highlightResult`, `_geoloc`, organization
  images/descriptions/sectors and the empty `profile`.

`testdata/README.md` records both, with recording date and trimming.

## Risks / Trade-offs

- **Algolia's acceptance of URL-encoded JSON in `params` is verified only by
  construction and fixtures** → the live verification task scans Partoo and
  checks `nbHits` in the scan result; a mismatch shows up immediately as zero or
  ~60k hits rather than as a silent partial list.
- **Dropping `website.reference` depends on the key continuing to embed it** →
  document the assumption as a known gap; the adapter still never decodes keys,
  so a key change cannot make it send a contradictory filter.
- **Picking the first credentials object** → WTTJ publishes one per page; if a
  page ever publishes two, discovery is deterministic and the scan either finds
  the page's real index or fails visibly.
- **Multi-organization pages return more hits** → paging and the 100-page guard
  are unchanged; a page exceeding that limit fails the scan instead of
  truncating, as today.
- **The Partoo fixture is hand-trimmed** → it preserves the structural shape and
  real filter values; the mapper test uses the hit fields the adapter reads.

## Migration Plan

No schema, data or API change. Before any other work, check that no `jobwatch`
process holds the database and back it up (the verification scans write). Then
rebuild the binary and:

1. re-scan the existing Fabriq source and confirm the same 7 offers, 0 new;
2. look for an existing Partoo `wttj` source and re-scan it instead of adding a
   duplicate row, asking the user before changing its URL or enabled state;
   otherwise add the careers page and scan it. Confirm the expected ~15 offers
   and that each URL names the hit's organization slug.

Rollback is restoring the previous binary; no data migration to undo.
