# Spec Delta

## Purpose

Defines what makes an offer the same offer across scans, and how its listing state and description history are maintained.

## ADDED Requirements

### Requirement: Offer identity comes from the source

An offer SHALL be identified by the pair `(source, external_uid)`. When a source supplies no uid, the pair `(source, url)` SHALL be used. Title, company name and location are attributes and MUST NOT participate in identity.

#### Scenario: Posting seen again on the same source

- **WHEN** a scan sees the same uid it saw on an earlier scan
- **THEN** it updates the existing offer and creates no new row

#### Scenario: Title edited under the same uid

- **WHEN** a scan sees the same uid with a different title
- **THEN** the existing offer's title is updated and no offer is created or marked disappeared

#### Scenario: Two postings share a title

- **WHEN** one source lists two postings with the same company and title but different uids
- **THEN** two offers exist

#### Scenario: Source supplies no uid

- **WHEN** a source reports an item without a uid
- **THEN** identity falls back to `(source, url)`

### Requirement: Listing state is active or disappeared

An offer SHALL have exactly one of two listing states: `active` or `disappeared`. A description change MUST NOT introduce a third state.

#### Scenario: Posting no longer listed

- **WHEN** a successful scan of a source no longer lists an offer, and no other successfully scanned source lists it
- **THEN** the offer becomes `disappeared`

#### Scenario: Posting listed again

- **WHEN** a later scan lists a `disappeared` offer
- **THEN** the offer becomes `active`

#### Scenario: Description edited by the employer

- **WHEN** a scan sees the same uid with a different description
- **THEN** the offer keeps its listing state and a new snapshot is recorded

### Requirement: Description history is kept

Every distinct description observed for an offer SHALL be retained as a snapshot with its capture time and origin. The system MUST NOT delete or coalesce snapshots automatically.

#### Scenario: Several revisions across scans

- **WHEN** an offer's description differs on three consecutive scans
- **THEN** three snapshots exist alongside the original one

### Requirement: Company display changes do not affect offers

Renaming a company, or the source reporting a different company name, MUST NOT create, duplicate or disappear offers.

#### Scenario: Board changes the reported company name

- **WHEN** a source starts reporting a different company name for postings it already listed
- **THEN** the existing offers keep their identity and listing state

### Requirement: Existing offers are re-attributed

When a source's adapter starts reporting an employer for offers already stored under another company, the system SHALL be able to move those offers to the employer without creating new offers, preserving their events, notes, snapshots and application state. The operation MUST be repeatable without further effect.

#### Scenario: Feed offers move to their employers

- **WHEN** re-attribution runs on a source whose offers were stored under the source's own name
- **THEN** each offer moves to the company the adapter now reports and keeps its history

#### Scenario: Re-attribution runs twice

- **WHEN** re-attribution runs a second time with the same source output
- **THEN** no offer changes

#### Scenario: Offer has no matching sighting

- **WHEN** a stored offer has no `(source, uid)` match in the new source output
- **THEN** it is left untouched

#### Scenario: Moving an offer empties a company

- **WHEN** re-attribution moves every offer away from a company
- **THEN** that company remains visible with no offers and is deleted only by the user

