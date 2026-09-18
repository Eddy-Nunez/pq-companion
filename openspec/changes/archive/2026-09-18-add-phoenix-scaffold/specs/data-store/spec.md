## Purpose

Opens PQ Companion's two SQLite databases at boot as independently configured
Ecto repositories, guaranteeing the shipped game database can never be mutated
while user data is edited in place.

## ADDED Requirements

### Requirement: User data is opened in place and remains writable

The application SHALL open the existing user database at the user's application
home directory as a read-write Ecto repository, and SHALL accept reads and writes
against it without requiring a schema migration to be applied first.

#### Scenario: Existing install is usable immediately

- **WHEN** the application boots against a user database created by the
  reference (Go) application
- **THEN** a read returns rows and a write commits and is visible on a
  subsequent read

#### Scenario: Writes are durable across restart

- **WHEN** a value is written and the application restarts
- **THEN** the value is still present

### Requirement: Game data is read-only and writes are rejected

The application SHALL open the shipped game database read-only, and SHALL reject
any attempt to write to it. Rejection SHALL occur before the statement reaches
SQLite, and SHALL raise rather than silently no-op.

#### Scenario: Reads succeed

- **WHEN** the application queries items, spells, NPC types, or zones
- **THEN** rows are returned

#### Scenario: Writes raise

- **WHEN** an insert, update or delete is attempted against the game database
  repository
- **THEN** it raises an error identifying the repository as read-only

#### Scenario: Guard is enforced by test

- **WHEN** the test suite runs
- **THEN** a test asserts that a write against the game database raises

### Requirement: Both repositories are supervised and available before serving

Both repositories SHALL be started as part of the application supervision tree,
so that any request handler or LiveView can assume them without a readiness
check.

#### Scenario: Boot order is deterministic

- **WHEN** the application starts
- **THEN** both repositories report healthy before the HTTP listener accepts
  traffic

#### Scenario: Repository failure aborts boot

- **WHEN** either database cannot be opened
- **THEN** the application fails to start and the failure names the database
  path

### Requirement: The on-disk footprint matches the reference application

The application SHALL resolve its user-data locations to the same paths the
reference application uses, so an existing install is found without migration:
the application home directory, the user database, the settings file, the backups
directory, and the server log.

#### Scenario: Existing install is discovered

- **WHEN** the application boots on a machine that has an existing reference
  install
- **THEN** it resolves and opens the existing user database and settings file
  rather than creating new empty ones

#### Scenario: Paths are resolved in one place

- **WHEN** the test suite runs
- **THEN** a test asserts each of the five resolved paths against the expected
  reference locations

#### Scenario: A fresh machine gets a valid footprint

- **WHEN** the application boots with no existing user data
- **THEN** it creates the directories it needs and starts successfully

### Requirement: A missing or unreadable game database produces an actionable error

When the game database is absent or unreadable, the application SHALL fail with
an error that names the expected path and states where to obtain the file, rather
than starting into a degraded state or returning an empty result set.

#### Scenario: Absent game database

- **WHEN** the game database file does not exist at the configured path
- **THEN** startup fails and the error message contains the expected path and
  the instruction to download the artifact

#### Scenario: Empty result sets are not used to mask a missing database

- **WHEN** the game database is missing
- **THEN** no query returns an empty result set in place of the error
