## Purpose

Stores and validates the application's user settings in the reference's YAML
file, through a single writer that saves atomically, applies documented defaults,
and pushes changes to every open window without a reload.

## ADDED Requirements

### Requirement: Settings are defined as a validated schema

The application SHALL define its settings as a typed schema with changeset
validation, and SHALL reject invalid values with field-level errors rather than
persisting them.

#### Scenario: Invalid value is rejected

- **WHEN** a settings update supplies a value outside its permitted range or type
- **THEN** the update is rejected, the error names the field, and the persisted
  settings are unchanged

#### Scenario: Valid update is accepted

- **WHEN** a settings update supplies valid values
- **THEN** the update is accepted and persisted

#### Scenario: Unknown keys do not crash loading

- **WHEN** the settings file contains a key the schema does not define
- **THEN** loading succeeds and the unknown key is preserved rather than dropped

### Requirement: Settings file format and key names are unchanged

The application SHALL read and write the settings file using the same key names
and nested structure the reference application uses, so the file remains valid
for the reference application and for rollback.

#### Scenario: Reference settings file loads

- **WHEN** the application loads a settings file written by the reference
  application
- **THEN** every setting is read with the reference's meaning

#### Scenario: Written file is reference-readable

- **WHEN** the application saves settings
- **THEN** the reference application can load the resulting file and its own
  settings are unchanged

#### Scenario: Unchanged values round-trip byte-identically

- **WHEN** the application loads settings and saves them without modification
- **THEN** every value is preserved exactly

### Requirement: Defaults match the reference application

The application SHALL apply the reference's default value for every setting that
is absent from the file, so a fresh install and an upgraded install behave
identically to the reference in the same situation.

#### Scenario: Fresh install uses reference defaults

- **WHEN** the application starts with no settings file
- **THEN** every setting resolves to the reference's default

#### Scenario: Missing keys fall back individually

- **WHEN** the settings file defines some keys but not others
- **THEN** the defined keys are used and each absent key resolves to its
  reference default

### Requirement: A single writer owns settings

The application SHALL serialize all settings writes through one owner process, so
that concurrent updates cannot interleave and produce a partially-applied or
truncated file.

#### Scenario: Concurrent updates do not corrupt the file

- **WHEN** multiple updates are issued concurrently
- **THEN** the file always parses, and its contents equal one of the accepted
  update sequences

#### Scenario: A reader never observes a partial file

- **WHEN** settings are being saved while another process reads them
- **THEN** the reader observes either the previous or the new complete settings,
  never a partial one

### Requirement: Settings are saved atomically

A settings save SHALL write to a temporary location and replace the settings file
in a single step, so that an interrupted save cannot leave a truncated or empty
settings file.

#### Scenario: Interrupted save leaves the previous settings intact

- **WHEN** a save is interrupted before the replacement step
- **THEN** the settings file still parses and contains the previous values

#### Scenario: No temporary file is left behind after success

- **WHEN** a save completes
- **THEN** no temporary artifact remains beside the settings file

### Requirement: Settings changes propagate to every open window without a reload

When settings change, the application SHALL notify every open window so each
reflects the new values without a reload or a manual refresh.

#### Scenario: Change is visible everywhere

- **WHEN** a setting is changed in one window
- **THEN** every other open window reflects the new value without being reloaded

#### Scenario: A window opened later sees current settings

- **WHEN** a window opens after a settings change
- **THEN** it renders the current values

#### Scenario: Origins are distinguishable

- **WHEN** the settings-change notification is delivered
- **THEN** a subscriber can tell a change made by another window apart from an
  update it initiated itself, so it can avoid redundant work

### Requirement: External edits to the settings file are picked up

When the settings file is modified outside the application, the application SHALL
detect the change, reload it, and propagate it to open windows.

#### Scenario: External edit is applied

- **WHEN** the settings file is edited by another program
- **THEN** the application reloads it and open windows reflect the new values

#### Scenario: An externally invalid file does not take the application down

- **WHEN** the settings file is edited to an invalid state
- **THEN** the application reports the problem, retains the last valid settings in
  memory, and continues running

#### Scenario: The application's own save does not trigger a reload loop

- **WHEN** the application saves the settings file
- **THEN** its own change-detection does not treat that write as an external edit
