-- 0017_application_lifecycle: replace the application state vocabulary,
-- make `saved` a state, and change the column default to `none`
-- (openspec change simplify-application-lifecycle).
--
-- The state enum becomes none, saved, applied, interview, offer, accepted,
-- turned_down, negative. `todo`, `responded`, `maybe` and `positive` are
-- removed. `saved` replaces the independent `saved_at` bookmark: a
-- bookmarked pre-application offer becomes `saved`, while a bookmarked
-- offer in a later stage keeps that stage and loses the stale bookmark.
--
-- Changing the column default requires rebuilding job_offers. The rebuild
-- preserves every offer ID and every child row (foreign keys are disabled
-- by the migration runner and checked before commit). The FTS triggers are
-- recreated and the external-content index is rebuilt, following 0012.
--
-- Rollback: the state mapping is not reversible in SQL; restore the
-- pre-migration backup written by `jobwatch backup`.

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
    application_state   TEXT NOT NULL DEFAULT 'none',
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
    match_reasons_json, created_at, updated_at, saved_at,
    incomplete_reason, incomplete_raw)
SELECT
    id, company_id, title, title_normalized, location, remote_policy, url,
    fingerprint, first_seen_at, last_seen_at, offer_state,
    CASE
        WHEN saved_at IS NOT NULL AND application_state IN ('todo', 'maybe')
            THEN 'saved'
        WHEN application_state IN ('todo', 'maybe') THEN 'none'
        WHEN application_state = 'responded' THEN 'applied'
        WHEN application_state = 'positive' THEN 'accepted'
        ELSE application_state
    END,
    raw_text, text_hash, title_confidence, location_confidence,
    match_reasons_json, created_at, updated_at,
    CASE
        WHEN saved_at IS NOT NULL AND application_state IN ('todo', 'maybe')
            THEN saved_at
        ELSE NULL
    END,
    incomplete_reason, incomplete_raw
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
