# Group-Composition Templates ("Group compositions") — plan

Status: **SPEC — awaiting user confirmation on the decisions in §8.**
Supersedes: the removed `focused` / `curated` preferences and the wildcard
pin/cap machinery (deleted in `14e85fed`). Trinity weaving is the baseline;
group-shape templates are the sanctioned way to hand-shape specific groups
inside a proposal, aimed primarily at multi-raid (cohort) proposals.

## 1. Goal

A raid leader can define named **group shapes** per encounter ("HealStack" =
5 healers + a necromancer; "CasterStack" = 3 wizards + 2 magicians + a
necromancer), persist them in the Raid Editor, and ask the Group Proposal
page to weave a proposal that includes those shapes. Fills are best-effort:
a shape group that cannot be fully staffed visibly reports what it missed
instead of silently degrading.

Why this replaces curated: curated was *named-member pins + global caps*
(you listed the people). There is no way to express "any 5 healers in a
group, I don't care which 5" — which is what raid leaders actually do.
Shapes are member-agnostic: the weave picks *who* fits a shape's role rows.

## 2. Terminology

- **Shape** — one named group template with an identifier (e.g. `healstack`).
- **Row** — one role + count entry inside a shape (e.g. `healer.ch_cleric × 5`).
- **Shape group** — a proposal group produced from a shape.
- **Enabled shapes** — the shapes a proposal request asks the weave to include.

## 3. Data model

Per-encounter (like comps — shapes describe *this encounter's* formations).

New child table (same lifecycle as `raid_encounter_comps`):

```sql
CREATE TABLE IF NOT EXISTS raid_group_shapes (
  encounter_id TEXT NOT NULL,
  position     INTEGER NOT NULL,                 -- shape order / display order
  shape_id     TEXT    NOT NULL,                 -- identifier, e.g. 'healstack'
  group_number INTEGER NOT NULL DEFAULT 0,       -- 0 = auto (first free per raid)
  row_index    INTEGER NOT NULL,                 -- order within the shape
  role         TEXT    NOT NULL,                 -- taxonomy role
  sub_role     TEXT,
  count        INTEGER NOT NULL DEFAULT 1,       -- how many seats for this role
  PRIMARY KEY (encounter_id, position, row_index)
);
```

- `shape_id` is a stable, user-editable identifier (slug-ish; rendered on
  proposal groups and used to reference the shape in requests). Ids must be
  **unique per encounter** (they're the request reference).
- Roles must exist in the live taxonomy (same validation `SaveEncounter`
  already applies to comp rows — roles are validated against `Leaves()`).
- An encounter may define any number of shapes; a shape may be any number of
  rows (a shape bigger than the group size simply yields overflow seats).

### DTO

`Shape` rides on the `Encounter` payload like comps do:

```go
type Shape struct {
    ID          string     `json:"shape_id"`
    GroupNumber int        `json:"group_number,omitempty"` // 0 = auto
    Rows        []ShapeRow `json:"rows"`
}
type ShapeRow struct {
    Role  string `json:"role"`
    Sub   string `json:"sub_role,omitempty"`
    Count int    `json:"count"`
}
```

- Persisted + loaded inside `SaveEncounter` / `GetEncounter` (deleted then
  re-inserted in the same transaction, exactly like comps/reqs/strategy —
  extend the child-table lists at store.go:563 and :788 and the load query).
- Pack export/import: include `shapes` in the pack DTO. Pack `version: 1`
  gains an optional field; importers missing it keep working (old packs
  simply have no shapes).

## 4. API

- **Editor**: no new endpoints. Shapes are fields on `POST/PUT
  /api/raids/encounters` (whichever the editor save path uses) and appear in
  `GET /api/raids/encounters`. Roster-shape editing is a pure DOM exercise.
- **Proposal**: `POST /api/raids/split` gains
  `shapes: []string` — the enabled shape ids (empty = trinity only, i.e.
  today's behavior) and `shape_distribution: "replicate" | "distribute"`
  (default `replicate`). **Shapes require cohort mode**: a request with
  `shapes` and `cohorts <= 1` is a 400. The engine loads the encounter's
  shapes, filters by id, and errors on unknown ids (loud, like the
  wildcards-400 precedent).

## 5. Weave integration (the core)

Priority ordering (stacking on the MIN-first rule from `38b4c201`):

1. **Shape groups seat first.** For each enabled shape, in declaration
   order. **Distribution** (per `shape_distribution`):
   - `replicate` (default) — every cohort gets every enabled shape; shape
     groups claim the first group numbers of each raid.
   - `distribute` — shape i applies to cohort `((i-1) mod K)+1`, so a
     2-raid split of [HealStack, CasterStack] gives raid 1 the heal stack
     and raid 2 the caster stack. Small templates (1–3 members) are the
     point of distribute: they slot into one raid without unbalancing it.
   **Group pinning**: a shape with `group_number != 0` lands on that group
   number (per-raid local numbering) instead of the auto first-free slot.
   Collisions at weave time (two enabled shapes pinned to the same number,
   or a pinned number beyond the raid's group budget) → the later shape
   falls back to first-free plus a warning. Seats are assigned from
   eligible members by the existing picker (same-cohort preference, name
   tie-break). Overflow rows (shape needs more than `group_size`) spill
   into the next group number, tagged with the same shape id.
2. **Then the trinity MIN-first weave** fills the remaining members and
   groups exactly as today (deadline ordering unchanged).
3. **Coverage attribution:** shape-group seats count toward the encounter
   template's MIN/REC coverage (a HealStack's 5 clerics satisfy
   `healer.ch_cleric` need). The per-cohort coverage table therefore
   reflects what the raid actually fields.
4. **Respect existing groups:** applies to every *non-shape* seat as today;
   shape groups take precedence and ignore live-group affinity.
5. **Warnings:** any shape row short of its count emits a warning in the
   report ("shape `healstack`: healer.ch_cleric 3/5") and the unfilled
   seat renders as an open seat — best-effort, never silent.

### Members → shape candidates

A member is eligible for a shape row exactly like a comp slot: their class
must be in the row role's class set. Members seated by a shape are consumed
for the whole proposal (not eligible for later weave slots — the existing
`slot == nil` rule handles this untouched).

## 6. Frontend

### Raid Editor (`RaidEditorPage` / `EncounterForm`)

New "Group compositions" section per encounter, mirroring the comp-row
grid: for each shape — identifier input + its role rows (role/path selector
fed by the live taxonomy, count steppers, remove-row) + add-row / add-shape
buttons + remove-shape. Shapes save with the encounter (no new save flow).

### Group Proposal (`RaidSplitPage` / `GroupProposal`)

- Toggle list of the encounter's shapes shown **only when cohorts > 1**
  (the respect-groups checkbox pattern); every checkbox starts **unchecked**
  and only the checked ids travel with the Generate request — shapes are an
  explicit opt-in per generate, never a silent default.
- Report: shape groups render their identifier in the group header (the
  existing per-group card + family summary at GroupProposal.tsx:467 is the
  natural home); open seats where a shape row under-filled.
- Drag-and-drop keeps working: shapes are the starting point, DnD edits are
  post-processing, exactly like today's "manually adjusted" flow.

## 7. Edge cases

- Encounter defines shapes but none enabled → pure trinity (today).
- `shapes` sent with `cohorts <= 1` → 400 (shapes are a multi-raid feature).
- Shape ids in the request that don't exist on the encounter → 400.
- Roster too small to fill a shape → warnings per under-filled row; rest of
  the raid weaves as usual.
- Pinned-group collisions or out-of-range pins → fallback to first-free
  plus a warning.
- K cohorts with `distribute`: cohorts beyond the shape count weave
  trinity-only; shapes beyond the cohort count are ignored with a warning.
- Legacy reports (pre-cohorts normalization) — unaffected; shapes only
  appear via fresh requests.
- No-null rule: `shapes` in the encounter DTO and any shape arrays in the
  report must marshal `[]`, never null (the c05c19b5 lesson).

## 8. Decisions (confirmed 2026-10-04 by the user)

1. **Row kind: role paths only.** The taxonomy already lets any role carry
   any class mix (Taxonomy Editor), so "5 healers" is a role whose classes
   include the healers the leader wants. No family/class rows needed.
2. **Distribution: both modes supported** (`replicate` default,
   `distribute` for small templates), §5.
3. **Cohort-only.** Shapes apply only when cohorts > 1; the proposal UI
   shows the toggles only in cohort mode.
4. **Opt-in per generate.** Unchecked by default; checked ids travel with
   the Generate request only.
5. **Optional group pinning** via `group_number` (0 = auto), collisions
   degrade to first-free + warning.

## 9. Test plan

- Store: shape rows persist/load with the encounter; validation rejects
  unknown role rows; delete-encounter cascades shapes.
- Weave (unit): a shape group seats eligible members first and consumes
  them; coverage counts shape seats toward MIN; under-filled shape warns;
  replicate-per-cohort for K=2..3; unknown shape id → 400; empty shapes ==
  trinity baseline; every existing split test stays green (no-shape path).
- API: encounter save/load round-trip with shapes; pack export includes
  shapes and import restores them.
- e2e (evergreen, when the seam exists): Raid Editor shape CRUD; proposal
  with a shape toggled produces the shape-labeled group; DnD still works.