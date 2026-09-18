## 1. Toolchain and project generation

- [x] 1.1 Install Erlang/OTP 27 and Elixir 1.18 and pin both in `.tool-versions`; verify `elixir --version` and `erl -eval 'erlang:display(erlang:system_info(otp_release)), halt().'` report the pinned versions — **verified 2026-09-18**: `elixir --version` → `Elixir 1.18.5 (compiled with Erlang/OTP 27)`; OTP release → `27`; `mix --version` → `Mix 1.18.5`. Pinned in `.tool-versions` as `erlang 27.3.4.17` / `elixir 1.18.5-otp-27`; `erlang.compile=false` + `erlang.precompiled_os=ubuntu-24.04` set globally so a source build can never be attempted (this box has no `make`, no build deps, and `sudo` needs a password). Installed via mise 2026.9.11 → `~/.local/bin/mise`, activation appended to `~/.bashrc`. Also installed for tasks 1.2–1.3: hex 2.5.1, rebar3, `phx_new` 1.8.14 archive.
- [x] 1.2 Generate the app at `phoenix/` with `mix phx.new phoenix --app pq_companion --module PQCompanion --database sqlite3 --no-mailer --no-gettext`; verify `mix compile` succeeds and the generated tree is present — **verified 2026-09-18**: tree generated at `phoenix/`; `mix deps.get` + `mix compile` → `Generated pq_companion app`; `mix test` → **5 tests, 0 failures**; `select sqlite_version()` → **3.53.4** through the Exqlite NIF (confirms the data layer works on Linux — the *Windows* NIF is still task 1.4). Generator produced `phoenix/AGENTS.md` (448 lines of Phoenix 1.8 conventions); kept verbatim with a migration-context header prepended pointing at the root `AGENTS.md`.
- [x] 1.3 Add and pin the runtime dependencies (`ecto_sqlite3`, `exqlite`, `file_system`, `nimble_options`, `heroicons`, `floki`) and dev/test dependencies (`credo`, `dialyxir`, `mix_audit`); verify `mix deps.get` resolves and `mix compile` is clean — **verified 2026-09-18, with two corrections to this task's premise.** (a) `ecto_sqlite3`/`exqlite`/`heroicons` were already generated (the `--database sqlite3` flag plus generator defaults), so only four deps were actually added: `file_system` 1.1.1, `nimble_options` ~> 1.1, and dev-only `credo` / `dialyxir` / `mix_audit`. (b) **`floki` was dropped as redundant** — Phoenix 1.8 generates `lazy_html`, which is the current default test HTML parser; carrying both would mean two parsers for no gain. **`daisyUI` was removed** on request: deleted from `mix.exs`, unlocked from `mix.lock` (`mix deps.unlock --unused`), and its three `@plugin` blocks removed from `assets/css/app.css`. Verified: `mix compile`, `mix assets.build` (heroicons plugin still works), `mix test` → **5 tests, 0 failures**.
- [ ] 1.4 Verify the Exqlite Windows NIF loads, since a failure here invalidates the database choice for the whole migration: verify `mix ecto.create` succeeds against a scratch SQLite file (design D-repo risk)
- [ ] 1.5 Verify `file_system` works on Windows by watching a scratch directory and recording whether a create event is observed; if not, record the poll-based fallback in the Wave 3 plan
- [x] 1.6 Add `phoenix/_build/`, `phoenix/deps/`, `phoenix/priv/static/` and `phoenix/priv/data/quarm.db` to `.gitignore`; verify `git status` stays clean after a build — **verified 2026-09-18**: added a `/phoenix/` block covering `_build/`, `deps/`, `tmp/`, `priv/static/`, `priv/data/`, `priv/repo/*.db*` and `*.ez`. `git add -An phoenix/` stages **39 files** and zero build artifacts. This had to land *before* the first `git add` — `phoenix/` is untracked as a whole directory, so a premature `git add` would have committed hundreds of MB of `_build`/`deps`.

## 2. Application shell and layouts

- [ ] 2.1 Configure the endpoint and LiveView socket; verify a client can establish a LiveView connection and a server-side assign change arrives as a diff without a page reload
- [ ] 2.2 Create the `:root`, `:overlay` and `:bare` layouts; verify a test asserts the root layout renders sidebar and titlebar regions, the overlay layout renders neither and applies the transparent-body class, and the bare layout renders no navigation
- [ ] 2.3 Add a placeholder main-window LiveView and one placeholder overlay LiveView for each of the 16 reference overlay routes; verify every route renders and a test enumerates the 16 routes so none can be silently dropped (reference: the 16 `OverlayPage` routes in `frontend/src/App.tsx`)
- [ ] 2.4 Ensure overlay routes never mount alert hooks; verify a test asserts overlay mount registers no audio owner, and that main-window mount registers exactly one
- [ ] 2.5 Reuse the reference Tailwind theme tokens so visual parity is achievable: copy the `--color-*` custom properties from `frontend/src/index.css` (254 LOC, 22 tokens) into the app's stylesheet; verify a rendered page resolves the primary and surface tokens. **Also restyle the generated components off daisyUI** — removing daisyUI (task 1.3) left `components/core_components.ex` (501 lines: `btn`, `alert`, `input`, `select`, `table`, `list`, `fieldset`, `label`, `checkbox`) and `components/layouts.ex` (`menu`, `navbar`, `card`, `toggle`, `skeleton`) carrying class names that no longer resolve, so they render unstyled. Nothing uses `@apply` with them, which is why the build still passes. **Decide the icon story in the same pass:** these components use heroicons (`hero-*` classes via the `@plugin "../vendor/heroicons"`), but the navigation work (wave 1, design D3 of `add-sidebar-navigation`) vendors the reference's Lucide SVG paths instead. Either keep heroicons for generic UI and vendor Lucide only for nav, or standardise on one set — do not end up with both by accident.

## 3. Shell adapter contract

- [ ] 3.1 Define the `PQ.Shell` behaviour with the window struct and the open/close/resize/set_click_through/focus/list callbacks; verify the behaviour compiles and feature code references only the behaviour (design D3)
- [ ] 3.2 Render the `pq-window` meta tag from the overlay layout and a main-window spec from the root layout; verify a test parses the tag as JSON and asserts every required key is present on both window classes
- [ ] 3.3 Implement `PQ.Shell.Browser` as the development default, recording received specs and reporting native-only properties as unmet rather than succeeding; verify a test asserts transparency is reported unmet
- [ ] 3.4 Implement the adapter conformance suite and run it against `PQ.Shell.Browser`; verify it passes, and verify it fails with a named missing operation when run against a deliberately incomplete stub adapter
- [ ] 3.5 Implement `push_event`/`handleEvent` propagation for bounds, zoom, display-only, click-through and lock; verify a test asserts a display-only toggle reaches the shell as an update with no reload, and that reported bounds are persisted and re-applied on next open

## 4. Data store

- [ ] 4.1 Configure `PQ.UserRepo` against `~/.pq-companion/user.db` (read-write, WAL, busy timeout) and `PQ.QuarmRepo` against the game database (read-only); verify both start in the supervision tree and report healthy before the listener accepts traffic
- [ ] 4.2 Make the game-data repo structurally write-protected; verify a test asserts an insert/update/delete against it raises an error naming the repository, and that it fails before reaching SQLite
- [ ] 4.3 Read from the game database (`items`, `spells_new`, `npc_types`, `zone`) using raw Ecto queries, since schemas are Wave 2; verify a test returns rows when the artifact is present and skips with an explicit warning when absent
- [ ] 4.4 Read and write against an existing reference-created user database; verify a value written is visible after a restart (this is the Wave 2 adoption prerequisite — no schema change is applied here)
- [ ] 4.5 Fail with an actionable error when the game database is missing: verify startup aborts, the message names the expected path, and no query returns an empty result set in its place
- [ ] 4.6 Resolve the on-disk footprint to the reference locations — application home, user database, settings file, backups directory, server log — in one place; verify a test asserts each resolved path, and that booting against an existing reference install opens the existing files rather than creating new ones (reference: `appHome`/`userDBPath`/`configPath` in `backend/cmd/server/main.go`)

## 5. Config and runtime announcement

- [ ] 5.1 Write `~/.pq-companion/runtime.json` (port, pid, version) after the listener binds, and emit the same record as one stdout line; verify a test asserts the record's port equals the actually-bound port, including when the preferred port was unavailable (design D6)
- [ ] 5.2 Remove or mark stale the runtime record on shutdown; verify a test asserts the record is gone or marked stale after a clean stop
- [ ] 5.3 Verify the settings file is not modified by this change: boot the application against an existing reference install and assert `config.yaml` is byte-identical after startup and shutdown (loading and saving settings is the separate `add-data-model` change)

## 6. Development workflow

- [ ] 6.1 Document and verify the one-command dev startup in `phoenix/README.md`; verify it works on a clean checkout with dependencies fetched
- [ ] 6.2 Verify live reload reflects a source edit without a manual restart
- [ ] 6.3 Verify every one of the 16 overlay routes renders in a browser with no native shell installed (task 2.3 covers the assertion; this verifies the dev default, not just the test)

## 7. Continuous integration

- [ ] 7.1 Add an Elixir CI job using `erlef/setup-beam` pinned to `.tool-versions`; verify the job fetches dependencies, compiles and runs the test suite on a push. **Also ensure the job runs a UTF-8 VM** (`LANG=C.UTF-8`, or `ELIXIR_ERL_OPTIONS="+fnu"`): with no locale the BEAM starts in latin1 name encoding and Elixir warns it "may malfunction". EQ item and NPC names are not guaranteed ASCII, so this is a correctness risk rather than a cosmetic warning. Locally it only shows under a stripped environment (`env -i`) — normal shells here have `LANG=C.UTF-8`.
- [ ] 7.2 Add a formatting gate and verify it fails on a deliberately misformatted file and passes once formatted
- [ ] 7.3 Mirror the reference's game-database download step and verify data-backed tests execute when the artifact is present and skip with an explicit warning when it is not — never reported as passing
- [ ] 7.4 Confirm the existing Go test job and TypeScript typecheck job still run; verify both appear green in the same workflow run

## 8. Wave 0 verification

- [ ] 8.1 Walk the six Wave 0 acceptance criteria in `docs/phoenix-migration-plan.md` §3.8 and record the result of each
- [ ] 8.2 Confirm no file under `backend/`, `frontend/` or `electron/` was modified: verify `git diff --stat` against the reference trees is empty
