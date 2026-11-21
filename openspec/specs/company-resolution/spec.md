# company-resolution Specification

## Purpose

Decides which company an offer belongs to, using identifiers and names a source attests to, and remembers confirmed equivalences so later scans resolve the same way.

## Requirements

### Requirement: Companies are identified by an attested identifier

A company SHALL be resolved by the identifier its source already uses: a job-board token, an organisation slug, a site name, an account, a subdomain, a tenant, or a company page slug. A resolution by identifier MUST NOT depend on the company's display name.

#### Scenario: Identifier seen before

- **WHEN** a scan reports an identifier already linked to a company
- **THEN** the offer is attached to that company regardless of the reported company name

#### Scenario: Identifier seen for the first time

- **WHEN** a scan reports an identifier that is not linked to any company
- **THEN** a company is created and the identifier is linked to it

### Requirement: Company names are not unique

Company display names SHALL NOT be required to be unique; several companies MAY share one. Name is an attribute of a company, not its identity, and a name search MAY return more than one company.

#### Scenario: Two employers share a name

- **WHEN** two companies have the same name and different identifiers
- **THEN** each resolves to its own company and a name search returns both

### Requirement: Name-only resolution is scoped to its source

When a source reports only a name, resolution SHALL consult the names recorded for that source first, then names confirmed for all sources, and otherwise create a new company. The resolution MUST NOT consult the source's own name.

#### Scenario: Feed repeats a known name

- **WHEN** a source reports a name it has resolved to a company before
- **THEN** the offer is attached to that company

#### Scenario: Source reports nothing usable

- **WHEN** a source reports neither an identifier nor a name
- **THEN** no company is invented and the item is reported as incomplete

### Requirement: A confirmed merge is remembered

When the user merges company A into company B, the identifiers and names of A SHALL afterwards resolve to B. Later scans MUST reach the same conclusion without asking again.

#### Scenario: Absorbed name appears in a later scan

- **WHEN** a source reports a name that belonged to a company absorbed by an earlier merge
- **THEN** the offer is attached to the absorbing company and no new company is created

### Requirement: Companies are never merged automatically

The system SHALL merge two existing companies only after explicit user confirmation. Detecting a possible duplicate MAY be offered as a suggestion; it MUST NOT change any data on its own.

#### Scenario: Two companies look like the same employer

- **WHEN** two companies share a name or a similar one
- **THEN** the system may propose a merge and changes nothing until the user confirms

### Requirement: A merge never leaves a ghost company

After a confirmed merge, the absorbed company SHALL no longer exist as a target of resolution, and its offers, notes and provenance SHALL belong to the absorbing company.

#### Scenario: Merged company is not recreated

- **WHEN** the next scan reports the absorbed company's identifier or name
- **THEN** no company row is created for it
