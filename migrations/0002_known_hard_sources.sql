-- 0002_known_hard_sources: seed the known-hard sources as disabled rows.
--
-- The user asked for these links to be present in the source list with an
-- explanation, without pretending the scanners can read them:
--
--   * Datadog and Cisco run on Workday: the public careers site uses a
--     POST/JSON API and an anti-bot layer, so they are out of scope for
--     v1 (SPEC section 7).
--   * SAP runs on SuccessFactors: its search endpoint needs a
--     session/CSRF handshake and answers bot-like requests with 403.
--   * Welcome to the Jungle exposes an internal API without a public
--     contract; it is kept disabled until a stable endpoint is verified.
--
-- Rows are disabled, use source_type html/wttj (so the UI knows there is
-- no dedicated adapter) and carry a human-readable "note" in config_json
-- that the sources page displays. The INSERTs are no-ops when a source
-- with the same URL already exists, so re-running the migration file or
-- upgrading a database that already has one of these rows is safe.

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Datadog (Workday)',
       'https://careers.datadoghq.com/all-jobs/',
       'html',
       '{"company":"Datadog","rate_limit_ms":5000,"note":"Datadog runs on Workday. Its careers site needs a POST/JSON API and an anti-bot layer; no adapter in v1. This row is disabled on purpose."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://careers.datadoghq.com/all-jobs/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Cisco (Workday)',
       'https://jobs.cisco.com/',
       'html',
       '{"company":"Cisco","rate_limit_ms":5000,"note":"Cisco runs on Workday. The jobs API is a POST/JSON surface; no adapter in v1. This row is disabled on purpose."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.cisco.com/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'SAP (SuccessFactors)',
       'https://jobs.sap.com/',
       'html',
       '{"company":"SAP","rate_limit_ms":5000,"note":"SAP runs on SuccessFactors. Its search needs a session/CSRF handshake and blocks bot-like requests with 403; no adapter in v1. This row is disabled on purpose."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://jobs.sap.com/');

INSERT INTO sources(name, url, source_type, config_json, enabled, created_at)
SELECT 'Welcome to the Jungle (search)',
       'https://www.welcometothejungle.com/en/jobs?query=platform%20engineer',
       'wttj',
       '{"rate_limit_ms":10000,"note":"WTTJ uses an internal API with no public contract. Kept disabled; use manual import or a company RSS/HTML source instead."}',
       0,
       '2026-11-16T00:00:00Z'
WHERE NOT EXISTS (SELECT 1 FROM sources WHERE url = 'https://www.welcometothejungle.com/en/jobs?query=platform%20engineer');
