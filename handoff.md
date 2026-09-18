# Handoff — PQ Companion → Elixir/Phoenix migration (2026-09-18, Wave 0 bootstrap)

> **This is the MIGRATION handoff.** The reference app's handoff is a different
> file in a different tree — `handoff.md` in `/mnt/c/Users/eddyn/pq-companion`
> (raidcomp / Playwright era). Do not merge the two. This one is specific to
> `feat/phoenix-migration` and the `~/pq-companion-phoenix` worktree.

## SESSION UPDATE — 2026-09-18 (migration bootstrapped; Wave 0 blocked on toolchain)

**Read this first — where it conflicts with anything below, this wins.**

### Where the trees are — they moved this session

| Tree | Path | Filesystem | Branch | Role |
|---|---|---|---|---|
| Migration | `/home/nunez/pq-companion-phoenix` | **ext4** (`/dev/sdd`) | `feat/phoenix-migration` @ `0aa89896` | The rewrite. All new work. |
| Reference | `/mnt/c/Users/eddyn/pq-companion` | Windows (9p) | `feat/raidcomp-pack` @ `81816394` | Shipping Go app. Frozen reference. |

`git worktree list` from either tree shows both. The migration tree **moved off
`/mnt/c` this session** — see the 9p gotcha below for why. The move was done with
`git worktree remove` + `git worktree add`, not `git worktree move` (which fails
cross-device: `Invalid cross-device link`).

`~/pq-companion-phoenix` is on ext4, so it is **not** directly reachable from
Windows Explorer. Windows-side work reaches it via
`\\wsl$\Ubuntu\home\nunez\pq-companion-phoenix`, or use a second detached
worktree on `/mnt/c` if that proves awkward. This trade was deliberate: the 9p
penalty is catastrophic for Elixir and the Windows side only needs the source
occasionally.

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

### What's next — the one blocker

**Phase 0 task 1.1: install the toolchain.** This is the only thing standing
between here and `mix phx.new`. Decision taken 2026-09-18: **native `mise` in
WSL**, not Docker. `mise` is not installed yet.

```bash
curl https://mise.run | sh                      # -> ~/.local/bin/mise (no sudo)
cd ~/pq-companion-phoenix
mise use -g erlang@27.3.4.17 elixir@1.18.5-otp-27
elixir --version && erl -noshell -eval 'io:format("~s~n",[erlang:system_info(otp_release)]),halt().'
```

**Watch for:** mise's `erlang` plugin may fall back to a **source build**, which
cannot work here (no `make`, no `autoconf`, no build deps, and no `sudo` to get
them). It must use precompiled binaries. Verified available:
`builds.hex.pm/builds/otp/ubuntu-24.04/OTP-27.2.tar.gz` → 200 and
`builds.hex.pm/builds/elixir/v1.18.5-otp-27.zip` → 200. If mise tries to
compile, force the precompiled path or fall back to extracting those two
artifacts into `~/.local` by hand. Then pin the result in `.tool-versions`.

After that: task 1.2 (`mix phx.new phoenix --app pq_companion --module
PQCompanion --database sqlite3 --no-mailer --no-gettext`), then 1.3 (deps).

### Environment state

**WSL** — Ubuntu 24.04.1 LTS, WSL2, 12 cores, 15 GB RAM, 941 GB free on ext4.
`systemd` is running (`/etc/wsl.conf` has `boot.systemd=true`).

| Tool | State |
|---|---|
| `elixir` / `mix` / `erl` / `iex` | **MISSING** — the blocker |
| `mise` / `asdf` / `kerl` | MISSING |
| `curl` `unzip` `git` `python3` `pip3` `gcc` | present |
| `make` `autoconf` `m4` `jq` | MISSING |
| `~/.local/bin` on `PATH` | yes, already |
| `gh` CLI | MISSING — use `curl` for `quarm.db` |

**`sudo` requires a password.** No system package installs are possible without
you. `apt` offers Elixir **1.14** / Erlang **25** (too old), Erlang Solutions has
**no `noble` repo** (404), and there are no build deps — so precompiled is the
only path, which is why mise is the plan.

**Windows** — has git (PortableGit), node, npm. Has **no** Elixir/Erlang/mise.
Docker Desktop is installed and `docker` works from WSL (the Windows
`com.docker.service` shows Stopped, which is normal — the engine runs in the
`docker-desktop` WSL distro).

**`quarm.db` is absent** from both trees (`*.db` is gitignored). Needed from
task 4.3 onward: `curl -L -o backend/data/quarm.db
https://github.com/jasonsoprovich/pq-companion/releases/download/data-latest/quarm.db`.

### User-side recommendation

**Install Erlang/Elixir on Windows natively** (the official Elixir installer
bundles OTP). This is unavoidable and not a Docker/mise question: Phase 0 tasks
**1.4** (Exqlite Windows NIF loads) and **1.5** (`file_system` Windows watching),
and Wave 3's named-pipe spike, can only be validated on Windows. A Linux
container or WSL would test the Linux NIF and prove nothing — 1.4 exists
specifically to settle whether `ecto_sqlite3` is viable as the data layer.

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
- **Two build spaces.** WSL/ext4 for dev; Windows for NIF/watcher/named-pipe
  validation. `_build` and `deps` must never be shared between them (NIFs are
  platform-specific: `.so` vs `.dll`).
