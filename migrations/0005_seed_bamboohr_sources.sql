-- 0005_seed_bamboohr_sources: ship the BambooHR source verified in Milestone 6a.
--
-- The source was registered with `jobwatch add-source
-- https://semble.bamboohr.com/careers --name Semble` after the adapter was
-- implemented and a real scan returned offers. `subdomain` is the only
-- config key the adapter needs; it is also derivable from the URL, but the
-- row stores it explicitly so the configuration is self-describing.
--
-- This is a data-only migration. The INSERT is a no-op when a source with
-- the same URL already exists, and no existing row is ever updated, so
-- re-applying the file cannot duplicate a row or overwrite user edits.

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Semble',
       'https://semble.bamboohr.com/careers',
       'bamboohr',
       '{"subdomain":"semble"}',
       1,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://semble.bamboohr.com/careers');
