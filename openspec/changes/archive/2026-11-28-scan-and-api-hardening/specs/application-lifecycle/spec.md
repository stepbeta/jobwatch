# Spec Delta

## MODIFIED Requirements

### Requirement: Automatic ghosting on disappearance

When a successful scan transitions an offer from `active` to `disappeared` and
its application state is `applied` or `interview`, the system SHALL create a
`ghosted` event and set the state to `negative` in the same transaction. The
event SHALL use the time the offer was last seen. Other states and
already-disappeared offers MUST NOT be ghosted, and the migration MUST NOT
backfill history. A disappearance that the scan guard rejects is not a
successful transition and MUST NOT ghost.

#### Scenario: Applied offer is ghosted

- **WHEN** a scan no longer lists an `applied` offer and no other successfully scanned source lists it
- **THEN** the offer becomes `disappeared`, one `ghosted` event is created, and its state becomes `negative`

#### Scenario: Interviewing offer is ghosted

- **WHEN** a scan no longer lists an `interview` offer and no other successfully scanned source lists it
- **THEN** one `ghosted` event is created and its state becomes `negative`

#### Scenario: Ghost event uses the last-seen time

- **WHEN** an applied offer was last seen two days before the scan that no longer lists it
- **THEN** the `ghosted` event's occurrence time is that last-seen time, not the scan time

#### Scenario: Offer stage is not ghosted

- **WHEN** a scan no longer lists an offer whose state is `offer`
- **THEN** the offer becomes `disappeared` but keeps `offer` and no `ghosted` event is created

#### Scenario: Saved and none offers are not ghosted

- **WHEN** a scan no longer lists an offer in `saved` or `none`
- **THEN** the offer becomes `disappeared`, keeps its state and no `ghosted` event is created

#### Scenario: Repeated scan does not duplicate the event

- **WHEN** a later scan runs after an offer was already ghosted
- **THEN** no additional `ghosted` event is created

#### Scenario: Reappearance and second disappearance

- **WHEN** a ghosted offer reappears and later disappears again
- **THEN** a second `ghosted` event is created and the state becomes `negative`

#### Scenario: Rejected disappearance does not ghost

- **WHEN** a scan is rejected by the empty or suspicious-drop guard
- **THEN** the offer keeps its listing and application state and no `ghosted` event is created

## ADDED Requirements

### Requirement: Date-only event times keep the entered date

When the user records or edits a timeline event with a date and no time, the
web UI SHALL send an RFC3339 instant at local midnight in the user's UTC
offset. The API SHALL continue to interpret a bare `YYYY-MM-DD` value as UTC
midnight for callers that send one. The date shown to the user MUST be the
date that was entered.

#### Scenario: Date-only event in a negative-offset zone

- **WHEN** a user in `America/New_York` records an event dated 2026-11-12 with no time
- **THEN** the stored instant is 2026-11-12T05:00:00Z and the event renders as 12 Nov 2026

#### Scenario: Date-time event keeps its instant

- **WHEN** an event is recorded with a date and a time
- **THEN** the stored instant matches the local date and time the user entered
