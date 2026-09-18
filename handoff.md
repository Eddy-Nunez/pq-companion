# Handoff — PQ Companion → Elixir/Phoenix migration (2026-09-18, Wave 0 in progress — 20/34)

> **This is the MIGRATION handoff.** The reference app's handoff is a different
> file in a different tree — `handoff.md` in `/mnt/c/Users/eddyn/pq-companion`
> (raidcomp / Playwright era). Do not merge the two. This one is specific to
> `feat/phoenix-migration` and the `~/pq-companion-phoenix` worktree.

## SESSION UPDATE 5 — 2026-09-18 (Wave 0 to 20/34: the two databases are open)

**Read this first — where it conflicts with anything below, this wins.**
This session landed all of section 4 (tasks 4.1–4.6). Updates 1–4 remain current
except where this section says otherwise.

### Where the work stands

`openspec list` → **`add-phoenix-scaffold` 20/34**. Done: 1.1, 1.2, 1.3, 1.6,
2.1–2.5, **3.1–3.5**, **4.1–4.6**. Untouched: 1.4, 1.5 (need Windows), 5.x, 6.x,
7.x, 8.x. Waves 1 and 2 changes are still 0/27 and 0/55.

Branch `feat/phoenix-migration`, working tree clean. `mix test` → **71 tests,
0 failures** (was 53), and the same 71 pass with `quarm.db` removed (data-backed
tests skip with an explicit warning). `mix compile --warnings-as-errors` clean,
`mix format --check-formatted` clean on every file this session touched. Real
server: `/`, `/assets/css/app.css` and `/assets/js/app.js` all 200.

### ⚠️ First thing that bit: the `priv` symlink was dangling

This is the most important finding of the session, and it pre-dated it.
Because `_build` is itself a symlink to the ext4 cache, the relative `priv` link
Mix writes *inside* it resolves against the cache directory and dangles. So
`Application.app_dir(:pq_companion, "priv/...")` pointed at a non-existent path:
**static assets were 404ing and `priv/data/quarm.db` looked missing.** The
earlier sessions never noticed because they only asserted HTTP 200 on HTML.

**Fix applied (per-machine, in the cache):**

```bash
ln -sfn /mnt/c/Users/eddyn/pq-companion-phoenix/phoenix/priv \
        ~/.cache/pq-companion-phoenix/priv
```

Recorded in `AGENTS.md` under the two-build-space rules, with the verification
test (`File.dir?(Application.app_dir(:pq_companion, "priv/static"))`). The
structural fix, if the symlink layout is ever revisited, is `MIX_BUILD_PATH` /
`MIX_DEPS_PATH` instead of symlinking `_build`/`deps`.

### What landed

| Module | What |
|---|---|
| `lib/pq_companion/paths.ex` | The reference footprint in one place: home, `user.db`, `config.yaml`, `backups/`, `logs/server.log`, plus `quarm.db` |
| `lib/pq_companion/user_repo.ex` | `Ecto.Repo` for `~/.pq-companion/user.db`, WAL + 5s busy timeout, path from `Paths` |
| `lib/pq_companion/quarm_database.ex` | `Ecto.Repo` for the shipped game DB, `mode: :readonly` |
| `lib/pq_companion/quarm_repo.ex` | Read-only facade feature code uses; every write raises before SQLite; the four wave-0 table reads; boot check |
| `lib/pq_companion/read_only_repo_error.ex`, `missing_game_database_error.ex` | The two actionable errors |

The generated `PQCompanion.Repo` was deleted; `ecto_repos` is now
`[PQCompanion.UserRepo]` (only user data has migrations). `quarm.db` was
downloaded to `phoenix/priv/data/quarm.db` (86 MB, gitignored).

### The decisions this session made

1. **`PQCompanion.QuarmRepo` is a facade, not an `Ecto.Repo`.** Ecto.Repo's
   generated `insert`/`update`/`delete` are not `defoverridable` — redefining
   them compiles to an unreachable clause. To make the write rejection
   *structural* (in Elixir, before SQLite, as the spec demands) the public name
   must be a plain module that delegates reads to the real repo
   (`PQCompanion.QuarmDatabase`) and only ever raises on writes. Two layers: the
   facade rejects in Elixir; the connection is opened `mode: :readonly` as
   defence in depth.
2. **The game DB is not in `ecto_repos`.** It has no migrations and is never
   created or altered. `PQCompanion.UserRepo` is the only migration target.
3. **The test suite boots without the artifact.** `quarm_db_required` is false in
   test, and `QuarmRepo.available?/0` keeps the read-only connection out of the
   supervision tree when the artifact is absent, so the suite runs and
   data-backed tests skip with an explicit `IO.warn` instead of the whole app
   failing to start (task 7.3 depends on this).

### Known deviation to fix before wave 11: `immutable=1`

The reference opens `quarm.db` as `file:<path>?mode=ro&immutable=1` (see
`backend/internal/db/db.go`). **exqlite 0.40 exposes neither `immutable` nor
`SQLITE_OPEN_URI`**, and ecto_sqlite3 forces `journal_mode: :wal`, so our
read-only connection can create `-wal`/`-shm` siblings next to the artifact and
would **fail on a genuinely unwritable install directory** (e.g. `Program
Files`). Contents still cannot change. The full note is in
`quarm_database.ex`; resolve it before the Windows installer ships — either by
upstreaming an `:immutable` option to exqlite or by keeping the artifact in a
per-user-writable location. Do not let it be discovered at wave 11.

### Next, in dependency order

1. **5.1–5.3** — `~/.pq-companion/runtime.json` (port, pid, version) plus the
   same record as one stdout line, and a test that the settings file is
   untouched by boot.
2. **6.1–6.3** — dev workflow docs (live reload is unblocked: `inotify-tools` is
   installed).
3. **7.1–7.4** — CI, including the UTF-8 locale requirement from 7.1 and the
   game-database download step in 7.3.
4. **8.1–8.2** — wave verification, then `openspec archive add-phoenix-scaffold`.

Windows-side 1.4/1.5 are still outstanding (hex/rebar/phx_new, remove the
`_build`/`deps` symlinks before running Windows `mix`, UTF-8 locale).

### Things learned the hard way this session

1. **A 0-byte SQLite file cannot be opened read-only.** The first design left a
   zero-byte scratch at the artifact path so the read-only repo could start; the
   connection pool then timed out (`connection not available`) because
   `sqlite3_open_v2(READONLY)` needs a real database. The fix was structural:
   omit the connection when the artifact is absent rather than fake one.
2. **`journal_mode` is a write.** Forcing `:delete` on a read-only WAL database
   fails the connection outright. Journal mode cannot be changed on a read-only
   connection; only `immutable=1` (which exqlite lacks) avoids the WAL siblings.
3. **A schemaless Ecto `select: source` is refused by ecto_sqlite3** — it cannot
   know the columns. On a 159-column table, raw `SELECT *` is the honest tool.
4. **Mix's `priv` symlink is relative and breaks under a symlinked `_build`.**
   See above; this is why task 4.x looked like the game DB was missing.
5. **`pwsh`/9p note still stands:** the 86 MB download and the two DBs live on
   `/mnt/c`; `mix test` is still ~1.3s, so the source-read cost is not yet
   biting.

---

## SESSION UPDATE 4 — 2026-09-18 (Wave 0 to 14/34: the shell contract is real)

**Read this first — where it conflicts with anything below, this wins.**
This session landed all of section 3 (tasks 3.1–3.5), the first slice that is a
real contract rather than a placeholder. Updates 1–3 remain current except where
this section says otherwise.

### Where the work stands

`openspec list` → **`add-phoenix-scaffold` 14/34**. Done: 1.1, 1.2, 1.3, 1.6,
2.1, 2.2, 2.3, 2.4, 2.5, **3.1–3.5**. Untouched: 1.4, 1.5 (need Windows), 4.x,
5.x, 6.x, 7.x, 8.x. Waves 1 and 2 changes (`add-sidebar-navigation`,
`add-data-model`) are still 0/27 and 0/55.

Branch `feat/phoenix-migration`, tip **`177736ea`**, working tree clean.
`mix test` → **53 tests, 0 failures** (was 37). `mix compile --warnings-as-errors`
clean, `mix assets.build` OK, `mix format --check-formatted` clean on every
file this session touched. Real server: `/` and `/w/npc` both 200 with a
complete `pq-window` spec.

### What landed

| Module | What |
|---|---|
| `lib/pq_companion/shell.ex` | The behaviour (six callbacks), the `window` type, `spec/1`, `to_json/1`, token signing/verification, config-selected dispatch |
| `lib/pq_companion/shell/browser.ex` | Dev-default adapter: records specs, reports native-only props as `:unmet` |
| `lib/pq_companion/shell/conformance.ex` | `run/1` — `:ok` or `{:error, reasons}` naming the failed operation |
| `lib/pq_companion/window_state.ex` | ETS-backed runtime overrides: bounds, zoom, display_only, click_through, locked |
| `lib/pq_companion_web/shell_events.ex` | `push_event`/`handleEvent` for the five live properties |

Both window classes now render the `pq-window` meta tag (the main window's comes
from `root.html.heex`), carrying the session token and bounds. `assets/js/app.js`
gained the `PqWindow` hook (debounced bounds reporting) and a `phx:pq:window`
listener that forwards pushed patches to `window.pqShell` when a native shell
injects it.

### The three decisions this session actually made

1. **The module is `PQCompanion.Shell`, not `PQ.Shell`.** The plan and the specs
   write the short `PQ.*`/`PQWeb.*` aliases, but the generated app's namespace is
   `PQCompanion` and every existing module uses it. Renaming the tree to match the
   alias is churn with no behavioural gain. **This is a standing discrepancy** —
   wave 1's specs say `PQWeb.Nav` where the code will be `PQCompanionWeb.Nav`.
   Either the specs adopt the real namespace or a deliberate rename happens; do
   not let the two spellings both appear in code. Resolve it in wave 1, when
   `PQWeb.Nav` is written.
2. **`token` and `bounds` live in `Shell.spec/1`, and `Windows.meta_payload/1`
   was deleted.** The static registry (`PQCompanion.Windows`) stays a pure
   declaration; the spec builder merges it with runtime state from
   `PQCompanion.WindowState` and signs a fresh per-window token
   (`Phoenix.Token.sign`, salt `pq-window`). Keeping the old string-keyed builder
   alongside would have been two sources of truth for the same JSON.
3. **The conformance suite is a function (`run/1`), not a test-generating
   macro.** Only that shape lets a test assert the *failure* path — it runs
   `run/1` against `IncompleteAdapter` and asserts the error names `close/1`.

### Next, in dependency order

1. **4.1–4.6 — the two Ecto repos.** `PQ.UserRepo` (read-write, WAL, busy
   timeout) and `PQ.QuarmRepo` (structurally read-only), the read-only guard
   test, raw reads of `items`/`spells_new`/`npc_types`/`zone`, and the on-disk
   footprint resolution. `quarm.db` is still absent — fetch it before 4.3:
   `curl -L -o backend/data/quarm.db
   https://github.com/jasonsoprovich/pq-companion/releases/download/data-latest/quarm.db`
2. **5.1–5.3** — `~/.pq-companion/runtime.json` plus the stdout record.
3. **6.1–6.3** — dev workflow docs (live reload unblocked: `inotify-tools` is
   installed).
4. **7.1–7.4** — CI (UTF-8 locale requirement recorded in 7.1).
5. **8.1–8.2** — wave verification, then `openspec archive add-phoenix-scaffold`.

Windows-side 1.4/1.5 are still outstanding and still need hex/rebar/phx_new plus
the `_build`/`deps` symlink removal described in update 2.

### Things learned the hard way this session

1. **`mix format` reformats the `<script>` tags in `.heex` shell files** into a
   two-line empty-element shape. `windows.ex` and both HTML shells were
   previously unformatted (update 3 recorded this); formatting them is a net
   gain, but the `</script>` on its own line is the formatter's doing, not a
   mistake to revert — reverting re-breaks `--check-formatted`.
2. **`function_exported?/3` is the right gate for the conformance suite, but it
   lies until the module is loaded.** `Code.ensure_loaded(adapter)` first. And a
   stub must *not* carry `@behaviour`, or the missing callbacks become compile
   warnings and the runtime check never gets a chance to be the interesting one.
3. **An ETS table read from a render path must not serialize behind a
   GenServer.** `WindowState.get/1` reads the `:public` table directly; writes go
   through `GenServer.call`. Window renders happen concurrently and
   `Shell.spec/1` is on that path.
4. **Tests that write global state must be `async: false`.** The browser adapter
   records state and `WindowState` is global; `shell_test.exs` and
   `shell_events_test.exs` are sync. ExUnit runs sync modules after async ones,
   so the async `windows_test` reads are not racing the writes.
5. **The real-server check is worth the 6 seconds.** `mix phx.server` + `curl`
   proved both layouts emit the tag end-to-end; the LiveView tests alone would
   not have caught e.g. a root-layout guard that only worked for overlays.

---

## SESSION UPDATE 3 — 2026-09-18 (Wave 0 to 9/34: task 2.5 complete — daisyUI out, Lucide in)

**Read this first — where it conflicts with anything below, this wins.**
This session finished the last open piece of task 2.5 (the component restyle
and the icon decision). Everything from updates 1 and 2 below is still current
except where this section says otherwise.

### Where the work stands

`openspec list` → **`add-phoenix-scaffold` 9/34**. Done: 1.1, 1.2, 1.3, 1.6,
2.1, 2.2, 2.3, 2.4, **2.5 (now fully complete — tokens, restyle, icons)**.
Untouched: 1.4, 1.5 (need Windows), 3.x, 4.x, 5.x, 6.x, 7.x, 8.x. Waves 1 and 2
changes (`add-sidebar-navigation`, `add-data-model`) are still 0/27 and 0/55.

Branch `feat/phoenix-migration`, tip **`551f178f`**, working tree clean, pushed
to `origin`. `mix test` → **37 tests, 0 failures** (was 27 — the 10 new tests
are `test/pq_companion_web/core_components_test.exs`, each pinning one 2.5
verification). `mix compile --warnings-as-errors` clean, `mix assets.build` OK,
`mix format --check-formatted` clean on the touched files.

### What landed (commit `551f178f`)

**Task 2.5, the restyle half:** `core_components.ex` had 93 daisyUI class
references (`btn`, `alert-*`, `toast-*`, `input`/`select`/`textarea`/`checkbox`
classes, `fieldset`, `label` class, `table-zebra`, `list-row`, `text-error`,
`text-base-content/70`) across 501 lines — none resolved after daisyUI was
removed, so every generated component rendered unstyled. All replaced with the
reference's token utilities (`bg-(--color-surface)` etc.), matching its measured
visual language: `px-3 py-1.5 text-sm rounded` buttons (primary =
`--color-primary` bg; soft = transparent + `--color-border`), `w-full rounded
border … px-3 py-1.5 text-sm outline-none` inputs with
`focus:border-(--color-primary)` and `border-(--color-danger)` on error, the
reference's `text-[10px] font-semibold uppercase tracking-widest` table
headers, and a bottom-right flash toast (`fixed bottom-4 right-4`, surface bg,
Lucide icons, `--color-info`/`--color-danger` accents). I restyled the flash
against `ZealNotification.tsx` (the reference's toast) rather than the old
daisyUI `toast`/`alert` classes. `layouts.ex` was already clean (2.2 rewrote it).

**Icon decision — standardise on vendored Lucide, drop heroicons.** The
reference renders every icon with `lucide-react` in **151 files** (pinned
`1.16.0`), and wave 1's design D3 already commits to vendoring those paths into
`PQWeb.Components.NavIcons`. Keeping heroicons for generic UI would mean two
icon sets — the exact hazard the task names. So `icon/1` now emits one vendored
`<svg>` per name (`info`, `circle-alert`, `x`; paths taken verbatim from the
reference's pinned lucide-react), with lucide's default stroke attributes
(fill none, `stroke: currentColor`, stroke-width 2, round caps/joins). Removed:
`@plugin "../vendor/heroicons"` from `app.css`, `assets/vendor/heroicons.js`,
and the `{:heroicons, …}` dep from `mix.exs` (unlocked from `mix.lock`).

### Next, in dependency order (updated — item 1 is done)

1. **3.1–3.5 — the `PQ.Shell` behaviour.** The behaviour + window struct
   (plan §2.4 has the exact callback draft: `open`/`close`/`resize`/
   `set_click_through`/`focus`/`list`), `PQ.Shell.Browser` as the dev default,
   the conformance suite (must fail *naming the missing operation* against an
   incomplete stub), and `push_event`/`handleEvent` for bounds/zoom/
   display-only/lock. Groundwork already in place: the overlay layout emits the
   `pq-window` meta and `PQCompanion.Windows.meta_payload/1` carries all native
   props. Watch: the spec requires a **session token** and **bounds** key in
   the payload, which `meta_payload/1` doesn't yet emit — decide during 3.2
   whether they ride in the struct, the payload builder, or `PQ.Shell`.
2. **4.1–4.6** — the two Ecto repos (`QuarmRepo` structurally read-only), the
   read-only guard test, and the on-disk footprint resolution.
3. **5.1–5.3** — `~/.pq-companion/runtime.json` plus the stdout record.
4. **6.1–6.3** — dev workflow docs (live reload unblocked: `inotify-tools` is
   installed).
5. **7.1–7.4** — CI (UTF-8 locale requirement recorded in 7.1).
6. **8.1–8.2** — wave verification, then `openspec archive add-phoenix-scaffold`.

Note the whole tree still fails `mix format --check-formatted` on pre-existing
files outside 2.5 (e.g. `windows.ex`, the two HTML shells) — I reverted mix
format's sweep of unrelated files to keep this change focused. The 7.2
formatting gate will force the full-tree sweep; do it then with a task of its
own.

### Things learned the hard way this session

1. **`~H` treats `@foo` as an assign, so a module attribute cannot be read
   inside a `~H` template.** `Map.fetch!(@lucide, @name)` inside `~H` compiled
   (attribute „set but never used" warning) and would have failed at runtime as
   a missing *assign*. The vendored icon map is read in a plain function
   (`lucide_svg/2`) that the template calls — module attributes live in normal
   code, assigns in templates.
2. **`render_component` calls a function component with the assigns map
   verbatim — a do-block slot is not wired into `assigns`.** Passing
   `render_component(&button/1, ...) do "Go" end` left `inner_block` unset and
   raised `KeyError`/`FunctionClauseError` on `render_slot`. A slot must be
   passed as its entry structure: `%{__slot__: :inner_block, inner_block: fn
   _changed, _arg -> content end}` (helper `slot/1` in the test file).
3. **`render_component` needs every required assign explicitly** — `input/1`
   without a `field:` needs `value:` passed, or it raises `KeyError`. The
   do-block/slot discover-y above and this are LiveViewTest ergonomics, not
   app bugs — worth a note when adding component tests in later waves.
4. **Flash maps use string keys.** `Phoenix.Flash.get(%{error: "boom"}, :error)`
   returns nil; `%{"error" => "boom"}` works. Test fixtures must use string
   keys (they come from `fetch_live_flash` that way).
5. **Grep assertions over source need care with substrings.** A test claiming
   no `btn` survives still passes on `def button` — `button` does not contain
   the contiguous substring `btn`. Assert the actual class strings
   (`class="btn"`, `table-zebra`, `alert-error`), not bare words, and keep
   `label`/`fieldset` out of the banned list (they're native HTML elements).

---

## SESSION UPDATE 2 — 2026-09-18 (Wave 0 to 8/34: app scaffolded, windows built)

**Read this first — where it conflicts with anything below, this wins.**
Session 1 got the toolchain working; this session built the app skeleton. The
sections under update 1 (trees, worktree git pointers, 9p cost, Burrito strategy,
gotchas) are **still current** — update 2 only adds to them.

### Where the work stands

`openspec list` → **`add-phoenix-scaffold` 8/34**. Done: 1.1, 1.2, 1.3, 1.6,
2.1, 2.2, 2.3, 2.4. **Partial: 2.5** (theme tokens done, component restyle not).
Untouched: 3.x, 4.x, 5.x, 6.x, 7.x, 8.x. Waves 1 and 2 changes
(`add-sidebar-navigation`, `add-data-model`) are still 0/27 and 0/55.

Branch `feat/phoenix-migration`, tip `e742fcdf`, working tree clean, pushed.
`mix test` → **27 tests, 0 failures**. `mix compile --warnings-as-errors` clean.
The real server returns 200 on `/` and all 16 overlay routes, 302 on an unknown
window.

### What landed (commits `8df5fee6` .. `e742fcdf`)

- **Toolchain** — mise, Erlang/OTP 27.3.4.17, Elixir 1.18.5, Mix 1.18.5, hex
  2.5.1, rebar3, phx_new 1.8.14. Pinned in `.tool-versions` with a global
  fallback so shims resolve outside the project.
- **The app** — `phoenix/` generated (`--database sqlite3 --no-mailer
  --no-gettext`), deps resolved, compiling. `select sqlite_version()` → 3.53.4
  through the Exqlite NIF.
- **Windows structure** — `PQCompanion.Windows` (registry: 16 overlays + main),
  three window classes as shell + inner layout, a parametrised `OverlayLive` for
  all 16, and `PQCompanion.Audio` (audio-owner registry). The generated
  `PageController`/`PageHTML` was deleted — `/` is a LiveView now.
- **Theme** — the reference's 22 `@theme` tokens ported verbatim into
  `assets/css/app.css`, including the `--color-danger` alias.

### Where the code lives

| Path (under `phoenix/`) | What |
|---|---|
| `lib/pq_companion/windows.ex` | Window registry — id, slug, route, native props |
| `lib/pq_companion/audio.ex` | Audio-owner `Registry` |
| `lib/pq_companion_web/live/window_live.ex` | Main window |
| `lib/pq_companion_web/live/overlay_live.ex` | All 16 overlays |
| `lib/pq_companion_web/components/layouts.ex` | `window/1`, `bare/1`, `flash_group/1` |
| `lib/pq_companion_web/components/layouts/{root,overlay}.html.heex` | The two HTML shells |
| `lib/pq_companion_web/router.ex` | `/` and `/w/:slug` |
| `test/pq_companion/windows_test.exs`, `test/pq_companion_web/live/*_test.exs` | The suite |

### Next, in dependency order

1. **2.5 remainder** — restyle `core_components.ex` off daisyUI. It still has
   **93 occurrences** of `btn`/`alert`/`input`/`select`/`table`/`list`/`fieldset`/
   `label`/`checkbox`/`textarea`/`badge`/`link` across 501 lines, so those
   components render unstyled. `layouts.ex` is already clean (2.2 rewrote it).
   **Also decide heroicons vs Lucide** in the same pass — these components use
   `hero-*` classes via `@plugin "../vendor/heroicons"`, but wave 1 vendors the
   reference's Lucide paths (design D3 of `add-sidebar-navigation`). Do not end up
   with both by accident.
2. **3.1–3.5** — the `PQ.Shell` behaviour, the browser adapter, the conformance
   suite, and `push_event`/`handleEvent` for bounds/zoom/display-only/lock. The
   overlay shell already emits the `pq-window` meta this depends on.
3. **4.1–4.6** — the two Ecto repos (`QuarmRepo` structurally read-only), the
   read-only guard test, and the on-disk footprint resolution.
4. **5.1–5.3** — `~/.pq-companion/runtime.json` plus the stdout record.
5. **6.1–6.3** — dev workflow docs. **Live reload should work now** —
   `inotify-tools` was installed this session, which unblocks task 6.2.
6. **7.1–7.4** — CI (remember the UTF-8 locale requirement recorded in 7.1).
7. **8.1–8.2** — wave verification, then `openspec archive add-phoenix-scaffold`.

### Windows-side work still outstanding (tasks 1.4 and 1.5)

Erlang/OTP and Elixir **are now installed on Windows**. Still to do before 1.4/1.5
can run:

1. `mix local.hex --force`, `mix local.rebar --force`,
   `mix archive.install hex phx_new --force` on the Windows side.
2. **Delete `phoenix/_build` and `phoenix/deps` before running `mix` from
   Windows** (they are symlinks to the ext4 cache and Windows sees them as 0-byte
   reparse points), or set `MIX_BUILD_PATH`/`MIX_DEPS_PATH` to Windows paths.
   Both are gitignored, so deleting them costs nothing.
3. Set a UTF-8 locale (see the latin1 gotcha under update 1).

Task 1.5 gained a finding this session: `file_system` shells out to `inotifywait`
on Linux, so it carries a platform prerequisite there *and* an open question on
Windows. A poll-based GenServer may be the better **primary** implementation for
wave 3's log watching rather than a fallback — worth deciding when 1.5 runs.

### Environment, as of now

| | |
|---|---|
| WSL | Ubuntu 24.04, ext4 source cache, `inotify-tools` installed |
| Elixir toolchain | mise → OTP 27.3.4.17 / Elixir 1.18.5 / Mix 1.18.5 |
| Elixir extras | hex 2.5.1, rebar3, phx_new 1.8.14 |
| Burrito toolchain | zig 0.15.2, 7zip 26.03, `7z` shim at `~/.local/bin/7z` |
| Source | `/mnt/c/Users/eddyn/pq-companion-phoenix` (Windows-visible) |
| Build cache | `~/.cache/pq-companion-phoenix/{_build,deps}` (ext4, symlinked in) |
| Windows | Erlang/OTP + Elixir installed; hex/rebar/phx_new still to add |

### Things learned the hard way this session

1. **`:root` became `:window`.** The spec named the main-window layout class
   `:root`, which collides with Phoenix's root-layout concept (`put_root_layout`).
   `root/1` keeps its Phoenix meaning; spec updated with the rationale.
2. **A LiveView layout receives `@inner_content`, not `slot :inner_block`.** The
   generator's `app/1` used `render_slot(@inner_block)` and worked only because
   `config :phoenix_live_view, layout: false` meant it was always called
   explicitly as `<Layouts.app>`. As a real layout it raised `KeyError`.
3. **Registry `id` and URL `slug` are different things.** Ids are the reference's
   camelCase `overlayKey`s (kept so per-overlay preferences survive); routes are
   kebab-case. Conflating them made every camelCase overlay silently redirect to
   the main window — a placeholder redirecting looks a lot like one working.
4. **HEEx HTML-escapes the `pq-window` meta attribute**, so a regex test captures
   `&quot;` and breaks Jason. Parse it (`LazyHTML`) — which is also what a native
   shell will do.
5. **`lazy_html`, not `floki`.** Phoenix 1.8 generates the former; task 1.3
   correctly dropped floki as redundant. I nearly reintroduced it in a test.
6. **Removing daisyUI is not a one-line deletion** — the generated components are
   built from its classes. No `@apply`, so the build survives; the components just
   render unstyled until 2.5 finishes.

---

## SESSION UPDATE 1 — 2026-09-18 (migration bootstrapped; toolchain blocker) — SUPERSEDED by update 2 above

> Kept for history. Update 2 at the top of this file wins wherever they conflict.
> Its "blocked on toolchain" framing is resolved; the trees/git-pointer/9p/Burrito
> sections below are still current and are referenced by update 2.

### Where the trees are

| Tree | Path | Branch | Role |
|---|---|---|---|
| Migration | `/mnt/c/Users/eddyn/pq-companion-phoenix` | `feat/phoenix-migration` @ `13d36fed` | The rewrite. All new work. |
| Reference | `/mnt/c/Users/eddyn/pq-companion` | `feat/raidcomp-pack` @ `81816394` | Shipping Go app. Frozen reference. |

`git worktree list` from either tree shows both.

**The migration tree has moved twice.** It was cut on `/mnt/c`, moved to ext4 for
speed, then moved **back** to `/mnt/c` on 2026-09-18 because Windows-side builds
were expected. The speed problem was then solved a better way — by relocating
only the build artefacts:

```
/mnt/c/Users/eddyn/pq-companion-phoenix     source — Windows-visible
/home/nunez/pq-companion-phoenix            symlink to the above, for WSL
  └── phoenix/_build -> /home/nunez/.cache/pq-companion-phoenix/_build
  └── phoenix/deps   -> /home/nunez/.cache/pq-companion-phoenix/deps
```

That recovers essentially all the lost speed — measured on the scaffold alone:

| operation | artefacts on 9p | artefacts on ext4 |
|---|---|---|
| `mix deps.get` | 1m31s | **3.7s** |
| `mix compile --force` | 2m35s | **6.8s** |
| `mix test` | 2m18s | **5.0s** |

Both moves used `git worktree remove` + `git worktree add`, never
`git worktree move` — that fails cross-device with `Invalid cross-device link`,
and `remove`+`add` is safe whenever the branch is already pushed.

**Caveat:** Windows sees `_build`/`deps` as 0-byte reparse points, not
directories, so it cannot follow them. Irrelevant if Windows never runs `mix` —
which is the plan (Burrito builds from WSL). If Windows-side `mix` is ever
needed, delete the symlinks first or set `MIX_BUILD_PATH`/`MIX_DEPS_PATH` there.

#### Residual cost of keeping the SOURCE on 9p

Measured 2026-09-18 with a dep-free 300-module mix project, `mix compile
--force`, **301 `.beam` files verified in every case**:

| configuration | full recompile | vs best |
|---|---|---|
| all ext4 | **3.4s** | — |
| **current** (source 9p, build ext4) | **9.2s** | **2.7×** |
| all 9p | **15.3s** | 4.5× |

So the symlink recovered ~40% (15.3s -> 9.2s) but **2.7× remains**, and the whole
residual is source reads: **~19ms per source file**, because 9p's per-file
open/stat overhead dominates. At a few hundred modules that is roughly 6–15s
extra per *full* compile. Real project today (15 files): full 7.4s, incremental
1.7s, `mix test` 5.2s, `git status` 3.0s, `rg` 0.09s.

*(A first run of this probe reported 1.14s for the current arrangement — faster
than the all-ext4 baseline, which is impossible. It was discarded and re-run with
the compiler output visible and beam counts checked. Recorded because it nearly
became a wrong finding.)*

**Honest assessment.** The symlink is unambiguously good. The *move* to `/mnt/c`
is a trade, and it was argued on a premise that did not survive: the stated reason
was "Windows-based builds", and Burrito then removed exactly that — it builds the
Windows artifact *from WSL*. What remains is genuine (Windows-side **validation**
for tasks 1.4/1.5, native editing, no `\\wsl$` UNC quirks) but it is a different
reason than the one given. The measurement should have come first.

**Why it is nevertheless probably right.** Full recompiles are rare — incremental
compile is 1.7s — so the penalty lands on first build, `mix clean`, dependency
changes and branch switches, not on every edit. **CI is unaffected** (GitHub
runners are Linux/ext4).

#### Fallback if the source-read cost starts to bite: two independent clones

A worktree cannot satisfy both sides at once — Windows validation needs source
reachable *and* a Windows-native `_build`; WSL wants source on ext4. The clean
answer is **two free-standing clones** (the repo is only ~34 MB packed): an ext4
clone for WSL, the `/mnt/c` one for Windows, synced through `origin`. Both sides
get optimal I/O, and it removes the absolute-`gitdir` sharp edge below entirely
because a clone's `.git` is self-contained. Not worth doing until the cost is
actually felt.

### Worktree git pointers — half relative, half absolute, on purpose

Windows-side git was **broken** in this tree until 2026-09-18:

```
fatal: not a git repository: /mnt/c/Users/eddyn/pq-companion/.git/worktrees/pq-companion-phoenix
```

The worktree's `.git` pointer held an *absolute WSL path*, which Windows git
cannot resolve. The two pointers need **different** treatments, which is easy to
get wrong:

```
worktree  .git  = gitdir: ../pq-companion/.git/worktrees/pq-companion-phoenix
                  ^ RELATIVE — git resolves this against the worktree root, so it
                    works from both WSL and Windows.

main repo .git/worktrees/pq-companion-phoenix/gitdir
                = /mnt/c/Users/eddyn/pq-companion-phoenix/.git
                  ^ ABSOLUTE — must stay this way. git 2.43 does NOT support a
                    relative path here; it resolves the value against the current
                    directory, so a relative value makes git report the worktree
                    as `prunable`. (Relative support arrived with
                    `worktree.useRelativePaths`, git 2.48+.)
```

**Verified:** WSL `git worktree list` is correct and `git worktree prune
--dry-run` is a no-op. Windows git works normally *inside* the worktree.

> ⚠️ **The one sharp edge:** because that absolute path is WSL-form, running
> `git worktree prune` or `git worktree remove` from **Windows** against the
> *reference* repo would delete the migration worktree's admin entry
> (confirmed via `--dry-run` from Windows: *"gitdir file points to non-existent
> location"*). Normal git commands are unaffected — only worktree-admin commands.
> **Run those from WSL.**
>
> No single absolute path can satisfy both Windows (`C:\...`) and WSL
> (`/mnt/c/...`), so this is inherent to a dual-access worktree. If the sharp
> edge ever bites, the robust fix is to make the migration tree a **separate
> clone** rather than a worktree — a clone's `.git` is self-contained, so no
> absolute cross-reference exists at all.

### Fork posture — permanent divergence, NOT upstream

The migration is a **personal aspiration**, confirmed 2026-09-18. It will never
be proposed to the maintainer's repo.

- Never push `feat/phoenix-migration` anywhere but `origin` (the fork). Never
  open a PR to `upstream`. GitHub prints a "Create a pull request" link on every
  push — **ignore it.**
- `upstream` is a **read-only reference and rebase base** only.
- Long-lived and rebase-prone; force-pushes to `origin/feat/phoenix-migration`
  after a rebase are expected.
- At Wave 11 this branch becomes the fork's `main`.

### What exists now

Committed and pushed as `0aa89896` (35 files, 5,592 insertions):

- **`docs/phoenix-migration-plan.md`** — the master plan. Assessment (measured
  inventory, five hard constraints), target architecture, 12-wave roadmap,
  **Part IX: duplication policy**, and appendices (user.db/quarm.db table
  inventories, ranked porting assets). Whitelisted in `.gitignore`.
- **`openspec/`** — 3 changes covering Waves 0–2, all passing
  `openspec validate --strict`: **42 requirements, 109 scenarios, 116
  verification-bearing tasks.** `openspec list` for live progress.
- **`AGENTS.md`** — fork posture, the two-build-space rule, delete-as-you-go,
  the duplication short version, and the OpenSpec workflow.
- **`.pi/`** — OpenSpec commands and skills (`opsx-*`).

Nothing has been written in `phoenix/` yet — it does not exist.

### What was next at the end of session 1 (SUPERSEDED — see update 2)

**Task 1.1 (toolchain) is DONE** — see the commit log and the environment table
below. The next command is the generator:

```bash
cd ~/pq-companion-phoenix
eval "$(mise activate bash)"   # or just open a new shell
mix phx.new phoenix --app pq_companion --module PQCompanion \
  --database sqlite3 --no-mailer --no-gettext
```

That is task 1.2. Then 1.3 (deps: `ecto_sqlite3`, `exqlite`, `file_system`,
`nimble_options`, `heroicons`, `floki`; dev/test: `credo`, `dialyxir`,
`mix_audit`), then **1.4 and 1.5 which must run on Windows** — see the user-side
recommendation below.

One thing to decide before 1.3: whether `mix phx.new`'s default `esbuild` +
`tailwind` setup is kept as-is. The `tailwind` hex package is the current
Phoenix way and avoids a Node dependency for CSS; `esbuild` still needs a Node
binary for the small JS hooks later (xyflow graphs, drag-reorder). Worth
confirming rather than inheriting the default blindly.

### Environment state

**WSL** — Ubuntu 24.04.1 LTS, WSL2, 12 cores, 15 GB RAM, 941 GB free on ext4.
`systemd` is running (`/etc/wsl.conf` has `boot.systemd=true`).

| Tool | State |
|---|---|
| `elixir` `mix` `erl` `iex` | **INSTALLED** — mise, OTP 27 / Elixir 1.18.5 / Mix 1.18.5 |
| `mise` | installed, 2026.9.11 at `~/.local/bin/mise` |
| `hex` `rebar3` `phx_new` | installed (2.5.1 / rebar3 / phx_new 1.8.14) |
| `curl` `unzip` `git` `python3` `pip3` `gcc` | present |
| `make` `autoconf` `m4` `jq` | MISSING |
| `gh` CLI | MISSING — use `curl` for `quarm.db` |

**How the toolchain resolves (three paths, all verified 2026-09-18):**

- `.tool-versions` in the project pins `erlang 27.3.4.17` / `elixir 1.18.5-otp-27`.
- A **global fallback** in `~/.config/mise/config.toml` pins the same two, so the
  shims resolve outside the project too (without it, mise errors with an
  unhelpful message when run from any other directory).
- `~/.bashrc` gets `mise activate bash` (interactive shells); `~/.profile` adds
  `~/.local/share/mise/shims` to `PATH` (login and non-interactive shells, which
  do **not** read `.bashrc` — that is why the shims line is needed for CI).
- `erlang.compile=false` + `erlang.precompiled_os=ubuntu-24.04` are set globally.
  A source build is impossible here — no `make`, no build deps, and no `sudo` —
  so precompiled is forced rather than merely preferred. The whole install took
  ~15s.

**`sudo` requires a password.** No system package installs are possible without
you. `apt` offers Elixir **1.14** / Erlang **25** (too old), Erlang Solutions has
**no `noble` repo** (404), and there are no build deps — so precompiled is the
only path, which is why mise is the plan. This is also why the toolchain lives
entirely under `~/.local` and `~/.config/mise`: no elevated permissions needed
anywhere.

**Windows** — has git (PortableGit), node, npm. Has **no** Elixir/Erlang/mise.
Docker Desktop is installed and `docker` works from WSL (the Windows
`com.docker.service` shows Stopped, which is normal — the engine runs in the
`docker-desktop` WSL distro).

**`quarm.db` is absent** from both trees (`*.db` is gitignored). Needed from
task 4.3 onward: `curl -L -o backend/data/quarm.db
https://github.com/jasonsoprovich/pq-companion/releases/download/data-latest/quarm.db`.

### User-side recommendation

**DONE 2026-09-18 — Erlang/OTP and Elixir are installed on the Windows side.**
(Confirmed necessary: tasks **1.4** (Exqlite Windows NIF loads) and **1.5**
(`file_system` watching), plus Wave 3's named-pipe spike, can only be validated on
Windows — a Linux pass tests the Linux NIF and proves nothing.)

Winget has **no Elixir package** (`winget search elixir` returns only Gleam and
Livebook, which merely tag it). `Erlang.ErlangOTP` *is* present; winget's newest
27.x is **27.3.4.13** (Linux has 27.3.4.17 — same patch level, immaterial). Elixir
comes from the official release assets, which include both
`elixir-otp-27.exe` (installer) and `elixir-otp-27.zip` (portable) with sha256
sums.

Remaining Windows-side setup before 1.4/1.5 can actually run:

1. `mix local.hex --force`, `mix local.rebar --force`,
   `mix archive.install hex phx_new --force` — the Windows toolchain needs its own
   copies.
2. **Do not run `mix` from Windows in this tree while the symlinks exist.**
   Delete `phoenix/_build` and `phoenix/deps` first (both gitignored, so git does
   not care), or set `MIX_BUILD_PATH`/`MIX_DEPS_PATH` to Windows-local paths.
   Windows must build its own artefacts anyway — sharing them across platforms is
   exactly what the `.so` vs `.dll` split forbids.
3. Set a UTF-8 locale (`LANG=C.UTF-8`) — see the latin1 gotcha below.

### Gotchas discovered this session

**9p is catastrophically slow — this is why the tree moved.** Measured, 3000
small files (the shape of `mix`'s `_build`/`deps` I/O):

| | ext4 | `/mnt/c` (9p) | penalty |
|---|---|---|---|
| write 3000 files | 5,202 ms | 29,789 ms | **5.7×** |
| read 3000 files | 82 ms | 11,669 ms | **142×** |
| delete 3000 files | 56 ms | 3,879 ms | 69× |

The read penalty hits every source read — `mix compile`, `mix test`, `git
status`, ripgrep, editor indexing. The reference app never felt this because it
is **built on Windows**; the migration is the first heavy WSL-side compile
workload.

**Windows-originated file writes produce ZERO inotify events.** Measured in a
container bind-mount: writes from the WSL side gave 30/30 events with no misses;
a write created by PowerShell produced **0 events**. This is a 9p property, not a
Docker one — native WSL has the same blind spot. Consequence: **live reload
(Phase 0 task 6.2) silently does nothing if you edit via a Windows-native editor
or Windows git.** Editing through VS Code's WSL Remote goes via the WSL server and
is seen normally.

**A container running as root creates files your user cannot delete.** Hit this
by accident while benchmarking — had to spin up a root container to clean up.
Moot while we use native mise, but if Docker is ever revisited, every invocation
needs `--user "$(id -u):$(id -g)"`. For reference, the verified-good image is
`hexpm/elixir:1.18.5-erlang-27.3.4.17-ubuntu-noble-20260911`.

**`git worktree move` does not work across filesystems.** Fails with
`Invalid cross-device link`. Use `git worktree remove` + `git worktree add` —
safe here because the branch was already pushed to `origin`.

**Reference-app gotchas still apply if you build that tree** (from the other
handoff): `electron-builder` strips `package.json`'s entire `scripts` block on
every build — restore with `git checkout -- package.json` immediately after.
Never `npm install` from WSL into the reference tree; it replaces the Windows
electron binaries in `node_modules`.

**A latin1 BEAM corrupts non-ASCII data.** With no locale set, the VM starts in
latin1 name encoding and Elixir warns it "may malfunction". Normal shells here
have `LANG=C.UTF-8` and run clean — this only appears under a stripped
environment (`env -i`), which is exactly what a CI runner looks like. Set
`LANG=C.UTF-8` (or `ELIXIR_ERL_OPTIONS="+fnu"`) in CI. Tracked in task 7.1.

**`file_system` needs `inotify-tools` on Linux, and it is not installed.**
`phoenix_live_reload` depends on `file_system`, which shells out to the
`inotifywait` executable. Without it the app still boots and serves, but logs
`` `inotify-tools` is needed to run `file_system` `` and
`{:error, :fs_inotify_bootstrap_error}` — and **live reload silently does
nothing**. Fix is one command: `sudo apt install inotify-tools` (3.22.6.0-4 is
available; it needs your password). There is no sudo-free path — upstream ships
source tarballs only, and `autoconf`/`make` are absent. Affects task 6.2
(live reload) and Wave 3 (log watching). See task 1.5 for why this makes a
poll-based watcher worth reconsidering as the primary implementation rather than
a fallback.

**`erl_crash.dump` on a clean exit.** Invoking `erl` from a captured-output
pipeline can produce a crash dump whose slogan is a boot-time `badarg` writing
to `standard_error` ("the device does not exist"). It is not an application
error — ignore the file, now gitignored.

---

## Migration reference (stable — unlikely to change)

### The plan and the specs

| | Authority |
|---|---|
| `docs/phoenix-migration-plan.md` | **Sequencing**, architecture, wave order, risk |
| `openspec/specs/` (after archiving) | **Behavior** — the durable contract |
| `openspec/changes/<wave>/tasks.md` | What to do, and how each task is verified |

When they disagree: the plan wins on *order*, the spec wins on *behavior*.

Wave → change mapping: Wave 0 = `add-phoenix-scaffold` (34 tasks), Wave 1 =
`add-sidebar-navigation` (27), Wave 2 = `add-data-model` (55). **Waves 3–11 have
no changes written yet.**

```bash
openspec list                                   # active changes + progress
openspec show <change>                          # full contents
openspec validate <change> --strict             # must pass before archiving
openspec instructions apply --change <change>   # work the task list
openspec archive <change>                       # promote deltas into specs/
```

### Porting rules (short version — `AGENTS.md` has the full set)

1. `backend/`, `frontend/`, `electron/` are **FROZEN REFERENCE**. Read, never
   edit. Each wave's closing task asserts the remaining reference trees are
   untouched.
2. **Delete-as-you-go.** Delete a reference package in the same change that lands
   its port, once tests are green. Nothing is lost — the Go app lives permanently
   on `upstream/main`, so `git show upstream/main:<path>` recovers anything.
   Waves 3–6 also record one golden-file parity pass before deleting.
3. **Port the reference tests before the code they exercise.** Ranked assets:
   Appendix C of the plan. Highest value: `internal/logparser/*_test.go` (real EQ
   log lines) and `internal/zealpipe/*_test.go` (reverse-engineered wire format).
4. **Do not port** `internal/converter` or `internal/mapgen` — build-time tooling,
   stays in Go, runs in GitHub Actions.
5. `LIMITATIONS.md` documents 18 known failure modes; each is a regression test.
   §13.1 (broadcast-everything hub) is fixed outright by the PubSub design.

### Decisions already taken (don't re-litigate)

- **Shell (Electron vs Tauri) deferred** behind a `PQ.Shell` behaviour.
  `PQ.Shell.Browser` ships first so nothing before Wave 8 is blocked.
- **Clean-room rewrite**, no dual-running.
- **Delete-as-you-go**, not delete-at-the-end.
- **`types/` (3,922 LOC) and `enumsCache.ts` are deleted, not ported.** Special
  abilities collapses from **three** copies to one. `combatantColor.ts`,
  `chChainPatterns.ts`, `overlayTextStyle.ts` stay separate — presentation, not
  duplication.
- **Two build spaces.** Source is Windows-visible on `/mnt/c`; build artefacts
  live on ext4 via symlinks (~25× faster). Windows never runs `mix` — see the
  Windows strategy below.
- **No `npm`, no `package.json`.** Verified: the `esbuild` and `tailwind` hex
  packages download standalone platform binaries, and `mix assets.build`
  succeeds with `node` absent from PATH. Tailwind resolves to v4.3.0, the exact
  version the reference pins, so its `@theme` tokens port verbatim. `floki` was
  dropped as redundant (`lazy_html` is the Phoenix 1.8 default). daisyUI was
  removed — see task 2.5 for the component restyle it forces.

### Windows strategy — Burrito, and what Windows is actually for

Windows is needed for three things, and **Docker cannot provide any of them**
(verified 2026-09-18: the engine is `OSType=linux`, and pulling a Windows base
image fails with `no matching manifest for linux/amd64/v3`; containers are also
headless, so they can't host transparent overlays over a DirectX game, and
container isolation severs the host-process visibility Zeal's `\\.\pipe\zeal_<pid>`
needs).

| Need | How it's met |
|---|---|
| Build the Windows artifact | **Burrito**, cross-building from WSL |
| Run the deliverable | The Burrito `.exe` (bundles ERTS — no Elixir install on Windows) |
| Zeal named pipe | The Burrito `.exe`, running natively beside the game |

**Burrito is the key decision.** Its README states it directly: *"We support
targeting Windows (x86_64) from MacOS and Linux, we do not officially support
building ON Windows, it's recommended you use WSL if your development machine is
Windows."* That is this setup exactly. It produces a self-extracting archive
bundling the BEAM code + the target ERTS + NIF artifacts, so the Windows side
never needs an Elixir install at all.

Toolchain installed via mise, no sudo: **zig 0.15.2** (Burrito requires exactly
0.15.2), **7zip 26.03**, and `xz` (already present). Note mise ships the binary as
`7zz`, while Burrito looks for `7z` — a shim was added at `~/.local/bin/7z`.

Use **`skip_nifs: true`** and drop in Exqlite's precompiled Windows NIF rather
than asking Zig to cross-compile SQLite. That NIF is confirmed to exist:
`exqlite-nif-2.17-x86_64-windows-msvc-0.40.0.tar.gz` is in exqlite's checksum
matrix, with `{:win32, :nt} => %{include_default_ones: true}` in `cc_precompiler`
— so Windows needs **no C toolchain** on the primary path. This sharply lowers
the "Exqlite has no usable Windows NIF" risk that `add-phoenix-scaffold`'s design
originally rated High.

**Still open for Wave 11:** Burrito builds the Elixir *sidecar*, not the GUI. If
the shell is Electron, that needs its own Windows build (electron-builder can
target Windows from Linux via wine, or build natively — the reference already has
a working Windows electron-builder flow). Wave 11 is two artifacts, not one.

**Still unresolved:** whether a Windows-native Elixir install is needed for tasks
1.4/1.5 validation. A Burrito build per check is slow to iterate on, so a native
install may still be the pragmatic choice for *validation*. Do not assume Burrito
removes that need for Phase 0.
