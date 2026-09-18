## Purpose

Boots the PQ Companion web application, routes its three classes of window
(main, overlay, bare) through distinct layouts, and announces its runtime
address so a native desktop shell can attach to it.

## ADDED Requirements

### Requirement: Serve the main window as a LiveView over a persistent connection

The application SHALL serve the main window as a `Phoenix.LiveView` mounted by
`phoenix_live_view`'s socket, such that stateful UI updates travel over the
LiveView WebSocket and no client-side API layer is required.

#### Scenario: Boot and connect

- **WHEN** the application starts and a client requests the root route
- **THEN** the server returns an HTML document that establishes a LiveView
  WebSocket connection

#### Scenario: Server-authoritative re-render

- **WHEN** server-side state changes and the LiveView assigns it
- **THEN** the client receives and applies a diff without a full page reload or
  an application-authored fetch call

### Requirement: Render each window class through a distinct layout

The application SHALL select one of three layouts per route: `:root` for the
main window (sidebar, titlebar, content), `:overlay` for overlay windows (no
chrome, transparent background), and `:bare` for onboarding and modal surfaces
(no sidebar).

#### Scenario: Main window uses the root layout

- **WHEN** the main-window route renders
- **THEN** the response includes the sidebar and titlebar regions

#### Scenario: Overlay window uses the overlay layout

- **WHEN** an overlay route renders
- **THEN** the response omits the sidebar and titlebar regions and applies the
  transparent-background class to the document body

#### Scenario: Bare route omits navigation

- **WHEN** a `:bare` route renders
- **THEN** the response contains no navigation region

### Requirement: Overlay windows never run main-window alert hooks

Overlay windows SHALL NOT mount the audio, text-to-speech, trigger-alert,
timer-alert, respawn-alert or metronome hooks, so that a single game event
cannot produce duplicate alerts from more than one renderer.

#### Scenario: Overlay mount is alert-free

- **WHEN** an overlay route mounts
- **THEN** no process registers as an audio owner and no alert hook is attached

#### Scenario: Main window mounts the alert hooks exactly once

- **WHEN** the main-window route mounts
- **THEN** exactly one audio owner is registered for that client

### Requirement: Announce the runtime address for a native shell to consume

On boot the application SHALL write a machine-readable runtime record containing
at least the bound port, the OS process id, and the app version, and SHALL also
emit the same record as a single line on standard output. The record SHALL be
written after the listener is bound, and SHALL reflect the actually-bound port
when the configured port was unavailable.

#### Scenario: Record reflects the bound port

- **WHEN** the configured port is already in use and the OS assigns a different
  port
- **THEN** the announced record contains the port the listener actually bound

#### Scenario: Native shell can discover the app

- **WHEN** a native shell reads the runtime record after spawning the
  application
- **THEN** it can construct a URL that reaches the main window

#### Scenario: Stale record is not left behind

- **WHEN** the application shuts down
- **THEN** the runtime record is removed or marked stale
