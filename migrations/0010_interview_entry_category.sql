-- 0010_interview_entry_category: one optional category per interview-prep
-- entry (Milestone 10).
--
-- `category` is a free-form display name shared by rows: there is no
-- categories table. NULL means "uncategorized". The web UI groups entries
-- by this value, orders the groups alphabetically (Uncategorized last) and
-- may persist a presentation-only group order locally. The column is
-- trimmed and must be non-blank when supplied.
--
-- This is an additive schema change: existing entries keep every column and
-- read as uncategorized (NULL). No row is deleted or rewritten.

ALTER TABLE interview_entries ADD COLUMN category TEXT;
