-- 0008_offer_saved_at: a persistent personal bookmark on each offer.
--
-- `saved_at` is NULL when the offer is not bookmarked and holds the UTC
-- RFC3339 time of the bookmark otherwise. It is orthogonal to both
-- offer_state and application_state: saving does not change either, and
-- an offer keeps its bookmark after it becomes an application until the
-- user explicitly removes it.
--
-- This is an additive schema change: existing offers keep every column
-- and read as not saved (NULL). No row is deleted or rewritten.

ALTER TABLE job_offers ADD COLUMN saved_at TEXT;
