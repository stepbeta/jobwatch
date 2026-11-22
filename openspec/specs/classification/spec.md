# classification Specification

## Purpose

Defines how JobWatch separates offer facts from user search preferences and how it computes title affinity, location affinity and remote policy acceptance at read time.

## Requirements

### Requirement: Offer facts are stored independently of preferences

The system SHALL store title, location, description and remote policy as offer facts. It MUST NOT store title affinity or location affinity as offer facts. The system SHALL derive remote policy from the offer text using a fixed detector.

#### Scenario: Ingesting an offer stores facts only

- **WHEN** an offer is ingested from a source
- **THEN** the system stores its title, location, description and derived remote policy
- **AND** it does not store a title or location affinity score for that offer

#### Scenario: Manual edit refreshes remote policy

- **WHEN** the user changes an offer's location or description
- **THEN** the system derives and stores the offer's remote policy again
- **AND** it does not store title or location affinity scores

### Requirement: Title affinity is computed from the current profile

The system SHALL compute title affinity when an offer is read, using the offer title and the current search profile's title terms. The score SHALL be the sum of the weights of matching title terms. The system MUST NOT use a stored title score.

#### Scenario: Reading an offer with matching title terms

- **WHEN** an offer is read and its title matches title terms in the current profile
- **THEN** the system returns a title affinity equal to the sum of those term weights
- **AND** it returns the matched terms and weights as reasons

#### Scenario: Changing the profile changes title affinity

- **WHEN** the user changes the title terms in the current profile
- **THEN** the next read of an offer computes title affinity from the new terms
- **AND** no reclassification step is required

### Requirement: Location affinity is computed from the current profile

The system SHALL compute location affinity when an offer is read, using the offer location and description and the current profile's location and description terms. The score SHALL be the sum of the weights of matching location and description terms. Description terms SHALL affect geography only and MUST NOT affect remote policy detection.

#### Scenario: Reading an offer with matching location terms

- **WHEN** an offer is read and its location or description matches location or description terms in the current profile
- **THEN** the system returns a location affinity equal to the sum of those term weights
- **AND** it returns the matched terms and weights as reasons

#### Scenario: Description terms do not change remote policy

- **WHEN** an offer's description matches a description term
- **THEN** the location affinity includes that term's weight
- **AND** the offer's remote policy remains the value derived by the fixed detector

### Requirement: Remote policy is a fact and a user preference

The system SHALL derive each offer's remote policy from its text using a fixed detector. The user SHALL select which remote policies are acceptable. The system SHALL report the offer's remote policy and whether the current profile accepts it. It MUST NOT compute a weighted remote score.

#### Scenario: Accepted remote policy

- **WHEN** an offer's derived remote policy is selected in the current profile
- **THEN** the system reports the offer as remote-policy accepted

#### Scenario: Unaccepted remote policy

- **WHEN** an offer's derived remote policy is not selected in the current profile
- **THEN** the system reports the offer as remote-policy not accepted

#### Scenario: Unknown remote policy is accepted by default

- **WHEN** a default profile is created and the user has not changed its remote selection
- **THEN** the unknown remote policy is accepted by default

### Requirement: Remote policy detection is fixed

The system SHALL detect full remote, hybrid, onsite and unknown policies from the offer text using fixed phrase rules. The user MUST NOT edit the detection phrases through the application.

#### Scenario: Full remote detection

- **WHEN** an offer's location or description contains a full-remote phrase
- **THEN** the system derives the full remote policy

#### Scenario: Hybrid detection

- **WHEN** an offer's text contains both remote and onsite phrases, or a hybrid phrase
- **THEN** the system derives the hybrid policy

#### Scenario: Onsite detection

- **WHEN** an offer's text contains an onsite phrase and no remote or hybrid phrase
- **THEN** the system derives the onsite policy

#### Scenario: Unknown detection

- **WHEN** an offer's text contains no remote, hybrid or onsite phrase
- **THEN** the system derives the unknown policy

### Requirement: Score reasons are available for title and location affinity

The system SHALL return the matched terms and weights that produced each title and location affinity score. The reasons SHALL be computed from the current profile and MUST NOT be read from a stored field.

#### Scenario: Explaining a title affinity

- **WHEN** an offer's title affinity is returned
- **THEN** the response includes the title terms and weights that matched the offer title

#### Scenario: Explaining a location affinity

- **WHEN** an offer's location affinity is returned
- **THEN** the response includes the location and description terms and weights that matched the offer

### Requirement: Stored affinity scores are not used

The system SHALL ignore any legacy stored title or location score fields. It MUST NOT expose them in offer responses and MUST NOT use them for filtering, sorting or display.

#### Scenario: Legacy score fields are ignored

- **WHEN** an offer is read
- **THEN** the response does not include legacy title or location score fields
- **AND** the system computes title and location affinity from the current profile

### Requirement: Profile changes take effect on the next read

The system SHALL use the current search profile whenever title or location affinity is computed. A change to the profile SHALL affect the next read without a separate reclassification command.

#### Scenario: Profile change applies immediately

- **WHEN** the user saves a changed search profile
- **THEN** the next offer read computes title and location affinity from the changed profile
- **AND** no reclassification command is required

### Requirement: Offer queries use computed affinity

The system SHALL allow offers to be filtered and sorted by title affinity and location affinity. It SHALL apply those filters and sorts to the affinity computed from the current profile. It MUST NOT use a stored score for those operations.

#### Scenario: Filtering by title affinity

- **WHEN** the user filters offers by a minimum title affinity
- **THEN** the system returns only offers whose computed title affinity meets the minimum

#### Scenario: Sorting by location affinity

- **WHEN** the user sorts offers by location affinity
- **THEN** the system orders offers by their computed location affinity
