# incomplete-listings Specification

## Purpose

Keeps postings a source could not normalize, marks them as incomplete, and lets the user repair them so no data is lost and no wrong data is created.

## Requirements

### Requirement: Incomplete items are retained

An item a source cannot normalize SHALL be stored with the values the source reported, its source, its uid or URL, and the reason it is incomplete. It MUST NOT be dropped, and it MUST NOT be turned into a guessed offer.

#### Scenario: Item reported without a company

- **WHEN** a scan reports an item whose company cannot be determined
- **THEN** the item is stored, marked incomplete, and no company is created

### Requirement: Incomplete items are kept apart from usable offers

Incomplete items SHALL be excluded from the offers list, the dashboard and search results, and SHALL be listed in a dedicated view with a count.

#### Scenario: Browsing offers

- **WHEN** the user opens the offers list
- **THEN** incomplete items are absent from it and the count of incomplete items is visible

### Requirement: The user can repair an incomplete item

The user SHALL be able to assign a company and a title to an incomplete item. After repair it SHALL behave as a normal offer.

#### Scenario: Repairing an item

- **WHEN** the user assigns an existing company and a title to an incomplete item
- **THEN** the item leaves the incomplete view and appears in the offers list

### Requirement: A repair survives later scans

A repaired item MUST NOT be marked incomplete again, and a scan MUST NOT overwrite the company or title the user supplied.

#### Scenario: Source rescanned after a repair

- **WHEN** the source reports the same uid again with the same unnormalized values
- **THEN** the offer keeps the repaired company and title and stays out of the incomplete view

### Requirement: Incomplete items are visible in scan results

Each scan summary SHALL report how many items were stored as incomplete.

#### Scenario: Feed loses its extraction rule

- **WHEN** a scan stores every item of a source as incomplete
- **THEN** the source is reported as succeeded with that count, and the count is visible to the user
