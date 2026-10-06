# Group Composition Proposal — usage guide

This guide covers the Group Composition Proposal feature (branch
`feat/raid-comp-split`, ships in v0.26.0-beta.1): generating balanced group
proposals for an encounter, splitting one roster into multiple raids, defining
named group-composition templates, and moving the formed raid into place
in-game with `#raidmove`.

Everything lives under **Raids → Group Proposal** in the app's left navigation.

---

## 1. Prerequisites

- **Zeal running with the pipe active.** The proposal seats your *live* roster
  — whoever is in the Zeal raid/group feed. The status banner at the bottom of
  the page tells you what the app sees (members, zone).
- **Members classed.** Classing drives every seating decision. If members show
  as *unclassed*, class them via the **Taxonomy Editor** (linked from the
  banner hint) — class → role family (tank / heal / sup / dmg / util) and
  optional sub-roles (e.g. `clr` for CH clerics).
- **An encounter with a target composition.** Pick or create one under
  **Raids → Raid Editor**. Its composition defines which roles the raid needs
  (MIN = must seat, REC = nice to have).

---

## 2. Generating a proposal

On **Raids → Group Proposal**:

1. **Pick the encounter** — the dropdown lists name + zone (placeholders are
   marked).
2. **Group size** — 1–12, default 6. EQ raid groups are 6.
3. **Split into** — 1 raid (default) up to 6 raids. See §5 for multi-raid.
4. **Keep live groups together** — seats members into their existing Zeal
   groups where possible. Always on in multi-raid mode (each raid keeps its
   live groups intact).
5. Click **Generate proposal**.

The engine seats in three passes: **MIN coverage first** (every required role
gets a seat), then **REC comfort slots**, then trinity interleaving — tank +
healer + support open every group, damage fills the rest. Groups that match a
live group or an enabled template keep that identity.

### The plan hint (multi-raid only)

With 2+ raids selected, the page asks the backend how many *complete* raids
the live roster can actually staff, and which role is the bottleneck:
`live roster can staff 3 complete raids — capped by CLR`. If your selected
raid count exceeds that, extra raids will show MIN gaps — the hint warns you
in red before you generate.

---

## 3. Reading the report

- **Group cards** — one per proposed group: member, class, role badge per
  seat. Slot 1 wears the **Group Leader** crown — the label belongs to the
  *position*, not the person; whoever you drag into slot 1 becomes that
  group's proposed leader.
- **Coverage pills** — per-raid MIN and REC rows show what the proposal
  fields. Green = covered, red = MIN gap (nobody available for that role).
- **Warnings** — amber rows above the cards explain anything best-effort
  (unfilled template rows, shapes beyond the raid count, scarce classes spread
  thin).
- **Unseated (bench)** — everyone who didn't fit, each with the *reason*
  (e.g. "no MIN demand for …", "groups full"). Bench members are still part of
  the raid — see §6 for moving them as ungrouped.

---

## 4. Editing the proposal

Everything is drag-and-drop (mouse only — hold and drag ~6px to start):

- **Swap** any member onto any other seat — contents travel wholesale,
  including across raids in multi-raid mode.
- **Insert** onto a **`+ open seat`** target in a not-full group. Full groups
  (6/6) show no open-seat target — drag member-onto-member to swap instead.
- **Bench** by dropping a member onto the **Unseated** panel; drag a benched
  member back onto a seat to un-bench.

Coverage recomputes after every edit — the MIN/REC tables always tell the
truth about the proposal *as adjusted*. A **manually adjusted** chip appears
in the header once you've touched anything, and any **copy button you click
afterwards reflects your edits** (see §6).

---

## 5. Group compositions (templates)

A *group composition* is a named template for one group — e.g.
**healstack**: 2 CH clerics + a defensive tank. Use them when an encounter
needs a specific group built a specific way.

### Defining templates (Raid Editor)

Open the encounter in **Raids → Raid Editor** and scroll to the **Group
compositions** section at the bottom of the encounter form:

- **Identifier** — the template's name (e.g. `healstack`). Shown on group
  cards in the proposal.
- **Group pin** — `0` = auto (the template claims the first free group number
  in each raid); `1–12` pins it to that exact group number (falls back to the
  first free one on collision).
- **Role rows** — one per role requirement: pick a taxonomy role
  (optionally a sub-role leaf, e.g. `clr.*`), set the **count**. Add rows as
  needed.
- The editor shows live counts and mirrors the backend's validation rails
  (same identifiers within an encounter, sane counts, known roles).

Don't forget to **save the encounter**. Templates travel with raid
composition packs (export/import).

### Using templates (proposal page)

Enabled templates appear as **checkboxes** (labelled by identifier) on the
Group Proposal page. Tick the ones this attempt should honor. The weave seats
template groups **first** — before trinity filling — and *best-effort*: a row
it can't fill (nobody classed for it) warns in amber and the seat stays open
rather than being stuffed with a mismatch.

With 2+ raids, a **placement** select appears:

- **replicate** — every raid fields every enabled template (e.g. both raids
  get their own healstack).
- **distribute** — template 1 → raid 1, template 2 → raid 2, … Templates
  beyond the raid count are ignored (with a warning).

Templates work in **single-raid proposals too** — you don't need cohort mode.

Groups produced from a template wear a **`shape: <id>`** badge on their card.

---

## 6. Moving the raid in-game (`#raidmove`)

Once the raid is formed in EQ, use `#raidmove` to arrange members into the
proposed groups. The command (Zeal):

```
#raidmove <name> <group number>     (short form: #rm)
```

Group `0` = ungrouped.

> **Important — one command per paste.** EQ's chat input collapses a
> multi-line paste into *one* giant chat message instead of sending each line
> separately, so pasting a whole script at once does **not** work — only the
> first move lands and the rest die inside a garbled message. Every copy
> button below therefore copies **one command at a time**:

**The workflow:** click a copy button → click into EQ chat → paste → Enter →
click the same button again for the next member.

### The copy buttons

| Where | Button | What it copies per click |
|---|---|---|
| Each seat row | copy icon | that member's command (`#raidmove <name> <group>`) — for one-off fixes |
| Each bench row | copy icon | `#raidmove <name> 0` (move them in ungrouped) |
| Each group card header | `#raidmove` | rotates through that group's members in seat order |
| Proposal header (single-raid) | `copy all #raidmove` | rotates through the whole raid: every group in order, then bench → group 0 |
| Each raid header (multi-raid) | `#raidmove` | rotates through that raid's members (per-raid local group numbers) |

Rotation buttons show progress — `#raidmove 3/6` means you're on member 3 of
6; it wraps around after the last. A green check-mark confirms each copy.

Why per-raid buttons in multi-raid mode: in-game raid groups are numbered
within *each* raid, so a single global script would mix raid 2's group
numbers into raid 1. Form raid 1, work its button, form raid 2, work its
button.

Drags you made in the editor are always reflected — commands are derived from
the proposal as currently displayed, at the moment you click.

---

## 7. Multi-raid (cohort) workflow, end to end

1. Set **split into** to 2–6 raids. The plan hint tells you how many complete
   raids the roster can staff and what's binding.
2. **Keep live groups together** is always on — each raid preserves its
   existing Zeal groups; scarce classes are dealt evenly across raids.
3. Tick any templates and choose **replicate** or **distribute**.
4. **Generate proposal** → each raid gets its own section: header with member
   count, per-raid coverage pills, and its own `#raidmove` rotation button;
   warnings show MIN gaps per raid.
5. Adjust by dragging (swaps work across raids).
6. Form each raid in EQ and work through its rotation button.

---

## 8. Troubleshooting

| Symptom | Fix |
|---|---|
| Members show *unclassed* | Class them in the Taxonomy Editor; the proposal can't seat what it can't classify. |
| Red MIN pills after generating | The roster lacks that role for this composition — bench lists the shortfall reasons; recruit or lower the composition's MIN. |
| `+ open seat` targets missing on a group | The group is full — drag member-onto-member to swap instead. |
| Dragging does nothing | Move the mouse ~6px while holding before hovering the target — that's the activation threshold. |
| Pasted script doesn't work in EQ | Expected — EQ collapses multi-line pastes. Use the rotation buttons: one click, one paste, one Enter, repeat. |
| Template row warns it can't fill | No live member matches that role/sub-role — the seat stays open on purpose; fix the template or the roster. |
| Roster stale / empty | Zeal pipe not running, or the backend attached to the wrong pipe — check the status banner, restart Zeal, refresh. |
