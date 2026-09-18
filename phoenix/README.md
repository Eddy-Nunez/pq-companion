# PQ Companion — Phoenix app (`phoenix/`)

The Elixir/Phoenix rewrite of PQ Companion. The Go + Electron + React app in
`../backend`, `../frontend` and `../electron` is the **frozen reference**: read
it for behaviour, fixtures and DDL, never edit it. See `../AGENTS.md` for the
migration rules and `../docs/phoenix-migration-plan.md` for the plan and wave
order.

## Prerequisites

- **Erlang/OTP 27 + Elixir 1.18** — pinned in `../.tool-versions`; `mise` installs
  and activates them.
- **The game database** at `priv/data/quarm.db`. It is ~86 MB and deliberately not
  in version control (`*.db` is gitignored). Fetch it once:

  ```bash
  curl -L -o priv/data/quarm.db \
    https://github.com/jasonsoprovich/pq-companion/releases/download/data-latest/quarm.db
  ```

  Without it the app refuses to start outside `MIX_ENV=test`, and tells you the
  expected path and this command.
- **WSL/Linux live reload** needs `inotify-tools`
  (`sudo apt install inotify-tools`). Without it the app still serves, but file
  watching logs `fs_inotify_bootstrap_error` and edits are not picked up.

## One-command development startup

```bash
cd phoenix
mix dev
```

`mix dev` fetches dependencies, creates and migrates the user database, builds
the assets, and starts the server — no `npm`, no separate asset process to run.

Once it is up:

- the main window is <http://localhost:4000/>
- each overlay is `/w/<slug>` — `/w/dps`, `/w/buff-timer`, `/w/npc`, …

**The port may not be 4000.** If the preferred port is busy the server binds an
OS-assigned free port. The actual port is announced in two places:

- `~/.pq-companion/runtime.json` — `{"port": …, "pid": …, "version": …}`
- the same record as one JSON line on stdout

Read the record rather than assuming 4000; a native shell does exactly this.

## Common commands

| Command | What |
|---|---|
| `mix dev` | One-command startup (deps, database, assets, server) |
| `mix setup` | Everything `mix dev` does except starting the server |
| `mix test` | The suite; data-backed tests skip with a warning when `quarm.db` is absent |
| `mix compile --warnings-as-errors` | Compile gate |
| `mix format` | Format |
| `mix assets.build` | Build CSS/JS without a server |
| `mix ecto.reset` | Drop and recreate the development user database |

## Where things live

| Area | Module(s) |
|---|---|
| Application home / paths | `PQCompanion.Paths` (`~/.pq-companion`, `config.yaml`, `user.db`, `backups/`, `logs/`) |
| User database | `PQCompanion.UserRepo` — read-write, `~/.pq-companion/user.db` |
| Game database | `PQCompanion.QuarmDatabase` (read-only connection) behind `PQCompanion.QuarmRepo` (read-only facade; writes raise) |
| Native-shell contract | `PQCompanion.Shell` + `PQCompanion.Shell.Browser`; live updates in `PQCompanionWeb.ShellEvents` |
| Runtime announcement | `PQCompanion.Runtime` (`runtime.json` + stdout line) |
| Window registry | `PQCompanion.Windows` (main window + 16 overlays) |
| Layouts | `PQCompanionWeb.Layouts` — main window, overlay, bare |
