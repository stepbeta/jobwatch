-- 0009_interview_entries: personal interview preparation questions and
-- answers (Milestone 9).
--
-- Answers are stored as markdown plaintext, never HTML. `answer_format`
-- is an enum-like TEXT value validated in Go (currently only
-- 'markdown'), so a richer format can be added without a table rebuild.
-- `display_order` is optional manual ordering; NULL falls back to id
-- order. Duplicate questions are allowed: this is a personal notebook,
-- not a deduplicated domain.
--
-- This is an additive schema change: no existing table or row changes.

CREATE TABLE interview_entries (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    question      TEXT NOT NULL,
    answer        TEXT NOT NULL DEFAULT '',
    answer_format TEXT NOT NULL DEFAULT 'markdown',
    display_order INTEGER,
    created_at    TEXT,
    updated_at    TEXT
);
