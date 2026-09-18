# dev-toolchain Specification

## Purpose
Provides a reproducible Elixir development workflow that requires no native
desktop shell, and a continuous-integration job that gates every push and pull
request for the migrated application.

## Requirements

### Requirement: Local development requires no native shell

A developer SHALL be able to run the full application — main window and every
overlay route — with a single command and no native desktop shell installed, and
SHALL get live reload on source changes.

#### Scenario: One-command startup

- **WHEN** a developer runs the documented dev command on a clean checkout with
  dependencies fetched
- **THEN** the server starts and the main window is reachable in a browser

#### Scenario: Every overlay route is reachable in a browser

- **WHEN** each overlay route is requested in development
- **THEN** every route renders without a native shell present

#### Scenario: Live reload

- **WHEN** a source file is edited while the dev server runs
- **THEN** the change is reflected without a manual restart

### Requirement: CI gates every push and pull request

The repository SHALL run the migrated application's test suite and formatting
check on every push and pull request, and SHALL fail the build when either fails.

#### Scenario: Tests run and gate

- **WHEN** a push or pull request touches the migrated application
- **THEN** the CI job fetches dependencies, compiles, and runs the test suite,
  and a failing test fails the job

#### Scenario: Formatting is enforced

- **WHEN** a file does not conform to the project formatter
- **THEN** the CI job fails and identifies the file

### Requirement: CI provisions the game database for data-backed tests

The CI job SHALL attempt to download the shipped game database artifact before
running tests, and SHALL run data-backed tests when it is present. When the
artifact is unavailable the platform SHALL clearly mark the affected tests as
skipped rather than reporting them as passing.

#### Scenario: Database present

- **WHEN** the game database artifact downloads successfully
- **THEN** data-backed tests execute

#### Scenario: Database absent

- **WHEN** the game database artifact cannot be downloaded
- **THEN** the job emits an explicit warning and data-backed tests skip, without
  the job reporting them as passing

### Requirement: Reference toolchains keep running during the migration

The Go and TypeScript CI jobs SHALL continue to run while the reference trees
exist, so the app being replaced stays verifiable, and SHALL be removed together
with the reference trees.

#### Scenario: Reference jobs still gate

- **WHEN** the CI workflow runs during the migration
- **THEN** the Go test job and the TypeScript typecheck job both still execute

#### Scenario: Toolchain versions are pinned in one place

- **WHEN** CI and a developer resolve the Elixir and Erlang versions
- **THEN** both use the same pinned versions from the repository's version file
