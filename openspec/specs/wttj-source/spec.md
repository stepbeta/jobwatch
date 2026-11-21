# wttj-source Specification

## Purpose

Defines how the Welcome to the Jungle adapter turns a company careers page into offers: it discovers the page's Algolia credentials, scopes the index query to the page's own filters, and addresses every job through the organization that actually owns it.

## Requirements

### Requirement: Credentials are located by property, not by block kind

The adapter SHALL find the Algolia settings by recursively traversing the bootstrap page tree, including nested `contents[]` arrays, and selecting the object whose `properties` contain `algolia_app_id`. The enclosing block's `kind` MUST NOT decide whether a subtree is searched, because a `jobs-search` block may be a pure layout container.

#### Scenario: Credentials sit under a layout-only block

- **WHEN** the bootstrap returns a `jobs-search` block whose own `properties` hold only layout keys and whose nested `contents[]` block holds the `algolia_*` keys
- **THEN** discovery returns the app id, key, index name, filters and language from the nested object

#### Scenario: Credentials sit directly in the block properties

- **WHEN** the bootstrap returns a block whose own `properties` carry the `algolia_*` keys
- **THEN** discovery returns those values

#### Scenario: Several blocks are present

- **WHEN** the bootstrap contains more than one block, only one of which carries `algolia_app_id`
- **THEN** discovery selects that block regardless of its position or kind

### Requirement: Discovery failures separate permanent from retryable errors

The adapter SHALL report a missing credentials object as a permanent error meaning the URL is not a WTTJ careers page. HTTP failures and undecodable JSON MUST remain retryable errors.

#### Scenario: Response has no credentials

- **WHEN** the bootstrap response decodes but contains no `properties` object with `algolia_app_id`
- **THEN** the scan records a permanent error and does not retry the source

#### Scenario: Bootstrap is unreachable or malformed

- **WHEN** the bootstrap answers with a server error or a body that is not valid JSON
- **THEN** the scan records a retryable error

### Requirement: Discovery supplies the query scope and page language

The adapter SHALL read the discovered `filters` and `language` verbatim, preserving every value of a multi-valued filter group. When the bootstrap omits them, the adapter SHALL fall back to the careers path for the language and organization slug.

#### Scenario: Filters contain several organization slugs

- **WHEN** discovery reports `{"organization.slug": ["localoo", "partoo-italia", "partoo"]}`
- **THEN** the adapter keeps all three values in one attribute group

#### Scenario: Language is missing from the bootstrap

- **WHEN** the credentials object has no `language` and the careers path starts with `/fr/`
- **THEN** the adapter uses `fr` for the query and for job URLs

#### Scenario: Organization slug is missing from the bootstrap

- **WHEN** neither the filters nor `website_organization` provide an organization slug
- **THEN** the adapter falls back to the slug of the careers path

### Requirement: Stale organization filters are resolved from the organization record

When the first page's `nbHits` differs from the page's `items_count` (which is above zero), the adapter SHALL resolve the canonical slug of every `organization.slug` filter value through `GET https://api.welcometothejungle.com/api/v1/organizations/{slug}`, deduplicate the results and retry page 0 once if any value changed. A page with `items_count` 0 MUST NOT trigger the lookup.

#### Scenario: Renamed organization lags in the index

- **WHEN** the filter says `organization.slug: swile`, the organization record says `lunchr`, and the page reports 15 jobs while the first query returns 0 hits
- **THEN** the adapter retries with `organization.slug: lunchr` and returns those 15 hits

#### Scenario: One brand on a multi-organization page was renamed

- **WHEN** a page filters on three slugs and reports 30 jobs, the first page returns 25 hits, and one slug resolves to a different canonical value
- **THEN** the adapter resolves all three slugs and retries with the canonical values

#### Scenario: Counts already match

- **WHEN** the first page's `nbHits` equals `items_count`
- **THEN** the adapter neither calls the organization endpoint nor retries

#### Scenario: Page reports no jobs

- **WHEN** the page's `items_count` is 0
- **THEN** the adapter neither calls the organization endpoint nor retries

#### Scenario: Counts disagree without a rename

- **WHEN** the counts differ but every slug already is canonical
- **THEN** the adapter does not retry and returns the hits it received

#### Scenario: Organization lookup fails

- **WHEN** the counts differ and the organization endpoint fails
- **THEN** the scan records a retryable error instead of silently reporting a partial list

### Requirement: The index query is scoped by facetFilters

The adapter SHALL express each discovered filter attribute as one `facetFilters` group, OR-ing the values of the same attribute, and SHALL URL-encode that JSON inside the request `params`. The filter embedded in the secured key (`website.reference`) MUST NOT be repeated in `params`, and the key MUST NOT be decoded or modified.

#### Scenario: Multi-organization page

- **WHEN** discovery reports three slugs under `organization.slug`
- **THEN** `params` carries one `facetFilters` group with three `organization.slug:<slug>` entries, so Algolia returns the jobs of all three organizations

#### Scenario: Single-organization page

- **WHEN** discovery reports one slug under `organization.slug`
- **THEN** `params` carries one `facetFilters` group with one entry

#### Scenario: No discovered filters

- **WHEN** discovery reports no filters
- **THEN** `params` still carries the paging parameters and no `facetFilters`

#### Scenario: Key-embedded filter

- **WHEN** the secured key embeds `website.reference:wttj_fr`
- **THEN** `params` does not contain a `website.reference` entry

### Requirement: The Algolia request keeps the verified transport contract

The adapter MUST send the discovered application id and the unmodified secured key in the `x-algolia-application-id` and `x-algolia-api-key` headers, the fixed `Referer: https://www.welcometothejungle.com/`, and the form-urlencoded content type with a raw JSON body on every query page.

#### Scenario: Every result page is requested the same way

- **WHEN** a scan pages through the index
- **THEN** each request carries the same credentials, `Referer`, content type and JSON body shape

### Requirement: Job URLs use the hit's organization slug

The adapter SHALL build each job URL from the hit's `organization.slug`, using the discovered page slug only when the hit does not carry one, because a page may list jobs owned by linked organizations.

#### Scenario: Hit belongs to a linked organization

- **WHEN** a source page for one slug returns a hit whose `organization.slug` is a different slug
- **THEN** the offer URL names the hit's organization slug

#### Scenario: Hit omits its organization slug

- **WHEN** a hit has no `organization.slug`
- **THEN** the adapter uses the discovered page slug

#### Scenario: No slug is available at all

- **WHEN** neither the hit nor discovery provides an organization slug
- **THEN** the adapter drops the hit instead of building an unusable URL
