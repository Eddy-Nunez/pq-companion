import React, { useMemo, useState } from 'react'
import {
  DndContext,
  DragOverlay,
  MeasuringStrategy,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core'
import { Users, UserX, AlertTriangle, ShieldCheck, Crown, GripVertical } from 'lucide-react'
import type {
  CohortReport,
  CompLevelFill,
  SplitCoverage,
  SplitReport,
  SplitSlot,
  SplitUnassigned,
} from '../../types/raid'

// GroupProposal renders the split proposal: one section per cohort (single
// raid by default), each with its proposed groups, role-filled seats, and
// its own MIN/REC coverage verdict against the encounter template; plus the
// shared unseated card and proposal warnings. It is a PROPOSAL view —
// nothing here mutates the raid or the roster.
//
// With an onEdit callback the report is interactively adjustable via
// dnd-kit (the house drag layer — native HTML5 drag is unreliable in
// Electron on Windows): drag a member onto another member to SWAP their
// seats (contents exchange wholesale — member, role and rank travel
// together, across cohorts too), onto an open seat to INSERT them (a bench
// member seats as fill; a member from another group/raid keeps their role),
// or onto the unseated card to BENCH them. Coverage is recomputed from the
// edited seats after every change, so the MIN/REC tables always tell the
// truth about the proposal as adjusted. Regenerating resets all edits.

interface Props {
  report: SplitReport
  onEdit?: (next: SplitReport) => void
  // class code → display name from the live taxonomy ("pal" → "Paladin");
  // codes render verbatim when absent.
  classNames?: Record<string, string>
}

// ── drag ids ────────────────────────────────────────────────────────────────
// seat:<cohort>:<group>:<index>  draggable seat + swap target
// open:<cohort>:<group>:<index>  open-seat insert target (unique per slot —
//                                dnd-kit registers droppables by id)
// bench:<index>                  draggable unseated member
// benchlist                      the unseated card as a bench drop target

type SeatRef = { cohort: number; group: number; index: number }
type DragRef = { kind: 'seat'; ref: SeatRef } | { kind: 'bench'; index: number }
type DropRef =
  | { kind: 'seat'; ref: SeatRef }
  | { kind: 'open'; cohort: number; group: number }
  | { kind: 'benchlist' }

function parseDragId(id: string): DragRef | DropRef | null {
  const p = id.split(':')
  const num = (s: string | undefined): number => (s !== undefined && Number.isFinite(Number(s)) ? Number(s) : NaN)
  if (p[0] === 'seat' && p.length === 4) {
    const cohort = num(p[1])
    const group = num(p[2])
    const index = num(p[3])
    if (Number.isFinite(cohort) && Number.isFinite(group) && Number.isFinite(index)) {
      return { kind: 'seat', ref: { cohort, group, index } }
    }
  }
  if (p[0] === 'open' && p.length === 4) {
    // open:<cohort>:<group>:<index> — the index exists only to keep
    // placeholder ids unique (dnd-kit registers droppables by id) and is
    // not needed to resolve the target group.
    const cohort = num(p[1])
    const group = num(p[2])
    if (Number.isFinite(cohort) && Number.isFinite(group)) return { kind: 'open', cohort, group }
  }
  if (p[0] === 'bench' && p.length === 2 && Number.isFinite(num(p[1]))) {
    return { kind: 'bench', index: num(p[1]) }
  }
  if (id === 'benchlist') return { kind: 'benchlist' }
  return null
}

// ── cohort normalization (legacy single-raid reports) ───────────────────────

// cohortsOf returns the report's cohorts, synthesizing one entry from the
// legacy flat fields when an older response omits them.
function cohortsOf(report: SplitReport): CohortReport[] {
  if (report.cohorts && report.cohorts.length > 0) return report.cohorts
  return [
    {
      number: 1,
      groups: report.groups ?? [],
      min: report.min ?? [],
      rec: report.rec ?? [],
      roster_count: (report.groups ?? []).reduce((n, g) => n + g.slots.length, 0),
      warnings: [],
    },
  ]
}

function cohortByNumber(r: SplitReport, number: number): CohortReport | undefined {
  return (r.cohorts ?? []).find((c) => c.number === number)
}

function groupIn(cohort: CohortReport | undefined, number: number) {
  return cohort?.groups.find((g) => g.number === number)
}

// ── role families ───────────────────────────────────────────────────────────
// The taxonomy's role ids bucket cleanly by prefix (same split the backend's
// roleBucket uses), which buys badge tinting and per-group shape summaries:
// a raid leader scans "2 tank · 2 heal · 1 sup · 1 dmg" without reading rows.

interface RoleFamily {
  label: string // short label for the group-shape summary
  color: string // badge tint (dark-theme friendly)
}

const FAMILY_TANK: RoleFamily = { label: 'tank', color: '#3b82f6' } // blue
const FAMILY_HEAL: RoleFamily = { label: 'heal', color: '#22c55e' } // green
const FAMILY_SUP: RoleFamily = { label: 'sup', color: '#a855f7' } // purple (slows/debuffs/CC)
const FAMILY_DMG: RoleFamily = { label: 'dmg', color: '#f97316' } // orange
const FAMILY_UTIL: RoleFamily = { label: 'util', color: '#64748b' } // slate (rgc, lockpicker, coth…)
const FAMILY_FILL: RoleFamily = { label: 'fill', color: '#475569' } // dim slate

export function roleFamily(slot: Pick<SplitSlot, 'role' | 'path'>): RoleFamily {
  const key = slot.role || slot.path || ''
  if (!key) return FAMILY_FILL
  if (key === 'tank' || key.startsWith('tank.')) return FAMILY_TANK
  if (key === 'healer' || key.startsWith('healer.')) return FAMILY_HEAL
  if (key === 'slower' || key === 'debuffer' || key === 'cc' || key.startsWith('debuffer.')) return FAMILY_SUP
  if (key === 'damage') return FAMILY_DMG
  return FAMILY_UTIL
}

function RoleBadge({ slot }: { slot: Pick<SplitSlot, 'role' | 'path' | 'label' | 'level'> }): React.ReactElement | null {
  if (!slot.role) {
    return <span className="text-xs opacity-50" style={{ color: 'var(--color-muted-foreground)' }}>fill (no comp role)</span>
  }
  const fam = roleFamily(slot)
  return (
    <span className="text-xs inline-flex items-center gap-1.5">
      <span style={{ color: 'var(--color-foreground)' }}>{slot.label || slot.path}</span>
      <span
        className="text-[10px] px-1 py-0.5 rounded font-medium"
        title={
          slot.level === 'min'
            ? `${fam.label} slot · MIN (hard floor)`
            : `${fam.label} slot · REC (comfort)`
        }
        style={{
          backgroundColor: fam.color + '26', // ~15% alpha wash
          color: fam.color,
          border: `1px solid ${fam.color}55`,
        }}
      >
        {slot.level} · {fam.label}
      </span>
    </span>
  )
}

// ── report mutations (pure over a clone; false = rejected, no change) ──────

function applySwap(r: SplitReport, a: DragRef, b: DragRef): boolean {
  const seatOf = (d: DragRef): SeatRef | null => (d.kind === 'seat' ? d.ref : null)
  const sa = seatOf(a)
  const sb = seatOf(b)
  if (sa && sb) {
    if (sa.cohort === sb.cohort && sa.group === sb.group && sa.index === sb.index) return false
    const ga = groupIn(cohortByNumber(r, sa.cohort), sa.group)
    const gb = groupIn(cohortByNumber(r, sb.cohort), sb.group)
    const slotA = ga?.slots[sa.index]
    const slotB = gb?.slots[sb.index]
    if (!ga || !gb || !slotA || !slotB) return false
    ga.slots[sa.index] = { ...slotB, group: ga.number }
    gb.slots[sb.index] = { ...slotA, group: gb.number }
    return true
  }
  // bench ↔ seat: the incoming member seats as FILL; the displaced member
  // (and their role assignment) goes to the bench — the coverage table
  // reflects the vacated slot honestly.
  if (a.kind === 'bench' && sb) {
    const gb = groupIn(cohortByNumber(r, sb.cohort), sb.group)
    const seat = gb?.slots[sb.index]
    const entry = r.unassigned?.[a.index]
    if (!gb || !seat || !entry) return false
    gb.slots[sb.index] = { member: entry.name, class: entry.class, role: '', path: '', label: '', group: gb.number }
    r.unassigned[a.index] = { name: seat.member, class: seat.class, reason: 'swapped out by hand' }
    return true
  }
  return false // bench ↔ bench: reorder, no-op
}

function applyInsert(r: SplitReport, from: DragRef, cohort: number, group: number): boolean {
  const g = groupIn(cohortByNumber(r, cohort), group)
  if (!g || g.slots.length >= g.size) return false // full: swap onto a member instead
  if (from.kind === 'bench') {
    const entry = r.unassigned?.[from.index]
    if (!entry) return false
    r.unassigned.splice(from.index, 1)
    g.slots.push({ member: entry.name, class: entry.class, role: '', path: '', label: '', group: g.number })
    return true
  }
  if (from.ref.cohort === cohort && from.ref.group === group) return false
  const src = groupIn(cohortByNumber(r, from.ref.cohort), from.ref.group)
  const slot = src?.slots[from.ref.index]
  if (!src || !slot) return false
  src.slots.splice(from.ref.index, 1) // their seat opens up; the role travels with them
  g.slots.push({ ...slot, group: g.number })
  return true
}

function applyBench(r: SplitReport, from: DragRef): boolean {
  if (from.kind !== 'seat') return false
  const g = groupIn(cohortByNumber(r, from.ref.cohort), from.ref.group)
  const slot = g?.slots[from.ref.index]
  if (!g || !slot) return false
  g.slots.splice(from.ref.index, 1)
  r.unassigned.push({ name: slot.member, class: slot.class, reason: 'benched by hand' })
  return true
}

// finalize re-stamps group numbers and recomputes per-cohort coverage from
// the edited seats, so MIN/REC placed counts always match what the proposal
// now shows. Single-cohort reports sync the legacy fields too, keeping the
// backend's mirror accurate.
function finalize(r: SplitReport): SplitReport {
  const cohorts = r.cohorts ?? []
  for (const c of cohorts) {
    for (const g of c.groups) {
      for (const s of g.slots) s.group = g.number
    }
    c.roster_count = c.groups.reduce((n, g) => n + g.slots.length, 0)
    const placedFor = (path: string, level: CompLevelFill): number =>
      c.groups.reduce((n, g) => n + g.slots.filter((s) => s.path === path && s.level === level).length, 0)
    c.min = (c.min ?? []).map((row) => ({ ...row, placed: placedFor(row.path, 'min') }))
    c.rec = (c.rec ?? []).map((row) => ({ ...row, placed: placedFor(row.path, 'rec') }))
  }
  if (cohorts.length === 1) {
    r.groups = cohorts[0].groups
    r.min = cohorts[0].min
    r.rec = cohorts[0].rec
  }
  r.unassigned = [...(r.unassigned ?? [])].sort((a, b) => a.name.localeCompare(b.name))
  return r
}

// ── coverage ────────────────────────────────────────────────────────────────

function CoveragePills({ rows, level }: { rows: SplitCoverage[]; level: string }): React.ReactElement {
  const short = rows.filter((r) => r.placed < r.need)
  if (rows.length === 0) {
    return (
      <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        {level}: no comp defined
      </span>
    )
  }
  if (short.length === 0) {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-success)' }}>
        <ShieldCheck size={13} /> {level} OK
      </span>
    )
  }
  const total = short.reduce((n, r) => n + (r.need - r.placed), 0)
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-danger)' }}>
      <AlertTriangle size={13} /> {level} short {total} ({short.length} role{short.length === 1 ? '' : 's'})
    </span>
  )
}

function LevelCell({ row }: { row?: SplitCoverage }): React.ReactElement {
  if (!row || row.need === 0) {
    return <span className="text-sm justify-self-end opacity-40" style={{ color: 'var(--color-muted-foreground)' }}>—</span>
  }
  const ok = row.placed >= row.need
  return (
    <span className="flex items-center gap-2 justify-self-end">
      <span className="text-sm tabular-nums" style={{ color: 'var(--color-muted-foreground)' }}>
        {row.placed}
        <span className="opacity-60">/{row.need}</span>
      </span>
      <span
        className="text-[11px] px-1.5 py-0.5 rounded font-medium"
        style={
          ok
            ? { backgroundColor: 'var(--color-surface-2)', color: 'var(--color-success)' }
            : { backgroundColor: 'var(--color-danger)', color: '#fff' }
        }
      >
        {ok ? 'OK' : `GAP ${row.need - row.placed}`}
      </span>
    </span>
  )
}

const COV_COLS = 'grid-cols-[minmax(160px,1.4fr)_minmax(90px,90px)_minmax(90px,90px)]'

function CoverageTable({ min, rec }: { min: SplitCoverage[]; rec: SplitCoverage[] }): React.ReactElement {
  const byPath = useMemo(() => {
    const m = new Map<string, { min?: SplitCoverage; rec?: SplitCoverage }>()
    for (const r of min) m.set(r.path, { ...m.get(r.path), min: r })
    for (const r of rec) m.set(r.path, { ...m.get(r.path), rec: r })
    return m
  }, [min, rec])
  const order = useMemo(() => (min.length > 0 ? min : rec).map((r) => r.path), [min, rec])

  return (
    <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
      <div
        className={`grid ${COV_COLS} gap-3 px-3 py-1.5 text-[11px] uppercase tracking-wide`}
        style={{ color: 'var(--color-muted-foreground)', borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
      >
        <span>Role</span>
        <span className="justify-self-end">Min</span>
        <span className="justify-self-end">Rec</span>
      </div>
      {order.map((path, i) => {
        const row = byPath.get(path)!
        return (
          <div key={path} className={`grid ${COV_COLS} gap-3 items-center px-3 py-1.5 ${i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''}`}>
            <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
              {row.min?.label ?? row.rec?.label ?? path}
            </span>
            <LevelCell row={row.min} />
            <LevelCell row={row.rec} />
          </div>
        )
      })}
    </div>
  )
}

// ── draggable/droppable pieces ──────────────────────────────────────────────

// One grid template shared by the header and every seat row so the columns
// stay aligned. Interactive rows gain a leading grip cell.
const seatGrid = (interactive: boolean): string =>
  interactive
    ? 'grid-cols-[18px_minmax(110px,1.2fr)_minmax(64px,0.7fr)_minmax(150px,1.6fr)]'
    : 'grid-cols-[minmax(110px,1.2fr)_minmax(64px,0.7fr)_minmax(150px,1.6fr)]'

const dropHighlight: React.CSSProperties = { borderColor: 'var(--color-primary)' }

// SeatRow is both a drag source and a swap target (drop member → member).
function SeatRow({ slot, cohort, groupId, index, interactive, stripe, classNames }: {
  slot: SplitSlot
  cohort: number
  groupId: number
  index: number
  interactive: boolean
  stripe: boolean
  classNames?: Record<string, string>
}): React.ReactElement {
  const id = `seat:${cohort}:${groupId}:${index}`
  const { setNodeRef: setDragRef, attributes, listeners, isDragging } = useDraggable({ id, disabled: !interactive })
  const { setNodeRef: setDropRef, isOver } = useDroppable({ id, disabled: !interactive })
  const setRef = React.useCallback(
    (el: HTMLElement | null) => {
      setDragRef(el)
      setDropRef(el)
    },
    [setDragRef, setDropRef],
  )
  const className = classNames?.[slot.class ?? '']
  // Slot 1 of every group is the Group Leader seat — the label belongs to
  // the POSITION, not the person, so dragging anyone into slot 1 makes them
  // that proposed group's leader and the label never moves.
  const leaderSeat = index === 0
  return (
    <div
      ref={setRef}
      {...listeners}
      {...attributes}
      className={`grid ${seatGrid(interactive)} gap-3 items-center px-3 py-1.5 border border-transparent ${stripe ? 'bg-(--color-surface-2)/50' : ''} ${interactive ? 'cursor-grab touch-none active:cursor-grabbing' : ''} ${isDragging ? 'opacity-40' : ''}`}
      style={isOver && interactive ? { ...dropHighlight, backgroundColor: 'var(--color-surface-2)' } : undefined}
    >
      {interactive ? <GripVertical size={12} className="opacity-40" style={{ color: 'var(--color-muted-foreground)' }} /> : null}
      <span className="text-sm truncate flex items-center gap-1.5" title={slot.member} style={{ color: 'var(--color-foreground)' }}>
        {slot.member}
        {leaderSeat ? (
          <span
            className="inline-flex items-center gap-0.5 text-[10px] px-1 py-0.5 rounded font-medium shrink-0"
            title="Slot 1 is the Group Leader seat — whoever sits here leads this proposed group"
            style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
          >
            <Crown size={10} /> Group Leader
          </span>
        ) : slot.rank ? (
          <span className="text-[11px] opacity-60 shrink-0">{slot.rank}</span>
        ) : null}
      </span>
      <span className="text-xs" title={className ? slot.class : undefined} style={{ color: 'var(--color-muted-foreground)' }}>
        {className ?? slot.class ?? '—'}
      </span>
      <RoleBadge slot={slot} />
    </div>
  )
}

// OpenSeat is an insert target: an unoccupied seat in a not-full group.
function OpenSeat({ cohort, groupId, index, interactive }: {
  cohort: number
  groupId: number
  index: number
  interactive: boolean
}): React.ReactElement {
  const id = `open:${cohort}:${groupId}:${index}`
  const { setNodeRef, isOver } = useDroppable({ id, disabled: !interactive })
  return (
    <div
      ref={setNodeRef}
      className={`mx-3 my-0.5 rounded border border-dashed px-3 py-1 text-[11px] text-center ${interactive ? 'touch-none' : ''}`}
      style={{
        borderColor: isOver && interactive ? 'var(--color-primary)' : 'var(--color-border)',
        color: isOver && interactive ? 'var(--color-primary)' : 'var(--color-muted-foreground)',
        backgroundColor: isOver && interactive ? 'var(--color-surface-2)' : 'transparent',
      }}
    >
      {isOver && interactive ? 'drop to seat here' : '+ open seat'}
    </div>
  )
}

// BenchRow is a drag source (insert or swap into groups).
function BenchRow({ entry, index, interactive, stripe, classNames }: {
  entry: SplitUnassigned
  index: number
  interactive: boolean
  stripe: boolean
  classNames?: Record<string, string>
}): React.ReactElement {
  const id = `bench:${index}`
  const { setNodeRef, attributes, listeners, isDragging } = useDraggable({ id, disabled: !interactive })
  const className = classNames?.[entry.class ?? '']
  return (
    <div
      ref={setNodeRef}
      {...listeners}
      {...attributes}
      className={`flex items-baseline gap-2 px-3 py-1.5 text-sm ${stripe ? 'bg-(--color-surface-2)/50' : ''} ${interactive ? 'cursor-grab touch-none active:cursor-grabbing' : ''} ${isDragging ? 'opacity-40' : ''}`}
    >
      {interactive ? <GripVertical size={12} className="-ml-1 self-center opacity-40" style={{ color: 'var(--color-muted-foreground)' }} /> : null}
      <span style={{ color: 'var(--color-foreground)' }}>{entry.name}</span>
      <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>{className ?? entry.class ?? 'unclassed'}</span>
      <span className="text-xs" style={{ color: 'var(--color-danger)' }}>{entry.reason}</span>
    </div>
  )
}

function GroupCard({ title, cohort, group, interactive, classNames }: {
  title: string
  cohort: number
  group: { number: number; size: number; slots: SplitSlot[]; shape_id?: string }
  interactive: boolean
  classNames?: Record<string, string>
}): React.ReactElement {
  const openSeats = Math.max(0, group.size - group.slots.length)

  // Group shape summary: family counts in a fixed order so every card reads
  // the same — "2 tank · 1 heal · 2 sup · 1 dmg".
  const shape = useMemo(() => {
    const order = [FAMILY_TANK, FAMILY_HEAL, FAMILY_SUP, FAMILY_DMG, FAMILY_UTIL, FAMILY_FILL]
    const counts = new Map<string, { fam: RoleFamily; n: number }>()
    for (const s of group.slots) {
      const fam = roleFamily(s)
      const e = counts.get(fam.label)
      if (e) e.n++
      else counts.set(fam.label, { fam, n: 1 })
    }
    return order
      .filter((f) => counts.has(f.label))
      .map((f) => ({ fam: f, n: counts.get(f.label)!.n }))
  }, [group.slots])

  return (
    <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
      <div
        className="flex items-center gap-2 px-3 py-1.5 flex-wrap"
        style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
      >
        <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
          {title}
        </span>
        <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
          {group.slots.length}/{group.size} seats
        </span>
        {group.shape_id ? (
          <span
            className="text-[10px] px-1.5 py-0.5 rounded font-medium"
            title="Formed from the group composition template — under-filled rows show open seats"
            style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
          >
            shape: {group.shape_id}
          </span>
        ) : null}
        {shape.length > 0 ? (
          <span className="ml-auto flex items-center gap-1.5 text-[11px]">
            {shape.map(({ fam, n }) => (
              <span key={fam.label} style={{ color: fam.color }}>
                {n} {fam.label}
              </span>
            ))}
          </span>
        ) : null}
      </div>
      <div>
        {group.slots.length > 0 ? (
          <>
            <div
              className={`grid ${seatGrid(interactive)} gap-3 px-3 py-1 text-[11px] uppercase tracking-wide`}
              style={{ color: 'var(--color-muted-foreground)' }}
            >
              {interactive ? <span /> : null}
              <span>Member</span>
              <span>Class</span>
              <span>Proposed role</span>
            </div>
            {group.slots.map((slot, i) => (
              <SeatRow key={`${slot.member}-${i}`} slot={slot} cohort={cohort} groupId={group.number} index={i} interactive={interactive} stripe={i % 2 === 1} classNames={classNames} />
            ))}
          </>
        ) : (
          <div className="px-3 py-1.5 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            empty — drag members in
          </div>
        )}
        {interactive
          ? Array.from({ length: openSeats }, (_, i) => (
              <OpenSeat key={i} cohort={cohort} groupId={group.number} index={i} interactive={interactive} />
            ))
          : null}
      </div>
    </div>
  )
}

export default function GroupProposal({ report, onEdit, classNames }: Props): React.ReactElement {
  // ?? [] guards: the backend's no-null contract makes these arrays always
  // present, but stale responses (or a mid-upgrade backend) could still send
  // null — render empty rather than crash (the null-classes lesson).
  const cohorts = cohortsOf(report)
  const unassigned = report.unassigned ?? []
  const interactive = onEdit != null

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const [dragging, setDragging] = useState<DragRef | null>(null)

  function handleDragStart(e: DragStartEvent): void {
    const parsed = parseDragId(String(e.active.id))
    setDragging(parsed && parsed.kind === 'seat' ? { kind: 'seat', ref: parsed.ref } : parsed && parsed.kind === 'bench' ? { kind: 'bench', index: parsed.index } : null)
  }

  function handleDragEnd(e: DragEndEvent): void {
    setDragging(null)
    if (!onEdit || !e.over) return
    const from = parseDragId(String(e.active.id))
    const to = parseDragId(String(e.over.id))
    if (!from || !to || from.kind === 'open' || from.kind === 'benchlist') return
    const next = structuredClone(report)
    // Normalize a legacy (pre-cohorts) report into one cohort before editing,
    // so mutations always work on the cohort shape.
    if (!(next.cohorts && next.cohorts.length > 0)) {
      next.cohorts = cohortsOf(report).map((c) => ({ ...c, groups: c.groups.map((g) => ({ ...g, slots: g.slots.map((s) => ({ ...s })) })) }))
    }
    let done = false
    if (to.kind === 'benchlist') {
      done = applyBench(next, from)
    } else if (to.kind === 'seat') {
      done = applySwap(next, from, { kind: 'seat', ref: to.ref })
    } else if (to.kind === 'open') {
      done = applyInsert(next, from, to.cohort, to.group)
    }
    if (done) onEdit(finalize(next))
  }

  const draggedSlot = dragging?.kind === 'seat'
    ? groupIn(cohortByNumber(report, dragging.ref.cohort), dragging.ref.group)?.slots[dragging.ref.index] ?? null
    : null
  const draggedEntry = dragging?.kind === 'bench' ? unassigned[dragging.index] ?? null : null

  const { setNodeRef: setBenchDropRef, isOver: benchOver } = useDroppable({ id: 'benchlist', disabled: !interactive })

  const body = (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3 text-sm flex-wrap" style={{ color: 'var(--color-muted-foreground)' }}>
        <Users size={14} />
        <span>
          Proposal for <span className="font-medium" style={{ color: 'var(--color-foreground)' }}>{report.encounter_name}</span>
          {' '}· {cohorts.length > 1 ? `${cohorts.length} raids` : `${cohorts[0]?.groups.length ?? 0} group${(cohorts[0]?.groups.length ?? 0) === 1 ? '' : 's'}`}
          {' '}of ≤{report.group_size} · {report.preference} seating
        </span>
        {interactive ? (
          <span className="text-xs opacity-80">
            drag members to swap · drop on an open seat to insert · drop on Unseated to bench
          </span>
        ) : null}
      </div>

      {report.warnings && report.warnings.length > 0 ? (
        <div className="rounded-lg px-3 py-2 flex flex-col gap-1" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}>
          {report.warnings.map((w, i) => (
            <div key={i} className="flex items-start gap-1.5 text-xs" style={{ color: 'var(--color-foreground)' }}>
              <AlertTriangle size={12} className="mt-0.5 shrink-0" style={{ color: 'var(--color-warning, #eab308)' }} />
              {w}
            </div>
          ))}
        </div>
      ) : null}

      {cohorts.map((cohort) => (
        <div key={cohort.number} className="flex flex-col gap-3">
          {cohorts.length > 1 ? (
            <div
              className="flex items-center gap-2 px-3 py-1.5 rounded"
              style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
            >
              <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
                Raid {cohort.number}
              </span>
              <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
                {cohort.roster_count} members · {cohort.groups.length} group{cohort.groups.length === 1 ? '' : 's'}
              </span>
              {cohort.warnings && cohort.warnings.length > 0 ? (
                <span className="text-[11px] flex items-center gap-1" style={{ color: 'var(--color-danger)' }}>
                  <AlertTriangle size={11} /> {cohort.warnings[0]}
                </span>
              ) : null}
              <span className="ml-auto flex items-center gap-3">
                <CoveragePills rows={cohort.min ?? []} level="MIN" />
                <CoveragePills rows={cohort.rec ?? []} level="REC" />
              </span>
            </div>
          ) : null}
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-3 items-start">
            {cohort.groups.map((g) => (
              <GroupCard
                key={g.number}
                title={cohorts.length > 1 ? `Raid ${cohort.number} · Group ${g.number}` : `Group ${g.number}`}
                cohort={cohort.number}
                group={g}
                interactive={interactive}
                classNames={classNames}
              />
            ))}
          </div>
          <CoverageTable min={cohort.min ?? []} rec={cohort.rec ?? []} />
        </div>
      ))}

      {unassigned.length > 0 || interactive ? (
        <div
          ref={setBenchDropRef}
          className="rounded-lg overflow-hidden"
          style={{
            border: `1px solid ${benchOver && dragging?.kind === 'seat' ? 'var(--color-primary)' : 'var(--color-border)'}`,
            backgroundColor: 'var(--color-surface)',
          }}
        >
          <div
            className="flex items-center gap-2 px-3 py-1.5"
            style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
          >
            <UserX size={13} style={{ color: 'var(--color-danger)' }} />
            <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
              Unseated ({unassigned.length})
            </span>
            {interactive && benchOver && dragging?.kind === 'seat' ? (
              <span className="text-xs" style={{ color: 'var(--color-primary)' }}>drop to bench</span>
            ) : null}
          </div>
          {unassigned.length === 0 ? (
            <div className="px-3 py-2 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
              everyone is seated — drag a member here to bench them
            </div>
          ) : (
            unassigned.map((u, i) => (
              <BenchRow key={`${u.name}-${i}`} entry={u} index={i} interactive={interactive} stripe={i % 2 === 1} classNames={classNames} />
            ))
          )}
        </div>
      ) : null}
    </div>
  )

  if (!interactive) return body

  return (
    <DndContext
      sensors={sensors}
      // Re-measure droppables while dragging: the Unseated card sits below
      // the fold of a long scrollable page, and droppable rects measured
      // once at drag start go stale the moment the user scrolls — the bench
      // drop then silently missed (no highlight, no mutation).
      measuring={{ droppable: { strategy: MeasuringStrategy.Always } }}
      onDragStart={handleDragStart}
      onDragEnd={handleDragEnd}
      onDragCancel={() => setDragging(null)}
    >
      {body}
      <DragOverlay>
        {draggedSlot ? (
          <div className="flex items-center gap-2 rounded border px-2 py-1 text-xs shadow-lg" style={{ backgroundColor: 'var(--color-surface-2)', borderColor: 'var(--color-primary)', color: 'var(--color-foreground)' }}>
            {draggedSlot.member}
            {draggedSlot.class ? <span className="opacity-60">({classNames?.[draggedSlot.class] ?? draggedSlot.class})</span> : null}
          </div>
        ) : draggedEntry ? (
          <div className="flex items-center gap-2 rounded border px-2 py-1 text-xs shadow-lg" style={{ backgroundColor: 'var(--color-surface-2)', borderColor: 'var(--color-primary)', color: 'var(--color-foreground)' }}>
            {draggedEntry.name}
            {draggedEntry.class ? <span className="opacity-60">({classNames?.[draggedEntry.class] ?? draggedEntry.class})</span> : null}
          </div>
        ) : null}
      </DragOverlay>
    </DndContext>
  )
}