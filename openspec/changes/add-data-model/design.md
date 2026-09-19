## Context

See proposal.md — Why. Current state that shapes this design:

- The user schema exists only as inline `CREATE TABLE IF NOT EXISTS` DDL spread
  across ~18 Go store packages, plus ad-hoc `ALTER TABLE` upgrade statements. There
  is no migration-version table, so the current schema of any given install is
  whatever that install's history produced.
- The game database is a shipped artifact built by `internal/converter` in the
  `data-release` workflow (see `docs/db-pipeline.md`). It is not migrated at
  runtime — it is replaced wholesale by app updates.
- The reference implements the raw-code → label catalog twice: Go
  `internal/db/enums` and frontend `enumsCache.ts`. `docs/enum-audit.md` is a
  read-only survey of that duplication.
- The reference's app-state backup/export swaps `user.db` as a file.
- The reference's `config.go` is 1,494 LOC with ~186 fields across 13 structs and
  already performs atomic saves (temp then rename).
- Wave 0 opened both databases with minimal schemas; `data-store` is an existing
  capability, so this change extends it rather than replacing it.

## Goals / Non-Goals

**Goals:**

- Every table in both databases has a schema, and parity is enforced by test.
- Adopting an existing `user.db` is safe, explicit, recoverable, and loud.
- `config.yaml` becomes a validated schema with a single writer and live
  propagation, with key names and defaults frozen.
- The duplicated enum catalog collapses to one definition.
- Game-math behavior is provably identical to the reference.

**Non-Goals:**

- Changing any user-data schema. This change describes the existing shape; it does
  not reshape it. Normalising name-keyed tables to identifier keys is explicitly
  out of scope (see D6).
- Porting the reference's HTTP handlers or query-layer DTOs — those are per-feature
  (Waves 4–10).
- Touching `quarm.db`: it is replaced, never migrated.
- Building feature UI on top of the model.

## Decisions

### D1. Ecto migrations that mirror the reference DDL, using create-if-not-exists semantics

**Decision:** express the user schema as Ecto migrations whose statements are
equivalent to the reference's DDL, so a fresh install and an adopted install
converge on the same schema. Accept that Ecto adds a `schema_migrations` table to
`user.db`.

**Why:** the alternative — no migrations at all, relying on schemas over whatever
the Go app created — means a fresh Elixir-only install has no way to create the
schema, so new users cannot be supported. The extra `schema_migrations` table is
inert (the reference app does not care about unknown tables) and it gives the
Elixir app a capability the reference never had.

**Alternative considered:** keeping the Go DDL as the single source of truth and
having Elixir apply it verbatim. Rejected — it forks schema ownership across two
languages for the duration of the migration, which is the drift this change is
trying to eliminate.

### D2. Adoption is explicit, not automatic on boot

**Decision:** a dedicated `mix pq.db.adopt` task performs adoption. On boot the
application *checks* schema compatibility: compatible installs run; incompatible
installs refuse to start with an actionable message naming the task to run.

**Why:** auto-migrating a 50-table file of irreplaceable user data on every boot is
how a bad migration silently destroys a user's history. Making adoption explicit
means it happens once, is backed up, and is observable. Refusing to start — rather
than starting degraded — means the failure is seen immediately instead of after
the user has been playing for an hour on a half-read database.

**Trade-off:** a manual step on first run after the upgrade. Acceptable: the
alternative failure mode is data loss, and the installer can invoke the task.

### D3. `mix pq.db.adopt` reports rather than repairs

**Decision:** the adoption task snapshots, compares, and either applies known
migrations or fails. It never guesses at an unknown column's meaning and never
drops anything. Tables with no schema are reported as unmapped, not deleted.

**Why:** an unknown column means either the install is newer than this build's
schemas (a downgrade) or something else wrote to the file. Both need a human. A
repair-by-guess is the one behavior that makes data loss possible, and the task's
job is to make drift visible — including revealing dead tables, of which the
reference already has at least one candidate pair in trigger categories.

### D4. Generate the schema modules from the reference DDL, then review them

**Decision:** write a one-off generator that reads the reference's DDL and emits
Ecto schema skeletons, then hand-review and hand-finish each schema (associations,
primary keys, typed columns).

**Why:** 65 tables of field lists is a mechanical transcription task that is
error-prone by hand and boring to review. Generating gets the field lists exactly
right (which is what the parity test then verifies) and leaves human attention for
the parts that need judgement: which columns are enumerations, which are
serialized, which are identifiers.

**Alternative considered:** hand-writing all 65. Rejected on transcription-risk
grounds alone. The generated skeleton is not the deliverable — the reviewed schema
and its parity test are.

### D5. Serialized columns decode strictly and re-encode losslessly

**Decision:** text columns holding structured data become `Ecto.Type`
implementations. Decoding raises on malformed input; encoding preserves the
reference's exact serialized form.

**Why:** the reference's JSON blobs are embedded in user data *and* in export
bundles, so the shape is a public contract, not an implementation detail.
Strict decoding is the point: a permissive decoder that silently drops unknown
fields is how a later write-back quietly deletes data the user set. Raising names
the table and column so the problem is diagnosable; a default value would hide it.

**Trade-off:** a genuinely corrupt row makes a page fail instead of rendering
partially. Accepted — the reference's failure mode here (silently dropping
unrecognised action fields) is worse, and a partial render looks like working
software.

### D6. Name-keyed tables stay name-keyed

**Decision:** preserve the reference's use of character *names* as keys where it
does so, and expose name-based lookup as a first-class accessor.

**Why:** the log pipeline identifies characters by name, because the game logs
name them and no identifier is available at parse time. Several tables key on the
name string for that reason. Converting them to identifier keys would mean a
translation step on the hottest write path in the app, and would break the
reference's ability to read the file for rollback. The spec therefore requires
that name lookup works, and that a mismatch is *reported* rather than silently
presented as absent data — which is the actual failure mode worth preventing
(wrong character, empty page, user thinks their data is gone).

### D7. One enum catalog, generated from a data file, guarded by a test

**Decision:** a single `PQCompanion.Game.Enums` module backed by one data source, with the
reference's audit test ported as the guard that no second definition appears.

**Why:** the duplication is already documented as a problem
(`docs/enum-audit.md`), and it is exactly the kind of drift that a rewrite should
delete rather than reproduce. The audit test is cheap and is the only thing that
keeps the duplication from reappearing once a second consumer wants a slightly
different shape.

### D8. Settings as an embedded schema with one owning process

**Decision:** `PQCompanion.Config` is an `embedded_schema` with changesets; a
`PQCompanion.Config.Server` GenServer owns the current value and serialises writes.

**Why:** embedded schema is the Ecto-idiomatic representation for validated data
that is not a database row, and changesets give field-level validation for free
across ~186 fields — which the reference implements by hand. A single owner is
required by the atomic-save requirement: two processes writing temp files and
renaming concurrently can interleave and produce a file that contains a mix.

**Alternative considered:** storing settings in a `user.db` table. Rejected — it
breaks existing installs, breaks the reference app's ability to read settings for
rollback, and makes the settings file unrecoverable by hand.

### D9. Settings change notifications carry their origin

**Decision:** the settings-changed broadcast identifies the window that made the
change.

**Why:** without it, a window that saved a settings change receives its own
notification and re-renders or re-saves, which is how feedback loops start.
The reference already distinguishes the `config:updated` and
`config:character_detected` events, so this preserves existing behavior rather
than inventing a new concern.

## Risks / Trade-offs

- **[Write-ahead logging breaks the reference's file-copy backup]** → Enabling WAL
  adds `-wal` and `-shm` sidecar files, so the reference's app-backup, which swaps
  `user.db` as a single file, can lose recent transactions. Mitigation: force a
  checkpoint before any backup, export or file swap, and verify the exported bundle
  is readable by the reference application. This is a concrete interaction with
  `app-backup` and is called out in the tasks.
- **[SQLite type affinity differs between the reference driver and Exqlite]** →
  The parity test normalizes type names (e.g. boolean/integer, datetime/text)
  before comparing, rather than comparing raw declared types, so a driver's
  reporting convention cannot masquerade as drift.
- **[A dead table gets ported as if it were live]** → Adoption reports unmapped
  tables; the trigger-category pair is resolved by inspecting the reference's read
  path before writing the schema, not by assuming the newer name is the live one.
- **[65 schemas is a large change to review]** → Generated field lists plus a
  mechanical parity test mean review attention goes to associations, keys and typed
  columns. Splitting the change by domain was considered and rejected: the parity
  test and adoption task only have meaning once all tables are covered, and a
  half-covered schema set would make adoption unusable.
- **[Case sensitivity of name lookups]** → Preserve the reference's comparison
  semantics exactly; a case-insensitive lookup that the reference did not have
  could match the wrong character. Covered by a scenario asserting a mismatch is
  reported.
- **[Strict serialized decoding surfaces pre-existing corrupt rows]** → Expected and
  desirable, but it means adoption may reveal bad data. The adoption task reports
  rows that fail to decode rather than blocking on them, so a user is told before
  the app fails on that page.

## Migration Plan

1. Generate and review the game-data schemas; add the parity test. Game data is
   read-only and replaceable, so this is the low-risk half and it validates the
   generator before it touches user data.
2. Generate and review the user-data schemas; add the parity test against a
   captured fixture database.
3. Add the migrations and verify a fresh database converges to the same schema as
   an adopted one.
4. Ship `mix pq.db.adopt`; run it against a captured real `user.db` and verify row
   counts are unchanged and a backup was taken.
5. Add `PQCompanion.Config` and `PQCompanion.Config.Server` with defaults ported from the reference's
   test file; verify round-trip and reference readability.
6. Wire the boot-time compatibility check and the change broadcast.

**Rollback:** the reference application remains the shipping build. A user who
runs adoption against a copy of their data can restore from the backup the task
took. Because the settings keys and every serialized shape are preserved, the
reference app can read both files after the migrated app has written to them —
that is the rollback guarantee, and it is tested rather than assumed.

## Open Questions

- Should the installer run adoption automatically on upgrade, or present it as an
  explicit first-run step? Deferrable — it does not change the adoption contract,
  only who invokes it, and the boot-time refusal gives a clear signal either way.
- Does the game database need indexes added by the application, or does the
  artifact ship fully indexed? Deferrable to the game-database change; the
  reference's indexes, if any, are part of the artifact.
