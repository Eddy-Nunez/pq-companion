## ADDED Requirements

### Requirement: Every table in both databases has a schema whose fields match the live table

The application SHALL define an Ecto schema for each of the game-data tables and
each of the user-data tables, and each schema's declared fields SHALL match the
columns that actually exist in the database.

#### Scenario: Game-data schemas match

- **WHEN** the test suite runs against the game database
- **THEN** every schema's declared fields equal the columns reported by the
  database's table metadata, ignoring ordering

#### Scenario: User-data schemas match

- **WHEN** the test suite runs against a user database
- **THEN** every user-data schema's declared fields equal the columns reported by
  the database's table metadata, ignoring ordering

#### Scenario: Drift fails the suite

- **WHEN** a column exists in the database but not in the schema, or vice versa
- **THEN** the parity test fails and names the table and the differing column

### Requirement: Serialized columns keep their on-disk shape

Where the reference stored structured data in a text column, the application
SHALL expose it as a validated type while preserving the serialized form
byte-for-byte, so existing rows remain readable by the reference application and
by existing export bundles.

#### Scenario: Existing rows decode

- **WHEN** a row written by the reference application is read through a typed
  schema
- **THEN** it decodes into the structured representation without error

#### Scenario: Round-trip is lossless

- **WHEN** a structured value is read and written back unchanged
- **THEN** the stored text is equivalent to the original

#### Scenario: Malformed data fails loudly

- **WHEN** a serialized column contains data that does not match its expected
  shape
- **THEN** reading it raises an error naming the table, the column and the row
  identity, rather than returning a default or a partially-populated value

#### Scenario: The reference application can still read the file

- **WHEN** the migrated application writes a row with a serialized column and the
  reference application then reads it
- **THEN** the reference application parses it successfully

### Requirement: Character identity is usable by name as well as by identifier

The application SHALL support looking up and keying character-scoped data by
character name, because several tables reference characters by name rather than
by the integer identifier and the log pipeline identifies characters by name.

#### Scenario: Name-keyed tables resolve

- **WHEN** character-scoped data stored against a character name is read
- **THEN** it resolves to the character without requiring an identifier

#### Scenario: A rename does not silently orphan data

- **WHEN** a character's stored name-keyed rows exist and the character is
  accessed by identifier
- **THEN** the application does not present that data as absent without
  reporting the mismatch

### Requirement: An existing user database is adopted in place, with a backup taken first

The application SHALL provide a command that adopts an existing user database in
place. Before making any change to the file it SHALL create a timestamped backup
copy, and it SHALL report what it did.

#### Scenario: Backup precedes modification

- **WHEN** the adoption command runs against an existing user database
- **THEN** a backup copy exists before any migration statement executes

#### Scenario: Data is preserved

- **WHEN** the adoption command completes against a populated user database
- **THEN** every row count is unchanged from before adoption

#### Scenario: Idempotent

- **WHEN** the adoption command runs a second time against an already-adopted
  database
- **THEN** it reports no pending work and changes nothing

#### Scenario: A fresh database converges to the same schema

- **WHEN** the schema is created on an empty file and the adoption command then
  runs
- **THEN** the resulting schema is identical to an adopted database's

### Requirement: Adoption fails loudly on schema drift

The adoption command SHALL refuse to proceed when a table's columns do not match
the application's expectations, identifying every discrepancy. It SHALL NOT
silently ignore unknown columns, and it SHALL report tables that exist in the
database but have no schema.

#### Scenario: Unknown column blocks adoption

- **WHEN** a table contains a column the application does not know about
- **THEN** adoption fails, names the table and column, and does not modify the
  database

#### Scenario: Missing column blocks adoption

- **WHEN** a table is missing a column the application requires
- **THEN** adoption fails and names the table and column

#### Scenario: Schema-less tables are reported

- **WHEN** the database contains a table with no corresponding schema
- **THEN** adoption reports it by name as an adopted-but-unmapped table without
  treating it as an error, so dead tables are visible rather than invisible

#### Scenario: Failure is recoverable

- **WHEN** adoption fails after taking a backup
- **THEN** the original database is unchanged and can be restored from the backup

### Requirement: The raw-code to label catalog is defined exactly once

The application SHALL define the mapping from game data raw codes to human
readable labels in exactly one module, and SHALL NOT maintain a second copy for
any consumer.

#### Scenario: One definition

- **WHEN** the test suite runs
- **THEN** a guard test asserts no second definition of the catalog exists

#### Scenario: Unknown codes degrade predictably

- **WHEN** a code is not present in the catalog
- **THEN** the label helper returns a deterministic placeholder rather than
  raising or returning an empty string

### Requirement: Ported game-math behavior matches the reference

The application SHALL reproduce the reference's computed game values — spell
duration and level scaling, buff effects, haste and instrument modifiers, special
ability labels, duplicate-named entity resolution, resist and charm calculations,
and upgrade scoring — for the reference's own test inputs.

#### Scenario: Ported cases pass

- **WHEN** the reference's table-driven cases for a ported function are run
  against the ported implementation
- **THEN** every case produces the reference's expected value

#### Scenario: Migration from Go is mechanical, not interpretive

- **WHEN** a ported function's cases are compared with the reference test file
- **THEN** a test asserts the case inventory is accounted for, so cases cannot be
  silently dropped during the port

### Requirement: The game database remains read-only after schema work

The read-only guarantee established for the game database SHALL continue to hold
once full schemas exist for its tables.

#### Scenario: Schema-backed writes still raise

- **WHEN** a write is attempted through a game-data schema
- **THEN** it raises an error identifying the repository as read-only
