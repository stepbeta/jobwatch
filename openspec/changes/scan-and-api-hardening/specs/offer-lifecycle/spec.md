# Spec Delta

## MODIFIED Requirements

### Requirement: Listing state is active or disappeared

An offer SHALL have exactly one of two listing states: `active` or
`disappeared`. A description change MUST NOT introduce a third state. A scan
observation SHALL count for disappearance only when it returned at least one
item and, for a source with four or more known active offers, returned at
least half of them. An observation below that threshold SHALL be recorded as
a scan error and MUST leave every listing state unchanged.

#### Scenario: Posting no longer listed

- **WHEN** a successful scan of a source no longer lists an offer, and no other successfully scanned source lists it
- **THEN** the offer becomes `disappeared`

#### Scenario: Posting listed again

- **WHEN** a later scan lists a `disappeared` offer
- **THEN** the offer becomes `active`

#### Scenario: Description edited by the employer

- **WHEN** a scan sees the same uid with a different description
- **THEN** the offer keeps its listing state and a new snapshot is recorded

#### Scenario: Empty result is not a disappearance

- **WHEN** a source that has active offers returns zero items
- **THEN** the scan is recorded as an error and every offer of that source keeps its listing state

#### Scenario: Suspicious drop is not a disappearance

- **WHEN** a source with at least four known active offers returns fewer than half of them
- **THEN** the scan is recorded as an error and every offer of that source keeps its listing state

#### Scenario: Small source may shrink without tripping the ratio guard

- **WHEN** a source with two known active offers returns one item
- **THEN** the missing offer follows the normal disappearance rules

#### Scenario: First scan of a source is never a drop

- **WHEN** a source with no active offers returns a partial list
- **THEN** the scan is applied normally and the ratio guard does not apply
