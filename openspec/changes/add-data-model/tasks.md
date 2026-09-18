## 1. Game-data schemas (quarm.db, 15 tables)

- [ ] 1.1 Write a one-off generator that reads the reference's game-data DDL and emits Ecto schema skeletons (design D4); verify regenerating produces no diff against the committed skeletons
- [ ] 1.2 Review and finish the 15 game-data schemas — `items` (~27k rows), `spells_new`, `npc_types`, `zone`, `spawn2`, `spawnentry`, `spawngroup`, `loottable`, `loottable_entries`, `lootdrop`, `lootdrop_entries`, `npc_spells`, `npc_spells_entries`, `skill_caps`, `merchantlist` — setting primary keys and associations (reference: `SCHEMA.md`, `backend/internal/db/models.go`)
- [ ] 1.3 Add a field-parity test comparing each game-data schema's fields against the live table's columns with normalized type names (design risk: driver affinity reporting); verify it passes against the real artifact and fails when a field is deliberately removed
- [ ] 1.4 Verify the read-only guarantee still holds with full schemas present: assert a write through a game-data schema raises (extends the Wave 0 `data-store` requirement)

## 2. User-data schemas (user.db, 50 tables)

- [ ] 2.1 Extend the generator to the user-data DDL and emit skeletons for all 50 tables across the ~18 owning packages; verify regeneration produces no diff
- [ ] 2.2 Review and finish the character-scoped schemas — `characters`, `character_aas`, `character_tradeskills`, `character_raid_buffs`, `character_skills`, `hidden_characters`, `character_faction_tally`, `character_faction_wishlist`, `character_tasks`, `character_task_subtasks`, `character_upgrade_focus`, `character_upgrade_weights`, `character_wishlist`, `character_wishlist_slot_layout`, `custom_leveling_recipes`, `favorite_recipes` (reference: `backend/internal/character/*.go`, 1,907 LOC)
- [ ] 2.3 Review and finish the trigger schemas — `triggers` (31 columns), `trigger_categories`, `trigger_categories_new`, `trigger_timer_groups`, `trigger_pattern_audits`, `action_templates`, `pack_baselines`, `pack_default_updates` (reference: `backend/internal/trigger/store.go`)
- [ ] 2.4 Resolve which of `trigger_categories` / `trigger_categories_new` is live by inspecting the reference's read path, record the finding, and schema only the live table (do not assume the newer name won); verify the decision is recorded in the change and the dead table is reported by adoption as unmapped (reference commits `8d7d3d23`, `59e65e32`)
- [ ] 2.5 Review and finish the remaining user-data schemas — players (4), progress (3), raidcomp (5), emote (2), popflag (2), and the single-table domains: `chat_messages`, `combat_fights`, `keyring_entries`, `lockout_entries`, `loot_events`, `map_annotations`, `character_stat_snapshots`, `saved_queries`, `trader_snapshots`, `backups`
- [ ] 2.6 Add the user-data field-parity test against a captured fixture database; verify it passes and fails on a deliberately removed field
- [ ] 2.7 Implement and test name-based character access: verify a test asserts name-keyed data resolves by name, and that a name mismatch is reported rather than presented as absent data (design D6)

## 3. Serialized column types

- [ ] 3.1 Implement typed decode/encode for the trigger action and timer-alert lists, preserving the reference's exact serialized shape; verify a round-trip test on a row captured from the reference produces equivalent text
- [ ] 3.2 Implement typed decode/encode for the combat fight combatant and healer lists; verify a round-trip test on a captured row
- [ ] 3.3 Implement typed decode/encode for upgrade weights, stat-snapshot parse results, trigger character lists, and role class codes; verify round-trip tests for each
- [ ] 3.4 Verify strict decoding: for each typed column, a malformed value raises an error naming the table, column and row identity rather than returning a default (design D5)
- [ ] 3.5 Verify unknown-field preservation: a stored value containing a field the schema does not know about survives a read-then-write unchanged, so a write-back cannot silently drop user data

## 4. Game-code catalog consolidation

- [ ] 4.1 Create the single `PQ.Game.Enums` module backed by one data source, covering the codes the reference exposes (reference: `backend/internal/db/enums/`)
- [ ] 4.2 Port the reference's audit test as the guard that no second definition of the catalog exists; verify it fails when a duplicate map is introduced (reference: `backend/internal/db/enums/audit_test.go`)
- [ ] 4.3 Verify every label the reference could produce is reproduced, including the deterministic placeholder for unknown codes; verify with a test generated from the reference's catalog rather than a hand-written list
- [ ] 4.4 Verify the frontend's duplicated catalog is accounted for: assert each mapping in `frontend/src/lib/enumsCache.ts` is present in `PQ.Game.Enums` and record any deliberate divergence (reference: `docs/enum-audit.md`)

## 5. Ported game math

- [ ] 5.1 Port spell duration and level scaling; verify the reference's table-driven cases all pass (reference: `backend/internal/db/spellformula.go` + tests)
- [ ] 5.2 Port buff-effect, haste and instrument-modifier calculations; verify the reference's cases pass (reference: `db/buffeffect.go`, `db/haste.go`, `db/instrument_mod.go` + tests)
- [ ] 5.3 Port special-ability parsing and labeling; verify the reference's cases pass (reference: `db/special_abilities.go` + tests, and the code table in `SCHEMA.md`)
- [ ] 5.4 Port duplicate-named entity resolution (the NPC/boss variant logic); verify the reference's cases pass (reference: `db/variants.go` + tests)
- [ ] 5.5 Port resist and charm calculations; verify the reference's cases pass (reference: `internal/resist/`, `internal/charm/` — 778 LOC production, 422 LOC tests)
- [ ] 5.6 Port upgrade scoring and cap-aware stat weighting; verify the reference's cases pass (reference: `internal/upgrade/`, 481 LOC production, 283 LOC tests)
- [ ] 5.7 Add a case-inventory test per ported function asserting the ported case count matches the reference test file, so cases cannot be dropped silently (see proposal — "mechanical, not interpretive")

## 6. Migrations and fresh-install convergence

- [ ] 6.1 Write the Ecto migrations mirroring the reference DDL with create-if-not-exists semantics, one per domain (design D1); verify `mix ecto.migrate` on an empty file produces the full schema
- [ ] 6.2 Verify convergence: create the schema on a fresh file, adopt an existing fixture database, and assert the two schemas are identical (column names, types, indexes, uniqueness)
- [ ] 6.3 Verify migrations are idempotent: running twice reports no pending work and changes nothing
- [ ] 6.4 Implement and test the boot-time compatibility check: a compatible install starts, an incompatible install refuses to start with a message naming the adoption task (design D2)

## 7. User-database adoption

- [ ] 7.1 Implement `mix pq.db.adopt` with a mandatory pre-migration snapshot; verify a test asserts the backup exists before any statement executes against the original
- [ ] 7.2 Implement parity comparison across all 50 tables; verify it fails and names the table and column for both an unknown column and a missing column, without modifying the database
- [ ] 7.3 Implement unmapped-table reporting; verify a fixture containing a schema-less table reports it by name without failing
- [ ] 7.4 Implement decoding diagnostics: rows that fail strict decoding are reported (table, column, identity) without blocking adoption (design risk: pre-existing corrupt rows)
- [ ] 7.5 Verify row-count preservation: run adoption against a captured real `user.db` and assert every table's row count is unchanged
- [ ] 7.6 Verify recoverability: force a failure mid-adoption and assert the original database is intact and restorable from the snapshot
- [ ] 7.7 Verify adoption against a captured reference-created database with real content: assert reads through the new schemas return the same values the reference's own queries return for a fixed sample
- [ ] 7.8 Implement WAL checkpointing before any backup, export or file swap, and verify an exported bundle produced after a checkpoint is readable by the reference application (design risk: WAL sidecar files break single-file copies)

## 8. Settings schema and defaults

- [ ] 8.1 Define `PQ.Config` as an embedded schema covering the reference's 13 settings structs, using the reference's key names and nesting (reference: `backend/internal/config/config.go`, 1,494 LOC)
- [ ] 8.2 Port the reference's defaults; verify the reference's defaults test cases all pass (reference: `backend/internal/config/config_test.go`, 482 LOC)
- [ ] 8.3 Add changeset validation with field-level errors; verify invalid values are rejected and persisted settings are unchanged
- [ ] 8.4 Verify unknown-key preservation: a settings file containing an undefined key loads successfully and the key survives a save
- [ ] 8.5 Verify round-trip fidelity: load a reference-written settings file and save it unmodified, asserting every value is preserved exactly

## 9. Settings server

- [ ] 9.1 Implement `PQ.Config.Server` as the single writer with atomic temp-then-rename saves (design D8); verify concurrent update tests show the file always parses and equals one of the accepted update sequences
- [ ] 9.2 Verify no-reader-sees-partial: run a reader concurrently with continuous saves and assert it never observes an unparseable file
- [ ] 9.3 Verify no temporary artifact remains after a successful save, and that an interrupted save leaves the previous settings intact
- [ ] 9.4 Implement change broadcast with origin identification (design D9); verify a test asserts another window is notified, a later-opened window sees current values, and the originating window can distinguish its own change
- [ ] 9.5 Implement external-edit detection; verify a test edits the file out-of-band and asserts a reload plus propagation, that the app's own save does not trigger a reload loop, and that an externally invalid file is reported without stopping the application

## 10. Rollback compatibility

- [ ] 10.1 Verify the reference application can read a `user.db` that the migrated application has written to: use it to open the file and run its own queries against a fixed sample
- [ ] 10.2 Verify the reference application can read a `config.yaml` the migrated application has saved, with its own settings unchanged
- [ ] 10.3 Verify an app-state export bundle produced by the migrated application is readable by the reference application, and that an export from the reference is readable by the migrated application (parity fixture test in both directions)

## 11. Wave 2 verification

- [ ] 11.1 Walk the seven Wave 2 acceptance criteria in `docs/phoenix-migration-plan.md` §5.6 and record the result of each
- [ ] 11.2 Record the enum-divergence findings and the trigger-category resolution as a durable note, since both are decisions a later reader will otherwise re-litigate
- [ ] 11.3 Verify no file under `backend/`, `frontend/` or `electron/` was modified: `git diff --stat` against the reference trees is empty
