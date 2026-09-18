## Context

See proposal.md — Why. Current state that shapes this design:

- The frozen reference is a three-process app: a Go sidecar (HTTP + WebSocket),
  an Electron main process (16 transparent overlay `BrowserWindow`s, tray,
  auto-update) and a React renderer. See `docs/phoenix-migration-plan.md` §1.1.
- Windows-only target, same machine as the game. Overlay windows must be
  transparent, always-on-top and click-through over a DirectX game, which a
  browser cannot do (`docs/phoenix-migration-plan.md` §1.2).
- `quarm.db` (~85 MB, 15 tables) is shipped read-only; `user.db` (50 tables)
  holds real user data and has no migration-version table.
- The Electron-vs-Tauri shell decision is **deliberately deferred**, so this
  change must not depend on either.

## Goals / Non-Goals

**Goals:**

- A running Phoenix LiveView app with the reference app's three window classes.
- A window contract expressive enough that either Electron or Tauri can be
  implemented against it later without changing application code.
- Both databases open, with the game database provably write-protected.
- `config.yaml` readable and writable in place, format unchanged.
- A dev workflow that needs no native shell, and CI that gates.
- Settle the two unverified platform assumptions (Windows SQLite NIF, Windows
  file watching) early rather than in Wave 3.

**Non-Goals:**

- The full schema set and `user.db` adoption tooling (Wave 2).
- Sidebar navigation (Wave 1).
- Creating any native window (Wave 8).
- Packaging, installer, auto-update (Wave 11).
- Any feature behavior: no log parsing, no game logic, no overlays rendering
  real data.
- Porting `internal/converter` or `internal/mapgen` — build-time tooling, stays
  in Go.

## Decisions

### D1. Single OTP app, not an umbrella

**Decision:** one app at `phoenix/`, with bounded contexts as namespaced modules
(`PQ.Log`, `PQ.Game`, `PQ.Character`, …).

**Why:** the reference's 61 packages map onto ~12 contexts. A single app keeps
`mix release` simple, and releases are already the riskiest platform surface on
Windows (plan §1.4). Umbrella buys compile-time boundary enforcement we do not
need yet.

**Alternative considered:** an umbrella with `pq_game_db` / `pq_user_db` /
`pq_log` / `pq_web`. The enforced boundaries are attractive at this codebase
size, but releases and dependency declarations get noisier, and the boundary
discipline is already documented in the capability tree. Revisit if compile times
or boundary drift become real problems.

### D2. Two Ecto repos, not one repo with two connections

**Decision:** `PQ.QuarmRepo` and `PQ.UserRepo` as separate `Ecto.Repo`s.

**Why:** the read-only guarantee must be structural, not conventional. A single
repo with two connection configs makes it possible for a developer to `insert`
into a game-data schema through the user connection. Two repos let us set
`read_only: true` at the connection level *and* wrap the game repo so only read
operations are exposed, with a test asserting a write raises.

**Alternative considered:** one repo, plus a code-review convention that game
schemas are never written. Rejected — the reference already relies on convention
here, and a clean-room rewrite is the moment to make it structural.

### D3. Window specs travel in the document, not over IPC or URL parameters

**Decision:** the overlay layout renders `<meta name="pq-window" content=...>`
with a JSON spec; the shell's preload script reads it on load and forwards it to
the shell's main process. Property *changes* go over the existing LiveView event
channel via `push_event`/`handleEvent`.

**Why:** the LiveView server is authoritative about *what* a window is; the shell
is authoritative about *how* to realise it. Putting the spec in the document
means a window is fully specified the moment it loads — no ordering problem where
a shell configures a window before the app knows what it wants, and no
shell-specific API in application code. It also makes every window testable in a
browser: the spec is just markup.

**Alternatives considered:**
- *IPC request/response on window creation* — requires shell-specific code paths
  and a boot ordering handshake; harder to test without a shell.
- *URL parameters* (`?transparent=1&clickThrough=1`) — no structured place for
  bounds/zoom, and it grows unboundedly as overlay options are added.
- *Two-way IPC for the whole spec* — the app would need to know it is not in a
  browser to avoid hanging. The meta tag degrades to a no-op instead.

### D4. Browser adapter ships first and is the development default

**Decision:** `PQ.Shell.Browser` is implemented in this change and is the default
adapter in development.

**Why:** it removes the deferred shell decision as a blocker for Waves 1–7 and
makes every overlay route reachable at a plain URL. It also forces the
conformance suite to exist early, which is what makes the later adapter
implementation a bounded task rather than an open-ended one.

**Trade-off:** the browser adapter reports transparency and click-through as
unmet rather than pretending. That is intentional — a silent no-op would let
overlay bugs hide until Wave 8.

### D5. Three layouts, not one layout with conditionals

**Decision:** `:root`, `:overlay`, `:bare` as distinct Phoenix layouts.

**Why:** the reference encodes the distinction by rendering a different React
tree (`MainWindowLayout` vs `OverlayPage`), and the main window's alert hooks are
mounted there. Making it a layout property keeps the alert hooks structurally
absent from overlay windows rather than guarded at runtime.

**Alternative considered:** one layout with a `window_class` assign and
conditionals. Rejected — it makes it possible to render an overlay with alert
hooks by passing the wrong assign, which is the exact class of bug that produces
duplicate TTS.

### D6. Runtime discovery via a file plus stdout line

**Decision:** write `~/.pq-companion/runtime.json` (port, pid, version) after the
listener binds, and print the same record as one stdout line.

**Why:** the reference already spawns its sidecar with piped stdio and needs port
resilience when the preferred port is taken; a native shell has to learn the
actual port somehow. Two channels means a shell can use whichever fits: file for
a shell that starts the app detached, stdout for one that spawns and supervises
it.

**Alternative considered:** a fixed port. Rejected — the reference explicitly
hardened against port conflicts, and that behavior is preserved.

### D7. No client-side framework; esbuild for the exceptions

**Decision:** no React, no LiveView JS framework wrapper. esbuild builds only the
small hooks that genuinely need the DOM.

**Why:** it deletes `services/api.ts` (3,387 LOC), the WebSocket singleton, and
the Go↔TS duplicated type layer — the single largest structural win of the
migration (plan §1.3). The known exceptions are the two graph pages (xyflow +
dagre) and drag-reorder; neither is in this change's scope.

## Risks / Trade-offs

- **[Exqlite has no usable Windows NIF build]** → Acceptance criterion 3 settles
  this in the first session; if it fails, `ecto_sqlite3` is replaced before any
  schema work begins, when the cost is lowest.
- **[`file_system` does not work on Windows]** → Fall back to a poll-based
  watcher GenServer. Only affects Wave 3, and the tailer needs a poll loop anyway
  to handle log rotation.
- **[The meta-tag delivery mechanism turns out to be insufficient for Tauri]** →
  Tauri's webview exposes a similar preload/IPC boundary, so the same scrape
  works; if it does not, the contract (not the transport) is what the specs
  guarantee, and only the adapter changes.
- **[Speculative generality: building a shell contract before choosing a shell]** →
  Mitigated by keeping the contract to operations the reference app already
  performs (open, close, resize, click-through, focus, list, bounds, zoom). If
  the chosen shell cannot express one of them, that is a finding about the shell,
  which is exactly what the conformance suite is for.
- **[Structural read-only enforcement makes legitimate maintenance harder]** —
  e.g. the release workflow regenerating `quarm.db`. → The guard is on the
  application's runtime repo, not on the file; tooling continues to write the
  artifact.

## Migration Plan

1. Land this change with `PQ.Shell.Browser` as the only adapter. The main window
   and all 16 overlay routes are reachable in a browser.
2. Wave 1 (sidebar) builds on `app-shell` and `navigation`.
3. Wave 2 extends `data-store` and adds `settings`.
4. Wave 8 implements the chosen adapter against the frozen contract and runs the
   conformance suite. No feature code changes.

**Rollback:** nothing in this change touches the reference tree or user data. The
reference app remains the shipping build throughout; rollback is deleting
`phoenix/`. The only shared surface is `~/.pq-companion`, and this change writes
only `runtime.json` (new) and reads `config.yaml`/`user.db`.

## Open Questions

- Should the runtime record be refreshed on port change, or is write-once at boot
  sufficient? Deferrable — the app does not currently rebind after startup.
- Is `runtime.json` the right location given `config.yaml` lives beside it, or
  should runtime state move to a temp/run directory? Deferrable; it does not
  affect the contract, only the path.
