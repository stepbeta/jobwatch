-- 0016_murena_detail_description: enable the full detail-page description
-- for the seeded Murena source.
--
-- Murena's listing stores a short summary followed by "Read full job
-- description."; the link target holds the full posting in
-- `section.entry-content`. The HTML adapter's
-- `detail_description_selector` fetches that page once for offers whose
-- stored text is still the listing summary, stores the full text, and
-- reuses it on later scans. See docs/SOURCE-DECISIONS.md.
--
-- This is a data-only, idempotent migration: only the exact seeded Murena
-- row is touched, only while it has no detail selector, and json_set
-- preserves every other config key. Re-applying the file is a no-op and a
-- user-edited selector is never overwritten.

UPDATE sources
   SET config_json = json_set(config_json,
                              '$.detail_description_selector',
                              'section.entry-content')
 WHERE url = 'https://murena.com/jobs/'
   AND json_valid(config_json)
   AND json_type(config_json) = 'object'
   AND json_extract(config_json, '$.detail_description_selector') IS NULL;
