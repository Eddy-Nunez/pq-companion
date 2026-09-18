## Why

PQ Companion is migrating from Go + Electron + React to a single Elixir/Phoenix
LiveView app (`docs/phoenix-migration-plan.md`, **Wave 0**). Nothing can be
ported until there is a running Phoenix app with the same three window classes
the current app has, the same on-disk footprint, and a CI gate.

Wave 0 is deliberately shell-independent and low-risk: it proves the toolchain,
the two-database access pattern, and the config round-trip **before** any
feature porting starts, and it ships `PQ.Shell.Browser` so every route —
including all 16 overlay routes — is reachable as a plain URL. That removes the
deferred Electron-vs-Tauri decision as a blocker for everything except Wave 8.

## What Changes

- Scaffold `phoenix/` as a Phoenix 1.8+ LiveView app with SQLite (`mix phx.new
  --database sqlite3 --no-mailer --no-gettext`), pinned to Erlang OTP 27 /
  Elixir 1.18 via `.tool-versions`.
- Introduce `PQ.Shell` — a behaviour defining the **native window contract**
  (id, route, token, transparency, always-on-top, click-through, frameless,
  resizable, display-only, bounds, zoom) plus a `PQ.Shell.Browser` adapter that
  ignores the native-only fields. The Electron/Tauri adapters are deferred.
- Introduce three Phoenix layouts matching the current app's window classes:
  `:root` (main window, sidebar + titlebar), `:overlay` (transparent, no chrome,
  emits a machine-readable `pq-window` meta tag), `:bare` (wizard/modals).
- Open both databases as separate Ecto repos: `PQ.UserRepo` (`~/.pq-companion/user.db`,
  read-write) and `PQ.QuarmRepo` (`priv/data/quarm.db`, **structurally
  write-protected**).
- Resolve the on-disk footprint (`~/.pq-companion/` — user database, settings file,
  backups, logs) to the same paths the reference uses, so an existing install is
  found rather than shadowed. Loading and saving settings is Wave 2.
- Announce the runtime (port/pid/version) to `~/.pq-companion/runtime.json` + stdout so a
  native shell can discover it.
- Add an Elixir CI job alongside the existing Go and TypeScript jobs.

Not in this change: the full schema set, settings changesets and adoption tooling
(Wave 2), the sidebar (Wave 1), overlay window creation (Wave 8), packaging
(Wave 11).

## Capabilities

### New Capabilities

- `app-shell`: The Phoenix application boots, serves LiveView over WebSocket,
  routes the main window and overlay windows through distinct layouts, and
  announces its runtime address for a native shell to consume.
- `desktop-shell`: A swappable native-shell adapter contract that describes what
  each window must be, with a browser adapter that satisfies the contract in
  development and a conformance test every future adapter must pass.
- `data-store`: Both databases open at boot — read-only game data
  (`quarm.db`) that can never be written, and read-write user data
  (`user.db`) that is opened in place without schema change.
- `dev-toolchain`: The Elixir app has a reproducible local dev workflow that
  needs no native shell, and a CI job that gates every push and PR.

### Modified Capabilities

None. This is the first change in the migration.

## Impact

- **Affected specs:** `app-shell`, `desktop-shell`, `data-store`, `dev-toolchain`
- **Affected code:** new tree `phoenix/` (does not exist yet); `.tool-versions`
  (new); `.github/workflows/ci.yml` (add Elixir job); `.gitignore`
- **Ported from (frozen reference, read-only):**
  - `electron/main/index.ts` — 4,165 LOC, 0 test LOC; the `BrowserWindow`
    option set (transparent, alwaysOnTop, frameless, `setIgnoreMouseEvents`)
    is the source for the `PQ.Shell` window contract
  - `frontend/src/App.tsx` — 375 LOC, 0 test LOC; the three window classes
    (`MainWindowLayout` / `OverlayPage` / bare) are the source for the layouts
  - `backend/cmd/server/main.go` — the runtime/sidecar announcement and the
    `~/.pq-companion` path contract (`appHome`, `userDBPath`, `configPath`)
  - `backend/internal/config/config.go` — 1,494 LOC, 482 test LOC; only the
    path resolution (~40 LOC) is in scope here, the schema is Wave 2
  - `.github/workflows/ci.yml` — the Go and TypeScript jobs to mirror
- **Porting risk:** low. No game logic, no log parsing, no native code. The one
  real unknown is Windows NIF support for Exqlite, which acceptance criterion 3
  settles immediately.
- **Dependencies (new):** `ecto_sqlite3`, `exqlite`, `file_system`,
  `nimble_options`, `credo`, `dialyxir`, `mix_audit` — and `heroicons`/
  `floki`/`daisyUI` were **deliberately dropped** during task 1.3/2.5 (see
  tasks.md for the rationale): `floki` is redundant with `lazy_html`, daisyUI
  collides with the reference's tokens, and the icon story is **vendored Lucide**
  (the reference uses `lucide-react` in 151 files; wave 1's design D3 vendors
  those paths), so the heroicons plugin, vendor JS and dep are gone.
- **User-data impact:** this change reads `user.db` and resolves the existing
  `config.yaml` path but writes neither. The only file it writes is the new
  `runtime.json`.
- **Risks carried forward:** `file_system` Windows support is unverified; the
  fallback is a polling GenServer, which only affects Wave 3. The named-pipe
  question (§2.5 of the plan) is untouched by this change.
