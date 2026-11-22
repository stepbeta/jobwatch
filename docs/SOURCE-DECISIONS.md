# Source decisions (Milestones 5b–5c, 10a)

Every URL in `docs/REMOTE-SOURCES.txt` that `sources.Detect` could not
classify was investigated with exactly one polite fetch through the
shared client (robots.txt, rate limit, no retries, no anti-bot
bypassing). Each row records the single action taken. Milestone 5c
revisited the Aggregators section; see [Aggregators](#aggregators-milestone-5c)
below. Milestone 10a's Welcome to the Jungle company-page decision is at
the end of the file.

The decided rows are now shipped by data-only migration
`0003_seed_remote_sources.sql`, alongside the 8 boards registered in 5a:
a fresh database contains all 65 sources with the same type, enabled
flag and config (or disabled with the same `note`). This document remains
the record of why each URL is tracked or not.

Actions:

- **underlying-board** — a direct Greenhouse/Lever/Ashby board was found
  and registered; the board was scanned and offers arrived.
- **rss** — the site publishes an RSS/Atom feed; the feed was registered
  with the mapping it needs.
- **html** — the listing page is server-rendered and CSS selectors
  isolate item/title/link; the source was registered and one real scan
  returned offers.
- **not-trackable** — registered disabled with an explanatory `note`;
  these offers enter through manual import only.

| URL | Company | Action | Reason |
|---|---|---|---|
| https://nearform.com/careers/#openpositions | Nearform | underlying-board | Job links point at Greenhouse; registered `https://job-boards.greenhouse.io/nearform` (25 offers). |
| https://buffer.com/journey#open-roles | Buffer | html | Server-rendered `/journey/<uuid>` links; `li:haschild(a[href^="/journey/"])` (2 offers). |
| https://about.gitlab.com/jobs/all-jobs/ | GitLab | underlying-board | Greenhouse board `gitlab`; registered `https://job-boards.greenhouse.io/gitlab` (249 offers). |
| https://wikimediafoundation.org/jobs/ | Wikimedia Foundation | underlying-board | Job links are Greenhouse `grnh.se` short links; board `wikimedia`; registered `https://job-boards.greenhouse.io/wikimedia` (15 offers). |
| https://zapier.com/jobs#openings | Zapier | underlying-board | Ashby board `zapier`; registered `https://jobs.ashbyhq.com/zapier` (10 offers). |
| https://www.percona.com/careers/ | Percona | underlying-board | Ashby board `percona`; registered `https://jobs.ashbyhq.com/percona` (13 offers). |
| https://careers.achievetestprep.com/jobs/Careers | ACHIEVE TEST PREP | not-trackable | Zoho Recruit career site; the listing is rendered client-side and exposes no public feed or stable selectors. |
| https://jobs.elastic.co/jobs/department/engineering | Elastic | not-trackable | JavaScript SPA; the fetched page is an empty shell with no job links. |
| https://automattic.com/work-with-us/jobs/ | Automattic | underlying-board | Greenhouse board `automatticcareers`; registered `https://job-boards.greenhouse.io/automatticcareers` (8 offers). |
| https://careers.ghost.org/?selectedType=location& | Ghost | rss | Homerun Atom feed `https://feed.homerun.co/ghost`; default mapping (2 offers). |
| https://www.todoist.com/it/careers#open-roles | Doist | not-trackable | Careers page loads openings client-side; no job links or feed in the server-rendered HTML. |
| https://toggl.com/jobs#positions | Toggl | not-trackable | Embeds an Ashby board (`jobs.ashbyhq.com/toggl`), but the Ashby public posting API returns 404 for that org; only the embed script is available. |
| https://www.getharvest.com/careers | Harvest | not-trackable | Server-rendered page currently lists no openings; selectors cannot be verified until roles appear. |
| https://www.atlassian.com/company/careers/all-jobs?location=France | Atlassian | not-trackable | Runs on iCIMS; no iCIMS adapter and the listing is not server-rendered. |
| https://www.algolia.com/careers | Algolia | underlying-board | Greenhouse board `algolia`; registered `https://job-boards.greenhouse.io/algolia` (30 offers). |
| https://www.tigerdata.com/careers | Tiger Data | html | Server-rendered `/careers/<uuid>` links; `div[class*="hidden md:flex"] div:haschild(a[href^="/careers/"])` (4 offers). |
| https://camunda.com/careers/all-jobs/ | Camunda | underlying-board | Ashby board `camunda`; registered `https://jobs.ashbyhq.com/camunda` (52 offers). |
| https://www.gitbook.com/careers | GitBook | underlying-board | Ashby board `GitBook`; registered `https://jobs.ashbyhq.com/GitBook` (3 offers). |
| https://jobs.blablacar.com/vacancies#jobs | BlaBlaCar | underlying-board | Lever site `blablacar`; registered `https://jobs.lever.co/blablacar` (6 offers). |
| https://prezi.com/cms/careers-prezi/#openings | Prezi | not-trackable | robots.txt disallows the careers path. |
| https://tailwarden.notion.site/Work-at-Tailwarden-269b2aca1c844abc822d8b0f73618a6e | Tailwarden | not-trackable | Notion page; no structured listing or feed. |
| https://linear.app/careers#join-us | Linear | underlying-board | Ashby board `linear`; registered `https://jobs.ashbyhq.com/linear` (33 offers). |
| https://eslfaceitgroup.com/career/#roles | ESL FACEIT Group | underlying-board | Greenhouse board `eslfaceitgrouplimited`; registered `https://job-boards.greenhouse.io/eslfaceitgrouplimited` (24 offers). |
| https://scalingo.com/jobs | Scalingo | not-trackable | Framer page with no server-rendered job links; openings are loaded client-side. |
| https://careers.epicompany.eu/jobs | Epi Company | rss | Teamtailor feed `https://careers.epicompany.eu/jobs.rss`; `location_field: locations/location/name` (6 offers). |
| https://www.docker.com/career-openings/ | Docker | underlying-board | Ashby board `docker`; registered `https://jobs.ashbyhq.com/docker` (54 offers). |
| https://careers.datadoghq.com/engineering/ | Datadog | not-trackable | Runs on Workday; the jobs API is a POST/JSON surface. Manual import only. |
| https://www.shopify.com/careers#find-your-quest | Shopify | html | Server-rendered `/careers/<slug>_<uuid>` links; `li:haschild(a[data-component-name="job-row"])` (25 offers). |
| https://careers.cisco.com/global/en/search-results | Cisco | not-trackable | Runs on Workday; the jobs API is a POST/JSON surface. Manual import only. |
| https://recrutement.tiime.fr/jobs?query=&remote_status_id=fully&employment_type=contract | Tiime | rss | Teamtailor feed `https://recrutement.tiime.fr/jobs.rss`; `location_field: locations/location/name` (22 offers). |
| https://elevenlabs.io/careers/positions?department=Engineering+%26+Product&location=France | ElevenLabs | html | Server-rendered `/careers/<uuid>/<slug>` links; `li.tw-group:has(article h4 a[href^="/careers/"])` (118 offers). |
| https://murena.com/jobs/ | Murena | html | Server-rendered `table#jobs_table`; `tr:has(td:nth-child(3))` (5 engineering offers; the separate "Other positions" table is not tracked). |
| https://circleci.com/careers/jobs/ | CircleCI | underlying-board | Greenhouse board `circleci`; registered `https://job-boards.greenhouse.io/circleci` (15 offers). |
| https://canonical.com/careers/all?filter=Engineering&location=emea | Canonical | underlying-board | Greenhouse board `canonical`; registered `https://job-boards.greenhouse.io/canonical` (316 offers). |
| https://www.pulumi.com/careers/ | Pulumi | underlying-board | Greenhouse board `pulumicorporation`; registered `https://job-boards.greenhouse.io/pulumicorporation` (4 offers). |
| https://jobs.sap.com/en/jobs/?pattern=Remote&country=France | SAP | not-trackable | Runs on SuccessFactors; search needs a session/CSRF token. Manual import only. |
| https://jobs.nvidia.com/careers?start=0&pid=893394115789&sort_by=timestamp&filter_work_location_option=remote | NVIDIA | not-trackable | Runs on Workday; the jobs API is a POST/JSON surface. Manual import only. |
| https://hubstaff.com/jobs#hsds-open-positions | Hubstaff | not-trackable | Job list is rendered client-side; no job links or feed in the server-rendered HTML. |
| https://www.knack.com/careers/#careers-list | Knack | underlying-board | Greenhouse board `knack`; registered `https://job-boards.greenhouse.io/knack` (1 offer). |
| https://www.ynab.com/careers#openings | YNAB | not-trackable | Uses Pinpoint (`ynab.pinpointhq.com`); no Pinpoint adapter and the listing is client-side. |
| https://www.toptal.com/careers#positions | Toptal | not-trackable | Talent network; the page has no public job listing, only candidate signup. |
| https://kestra.io/careers | Kestra | not-trackable | Careers page loads openings client-side; no job links or feed in the server-rendered HTML. |
| https://www.exoscale.com/jobs/ | Exoscale | not-trackable | Job list is rendered client-side; no job links or feed in the server-rendered HTML. |
| https://www.deel.com/careers/?location=france | Deel | not-trackable | Careers page loads openings client-side; no job links or feed in the server-rendered HTML. |
| https://www.welcometothejungle.com/en/companies-v1/joko/jobs | Welcome to the Jungle (Joko) | not-trackable | WTTJ uses an internal API with no public contract. Manual import only. |
| https://careers.eutelsat.com/search/ | Eutelsat | not-trackable | Runs on SuccessFactors; search needs a session/CSRF token. Manual import only. |
| https://www.ashbyhq.com/careers?ashby_location_id=cfda037a-823f-4ee2-b732-40a6b55a6b17 | Ashby | underlying-board | Ashby board `ashby`; registered `https://jobs.ashbyhq.com/ashby` (79 offers). |
| https://careers.sekoia.com/jobs?split_view=true&query=&remote_status_id=fully | Sekoia | rss | Teamtailor feed `https://careers.sekoia.com/jobs.rss`; `location_field: locations/location/name` (1 offer). |
| https://supabase.com/careers#positions | Supabase | underlying-board | Ashby board `supabase`; registered `https://jobs.ashbyhq.com/supabase` (77 offers). |
| https://www.flexjobs.com/search | FlexJobs | not-trackable | Requires a paid membership/login and its robots.txt was unreachable; not crawled. Offers enter through `jobwatch import` or the UI import form. |
| https://jobs.workable.com/search?location=France&workplace=remote | Workable (search) | not-trackable | The query URL is disallowed by robots.txt (`Disallow: /search*?*`). Allowed clean category pages exist but are incomplete; no targeted adapter was added. Offers enter through manual import. See [Aggregators](#aggregators-milestone-5c). |
| https://weworkremotely.com/remote-jobs/search?country%5B%5D=FR | We Work Remotely | rss | Public RSS feed `https://weworkremotely.com/remote-jobs.rss`; registered config `{"location_field":"region","employer":{"separator":":"}}` (99 offers at 5c). Re-verified in 5c with a fixture test; the employer rule was added by OpenSpec change `offer-identity-from-source-uid`. |
| https://jobgether.com | Jobgether | not-trackable | Aggregator that requires an account and renders matches client-side. Not scraped; offers enter through `jobwatch import` or the UI import form. |
| https://www.welcometothejungle.com/fr/jobs-matches | Welcome to the Jungle (matches) | not-trackable | WTTJ uses an internal API with no public contract. Not scraped; manual import only. |
| https://my.greenhouse.io/dashboard | Greenhouse (candidate dashboard) | not-trackable | Candidate dashboard; requires a candidate login. Not scraped; offers enter through manual import. |
| https://app.theproductcrew.io/ | The Product Crew | not-trackable | Candidate-only platform; requires login. Not scraped; offers enter through manual import. |
| https://remotemore.com/candidates/ | RemoteMore | not-trackable | Candidate signup platform; no public job listing. Not scraped; offers enter through manual import. |

## Totals

| Action | URLs |
|---|---|
| underlying-board | 19 |
| rss | 5 |
| html | 5 |
| not-trackable | 28 |
| **Total** | **57** |

The 8 URLs already registered by Milestone 5a are not repeated here.

## Aggregators (Milestone 5c)

### We Work Remotely

The 5b row is correct and stays enabled. The public feed
`https://weworkremotely.com/remote-jobs.rss` is RSS 2.0 with these
per-item fields: `title` (`Company: Role`), `region` (e.g. `USA Only`),
`category`, `type`, `description` (escaped HTML), `pubDate`, `guid` and
`link`. The feed has no company element, so the config maps the location
and declares the employer rule:

```json
{"location_field": "region", "employer": {"separator": ":"}}
```

`internal/sources/testdata/rss_feed.xml` records the first three items;
`TestRSSAdapter/weworkremotely employer rule` asserts that the rule splits
the titles and sets the employer. `internal/sources/testdata/wwr_feed.xml`
is a 169-employer fixture (one item per employer, recorded from the local
database) and `TestWWRFeedEmployerRule` scans it twice: 169 distinct
employers, stable uids. A real scan in 5c returned `ok`: 100 feed items,
99 offers, 0 new (all already tracked), 0 changed, 0 disappeared.

Offers recorded before the employer rule keep the feed as their company
(a later sighting resolves by uid to the existing offer and does not
rewrite its company); new offers and a future explicit reattribution use
the extracted employer.

### Workable public search

`https://jobs.workable.com/search?location=France&workplace=remote`
cannot be an `html` source, and a targeted adapter was not judged
worthwhile:

- robots.txt allows `/search/*` but disallows `/search*?*` and
  `/search`; the requested France+remote URL is a query URL.
- The allowed clean pages (for example
  `/search/france/remote-software-engineer-jobs`, 36 France URLs in the
  sitemap) are server-rendered and expose job links plus a JSON-LD
  `ItemList`, but each is a single category capped at 15 results, with
  heavy overlap and no Platform/SRE category. Registering several of
  them would still not give a complete listing.
- The complete search is served by an internal API with no public
  contract, which is the same class of risk as WTTJ.
- The per-company Workable widget API is already supported. When a
  company is found through the search, the right stable source is its
  `apply.workable.com/<account>` board.

The disabled row stays, with the note "robots.txt disallows the Workable
job search path." No request was made to the disallowed search URL.

### Candidate-account and JS-only aggregators

These pages are not scraped at all. They either require a candidate
account or render their matches client-side, and JobWatch has no adapter
that could read them reliably:

| Page | Why it is not scraped |
|---|---|
| flexjobs.com/search | Paid membership/candidate login; robots.txt unreachable |
| remotemore.com/candidates/ | Candidate signup platform; no public listing |
| my.greenhouse.io/dashboard | Candidate dashboard behind a login |
| app.theproductcrew.io | Candidate-only platform behind a login |
| welcometothejungle.com/fr/jobs-matches | Internal API, no public contract; JS marketplace |
| jobgether.com | Account required; matches rendered client-side |

Offers found on these pages enter JobWatch manually:

```sh
# Paste the public offer URL; metadata is extracted when possible.
./jobwatch import https://www.example.com/jobs/senior-sre

# If the page is login-gated or has no metadata, supply the fields.
./jobwatch import https://www.example.com/jobs/senior-sre \
  --company "Example" --title "Senior SRE" --location "Remote (France)"
```

In the UI, the same workflow is **Offers → Import an offer seen
elsewhere**: paste the URL and click **Fetch & import**; the form
pre-fills missing company/title fields for manual entry. A manual import
is matched to an existing offer when company and title agree, and its
provenance row has `source_id = NULL`. These aggregator rows remain
disabled and are never scanned.

## Welcome to the Jungle (Milestone 10a)

Company careers pages on WTTJ (`/<lang>/companies/<slug>/jobs`) became a
`wttj` source in Milestone 10a, so the 5b decision "not-trackable" is
superseded for company pages only. The two 5b URLs stay disabled and are
never scanned: `.../en/companies-v1/joko/jobs` (a legacy `companies-v1`
path that is not a `/companies/` page) and `.../fr/jobs-matches` (the
marketplace, not a company page). The seeded Fabriq page is the live
reference source, and the Partoo page is the multi-organization reference
(three `organization.slug` values in one filter group):

```
https://www.welcometothejungle.com/en/companies/fabriq/jobs
https://www.welcometothejungle.com/fr/companies/partoo/jobs
```

### The discovery and query flow

The careers page HTML is a shell; the offer list is an Algolia index. The
index credentials are not hardcoded in JobWatch: they are published per
company page by the site's own bootstrap endpoint.

```mermaid
sequenceDiagram
    participant Adapter as wttj adapter
    participant API as api.welcometothejungle.com
    participant Algolia as "{app_id}-dsn.algolia.net"
    Adapter->>API: GET /api/v1/pages?path=/en/companies/fabriq/jobs
    API-->>Adapter: 200 JSON page tree
    Note over Adapter: walk the page tree for the properties<br/>object with algolia_app_id; read the key,<br/>index, filters and language
    loop page 0, 1, ... up to nbPages
        Adapter->>Algolia: POST /1/indexes/*/queries<br/>x-algolia-application-id, x-algolia-api-key,<br/>Referer, form content type, JSON body
        Algolia-->>Adapter: results[0].hits[], nbPages
    end
    Note over Adapter: if page 0 is empty but items_count > 0,<br/>resolve the canonical organization slug<br/>and retry page 0 once
    Note over Adapter: hits become RawOffer
    loop each hit whose profile and summary are blank
        Adapter->>Site: GET /{lang}/companies/{org}/jobs/{slug}
        Site-->>Adapter: 200 HTML with JSON-LD JobPosting
    end
    Note over Adapter: description = JSON-LD description as plain text;<br/>a failed page leaves it empty and does not fail the source
```

1. **Discovery** — `GET https://api.welcometothejungle.com/api/v1/pages?path=<careers path>`
   with a normal browser User-Agent. The settings are published in a
   `properties` object somewhere in the page tree. On live company pages
   the `jobs-search` block is a layout container (its own `properties`
   hold keys such as `columns`) and the credentials sit in a nested
   `contents[*]` block, so the adapter walks the whole tree — objects and
   arrays, including nested `contents[]` — and picks the first
   `properties` object carrying `algolia_app_id`. Block `kind` values are
   ignored. A response with no such object is a permanent error
   (`URL is not a WTTJ careers page`). HTTP or network failures and
   undecodable JSON are retryable errors.
2. **Algolia multi-query** —
   `POST https://{app_id}-dsn.algolia.net/1/indexes/*/queries`.
   The discovered filters are sent as `facetFilters`: one JSON array per
   attribute, so several organization slugs are OR-ed, URL-encoded inside
   `params`.
3. **Stale slug resolution** — after page 0, the adapter compares `nbHits`
   with `path[0].items_count`. When they differ and `items_count` is above
   zero, it reads the canonical slug of every distinct
   `organization.slug` value from
   `GET https://api.welcometothejungle.com/api/v1/organizations/{slug}`,
   deduplicates them, and retries page 0 once if any value changed. A
   lookup failure is a retryable error; a page with `items_count` 0 never
   triggers the lookup, and a mismatch without a rename returns the
   received hits.
4. **Description fallback** — some organizations publish neither `profile`
   nor `summary` in the index (observed on Joko and Partoo). For each such
   hit the adapter fetches the offer's public page through the shared
   client (robots.txt and the per-source interval apply), reads the
   JSON-LD `JobPosting.description`, and converts it with
   `sources.HTMLToText`. The page is fetched only while the index text
   stays blank and the offer has no stored description; once text is
   stored, later scans skip that offer, so the WAF allowance goes to the
   offers that still need it. A failed request, a non-2xx response,
   malformed HTML or a missing `JobPosting` leaves that offer's
   description empty and keeps the source `ok`: the listing itself
   succeeded.

   WTTJ's CloudFront WAF answers the identifying JobWatch UA on HTML
   pages with HTTP 202 and an empty body, so detail requests use the same
   single browser User-Agent the adapter already sends to the site's API
   endpoints. There is no per-request UA rotation. A source with many
   description-less hits needs a slower `rate_limit_ms` to stay under the
   WAF's burst threshold; Partoo uses 90 s.

Request quirks, all verified against the live site (2026-11) and
deliberately replicated:

| Quirk | Why it is reproduced as is |
|---|---|
| The `Referer: https://www.welcometothejungle.com/` header is mandatory | the secured key is referrer-restricted; without it Algolia answers 403 |
| `Content-Type: application/x-www-form-urlencoded` with a JSON body | this is what the site's Algolia JS client 4.x sends; changing it to `application/json` is unnecessary and untested against the site |
| `params` carries `facetFilters=` | Algolia parses `params` as a query string; the discovered filter groups are serialized as URL-encoded JSON, e.g. `facetFilters=%5B%5B%22organization.slug%3Afabriq%22%5D%5D&hitsPerPage=50&page=N`. Space-joining `attribute:value` pairs would AND a multi-value group and select nothing |
| The page's filters are sent explicitly | the secured key only embeds `website.reference:wttj_fr`; without `facetFilters` the query returns the whole site (~60k hits instead of the page's own offers). The adapter never decodes the key and does not repeat the embedded filter |
| A browser User-Agent for these API calls and for offer pages | the bootstrap endpoint, Algolia and HTML offer pages reject or challenge the identifying JobWatch UA; every other adapter keeps it. One fixed browser UA is used, never per-request rotation |
| Pagination is `hitsPerPage=50`, `page=0..nbPages-1` | same page size as the site; the loop stops at `nbPages` or the first empty page |
| An empty first page can trigger one organization lookup | the CMS filters can lag behind a rename (Swile: filters `swile`, index `lunchr`); page 0's `nbHits` is compared with `items_count`, every `organization.slug` value is canonicalized only on a mismatch, and a lookup failure is retryable |

Hit mapping (both index shapes observed in the wild are tolerated):

| RawOffer field | Source |
|---|---|
| ExternalUID | `hit.reference`; fallback `hit.objectID` as a string (numeric on some indexes) |
| Title | `hit.name` |
| Company | `hit.organization.name` |
| Location | `offices[]` joined as `City, Country` (`; ` between offices); fallback `office.*` |
| Description | `hit.profile` (markdown) verbatim; fallback `hit.summary`; when both are blank, the JSON-LD `JobPosting.description` of the public offer page, converted from HTML to plain text |
| URL | `https://www.welcometothejungle.com/{language}/companies/{organization.slug}/jobs/{hit.slug}` with the discovery `language`. The organization slug is the hit's own, because a page may list jobs of linked organizations; the discovered page slug is used only when the hit omits it. The hit language is only used when discovery has none |
| PublishedAt | `hit.published_at` (RFC3339 with `Z` or a numeric offset); parsed and validated but not stored — `job_offers` has no publication-date column and 10a adds none |

`salary_yearly_minimum/maximum`, `salary_currency`, `remote` and
`contract_type` are explicitly out of scope: there is no offer column for
them, so they are ignored rather than mapped into an unrelated field.

### Why the credentials are discovered at runtime

WTTJ can rotate the secured Algolia key (and change the index behind a
careers page) at any time without notice. Copying the key into a config
file would break every source the next time they do, with a 403 that
looks like a bug in JobWatch. Reading the bootstrap on every scan makes
rotation transparent and keeps the only user setting a URL.

The escape hatch — `config_json` keys `algolia_app_id` and
`algolia_api_key` — overrides the discovered values. It exists for
debugging a failure (reproducing a request outside JobWatch, or isolating
a discovery problem from an Algolia problem) and is not a supported way
to run a source. Discovery still runs when the overrides are present,
because the index name, filters and language are needed anyway.

The discovery response is held in memory only, for the duration of one
scan; nothing is persisted and the next scan discovers again (one extra
GET per scan per source).

### Known gaps

1. **The bootstrap response shape is undocumented.** The adapter depends
   on finding a `properties` object with the `algolia_*` keys somewhere in
   the page tree. Nested layouts are handled (see gap 4), but if WTTJ
   renames those properties or publishes them outside a `properties`
   object, discovery breaks; the scan then reports the permanent error
   `URL is not a WTTJ careers page` instead of silently returning zero
   offers.
2. **The secured key is referrer-restricted.** The adapter must keep
   sending the fixed `Referer`. If WTTJ tightens the restriction to an
   `Origin`/session pair, the Algolia call returns 403 and the scan marks
   the source `blocked`.
3. **Two index shapes have been observed.** `reference`/`objectID` may be
   a business id (`FABRI_...`) or a CMS row id (a UUID or a number),
   organizations may or may not carry a slug, and locations may come as
   `offices[]` or `office`. The mapper tolerates both, but a third shape
   (for example a renamed `profile` field) would need a code change.
4. **`jobs-search` blocks are nested.** The outer block can be a layout
   container whose `properties` hold only keys such as `columns`, with the
   credentials in `contents[*].properties`; the adapter walks the tree by
   property rather than by block kind, so depth is not a problem. It does
   assume one credentials object per page — the first one found in
   deterministic order wins.
5. **Discovery filters must be sent as `facetFilters`.** The query is
   scoped only by copying the page's own `filters` object. The secured key
   covers `website.reference:wttj_fr` and nothing else, so if a page stops
   publishing its filters the adapter sends paging params only, Algolia
   answers with the whole site (~60k hits), and the 100-page guard fails
   the scan instead of storing a truncated list. The adapter never decodes
   the key to recover the scope.
6. **A count mismatch without a rename is not corrected.** If `nbHits`
   differs from `items_count` because the index lags, the lookups return
   the same slugs, no retry occurs, and the partial list is returned. The
   adapter does not fail the scan on a mismatch.
7. **The organization endpoint is a third upstream surface.** It is
   called once per distinct slug only on a count mismatch; if WTTJ changes
   its response shape, the retry fails as a retryable error rather than
   silently reporting a partial list.
8. **The description fallback depends on the page's JSON-LD.** When the
   index text is blank, the adapter reads `JobPosting.description` from the
   offer page. If WTTJ drops that block (or renders the page client-side),
   descriptions stay empty while the scan still succeeds. A systemic
   failure of detail pages is therefore silent: the source is `ok`, but
   its offers have no text. The smoke test checks Partoo's descriptions
   after a scan; a dedicated warning field would be needed to surface this
   in the UI. The same applies to a WAF challenge: the adapter sends one
   fixed browser UA and a conservative interval, but a burst can still be
   challenged, leaving that offer's text empty.

A further gap: offers are identified by `(source, external_uid)`, where
the uid is `reference` (or the `objectID` fallback). The two observed
shapes use different values for the same job on different indexes, so
switching a source to a different index would change external UIDs and
create new offers; the previous ones would disappear on that scan.

### Live verification (2026-11-18)

- Discovery returned the Fabriq `jobs-search` block: app id
  `CSEKHVMS53`, index `wk_cms_jobs_production`, language `en`, filters
  `{"organization.slug":["fabriq"]}`.
- Algolia reported `nbHits: 7` for the 10a `filters=organization.slug:fabriq` parameter.
- The first scan of source 76 (`Fabriq`) fetched 7 offers and created 7:
  `ok`, 7 new, 0 changed, 0 disappeared, `raw_text` filled from
  `profile`, location `Paris, France` on all seven.
- `https://www.welcometothejungle.com/en/companies/fabriq/jobs/senior-full-stack-engineer-front-end-focus_paris`
  resolves with HTTP 200 in a browser.
- A second scan fetched the same 7 offers: 0 new, 7 seen, 0 changed, 0
  disappeared, still 7 provenance rows and 7 snapshots — no duplicates
  and no false "changed".

### Live verification (2026-11-21)

- Discovery on the Partoo page (`/fr/companies/partoo/jobs`) found the
  credentials in `contents[].properties` under an outer `jobs-search`
  block whose own `properties` held only `{"columns": 3}`. It returned
  app id `CSEKHVMS53`, index `wk_cms_jobs_production`, language `en` and
  filters `{"organization.slug":["localoo","partoo-italia","partoo"]}`.
- The same page queried with the new params returned `nbHits: 15` and
  `nbPages: 1`, matching its `items_count` of 15; an unscoped query is
  the whole site (~60k hits).
- Re-scanning source 76 (Fabriq) fetched 7 offers: 0 new, 7 seen,
  status `ok`, so the 10a page is unaffected.
- Re-scanning the existing Partoo source (id 82, `/fr/...`, previously
  `error`) moved it to `ok`: 15 fetched, 15 new, 0 incomplete. All 15
  offers belong to company Partoo and use
  `/en/companies/partoo/jobs/...` URLs (the discovered page language);
  one spot-checked URL answered HTTP 200. No duplicate source was
  created.
- Swile (`/swile/jobs`, source 85): discovery returned `items_count` 15
  and filters `{"organization.slug":["swile"]}`, but the index still
  stores the organization under `lunchr`, so the first query returned
  `nbHits: 0`. `GET /api/v1/organizations/swile` reported the canonical
  slug `lunchr`; the retry returned `nbHits: 15`.
- Re-scanning source 85 after the retry: 15 fetched, 15 new, 0
  incomplete, status `ok`. All 15 offers belong to company Swile and use
  `/en/companies/lunchr/jobs/...` URLs; a spot-checked URL answered
  HTTP 200.
- Re-scanning Fabriq (7 fetched, 0 new) and Partoo (15 fetched, 0 new)
  after the change left both sources unchanged (status `ok`).
- After broadening the retry trigger from an empty page to
  `nbHits != items_count` and canonicalizing every slug, the scans were
  repeated: Swile 15 fetched (0 new, 15 seen), Fabriq 7 (0 new) and
  Partoo 15 (0 new), all `ok`.

### Joko moved from WTTJ to Ashby (2026-11-22)

Joko published the same 46 roles on both surfaces, but the WTTJ index
carried neither `profile` nor `summary` for any of them while the Ashby
board includes the full descriptions. The WTTJ source (id 87) and its 46
description-less offers were therefore removed after a database backup,
and `https://jobs.ashbyhq.com/joko` was added as an `ashby` source (named
`Joko`, `{"company":"Joko"}`).

To avoid a duplicate company, `ashby:joko` was inserted into
`company_keys` for the existing company `Joko` (id 115) before the first
scan; the scan reused company 115 instead of creating a second `Joko`,
and added the source-scoped `joko` alias. The one bookmark on old offer
2869 was restored on the Ashby homologue
`b83d7f0f-a0d0-4dc6-b305-7d9336e4d389` (`saved_at =
2026-11-22T17:09:35Z`).

This was a one-off, user-confirmed data operation; it does not change the
rule that scans never delete offers. The 46 old offers had no events or
notes, and their snapshots were empty. The WTTJ adapter still serves
companies whose index does carry text (Fabriq, Partoo, Swile); Joko is
simply no longer one of them.
