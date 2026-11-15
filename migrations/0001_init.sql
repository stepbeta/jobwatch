-- 0001_init: initial schema (SPEC section 4).
-- Enum-like TEXT columns are validated in Go rather than with CHECK
-- constraints, so that adding a value later does not require a table
-- rebuild. Timestamps are RFC3339 TEXT in UTC.

CREATE TABLE companies (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    name_normalized TEXT NOT NULL UNIQUE,
    website         TEXT,
    created_at      TEXT
);

CREATE TABLE sources (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    source_type      TEXT NOT NULL,
    config_json      TEXT NOT NULL DEFAULT '{}',
    enabled          INTEGER NOT NULL DEFAULT 1,
    last_scan_at     TEXT,
    last_scan_status TEXT,
    created_at       TEXT
);

CREATE INDEX idx_sources_url ON sources(url);

CREATE TABLE job_offers (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    company_id          INTEGER NOT NULL REFERENCES companies(id),
    title               TEXT NOT NULL,
    title_normalized    TEXT NOT NULL,
    location            TEXT,
    remote_policy       TEXT NOT NULL DEFAULT 'unknown',
    url                 TEXT,
    fingerprint         TEXT NOT NULL UNIQUE,
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
    updated_at          TEXT
);

CREATE INDEX idx_job_offers_company ON job_offers(company_id);
CREATE INDEX idx_job_offers_offer_state ON job_offers(offer_state);
CREATE INDEX idx_job_offers_application_state ON job_offers(application_state);
CREATE INDEX idx_job_offers_last_seen ON job_offers(last_seen_at);

CREATE TABLE job_offer_provenance (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_id      INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
    source_id     INTEGER REFERENCES sources(id),
    external_uid  TEXT,
    url           TEXT NOT NULL,
    first_seen_at TEXT,
    last_seen_at  TEXT,
    UNIQUE (offer_id, url)
);

CREATE INDEX idx_provenance_source ON job_offer_provenance(source_id, external_uid);

CREATE TABLE events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_id    INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
    type        TEXT NOT NULL,
    description TEXT,
    occurred_at TEXT NOT NULL,
    created_at  TEXT
);

CREATE INDEX idx_events_offer ON events(offer_id, occurred_at);

CREATE TABLE notes (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    entity_type     TEXT NOT NULL,
    entity_id       INTEGER NOT NULL,
    body            TEXT NOT NULL,
    pinned_event_id INTEGER REFERENCES events(id) ON DELETE SET NULL,
    created_at      TEXT,
    updated_at      TEXT
);

CREATE INDEX idx_notes_entity ON notes(entity_type, entity_id);
CREATE INDEX idx_notes_pinned_event ON notes(pinned_event_id);

CREATE TABLE snapshots (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_id    INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
    raw_text    TEXT NOT NULL,
    captured_at TEXT NOT NULL
);

CREATE INDEX idx_snapshots_offer ON snapshots(offer_id, captured_at);

-- Full-text search. External-content FTS5 tables mirror the base tables
-- and are kept in sync by triggers.

CREATE VIRTUAL TABLE job_offers_fts USING fts5(
    title,
    raw_text,
    content = 'job_offers',
    content_rowid = 'id',
    tokenize = 'unicode61 remove_diacritics 2'
);

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

CREATE VIRTUAL TABLE notes_fts USING fts5(
    body,
    content = 'notes',
    content_rowid = 'id',
    tokenize = 'unicode61 remove_diacritics 2'
);

CREATE TRIGGER notes_fts_ai AFTER INSERT ON notes BEGIN
    INSERT INTO notes_fts(rowid, body) VALUES (new.id, new.body);
END;

CREATE TRIGGER notes_fts_ad AFTER DELETE ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, body) VALUES ('delete', old.id, old.body);
END;

CREATE TRIGGER notes_fts_au AFTER UPDATE OF body ON notes BEGIN
    INSERT INTO notes_fts(notes_fts, rowid, body) VALUES ('delete', old.id, old.body);
    INSERT INTO notes_fts(rowid, body) VALUES (new.id, new.body);
END;
