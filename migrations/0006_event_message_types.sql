-- 0006_event_message_types: rename reply_received to message_received.
--
-- Milestone 7b simplifies the event vocabulary. `message_received` now
-- covers both a recruiter's inbound first contact and a reply to an
-- application, so the old `reply_received` value is renamed in place.
-- `message_sent` is a new value for user-initiated pings/follow-ups; it is
-- validated in Go and needs no data change.
--
-- This is a data-only migration. No CHECK constraint exists on
-- events.type, so no table rebuild is required. The WHERE clause makes the
-- statement idempotent: re-applying the file matches no rows and cannot
-- duplicate or revert anything.

UPDATE events SET type = 'message_received' WHERE type = 'reply_received';
