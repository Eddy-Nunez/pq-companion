## Why

Everything above the data layer depends on it, and this is the last point in the
migration where the data model can be got wrong cheaply (`docs/phoenix-migration-plan.md`,
**Wave 2**). Two risks drive the shape of this change:

1. **Real user data must survive.** `user.db` has 50 tables of live data and,
   in the reference, no migration-version table — its schema is created by inline
   `CREATE TABLE IF NOT EXISTS` DDL scattered across Go stores, plus ad-hoc
   `ALTER TABLE` upgrades. The migration must adopt that file in place, and a
   silently-ignored column is how data loss happens.
2. **The on-disk formats are public contracts.** `config.yaml` key names are user
   data, and the app-state export bundle embeds schema shapes and JSON blobs.
   Both must stay readable by the reference application for rollback.

## What Changes

- Define Ecto schemas for all **15 game-data tables** (`quarm.db`) and all **50
  user-data tables** (`user.db`), with repository-wide field-parity tests against
  `PRAGMA table_info`.
- Add `mix pq.db.adopt`: snapshot the user database, compare every schema's
  fields against the live tables, fail loudly on drift, and report tables present
  in SQLite but absent from the schemas.
- Map the reference's serialized TEXT columns (trigger actions and alerts,
  combatant and healer lists, upgrade weights, parse results, role class codes) to
  `Ecto.Type` embedded schemas **without changing their on-disk JSON shape**.
- Collapse the reference's duplicated game-data catalogs into single definitions.
  The sharpest is special abilities, which exists in **three** places today — Go
  `db/special_abilities.go` (parsing), Go `db/enums/special_abilities.go`
  (labels), and `frontend/src/lib/npcHelpers.ts` (both, 175 LOC) — and the
  reference's own `claude.md` documents the mirror. The game-data members of the
  31 files containing code→label maps collapse too, with the reference's audit
  test ported as the anti-duplication guard.
- Delete the `frontend/src/types/` tree (3,922 LOC, 35 files) rather than port it.
  It exists only because of the Go↔TS boundary and has no Elixir equivalent —
  server-rendered HEEx reads the schema structs directly.
- Delete the enum fetch-and-cache layer rather than port it. `enumsCache.ts`
  (177 LOC) is a runtime *cache* hydrated from the backend, not a second
  definition — but on fetch failure it silently renders stub labels such as
  "Tradeskill 75", which is a real bug class that server-side lookup removes.
- Port the pure game-math functions the schema layer sits on, using the
  reference's table-driven tests as the contract.
- Introduce `PQ.Config` as an `Ecto.Schema` with changesets over `config.yaml`,
  owned by a single `PQ.Config.Server` that performs atomic saves and broadcasts
  changes to subscribers.

Not in this change: the features that read this model (Waves 4–10), and the
adoption of *feature* data beyond schema-level parity.

## Capabilities

### New Capabilities

- `settings`: Application settings stored in the reference's YAML file — schema
  and validation, a single writer, atomic persistence, defaults, and live
  propagation of changes to every open window.

### Modified Capabilities

- `data-store`: extends the Wave 0 capability from "both databases open, game
  data write-protected" to the full schema set, field parity, safe adoption of an
  existing user database, typed serialized columns, a single game-code catalog,
  and ported game-math behavior.

## Impact

- **Affected specs:** `data-store` (ADDED requirements), `settings` (new)
- **Affected code:** new tree `phoenix/` — `PQ.Game.*` schemas,
  `PQ.Character.*`, `PQ.Trigger.*`, `PQ.Combat.*`, `PQ.Config.*`, `PQ.Game.Enums`;
  `priv/repo/migrations/`; `lib/mix/tasks/pq.db.adopt.ex`
- **Ported from (frozen reference, read-only):**
  - `backend/internal/db/*.go` — 11,187 LOC production, 3,719 LOC tests; game
    schemas, queries and the pure game-math functions
  - `backend/internal/db/enums/` — 2,662 LOC across 21 files; the canonical
    catalog (`spell.go` alone is ~16.5 KB, `npc_race.go` ~9 KB)
  - `frontend/src/lib/npcHelpers.ts` — 175 LOC; a true mirror of special-ability
    parsing *and* labels, the duplication the reference's `claude.md` admits to
  - `frontend/src/lib/enumsCache.ts` — 177 LOC; a runtime cache of the backend
    catalog with a stub-label fallback, deleted rather than ported
  - `frontend/src/types/` — 3,922 LOC across 35 files; mechanical mirrors of Go
    structs, deleted rather than ported
  - `docs/enum-audit.md` — documents the duplication and the provenance chain
    (EQMacEmu → EQEmu → Quarm tweaks) that determines which label wins; also
    records the `targettype=4` label bug found by hand, which is the evidence
    that no parity guard exists today
  - `.github/workflows/ci.yml` — no enum/parity/generation check; the absence is
    why this change adds an anti-duplication guard
  - `backend/cmd/enum-audit` — coverage-vs-database audit worth porting as-is
  - `backend/internal/character/*.go` — 1,907 LOC production, 767 LOC tests
  - `backend/internal/config/config.go` — 1,494 LOC production, 482 LOC tests;
    the settings schema and the atomic-save behavior
  - The inline `CREATE TABLE` DDL across the ~18 user-data store packages, which
    is the only existing definition of the user schema
  - `SCHEMA.md` — the reference's own schema documentation
  - `docs/db-pipeline.md` — how the game database artifact is produced
- **Reference behavior that must be preserved:** `config.yaml` key names; the
  JSON shape of every serialized TEXT column; the game database read-only
  contract; character identity being usable by **name** (several tables key on a
  character name string, not the integer id, because the log parser identifies
  characters by name).
- **Known orphan risk:** `openspec` adoption must report unexpected tables. The
  reference has at least one dead pair in trigger categories (the id-keyed
  `trigger_categories_new` alongside the older `trigger_categories`) — confirm
  which is live before porting, and surface any others `mix pq.db.adopt` finds.
- **Risk:** **High.** This is the change where a mistake is destructive. The
  snapshot-before-migrate and fail-loudly-on-drift requirements exist specifically
  to make a mistake recoverable and visible.
