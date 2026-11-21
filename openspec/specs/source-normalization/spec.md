# source-normalization Specification

## Purpose

Defines the contract a source adapter fulfils: it is the only place that knows how a source encodes a posting, and it hands the pipeline values that need no repair.

## Requirements

### Requirement: Adapters emit normalized values

An adapter SHALL emit, for each item, a stable uid, a canonical title, a location, a URL, a description where the source provides one, and company evidence. The pipeline MUST NOT rewrite, split or infer these values.

#### Scenario: Feed prefixes the employer to the title

- **WHEN** a feed reports "EPAM Systems: Senior DevOps Engineer"
- **THEN** the adapter emits company evidence for EPAM Systems and the title "Senior DevOps Engineer"

#### Scenario: Board reports the parts separately

- **WHEN** a job board reports company and title as separate fields
- **THEN** the adapter passes them through unchanged

### Requirement: The uid is stable across scans

An adapter MUST derive the uid from something the source guarantees to keep constant for a posting. Content hashes, titles and URLs that change between scans MUST NOT be used as uid when a stable identifier exists.

#### Scenario: Posting is fetched twice

- **WHEN** the same source is scanned twice without the posting changing
- **THEN** both scans produce the same uid for it

#### Scenario: Description changes between scans

- **WHEN** a posting's description changes but the source keeps its identifier
- **THEN** the uid is unchanged

### Requirement: The pipeline never invents a company

When an adapter supplies no company evidence and the source declares no company, the pipeline MUST NOT substitute the source's own name. The item is reported as incomplete instead.

#### Scenario: Aggregator without company extraction

- **WHEN** a source that lists many employers reports items without company evidence
- **THEN** no company is created from the source name and the items are reported as incomplete

#### Scenario: Single-employer source declares its company

- **WHEN** a source that publishes for one employer declares that employer in its configuration
- **THEN** the adapter's items are attached to it

### Requirement: Bad items do not fail the source

An adapter SHALL report a per-item failure for an item it cannot normalize, and MUST NOT fail the whole source for it. Only a failure that prevents fetching (network, robots.txt, refused access) fails the source.

#### Scenario: One item has no usable title

- **WHEN** a scan fetches ten items and one of them lacks a title
- **THEN** nine items are processed and the tenth is reported as incomplete
