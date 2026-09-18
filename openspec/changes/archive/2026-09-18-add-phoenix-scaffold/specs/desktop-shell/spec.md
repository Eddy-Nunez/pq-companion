## Purpose

Defines the contract between the LiveView application and whatever native
desktop shell hosts its windows, so the shell can be swapped without changing
application behavior, and provides a browser adapter that satisfies the contract
during development.

## ADDED Requirements

### Requirement: A window's native properties are declared by the application, not the shell

Each window SHALL declare the native properties it requires — stable id, route,
session token, title, transparency, always-on-top, click-through, frameless,
resizable, display-only, bounds, and zoom — in a form the shell can read from the
rendered document without application-specific shell code.

#### Scenario: Overlay window spec is machine-readable

- **WHEN** an overlay route renders
- **THEN** the document contains a single machine-readable element carrying the
  window spec, and the spec parses as JSON with all required keys present

#### Scenario: Main window also declares a spec

- **WHEN** the main-window route renders
- **THEN** its spec declares a non-transparent, non-click-through, resizable
  window so a shell can configure it without special-casing

### Requirement: The shell is replaceable behind a single behaviour

The application SHALL access window operations only through one behaviour, and
SHALL NOT reference any specific shell's API from feature code. Operations
SHALL cover at least: open, close, resize, set click-through, focus, and list.

#### Scenario: Feature code is shell-agnostic

- **WHEN** a feature opens, closes, resizes, or changes click-through on a window
- **THEN** it calls the behaviour, and no feature module names a concrete shell
  implementation

#### Scenario: Adapters are selected by configuration

- **WHEN** the configured adapter changes
- **THEN** the running application uses the new adapter with no change to
  feature code

### Requirement: A browser adapter satisfies the contract in development

The application SHALL ship a browser adapter that accepts every window spec,
ignores the native-only properties it cannot honour, and records the requests it
received so they can be asserted in tests. The browser adapter SHALL be the
default in development.

#### Scenario: Overlay route is reachable without a native shell

- **WHEN** the application runs in development with the browser adapter and an
  overlay route is requested
- **THEN** the route renders successfully and the spec is recorded as received

#### Scenario: Native-only properties are reported as unmet, not silently dropped

- **WHEN** a spec requests transparency or click-through and the browser adapter
  is active
- **THEN** the adapter reports those properties as unmet rather than claiming
  success

### Requirement: Every shell adapter passes one conformance suite

The application SHALL provide a shell conformance suite that runs against any
adapter implementation and asserts the behaviour contract. A new adapter SHALL
NOT be considered complete until the suite passes against it.

#### Scenario: Conformance suite accepts a compliant adapter

- **WHEN** the conformance suite runs against the browser adapter
- **THEN** every assertion passes

#### Scenario: Conformance suite rejects an incomplete adapter

- **WHEN** the conformance suite runs against an adapter that does not implement
  a required operation
- **THEN** the suite fails and names the missing operation

### Requirement: Window property changes are pushed without a page reload

The application SHALL propagate changes to a live window's properties (bounds,
zoom, display-only, click-through, lock) to the shell as incremental updates,
without requiring the window to navigate or reload.

#### Scenario: Display-only toggle applies live

- **WHEN** display-only mode is enabled for an open window
- **THEN** the shell receives an update for that window and applies it without a
  reload

#### Scenario: Moved bounds persist

- **WHEN** the user moves or resizes a window and the shell reports the new
  bounds
- **THEN** the application stores them and re-applies them the next time that
  window opens
