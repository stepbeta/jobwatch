# Spec Delta

## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Description terms are advanced and explained

The system SHALL place description terms in a dedicated Description tab of the
Settings editor. The system SHALL explain that description terms add weaker
location evidence and do not affect remote policy detection.

#### Scenario: Viewing the description terms section

- **WHEN** the user opens the Description tab of the Settings editor
- **THEN** the system shows description terms and an explanation of their effect on location affinity only
