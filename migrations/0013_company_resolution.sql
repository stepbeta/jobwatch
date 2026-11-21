-- 0013_company_resolution: company display names are no longer unique,
-- and single-employer sources declare their company in config_json.
--
-- Company identity now comes from company_keys and company_aliases
-- (migration 0012). Two employers may share a display name, so the
-- UNIQUE constraint on companies.name_normalized is removed and replaced
-- by a non-unique lookup index. Keys and aliases carry identity; name is
-- an attribute.
--
-- The scan pipeline no longer falls back to sources.name for a company
-- (task 3.2 of the OpenSpec change). Enabled single-employer sources get
-- `config.company` set to their current display name, preserving the old
-- attribution without the pipeline guessing. We Work Remotely lists many
-- employers and is left for the RSS/HTML employer extraction task.
--
-- Rollback: the config update is additive (it only fills a missing key)
-- and the companies rebuild preserves every row, id and child reference.
-- Re-adding the UNIQUE constraint is not possible while duplicate names
-- exist, so a rollback requires the pre-migration backup. The migration
-- runner disables foreign keys on a pinned connection and checks them
-- before committing, so the DROP TABLE cannot cascade into offers,
-- keys or aliases.

CREATE TABLE companies_new (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    name_normalized TEXT NOT NULL,
    website         TEXT,
    created_at      TEXT
);

INSERT INTO companies_new(id, name, name_normalized, website, created_at)
SELECT id, name, name_normalized, website, created_at FROM companies;

DROP TABLE companies;
ALTER TABLE companies_new RENAME TO companies;

CREATE INDEX idx_companies_name_normalized ON companies(name_normalized);

-- Every company needs at least one alias so name-only resolution can find
-- it. 0012 backfilled one per company; this covers rows created since.
INSERT OR IGNORE INTO company_aliases(company_id, name_normalized, source_id, created_at)
SELECT c.id, c.name_normalized, NULL, '2026-11-21T00:00:00Z'
FROM companies c
WHERE NOT EXISTS (SELECT 1 FROM company_aliases a WHERE a.company_id = c.id);

-- Single-employer sources declare their company. Typed ATS boards and the
-- enabled single-employer RSS/HTML pages keep the display name they had
-- through the old sources.name fallback. We Work Remotely is excluded.
UPDATE sources
SET config_json = json_set(config_json, '$.company', name)
WHERE enabled = 1
  AND source_type IN ('greenhouse','lever','ashby','personio','workable','bamboohr','rss','html','wttj')
  AND name <> 'We Work Remotely'
  AND COALESCE(json_extract(config_json, '$.company'), '') = '';
