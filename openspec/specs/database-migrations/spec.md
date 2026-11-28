# database-migrations Specification

## Purpose

Defines how the application applies schema migrations to the single SQLite
file and which safety guarantees protect the user's only copy of it.

## Requirements

### Requirement: Pending migrations are backed up first

Before applying any pending migration to a database that already contains
schema history, the application SHALL write a validated backup next to the
database file and SHALL name it so that it cannot overwrite an existing
file. A fresh database, or one with no schema history, MUST NOT trigger a
backup. A backup failure SHALL abort the migration and leave the database
unchanged.

#### Scenario: Existing database is backed up before a migration

- **WHEN** a database with recorded schema history has a pending migration
- **THEN** a validated backup file exists next to it before the migration runs

#### Scenario: Fresh database is not backed up

- **WHEN** a new empty database file is opened and every migration is pending
- **THEN** no backup file is written

#### Scenario: Backup failure aborts the run

- **WHEN** the pre-migration backup cannot be written or fails validation
- **THEN** the migration does not run and the command reports the backup error

#### Scenario: A retried migration does not overwrite the first backup

- **WHEN** a migration fails after its backup was written and the application is started again
- **THEN** a second backup is written under a new name and the first backup is preserved

### Requirement: Migrations are ordered and transactional

The application SHALL apply migrations in ascending version order, each in
its own transaction together with its recorded version. A database that
records a version unknown to the binary MUST be refused instead of
partially migrated.

#### Scenario: Migration failure leaves no partial version

- **WHEN** a migration statement fails
- **THEN** its transaction rolls back and its version is not recorded

#### Scenario: Database newer than the binary is refused

- **WHEN** the database records a migration version the binary does not ship
- **THEN** startup fails with an explanatory error
