# Source decisions (Milestones 5b–5c)

Every URL in `docs/REMOTE-SOURCES.txt` that `sources.Detect` could not
classify was investigated with exactly one polite fetch through the
shared client (robots.txt, rate limit, no retries, no anti-bot
bypassing). Each row records the single action taken. Milestone 5c
revisited the Aggregators section; see [Aggregators](#aggregators-milestone-5c)
below.

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
| https://weworkremotely.com/remote-jobs/search?country%5B%5D=FR | We Work Remotely | rss | Public RSS feed `https://weworkremotely.com/remote-jobs.rss`; registered config `{"location_field":"region"}` (99 offers). Re-verified in 5c with a fixture test. |
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
`link`. The feed has no company element, so the registered config maps
only the location:

```json
{"location_field": "region"}
```

`internal/sources/testdata/rss_feed.xml` records that shape (first three
items, descriptions trimmed) and `TestRSSAdapter/weworkremotely_registered_mapping`
asserts the mapping, the verbatim `Company: Role` titles and the empty
company. A real scan in 5c returned `ok`: 100 feed items, 99 offers, 0
new (all already tracked), 0 changed, 0 disappeared.

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
