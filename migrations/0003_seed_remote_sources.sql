-- 0003_seed_remote_sources: seed every source registered during Milestones 5a/5b.
--
-- Milestone 5a registered the 8 ATS boards detected in
-- docs/REMOTE-SOURCES.txt; Milestone 5b investigated the remaining 57 URLs
-- and registered 19 underlying ATS boards, 5 RSS/Atom feeds, 5 HTML
-- listings and 28 disabled not-trackable rows (docs/SOURCE-DECISIONS.md).
-- This data-only migration ships that decided state with the binary, so a
-- fresh database (or a deleted jobwatch.db) already contains all 65
-- sources without re-running add-sources or repeating the investigation.
--
-- The four disabled known-hard rows from 0002 (Datadog/Cisco Workday, SAP
-- SuccessFactors, WTTJ) are intentionally not repeated. Every INSERT is a
-- no-op when a source with the same URL already exists, and no existing
-- row is ever updated: a user-edited source is never overwritten. Applying
-- this file twice, or on a database that already has some of these
-- sources, therefore cannot create duplicates.
--
-- name, url, source_type, enabled and config_json (board token/site/org/
-- tenant, RSS/HTML field mappings and selectors, explanatory notes) are
-- reproduced exactly as configured after 5b. Disabled rows are the 28
-- not-trackable decisions plus no others.


INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'grafanalabs',
       'https://job-boards.greenhouse.io/grafanalabs/',
       'greenhouse',
       '{"board_token":"grafanalabs"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/grafanalabs/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'malt',
       'https://jobs.lever.co/malt',
       'lever',
       '{"site":"malt"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.lever.co/malt');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'mural',
       'https://jobs.ashbyhq.com/mural?workplaceType=Remote',
       'ashby',
       '{"org":"mural"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/mural?workplaceType=Remote');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'overstory',
       'https://jobs.ashbyhq.com/overstory',
       'ashby',
       '{"org":"overstory"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/overstory');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'cycloid',
       'https://cycloid.jobs.personio.de',
       'personio',
       '{"tenant":"cycloid"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://cycloid.jobs.personio.de');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'everbridge',
       'https://jobs.lever.co/everbridge?workplaceType=remote',
       'lever',
       '{"site":"everbridge"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.lever.co/everbridge?workplaceType=remote');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT '360learning',
       'https://jobs.lever.co/360learning?commitment=Full%20Time&workplaceType=remote&location=Paris%2C%20Remote',
       'lever',
       '{"site":"360learning"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.lever.co/360learning?commitment=Full%20Time&workplaceType=remote&location=Paris%2C%20Remote');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'n8n',
       'https://jobs.ashbyhq.com/n8n',
       'ashby',
       '{"org":"n8n"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/n8n');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Nearform',
       'https://job-boards.greenhouse.io/nearform',
       'greenhouse',
       '{"board_token":"nearform"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/nearform');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'GitLab',
       'https://job-boards.greenhouse.io/gitlab',
       'greenhouse',
       '{"board_token":"gitlab"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/gitlab');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Wikimedia Foundation',
       'https://job-boards.greenhouse.io/wikimedia',
       'greenhouse',
       '{"board_token":"wikimedia"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/wikimedia');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Zapier',
       'https://jobs.ashbyhq.com/zapier',
       'ashby',
       '{"org":"zapier"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/zapier');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Percona',
       'https://jobs.ashbyhq.com/percona',
       'ashby',
       '{"org":"percona"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/percona');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Automattic',
       'https://job-boards.greenhouse.io/automatticcareers',
       'greenhouse',
       '{"board_token":"automatticcareers"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/automatticcareers');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Algolia',
       'https://job-boards.greenhouse.io/algolia',
       'greenhouse',
       '{"board_token":"algolia"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/algolia');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Camunda',
       'https://jobs.ashbyhq.com/camunda',
       'ashby',
       '{"org":"camunda"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/camunda');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'GitBook',
       'https://jobs.ashbyhq.com/GitBook',
       'ashby',
       '{"org":"GitBook"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/GitBook');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'BlaBlaCar',
       'https://jobs.lever.co/blablacar',
       'lever',
       '{"site":"blablacar"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.lever.co/blablacar');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Linear',
       'https://jobs.ashbyhq.com/linear',
       'ashby',
       '{"org":"linear"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/linear');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'ESL FACEIT Group',
       'https://job-boards.greenhouse.io/eslfaceitgrouplimited',
       'greenhouse',
       '{"board_token":"eslfaceitgrouplimited"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/eslfaceitgrouplimited');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Docker',
       'https://jobs.ashbyhq.com/docker',
       'ashby',
       '{"org":"docker"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/docker');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'CircleCI',
       'https://job-boards.greenhouse.io/circleci',
       'greenhouse',
       '{"board_token":"circleci"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/circleci');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Canonical',
       'https://job-boards.greenhouse.io/canonical',
       'greenhouse',
       '{"board_token":"canonical"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/canonical');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Pulumi',
       'https://job-boards.greenhouse.io/pulumicorporation',
       'greenhouse',
       '{"board_token":"pulumicorporation"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/pulumicorporation');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Knack',
       'https://job-boards.greenhouse.io/knack',
       'greenhouse',
       '{"board_token":"knack"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://job-boards.greenhouse.io/knack');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Ashby',
       'https://jobs.ashbyhq.com/ashby',
       'ashby',
       '{"org":"ashby"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/ashby');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Supabase',
       'https://jobs.ashbyhq.com/supabase',
       'ashby',
       '{"org":"supabase"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.ashbyhq.com/supabase');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Ghost',
       'https://feed.homerun.co/ghost',
       'rss',
       '{}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://feed.homerun.co/ghost');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Epi Company',
       'https://careers.epicompany.eu/jobs.rss',
       'rss',
       '{"location_field":"locations/location/name"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.epicompany.eu/jobs.rss');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Tiime',
       'https://recrutement.tiime.fr/jobs.rss',
       'rss',
       '{"location_field":"locations/location/name"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://recrutement.tiime.fr/jobs.rss');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Sekoia',
       'https://careers.sekoia.com/jobs.rss',
       'rss',
       '{"location_field":"locations/location/name"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.sekoia.com/jobs.rss');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'We Work Remotely',
       'https://weworkremotely.com/remote-jobs.rss',
       'rss',
       '{"location_field":"region"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://weworkremotely.com/remote-jobs.rss');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Buffer',
       'https://buffer.com/journey#open-roles',
       'html',
       '{"item_selector":"li:haschild(a[href^=\"/journey/\"])","title_selector":"a[href^=\"/journey/\"] > span:first-child","link_selector":"a[href^=\"/journey/\"]"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://buffer.com/journey#open-roles');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Tiger Data',
       'https://www.tigerdata.com/careers',
       'html',
       '{"item_selector":"div[class*=\"hidden md:flex\"] div:haschild(a[href^=\"/careers/\"])","title_selector":"a[href^=\"/careers/\"] p","link_selector":"a[href^=\"/careers/\"]"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.tigerdata.com/careers');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Shopify',
       'https://www.shopify.com/careers#find-your-quest',
       'html',
       '{"item_selector":"li:haschild(a[data-component-name=\"job-row\"])","title_selector":"h3","link_selector":"a[data-component-name=\"job-row\"]","location_selector":"p"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.shopify.com/careers#find-your-quest');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'ElevenLabs',
       'https://elevenlabs.io/careers/positions?department=Engineering+%26+Product&location=France',
       'html',
       '{"item_selector":"li.tw-group:has(article h4 a[href^=\"/careers/\"])","title_selector":"h4 a","link_selector":"h4 a","location_selector":"dd"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://elevenlabs.io/careers/positions?department=Engineering+%26+Product&location=France');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Murena',
       'https://murena.com/jobs/',
       'html',
       '{"item_selector":"table#jobs_table tr:has(td:nth-child(3))","title_selector":"td:first-child","link_selector":"td a[href^=\"https://murena.com/jobs/\"]","location_selector":"td:nth-child(2)","description_selector":"td:nth-child(3)","uid_selector":"td a[href^=\"https://murena.com/jobs/\"]","uid_attr":"href"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://murena.com/jobs/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'ACHIEVE TEST PREP',
       'https://careers.achievetestprep.com/jobs/Careers',
       'html',
       '{"note":"Zoho Recruit career site; the listing is rendered client-side and exposes no public feed or stable selectors."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.achievetestprep.com/jobs/Careers');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Elastic',
       'https://jobs.elastic.co/jobs/department/engineering',
       'html',
       '{"note":"Elastic careers site is a JavaScript SPA; the fetched page is an empty shell with no job links."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.elastic.co/jobs/department/engineering');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Doist',
       'https://www.todoist.com/it/careers#open-roles',
       'html',
       '{"note":"Doist careers page loads openings client-side; no job links or feed in the server-rendered HTML."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.todoist.com/it/careers#open-roles');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Toggl',
       'https://toggl.com/jobs#positions',
       'html',
       '{"note":"Toggl embeds an Ashby board (jobs.ashbyhq.com/toggl), but the Ashby public posting API returns 404 for that org; only the embed script is available."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://toggl.com/jobs#positions');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Harvest',
       'https://www.getharvest.com/careers',
       'html',
       '{"note":"Server-rendered page currently lists no openings; selectors cannot be verified until roles appear."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.getharvest.com/careers');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Atlassian',
       'https://www.atlassian.com/company/careers/all-jobs?location=France',
       'html',
       '{"note":"Atlassian runs on iCIMS; no iCIMS adapter and the listing is not server-rendered."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.atlassian.com/company/careers/all-jobs?location=France');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Prezi',
       'https://prezi.com/cms/careers-prezi/#openings',
       'html',
       '{"note":"robots.txt disallows the careers path."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://prezi.com/cms/careers-prezi/#openings');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Tailwarden',
       'https://tailwarden.notion.site/Work-at-Tailwarden-269b2aca1c844abc822d8b0f73618a6e',
       'html',
       '{"note":"Notion page; no structured listing or feed."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://tailwarden.notion.site/Work-at-Tailwarden-269b2aca1c844abc822d8b0f73618a6e');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Scalingo',
       'https://scalingo.com/jobs',
       'html',
       '{"note":"Framer page with no server-rendered job links; openings are loaded client-side."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://scalingo.com/jobs');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Datadog',
       'https://careers.datadoghq.com/engineering/',
       'html',
       '{"note":"Datadog runs on Workday; the jobs API is a POST/JSON surface. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.datadoghq.com/engineering/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Cisco',
       'https://careers.cisco.com/global/en/search-results',
       'html',
       '{"note":"Cisco runs on Workday; the jobs API is a POST/JSON surface. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.cisco.com/global/en/search-results');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'SAP',
       'https://jobs.sap.com/en/jobs/?pattern=Remote&country=France',
       'html',
       '{"note":"SAP runs on SuccessFactors; search needs a session/CSRF token. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.sap.com/en/jobs/?pattern=Remote&country=France');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'NVIDIA',
       'https://jobs.nvidia.com/careers?start=0&pid=893394115789&sort_by=timestamp&filter_work_location_option=remote',
       'html',
       '{"note":"NVIDIA runs on Workday; the jobs API is a POST/JSON surface. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.nvidia.com/careers?start=0&pid=893394115789&sort_by=timestamp&filter_work_location_option=remote');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Hubstaff',
       'https://hubstaff.com/jobs#hsds-open-positions',
       'html',
       '{"note":"Hubstaff job list is rendered client-side; no job links or feed in the server-rendered HTML."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://hubstaff.com/jobs#hsds-open-positions');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'YNAB',
       'https://www.ynab.com/careers#openings',
       'html',
       '{"note":"YNAB uses Pinpoint (ynab.pinpointhq.com); no Pinpoint adapter and the listing is client-side."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.ynab.com/careers#openings');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Toptal',
       'https://www.toptal.com/careers#positions',
       'html',
       '{"note":"Toptal is a talent network; the page has no public job listing, only candidate signup."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.toptal.com/careers#positions');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Kestra',
       'https://kestra.io/careers',
       'html',
       '{"note":"Kestra careers page loads openings client-side; no job links or feed in the server-rendered HTML."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://kestra.io/careers');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Exoscale',
       'https://www.exoscale.com/jobs/',
       'html',
       '{"note":"Exoscale job list is rendered client-side; no job links or feed in the server-rendered HTML."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.exoscale.com/jobs/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Deel',
       'https://www.deel.com/careers/?location=france',
       'html',
       '{"note":"Deel careers page loads openings client-side; no job links or feed in the server-rendered HTML."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.deel.com/careers/?location=france');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Welcome to the Jungle (Joko)',
       'https://www.welcometothejungle.com/en/companies-v1/joko/jobs',
       'wttj',
       '{"note":"WTTJ uses an internal API with no public contract. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.welcometothejungle.com/en/companies-v1/joko/jobs');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Eutelsat',
       'https://careers.eutelsat.com/search/',
       'html',
       '{"note":"Eutelsat runs on SuccessFactors; search needs a session/CSRF token. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.eutelsat.com/search/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'FlexJobs',
       'https://www.flexjobs.com/search',
       'html',
       '{"note":"FlexJobs requires a paid membership/login and its robots.txt was unreachable; not crawled."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.flexjobs.com/search');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Workable (search)',
       'https://jobs.workable.com/search?location=France&workplace=remote',
       'html',
       '{"note":"robots.txt disallows the Workable job search path."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.workable.com/search?location=France&workplace=remote');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Jobgether',
       'https://jobgether.com',
       'html',
       '{"note":"Jobgether is an aggregator that requires an account and renders matches client-side."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobgether.com');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Welcome to the Jungle (matches)',
       'https://www.welcometothejungle.com/fr/jobs-matches',
       'wttj',
       '{"note":"WTTJ uses an internal API with no public contract. Manual import only."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.welcometothejungle.com/fr/jobs-matches');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Greenhouse (candidate dashboard)',
       'https://my.greenhouse.io/dashboard',
       'html',
       '{"note":"Greenhouse candidate dashboard; requires a candidate login."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://my.greenhouse.io/dashboard');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'The Product Crew',
       'https://app.theproductcrew.io/',
       'html',
       '{"note":"The Product Crew is a candidate-only platform; requires login."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://app.theproductcrew.io/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'RemoteMore',
       'https://remotemore.com/candidates/',
       'html',
       '{"note":"RemoteMore is a candidate signup platform; no public job listing."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://remotemore.com/candidates/');
