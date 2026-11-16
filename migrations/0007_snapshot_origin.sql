-- 0007_snapshot_origin: record where each description snapshot came from.
--
-- `scan` is written by the scan pipeline and manual URL imports; `manual`
-- is written when the user saves a description through
-- `PATCH /api/offers/:id`. The column is added with a default so existing
-- snapshot rows keep their data and become `scan`.
--
-- Historical manual snapshots cannot be identified reliably: both scan and
-- manual inserts set `captured_at` to the moment of the write, and scans
-- also bump `job_offers.updated_at` in the same transaction. The only
-- partial signal (a latest snapshot whose captured_at equals updated_at
-- but differs from last_seen_at) cannot distinguish older manual versions
-- from scan versions. The backfill therefore deliberately classifies every
-- pre-0007 snapshot as `scan`; origins are exact from this migration on.
--
-- SQLite applies the default to existing rows when the column is added, so
-- no UPDATE is required. This is an additive schema change: no row, column
-- or snapshot text is removed or rewritten.

ALTER TABLE snapshots ADD COLUMN origin TEXT NOT NULL DEFAULT 'scan';
