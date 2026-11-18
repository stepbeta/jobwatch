-- 0011_seed_wttj_sources: ship the Welcome to the Jungle source verified
-- in Milestone 10a.
--
-- Fabriq's careers page is served by WTTJ's Algolia index. The `wttj`
-- adapter discovers the index name, filters and secured credentials from
-- the page bootstrap on every scan, so the only setting is the careers
-- page URL; config_json stays empty. See docs/SOURCE-DECISIONS.md.
--
-- Migration 0002 seeds a different, still-disabled WTTJ row
-- (https://www.welcometothejungle.com/en/jobs?query=...). The URLs do not
-- collide and applied migrations are immutable, so that row is left as is.
--
-- This is a data-only migration. The INSERT is a no-op when a source with
-- the same URL already exists, and no existing row is ever updated, so
-- re-applying the file cannot duplicate a row or overwrite user edits.

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Fabriq',
       'https://www.welcometothejungle.com/en/companies/fabriq/jobs',
       'wttj',
       '{}',
       1,
       '2026-11-18T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.welcometothejungle.com/en/companies/fabriq/jobs');
