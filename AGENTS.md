# AGENTS.md — agent session notes

## Which checkout am I in?

This is the **migration tree**:

- Branch: `feat/phoenix-migration`, branched from `upstream/main` @ `d2c35557`
- Path: `/mnt/c/Users/eddyn/pq-companion-phoenix` (= `C:\Users\eddyn\pq-companion-phoenix`)
- Worktree — `.git` here is a **file**, not a directory
- The **original checkout** (`/mnt/c/Users/eddyn/pq-companion`) is the shipping Go
  app on its own branches. **Do not do migration work there.** Run
  `git worktree list` to see both.

## Fork posture — this migration is NOT for upstream

This is a **personal aspiration and a permanent fork divergence.** It will never
be proposed to the maintainer's repo. Consequences that must be respected:

- **Never open a PR to `upstream`** from `feat/phoenix-migration`, and never push
  this branch anywhere but `origin` (the fork). A whole-app rewrite is not
  reviewable as a PR and is not wanted upstream.
- **`upstream` is a read-only reference.** It exists so this tree can be based on
  and re-based onto the canonical Go app while that app is still the thing being
  ported. Fetch it; never push to it.
- The branch is **long-lived and expected to rebase** onto moving
  `upstream/main`. Force-pushes to `origin/feat/phoenix-migration` after a rebase
  are normal. Do not open PRs from it, do not treat its history as stable.
- At **Wave 11** the reference trees are deleted and this branch becomes the
  fork's `main`. The Go app continues to live upstream.

## Git remote naming (ALWAYS use these terms — never swap them)

- **`origin`** = `git@github.com:Eddy-Nunez/pq-companion.git` — **this fork** (the
  user's own repo, Eddy-Nunez). All feature branches live here, including
  `feat/phoenix-migration`.
- **`upstream`** = `https://github.com/jasonsoprovich/pq-companion.git` — the
  **forked originator** (maintainer's repo, jasonsoprovich). Canonical `main`;
  it is the migration's **reference and rebase base**, nothing more.

Shorthand when talking to the user: "origin" means *my fork*, "upstream" means
*the maintainer's repo I forked from*. Do not call upstream "origin" or the
fork "upstream" — the handoff docs and PR descriptions depend on this wording.

## Two build spaces — WSL and Windows

The target is Windows, but the primary development environment is WSL. Both are
pointed at this same directory (`/mnt/c/...` is the Windows filesystem), which
makes one thing dangerous:

> **`_build/` and `deps/` artifacts must never be shared between the WSL side and
> the Windows side.** Compiled NIFs are platform-specific (`.so` vs `.dll`) —
> Exqlite is a NIF, and it is the data layer. Sharing a build directory across
> the two sides produces confusing load failures.

Rules:

- **WSL: everything by default.** Day-to-day dev, `mix test`, `mix format`,
  `openspec`, all feature work. This is the fast path.
- **Windows: only what must actually run on Windows.** At minimum, and *early*:
  - the Exqlite NIF load (Phase 0 task 1.4) — this decides whether `ecto_sqlite3`
    is viable at all, and it is the one check that is worthless in WSL
  - `file_system` directory watching (Phase 0 task 1.5) — fallback is a polling
    GenServer if the Windows backend is unavailable
  - the named-pipe bridge spike (Wave 3)
  - anything touching the real EQ directory, Zeal exports, or overlay windows
- **Separate the build path per side.** Set `MIX_BUILD_PATH` to a
  platform-specific directory (e.g. `_build/wsl` / `_build/win`). Verify during
  Phase 0 task 1.4 whether any dependency's native build also writes into
  `deps/`; if it does, give the Windows side its own **detached** worktree
  (`git worktree add --detach`) rather than fighting the build path.
- Never `npm install` from WSL into either tree — the reference warns that this
  breaks the Windows electron binaries. The Elixir app should need no `npm` at
  all; if it does, that is worth questioning.

## Elixir/Phoenix migration — read this before writing any code

Two documents govern this work:

- **`docs/phoenix-migration-plan.md`** — the master plan: architecture assessment,
  the five hard constraints, target design, and the 12-wave roadmap. **Wins on
  sequencing.**
- **`openspec/`** — the executable breakdown: one change per wave, with proposal,
  delta specs, design and tasks. **Wins on behavior.**

Working rules:

1. **`backend/`, `frontend/` and `electron/` are FROZEN REFERENCE.** Read them for
   behavior, fixtures, DDL and tests. **Never edit them.**
2. **Delete-as-you-go.** When a port lands with green tests, delete the reference
   package it replaces **in the same change**. Do not hold the Go tree until
   Wave 11. Nothing is lost — the Go app lives permanently on `upstream/main`, so
   any deleted file is one `git show upstream/main:backend/internal/<pkg>/<file>`
   away. Waves 3–6 also record one golden-file parity pass (same input through
   both apps, diffed) before each deletion. Every wave's closing task states what
   was deleted and asserts the *remaining* reference trees are untouched.
3. **All new code goes in `phoenix/`.** It does not exist yet — Wave 0 creates it.
   Never co-locate a Go and an Elixir implementation of the same behavior in one
   folder.
4. **Port the reference tests before the code they exercise.** They are the only
   safety net for a clean-room rewrite. Ranked porting assets: Appendix C of the
   migration plan. Highest value: `internal/logparser/*_test.go` (real EQ log
   lines) and `internal/zealpipe/*_test.go` (reverse-engineered wire format).
5. **Do not port `internal/converter` or `internal/mapgen`** — build-time tooling;
   it stays in Go and runs in GitHub Actions.
6. Read `LIMITATIONS.md` alongside the specs. It documents 18 sections of known
   failure modes; each is a regression test, and several are fixed outright by the
   PubSub design (notably §13.1).
7. `quarm.db` is **not** in this tree (`*.db` is gitignored). Download it from the
   `data-latest` release into `backend/data/` before any data-backed test.

### Duplicated logic — the short version

Full policy: **Part IX** of the migration plan. What matters while working:

- The reference has **no CI parity guard**, so Go and TypeScript drift silently.
  A real label bug (`spell targettype=4` → "Single (Pet)" instead of "PB AE",
  **325 spells**) was found by a human reading a report, not by a test.
- **`frontend/src/types/` (3,922 LOC, 35 files) and `enumsCache.ts` (177 LOC) are
  deleted, not ported.** The types tree exists only because of the Go↔TS seam;
  the cache exists only to hydrate labels client-side and silently renders stub
  labels like `"Tradeskill 75"` when its fetch fails.
- **Special abilities is duplicated three times** — Go `db/special_abilities.go`
  (parsing), Go `db/enums/special_abilities.go` (labels), and
  `frontend/src/lib/npcHelpers.ts` (both). It collapses to one definition.
- **Not everything that looks duplicated is.** `combatantColor.ts`,
  `chChainPatterns.ts`, `overlayTextStyle.ts` are presentation and stay separate.
  Merging them would push UI decisions into the data layer.
- **The anti-drift rule:** if a remaining JS hook needs labels or codes, the
  server renders them into the page payload. The hook never carries its own copy.
- **Grep the reference before implementing anything non-trivial.** There are 31
  files of code→label maps; check whether the fact already exists once, and
  collapse rather than re-copy.

### OpenSpec workflow

```bash
openspec list                                   # active changes + task progress
openspec show <change>                          # full change contents
openspec validate <change> --strict             # must pass before archiving
openspec instructions apply --change <change>   # work the task list
openspec archive <change>                       # promote deltas into openspec/specs/
```

- One change per wave. Wave 0 = `add-phoenix-scaffold`, Wave 1 =
  `add-sidebar-navigation`, Wave 2 = `add-data-model`. Waves 3–11 are not yet
  written.
- **Every task must state how it is verified.** Mark a task `[x]` only with its
  verification result recorded.
- Specs describe the **Elixir app's behavior**, not the Go app and not the
  migration process. After all waves archive, `openspec/specs/` is the complete
  behavioral contract and doubles as the parity checklist.
- `openspec/config.yaml` holds the project context and per-artifact rules — read it
  before writing a proposal.
- The `docs/` directory is mostly gitignored working-doc space; only whitelisted
  plans are committed. `docs/phoenix-migration-plan.md` is whitelisted.

## History notes that still apply

The original checkout's `handoff.md` records hard-won details about the reference
app that are easy to rediscover painfully. The two worth repeating here:

- `electron-builder` strips `package.json`'s entire `scripts` block on **every**
  build. If the reference tree is ever built from this branch, restore it with
  `git checkout -- package.json` immediately afterwards.
- Zeal's raid roster wire format did not match the reference decoder's assumption
  for a long time. When porting `internal/zealpipe`, treat the reference's tests —
  not its prose — as the contract.
