# search-profiles Specification

## Purpose

Defines how JobWatch stores user search profiles, how term weights and remote policy preferences are chosen, and how profiles are seeded, validated and selected for scoring.

## Requirements

### Requirement: Search profiles are persisted

The system SHALL store search profiles. Each profile SHALL contain a name, title terms, location terms, optional description terms and a remote policy selection. The system SHALL support more than one profile in storage.

#### Scenario: Reading a profile

- **WHEN** the user opens a search profile
- **THEN** the system returns its name, title terms, location terms, description terms and remote policy selection

#### Scenario: Multiple profiles in storage

- **WHEN** more than one search profile exists
- **THEN** the system can list and select each profile by its identifier

### Requirement: A default profile is seeded from the current hardcoded decisions

The system SHALL create a default search profile during migration. The default profile SHALL contain the title, location and description terms and weights currently hardcoded in the classifier. It SHALL select the remote policies that match the current target, with unknown accepted.

#### Scenario: Default profile preserves current terms

- **WHEN** the migration creates the default profile
- **THEN** its title, location and description terms and weights match the current hardcoded classifier tables

#### Scenario: Default remote selection

- **WHEN** the migration creates the default profile
- **THEN** full remote and unknown policies are accepted
- **AND** hybrid and onsite policies are not accepted

### Requirement: Profile editor is organized as tabs

The Settings page SHALL present the search profile editor as tabs: Profile,
Title terms, Location terms, Description and Remote. Switching tabs SHALL
preserve unsaved edits, and one Save SHALL apply the whole profile. The active
tab SHALL be part of the page URL.

#### Scenario: Switching tabs preserves edits

- **WHEN** the user edits a title term and switches to the Remote tab
- **THEN** the title term edit is still present when the user returns

#### Scenario: One save applies every tab

- **WHEN** the user changes a term and a remote policy, then saves
- **THEN** both changes are stored in the same profile update

#### Scenario: Validation reveals a hidden tab

- **WHEN** the user saves while a field on another tab is invalid
- **THEN** the editor switches to the tab containing the field
- **AND** the error is shown there

#### Scenario: Active tab survives a reload

- **WHEN** the Settings URL names a tab
- **THEN** the editor opens on that tab

### Requirement: Term weights use selectable buckets

The system SHALL offer three or five weight buckets for title and location terms. The buckets SHALL map to numeric weights. The system SHALL allow advanced users to enter numeric weights. Title terms and location or description terms SHALL use separate bucket scales.

#### Scenario: Choosing a five-bucket scale

- **WHEN** the user selects the five-bucket scale
- **THEN** the system offers strong positive, mild positive, neutral, mild negative and strong negative buckets

#### Scenario: Choosing a three-bucket scale

- **WHEN** the user selects the three-bucket scale
- **THEN** the system offers positive, neutral and negative buckets

#### Scenario: Entering a numeric weight

- **WHEN** the user opens the advanced weight option for a term
- **THEN** the system accepts a numeric weight for that term

### Requirement: Description terms are advanced and explained

The system SHALL place description terms in a dedicated Description tab of the
Settings editor. The system SHALL explain that description terms add weaker
location evidence and do not affect remote policy detection.

#### Scenario: Viewing the description terms section

- **WHEN** the user opens the Description tab of the Settings editor
- **THEN** the system shows description terms and an explanation of their effect on location affinity only

### Requirement: Remote policy is selected with checkboxes

The system SHALL present remote policies as a multi-select checkbox set. The system SHALL allow selecting full remote, hybrid, onsite and unknown. The system MUST NOT assign numeric weights to remote policies.

#### Scenario: Selecting accepted policies

- **WHEN** the user checks full remote and unknown
- **THEN** the profile accepts offers with full remote or unknown policy
- **AND** it does not accept offers with hybrid or onsite policy

### Requirement: The profile editor explains term effects

The system SHALL provide concise explanatory text for title terms, location terms, description terms and remote policy selection. The text SHALL state that positive terms raise affinity, negative terms lower it, neutral terms only appear in reasons, and remote policy is a membership check.

#### Scenario: Reading the title terms explanation

- **WHEN** the user opens the title terms section
- **THEN** the system explains that positive terms raise title affinity and negative terms lower it

#### Scenario: Reading the remote policy explanation

- **WHEN** the user opens the remote policy section
- **THEN** the system explains that the policy is detected from the offer text and the user selects which policies are acceptable

### Requirement: Profile input is validated

The system SHALL reject empty terms and invalid numeric weights. It SHALL keep the profile unchanged when validation fails.

#### Scenario: Rejecting an empty term

- **WHEN** the user saves a profile containing an empty term
- **THEN** the system rejects the save and reports the invalid term

#### Scenario: Rejecting an invalid weight

- **WHEN** the user saves a profile with a non-numeric weight
- **THEN** the system rejects the save and reports the invalid weight

### Requirement: A profile can be selected for offer scoring

The system SHALL allow an offer query to use a selected search profile. When no profile is selected, the system SHALL use the default profile.

#### Scenario: Using the default profile

- **WHEN** an offer query does not specify a profile
- **THEN** the system computes affinity using the default profile

#### Scenario: Using a selected profile

- **WHEN** an offer query specifies a profile
- **THEN** the system computes affinity using that profile
