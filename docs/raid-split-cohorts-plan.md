# Raid Cohort Splits — plan & milestones

Status: planned (feat/raid-comp-split). Decision recorded below: **full
template per cohort** is the v1 model.

## Goal

Split the raid roster into **multiple smaller raids** ("cohorts"), each one an
independently staffed composition measured against the encounter's comp
template on its own. Use cases: BoT tower splits (N balanced tower groups),
AoE teams, dual-force pulls, fielding two full raids from one roster.

## Today vs. target

Today the split assigns the whole roster against the comp **raid-wide**: the
trinity weave offers tank/healer/CC seats early per group, but coverage is
only measured for the raid as a whole — no individual group is guaranteed
complete. Cohort mode partitions the roster into N mini-raids, each with its
own groups **and its own MIN/REC coverage verdict**.

## Decision: full template per cohort

Each cohort targets the **FULL encounter template** (its own copy of every
comp row), best effort. Where the roster cannot staff K complete comps, the
interleaved allocator spreads scarce classes evenly and the per-cohort
coverage + warnings report the shortfall honestly ("raid 2 short 1 slower of
MIN"). Rationale (2026-10-03): matches "can we field two full raids?"
directly, keeps the coverage semantics identical to the single-raid path, and
avoids inventing scaled-template rounding rules (ceil MIN/K) in v1. A scaled
mode (comp ÷ K per cohort) remains a possible later toggle for strict tower
comps.

## Engine design (split.go)

- `SplitRequest.Cohorts int` — 0/1 = legacy single-raid shape; 2..6 = cohort
  mode. Cap: `maxSplitCohorts = 6`.
- **v1 preference support: trinity only.** Curated + cohorts is rejected
  (wildcard group pins are ambiguous across cohorts until cohort-scoped
  wildcards land); focused + cohorts is rejected (class clustering contradicts
  pre-formed tower groups). Both rejections are explicit 400s, not silent
  fallbacks.
- Per-cohort slots: the full comp expanded per cohort, trinity-woven per
  cohort, each slot tagged with its cohort.
- **Cohort seeding from live groups**: the distinct live Zeal group numbers
  present in the roster are sorted and chunked into K contiguous blocks; each
  live group belongs to its block's cohort (6 live groups, K=2 → groups
  1–3 → raid 1, 4–6 → raid 2). Members without a live group are free agents.
  This is what preserves pre-formed tower groups.
- **Interleaved allocation**: slot order walks the cohorts round-robin
  (cohort 1 tank, cohort 2 tank, cohort 1 healer, cohort 2 healer, …), so
  scarce classes distribute evenly instead of cohort 1 hoarding them.
  Candidates prefer members already seeded into the slot's cohort; ties break
  by name (deterministic, as everywhere else in the engine).
- **Fill distribution**: members no slot claimed land in the cohort with the
  smallest roster (round-robin), keeping cohort sizes even.
- **Per-cohort layout**: seeded members seat at their live group renumbered
  within its block; remaining members seat in woven order via the group
  cursor; every cohort's group count derives from its own headcount.
- Report: `SplitReport.Cohorts []CohortReport{number, groups, min, rec,
  roster_count, warnings}`. The legacy flat fields stay populated for
  cohorts ≤ 1 (older clients keep working); for cohorts > 1 they are empty
  arrays (no-null rule). The shared Unseated bench stays top-level.
- Warnings: per-cohort MIN shortfalls inside `CohortReport.Warnings`; global
  notes (e.g. cohort clamping) top-level.

## API

- `POST /api/raids/split` gains `cohorts`. Rejections surface as 400 with the
  reason (curated/focused + cohorts).
- New `GET /api/raids/split/plan?encounter_id=` — the auto-suggest: per-leaf
  eligible-vs-need ratios against the live roster → `max_cohorts` (how many
  complete MIN comps the roster can staff, capped at 6), plus the per-leaf
  detail so the UI can say *which* class caps the split ("clerics: 4 eligible
  / 2 per raid → caps at 2 raids"). Advisory only; the user may override N.

## UI

- Control: **"Split into [N] raids"** selector with the plan suggestion as
  its hint.
- Report: one section per cohort — header `Raid N · M members · MIN/REC
  pills` — each with its own group cards (family badges + shape summaries as
  today) and its own coverage table; shared Unseated card at the bottom.
- **Cross-cohort drag-and-drop**: seat ids gain a cohort dimension
  (`seat:<cohort>:<group>:<index>`); dragging a cleric from raid 1 into a
  raid 2 seat is the core rebalancing move. Swap/insert/bench semantics
  unchanged; per-cohort coverage recomputed after every edit.

## Later (explicitly out of v1)

- `Wildcard.Cohort` — cohort-scoped pins/caps ("pin Bonce to raid 2",
  "max 2 clerics in raid 3"), which also unlocks curated + cohorts.
- Scaled-template mode (comp ÷ K per cohort) as a UI toggle.
- Cohort-aware encounter templates (per-tower comp rows) if a real need
  shows up.

## Milestones

- **M1 — engine + tests**: cohort slots, seeding, interleaved allocation,
  fill distribution, per-cohort layout/coverage, curated+focused rejections,
  legacy shape for cohorts ≤ 1, no-null pins.
- **M2 — API**: `cohorts` passthrough + `/split/plan` endpoint + API tests.
- **M3 — UI**: cohort selector + plan hint, per-cohort report sections,
  cross-cohort DnD, adjusted badge carried over.

## Tests (house style, table-driven where it fits)

- Scarce-class fairness: 3 clerics / 2 cohorts of need 2 → 2/1 split, cohort 2
  carries the MIN warning.
- Full-staff case: 4 clerics / 2 cohorts → 2/2, zero warnings.
- Seeding: distinct live groups chunk into contiguous cohort blocks; seeded
  members never cross cohorts.
- Per-cohort coverage math; legacy fields for cohorts ≤ 1; `cohorts` never
  null; curated/focused + cohorts → 400.
- Determinism: identical input → identical report, twice.
