-- 0012_company_identity: company keys and aliases, offer identity carried
-- by provenance, the incomplete marker, and the end of the `changed`
-- state (openspec change offer-identity-from-source-uid).
--
-- Offer identity moves from job_offers.fingerprint to
-- job_offer_provenance(source_id, external_uid). The fingerprint column is
-- kept for compatibility with the current scan code but loses its UNIQUE
-- constraint; later tasks stop writing it.
--
-- Existing duplicate provenance rows are squashed onto the surviving
-- offer (active, then changed, then most recently seen). This is the one
-- place where links move between offers: the old duplicate offer rows are
-- not deleted and keep their events, notes and snapshots, but no longer
-- hold a source link. The user confirmed this reconciliation.
--
-- company_id becomes nullable so an item a source cannot normalize can be
-- stored as an incomplete offer instead of being dropped or attributed to
-- an invented company.
--
-- Rollback: the two tables and the two columns are additive. The only
-- lossy steps are the `changed` -> `active` update (recoverable by the
-- next scan) and the squashed provenance rows, which cannot be restored
-- without the pre-migration backup. `jobwatch backup` before migrating is
-- the supported safety net. Rebuilding job_offers preserves every row,
-- id and child table; the migration runner checks foreign keys before
-- committing.

-- 1. Company identity: one company may hold several attested keys, and
--    names are per-source claims. A NULL source_id alias is confirmed for
--    every source.
CREATE TABLE company_keys (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    created_at TEXT
);

CREATE UNIQUE INDEX idx_company_keys_key ON company_keys(key);
CREATE INDEX idx_company_keys_company ON company_keys(company_id);

CREATE TABLE company_aliases (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id      INTEGER NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    name_normalized TEXT NOT NULL,
    source_id       INTEGER REFERENCES sources(id) ON DELETE CASCADE,
    created_at      TEXT
);

CREATE UNIQUE INDEX idx_company_aliases_source_name
    ON company_aliases(source_id, name_normalized) WHERE source_id IS NOT NULL;
CREATE UNIQUE INDEX idx_company_aliases_global_name
    ON company_aliases(name_normalized) WHERE source_id IS NULL;
CREATE INDEX idx_company_aliases_company ON company_aliases(company_id);
CREATE INDEX idx_company_aliases_name ON company_aliases(name_normalized);

-- 2. Backfill one confirmed alias per existing company. Every company was
--    resolved by its normalized name until now, so the name is confirmed
--    for all sources.
INSERT INTO company_aliases(company_id, name_normalized, source_id, created_at)
SELECT id, name_normalized, NULL, '2026-11-21T00:00:00Z' FROM companies;

-- 3. Rebuild job_offers: nullable company, no fingerprint uniqueness, and
--    the incomplete marker. Foreign keys are off for the migration, so the
--    DROP TABLE below cannot cascade into events, notes or snapshots.
DROP TRIGGER job_offers_fts_ai;
DROP TRIGGER job_offers_fts_ad;
DROP TRIGGER job_offers_fts_au;

CREATE TABLE job_offers_new (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id          INTEGER REFERENCES companies(id),
    title               TEXT NOT NULL,
    title_normalized    TEXT NOT NULL,
    location            TEXT,
    remote_policy       TEXT NOT NULL DEFAULT 'unknown',
    url                 TEXT,
    fingerprint         TEXT NOT NULL DEFAULT '',
    first_seen_at       TEXT NOT NULL,
    last_seen_at        TEXT NOT NULL,
    offer_state         TEXT NOT NULL DEFAULT 'active',
    application_state   TEXT NOT NULL DEFAULT 'todo',
    raw_text            TEXT,
    text_hash           TEXT,
    title_confidence    INTEGER CHECK (title_confidence BETWEEN 0 AND 100),
    location_confidence INTEGER CHECK (location_confidence BETWEEN 0 AND 100),
    match_reasons_json  TEXT,
    created_at          TEXT,
    updated_at          TEXT,
    saved_at            TEXT,
    incomplete_reason   TEXT,
    incomplete_raw      TEXT
);

INSERT INTO job_offers_new (
    id, company_id, title, title_normalized, location, remote_policy, url,
    fingerprint, first_seen_at, last_seen_at, offer_state, application_state,
    raw_text, text_hash, title_confidence, location_confidence,
    match_reasons_json, created_at, updated_at, saved_at)
SELECT
    id, company_id, title, title_normalized, location, remote_policy, url,
    fingerprint, first_seen_at, last_seen_at, offer_state, application_state,
    raw_text, text_hash, title_confidence, location_confidence,
    match_reasons_json, created_at, updated_at, saved_at
FROM job_offers;

DROP TABLE job_offers;
ALTER TABLE job_offers_new RENAME TO job_offers;

CREATE INDEX idx_job_offers_company ON job_offers(company_id);
CREATE INDEX idx_job_offers_offer_state ON job_offers(offer_state);
CREATE INDEX idx_job_offers_application_state ON job_offers(application_state);
CREATE INDEX idx_job_offers_last_seen ON job_offers(last_seen_at);

CREATE TRIGGER job_offers_fts_ai AFTER INSERT ON job_offers BEGIN
    INSERT INTO job_offers_fts(rowid, title, raw_text)
    VALUES (new.id, new.title, new.raw_text);
END;

CREATE TRIGGER job_offers_fts_ad AFTER DELETE ON job_offers BEGIN
    INSERT INTO job_offers_fts(job_offers_fts, rowid, title, raw_text)
    VALUES ('delete', old.id, old.title, old.raw_text);
END;

CREATE TRIGGER job_offers_fts_au AFTER UPDATE OF title, raw_text ON job_offers BEGIN
    INSERT INTO job_offers_fts(job_offers_fts, rowid, title, raw_text)
    VALUES ('delete', old.id, old.title, old.raw_text);
    INSERT INTO job_offers_fts(rowid, title, raw_text)
    VALUES (new.id, new.title, new.raw_text);
END;

-- The external-content FTS index is unchanged, but rebuild it from the
-- rebuilt content table so it cannot reference stale rowids.
INSERT INTO job_offers_fts(job_offers_fts) VALUES('rebuild');

-- 4. `changed` is no longer a listing state. A description change is kept
--    in snapshots and compared at presentation time.
UPDATE job_offers SET offer_state = 'active' WHERE offer_state = 'changed';

-- 5. Squash duplicate provenance rows. For each (source, uid) the
--    surviving offer is the active one, then the changed one, then the
--    most recently seen; the kept link inherits the earliest first-seen
--    and the latest last-seen time. The other links are removed, but their
--    offers, events, notes and snapshots are untouched.
CREATE TEMP TABLE _provenance_squash AS
WITH winners AS (
    SELECT p.source_id, p.external_uid,
           (SELECT p2.offer_id
            FROM job_offer_provenance p2
            JOIN job_offers o2 ON o2.id = p2.offer_id
            WHERE p2.source_id = p.source_id
              AND p2.external_uid = p.external_uid
            ORDER BY CASE o2.offer_state
                         WHEN 'active' THEN 0
                         WHEN 'changed' THEN 1
                         ELSE 2
                     END,
                     COALESCE(p2.last_seen_at, '') DESC,
                     p2.offer_id DESC,
                     p2.id DESC
            LIMIT 1) AS winner_offer
    FROM job_offer_provenance p
    WHERE p.external_uid IS NOT NULL AND p.external_uid <> ''
    GROUP BY p.source_id, p.external_uid
    HAVING COUNT(*) > 1
)
SELECT p.id,
       w.winner_offer,
       ROW_NUMBER() OVER (
           PARTITION BY p.source_id, p.external_uid
           ORDER BY (p.offer_id = w.winner_offer) DESC,
                    COALESCE(p.last_seen_at, '') DESC,
                    p.id DESC) AS rn,
       MIN(p.first_seen_at) OVER (PARTITION BY p.source_id, p.external_uid) AS min_first,
       MAX(p.last_seen_at) OVER (PARTITION BY p.source_id, p.external_uid) AS max_last
FROM job_offer_provenance p
JOIN winners w
  ON w.source_id = p.source_id AND w.external_uid = p.external_uid;

DELETE FROM job_offer_provenance
WHERE id IN (SELECT id FROM _provenance_squash WHERE rn > 1);

UPDATE job_offer_provenance
SET offer_id = (SELECT winner_offer FROM _provenance_squash WHERE rn = 1 AND id = job_offer_provenance.id),
    first_seen_at = (SELECT min_first FROM _provenance_squash WHERE rn = 1 AND id = job_offer_provenance.id),
    last_seen_at = (SELECT max_last FROM _provenance_squash WHERE rn = 1 AND id = job_offer_provenance.id)
WHERE id IN (SELECT id FROM _provenance_squash WHERE rn = 1);

DROP TABLE _provenance_squash;

DROP INDEX IF EXISTS idx_provenance_source;
CREATE UNIQUE INDEX idx_provenance_source_uid
    ON job_offer_provenance(source_id, external_uid)
    WHERE external_uid IS NOT NULL AND external_uid <> '';

-- 6. Backfill company keys from the typed adapters' config tokens, but
--    only for a source whose offers all belong to one company. Sources
--    that list several employers (or have no typed token) resolve by name
--    and learn their keys on the next scan. A duplicated token between two
--    sources is ignored here rather than failing the upgrade; the next
--    scan resolves it through the alias tables.
INSERT OR IGNORE INTO company_keys(company_id, key, created_at)
SELECT DISTINCT
       sc.company_id,
       s.source_type || ':' || CASE s.source_type
           WHEN 'greenhouse' THEN json_extract(s.config_json, '$.board_token')
           WHEN 'lever'      THEN json_extract(s.config_json, '$.site')
           WHEN 'ashby'      THEN json_extract(s.config_json, '$.org')
           WHEN 'personio'   THEN json_extract(s.config_json, '$.tenant')
           WHEN 'workable'   THEN json_extract(s.config_json, '$.account')
           WHEN 'bamboohr'   THEN json_extract(s.config_json, '$.subdomain')
       END,
       '2026-11-21T00:00:00Z'
FROM (
    SELECT p.source_id, MIN(o.company_id) AS company_id
    FROM job_offer_provenance p
    JOIN job_offers o ON o.id = p.offer_id
    WHERE p.source_id IS NOT NULL
    GROUP BY p.source_id
    HAVING COUNT(DISTINCT o.company_id) = 1
) sc
JOIN sources s ON s.id = sc.source_id
WHERE s.source_type IN ('greenhouse','lever','ashby','personio','workable','bamboohr')
  AND CASE s.source_type
          WHEN 'greenhouse' THEN json_extract(s.config_json, '$.board_token')
          WHEN 'lever'      THEN json_extract(s.config_json, '$.site')
          WHEN 'ashby'      THEN json_extract(s.config_json, '$.org')
          WHEN 'personio'   THEN json_extract(s.config_json, '$.tenant')
          WHEN 'workable'   THEN json_extract(s.config_json, '$.account')
          WHEN 'bamboohr'   THEN json_extract(s.config_json, '$.subdomain')
      END IS NOT NULL
  AND CASE s.source_type
          WHEN 'greenhouse' THEN json_extract(s.config_json, '$.board_token')
          WHEN 'lever'      THEN json_extract(s.config_json, '$.site')
          WHEN 'ashby'      THEN json_extract(s.config_json, '$.org')
          WHEN 'personio'   THEN json_extract(s.config_json, '$.tenant')
          WHEN 'workable'   THEN json_extract(s.config_json, '$.account')
          WHEN 'bamboohr'   THEN json_extract(s.config_json, '$.subdomain')
      END <> '';
