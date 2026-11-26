-- 0018_offer_documents: documents attached to an offer, stored inside the
-- database as content-addressed PDF blobs (OpenSpec change offer-documents).
--
-- Additive only: two new tables and their indexes. job_offers and every
-- existing child table are untouched, so this migration needs no table
-- rebuild, no FTS rebuild and causes no ID churn.
--
-- attachment_blobs keeps exactly one row per distinct byte sequence, keyed
-- by the SHA-256 of the raw bytes. offer_attachments is the thin reference
-- row that tags those bytes for one offer. A blob is deleted only when the
-- last reference is detached; the foreign key without ON DELETE CASCADE
-- makes a premature blob delete fail instead of silently stranding rows.
--
-- Rollback: drop the two tables, or restore the pre-migration backup
-- written by `jobwatch backup` before 0018 was applied.

CREATE TABLE attachment_blobs (
    sha256            TEXT PRIMARY KEY,
    bytes             BLOB NOT NULL,
    size              INTEGER NOT NULL,
    mime              TEXT NOT NULL DEFAULT 'application/pdf',
    pdf_title         TEXT,
    pdf_author        TEXT,
    pdf_producer      TEXT,
    pdf_creation_date TEXT,
    pdf_mod_date      TEXT,
    pdf_page_count    INTEGER,
    pdf_version       TEXT,
    created_at        TEXT NOT NULL
);

CREATE TABLE offer_attachments (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    offer_id    INTEGER NOT NULL REFERENCES job_offers(id) ON DELETE CASCADE,
    blob_sha256 TEXT NOT NULL REFERENCES attachment_blobs(sha256),
    kind        TEXT NOT NULL,
    filename    TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (offer_id, blob_sha256)
);

CREATE INDEX idx_offer_attachments_offer ON offer_attachments(offer_id);
CREATE INDEX idx_offer_attachments_blob ON offer_attachments(blob_sha256);
