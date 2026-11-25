# application-lifecycle Specification

## Purpose

Defines the application tracking lifecycle for an offer: its state vocabulary,
the saved shortlist, manual and event-driven transitions, automatic ghosting
when a listing disappears, and the dashboard views that present it.

## Requirements

### Requirement: Application state vocabulary

Every offer SHALL hold exactly one application state from `none`, `saved`,
`applied`, `interview`, `offer`, `accepted`, `turned_down` or `negative`.
`none` SHALL be the default for a newly created offer. `todo`, `responded`,
`maybe` and `positive` MUST be rejected as invalid values.

#### Scenario: New offer starts at none

- **WHEN** a scan or a manual import creates an offer
- **THEN** its application state is `none`

#### Scenario: Removed value is rejected

- **WHEN** a client updates an offer with `application_state = "todo"`
- **THEN** the request returns `400 validation_failed` and the state is unchanged

#### Scenario: State list is validated

- **WHEN** an offer list request filters on `application_state=accepted,turned_down`
- **THEN** only offers in those two states are returned

### Requirement: Saved is an application state

Saving an offer SHALL be the `saved` application state, not an independent
bookmark. Setting an offer to `saved` SHALL record the current server time in
`saved_at`. Moving an offer to any other state SHALL clear `saved_at`. A saved
offer MUST NOT be treated as an application.

#### Scenario: Offering saved records the timestamp

- **WHEN** the user sets an offer from `none` to `saved`
- **THEN** its state is `saved` and `saved_at` is set to the current server time

#### Scenario: Applying a saved offer clears the timestamp

- **WHEN** a `cv_sent` event is recorded on a `saved` offer
- **THEN** its state becomes `applied` and `saved_at` is cleared

#### Scenario: Unsaving returns to none

- **WHEN** the user sets a `saved` offer back to `none`
- **THEN** its state is `none` and `saved_at` is cleared

#### Scenario: Saved is not an application

- **WHEN** an offer is in `saved`
- **THEN** it does not appear in the applications view or its refresh scope

### Requirement: Manual state changes are free-form

The user SHALL be able to set any valid application state manually. A manual
state change MUST NOT create a timeline event and MUST NOT trigger a transition
derived from existing events.

#### Scenario: Manual advance to offer

- **WHEN** the user changes an offer from `interview` to `offer`
- **THEN** the state is `offer` and no timeline event is created

#### Scenario: Manual correction after a rejection

- **WHEN** the user changes a `negative` offer to `applied`
- **THEN** the state is `applied` and no event is created

### Requirement: CV sent opens the application

Creating a `cv_sent` event SHALL set the state to `applied` when the current
state is `none`, `saved` or `negative`. It MUST NOT change an offer already in
`applied`, `interview`, `offer`, `accepted` or `turned_down`. The event is
always recorded. The transition depends on the state at creation time, not on
the event's `occurred_at`.

#### Scenario: First CV opens the application

- **WHEN** a `cv_sent` event is created on an offer in `none`
- **THEN** the state becomes `applied`

#### Scenario: Re-application reopens a negative offer

- **WHEN** a `cv_sent` event is created on a `negative` offer
- **THEN** the state becomes `applied`, even when the event is backdated

#### Scenario: CV sent never downgrades

- **WHEN** a `cv_sent` event is created on an `interview` offer
- **THEN** the state stays `interview`

### Requirement: Rejection closes the application

Creating a `rejected` event SHALL set the state to `negative` from any current
state, including `none`, `saved`, `offer`, `accepted` and `turned_down`. An
offer already in `negative` SHALL stay in `negative`. The event is always
recorded.

#### Scenario: Rejection after applying

- **WHEN** a `rejected` event is created on an `applied` offer
- **THEN** the state becomes `negative`

#### Scenario: Rejection after accepting

- **WHEN** a `rejected` event is created on an `accepted` offer
- **THEN** the state becomes `negative`

#### Scenario: Repeated rejection is idempotent

- **WHEN** a `rejected` event is created on a `negative` offer
- **THEN** the state stays `negative` and the event is recorded

### Requirement: Inbound rejection shortcut

When recording an inbound message, the user interface SHALL offer a "this is a
rejection" shortcut. Using it SHALL record a `rejected` event with the same
description and occurrence time and SHALL apply the rejection transition. It
MUST NOT create a separate `message_received` event.

#### Scenario: Rejection email is recorded as a rejection

- **WHEN** the user records a message and enables the rejection shortcut
- **THEN** one `rejected` event is stored and the state becomes `negative`

### Requirement: Automatic ghosting on disappearance

When a successful scan transitions an offer from `active` to `disappeared` and
its application state is `applied` or `interview`, the system SHALL create a
`ghosted` event and set the state to `negative` in the same transaction. The
event SHALL use the scan's detection time. Other states and already-disappeared
offers MUST NOT be ghosted, and the migration MUST NOT backfill history.

#### Scenario: Applied offer is ghosted

- **WHEN** a scan no longer lists an `applied` offer and no other successfully scanned source lists it
- **THEN** the offer becomes `disappeared`, one `ghosted` event is created, and its state becomes `negative`

#### Scenario: Interviewing offer is ghosted

- **WHEN** a scan no longer lists an `interview` offer and no other successfully scanned source lists it
- **THEN** one `ghosted` event is created and its state becomes `negative`

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

### Requirement: Event edits and deletions do not recompute state

Application transitions SHALL be applied only when an event is created. Updating
an event's type, description or occurrence time, and deleting an event, MUST NOT
change the application state.

#### Scenario: Editing a message into a rejection

- **WHEN** an existing `message_received` event is changed to `rejected`
- **THEN** the application state is unchanged

#### Scenario: Deleting the rejection

- **WHEN** a `rejected` event is deleted
- **THEN** the offer stays `negative` until the user changes it

### Requirement: Dashboard sections and filters

The dashboard SHALL show `saved` offers in a Saved section and all other
non-`none` offers in an Applications section. It SHALL provide a listing filter
(`all` / `active only`) applying to both sections and an application filter
(`all` / `hide negative`) applying to Applications. The refresh action SHALL
scan only the enabled sources behind the currently visible applications.
Filter choices SHALL persist in the dashboard URL.

#### Scenario: Active-only hides disappeared listings

- **WHEN** the listing filter is `active only`
- **THEN** offers whose listing state is `disappeared` are hidden from both sections

#### Scenario: Hide-negative keeps closed applications

- **WHEN** the application filter is `hide negative`
- **THEN** `negative` offers are hidden while `accepted` and `turned_down` remain visible

#### Scenario: Refresh follows the visible applications

- **WHEN** the user refreshes with a filter hiding negative applications
- **THEN** only sources behind the visible applications are scanned

#### Scenario: Filters survive a reload

- **WHEN** the dashboard URL contains `listing=active` and `application=hide_negative`
- **THEN** the same filters are restored on load

### Requirement: Existing data is migrated

Upgrading an existing database SHALL map `todo` and `maybe` to `none`,
`responded` to `applied`, and `positive` to `accepted`. An offer with
`saved_at` set and a `todo` or `maybe` state SHALL become `saved`; an offer with
`saved_at` set and any other state SHALL keep that state and clear `saved_at`.
The default application state SHALL become `none`. No offer, event, note,
snapshot or provenance row is deleted.

#### Scenario: Default state is migrated

- **WHEN** an existing `todo` offer without a bookmark is migrated
- **THEN** its state is `none`

#### Scenario: Bookmarked pre-application offer becomes saved

- **WHEN** an existing `todo` offer has `saved_at` set
- **THEN** its state is `saved` and it keeps its `saved_at`

#### Scenario: Closed application keeps its stage

- **WHEN** an existing `applied` offer has `saved_at` set
- **THEN** its state remains `applied` and `saved_at` is cleared

#### Scenario: History is preserved

- **WHEN** the migration completes
- **THEN** every offer ID and every event, note, snapshot and provenance row still exists

### Requirement: Scan summary reports ghosted offers

The scan summary SHALL report how many offers were automatically ghosted during
the scan, separately from the total number of disappeared offers.

#### Scenario: Ghosted count is reported

- **WHEN** a scan ghosts two applied offers
- **THEN** its summary reports two disappeared offers and two ghosted offers
