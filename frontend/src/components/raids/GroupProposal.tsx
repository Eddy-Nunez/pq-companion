import React, { useMemo, useState } from 'react'
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import type { DragEndEvent, DragStartEvent } from '@dnd-kit/core'
import { Users, UserX, AlertTriangle, ShieldCheck, GripVertical } from 'lucide-react'
import type { CompLevelFill, SplitCoverage, SplitReport, SplitSlot, SplitUnassigned } from '../../types/raid'

// GroupProposal renders the split proposal: proposed groups with their
// role-filled seats, unseated members with reasons, coverage rollups against
// the encounter's MIN/REC staffing, and any proposal warnings. It is a
// PROPOSAL view — nothing here mutates the raid or the roster.
//
// With an onEdit callback the report is interactively adjustable via
// dnd-kit (the house drag layer — native HTML5 drag is unreliable in
// Electron on Windows): drag a member onto another member to SWAP their
// seats (contents exchange wholesale — member, role and rank travel
// together), onto an open seat to INSERT them (a bench member seats as
// fill; a member from another group keeps their role), or onto the
// unseated card to BENCH them. Coverage is recomputed from the edited
// seats after every change, so the MIN/REC table always tells the truth
// about the proposal as adjusted. Regenerating resets all edits.

interface Props {
  report: SplitReport
  onEdit?: (next: SplitReport) => void
  // class code → display name from the live taxonomy ("pal" → "Paladin");
  // codes render verbatim when absent.
  classNames?: Record<string, string>
}

// ── drag ids ────────────────────────────────────────────────────────────────
// seat:<group>:<index>  draggable seat + swap target
// open:<group>:<index>  open-seat insert target (ids must be unique per
//                       placeholder — dnd-kit registers droppables by id)
// bench:<index>         draggable unseated member
// benchlist             the unseated card as a bench drop target

type DragRef = { kind: 'seat'; group: number; index: number } | { kind: 'bench'; index: number }

function parseDragId(id: string): DragRef | null {
  const p = id.split(':')
  if (p[0] === 'seat' && p.length === 3) {
    const group = Number(p[1])
    const index = Number(p[2])
    if (Number.isFinite(group) && Number.isFinite(index)) return { kind: 'seat', group, index }
  }
  if (p[0] === 'bench' && p.length === 2 && Number.isFinite(Number(p[1]))) {
    return { kind: 'bench', index: Number(p[1]) }
  }
  return null
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

function findGroup(r: SplitReport, number: number) {
  return (r.groups ?? []).find((g) => g.number === number)
}

function applySwap(r: SplitReport, a: DragRef, b: DragRef): boolean {
  if (a.kind === 'bench' && b.kind === 'seat') return applySwap(r, b, a)
  if (a.kind === 'seat' && b.kind === 'seat') {
    if (a.group === b.group && a.index === b.index) return false
    const ga = findGroup(r, a.group)
    const gb = findGroup(r, b.group)
    const sa = ga?.slots[a.index]
    const sb = gb?.slots[b.index]
    if (!ga || !gb || !sa || !sb) return false
    ga.slots[a.index] = { ...sb, group: ga.number }
    gb.slots[b.index] = { ...sa, group: gb.number }
    return true
  }
  if (a.kind === 'bench' && b.kind === 'bench') return false // reorder: no-op
  // bench → seat: the incoming member seats as FILL; the displaced member
  // (and their role assignment) goes to the bench — the coverage table
  // reflects the vacated slot honestly.
  const gb = findGroup(r, (b as Extract<DragRef, { kind: 'seat' }>).group)
  const bi = (a as Extract<DragRef, { kind: 'bench' }>).index
  const si = (b as Extract<DragRef, { kind: 'seat' }>).index
  const seat = gb?.slots[si]
  const entry = r.unassigned?.[bi]
  if (!gb || !seat || !entry) return false
  gb.slots[si] = { member: entry.name, class: entry.class, role: '', path: '', label: '', group: gb.number }
  r.unassigned[bi] = { name: seat.member, class: seat.class, reason: 'swapped out by hand' }
  return true
}

function applyInsert(r: SplitReport, from: DragRef, targetGroup: number): boolean {
  const g = findGroup(r, targetGroup)
  if (!g || g.slots.length >= g.size) return false // full: swap onto a member instead
  if (from.kind === 'bench') {
    const entry = r.unassigned?.[from.index]
    if (!entry) return false
    r.unassigned.splice(from.index, 1)
    g.slots.push({ member: entry.name, class: entry.class, role: '', path: '', label: '', group: g.number })
    return true
  }
  if (from.group === targetGroup) return false
  const src = findGroup(r, from.group)
  const slot = src?.slots[from.index]
  if (!src || !slot) return false
  src.slots.splice(from.index, 1) // their seat opens up; the role travels with them
  g.slots.push({ ...slot, group: g.number })
  return true
}

function applyBench(r: SplitReport, from: DragRef): boolean {
  if (from.kind !== 'seat') return false
  const g = findGroup(r, from.group)
  const slot = g?.slots[from.index]
  if (!g || !slot) return false
  g.slots.splice(from.index, 1)
  r.unassigned.push({ name: slot.member, class: slot.class, reason: 'benched by hand' })
  return true
}

// finalize re-stamps group numbers and recomputes coverage from the edited
// seats, so MIN/REC placed counts always match what the proposal now shows.
function finalize(r: SplitReport): SplitReport {
  for (const g of r.groups ?? []) {
    for (const s of g.slots) s.group = g.number
  }
  const placedFor = (path: string, level: CompLevelFill): number =>
    (r.groups ?? []).reduce((n, g) => n + g.slots.filter((s) => s.path === path && s.level === level).length, 0)
  return {
    ...r,
    min: (r.min ?? []).map((row) => ({ ...row, placed: placedFor(row.path, 'min') })),
    rec: (r.rec ?? []).map((row) => ({ ...row, placed: placedFor(row.path, 'rec') })),
  }
}

// ── coverage table (fed live-recomputed rows) ───────────────────────────────

function CoverageRows({ rows, level }: { rows: SplitCoverage[]; level: string }): React.ReactElement {
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
        <ShieldCheck size={13} /> {level} fully staffed
      </span>
    )
  }
  const total = short.reduce((n, r) => n + (r.need - r.placed), 0)
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-danger)' }}>
      <AlertTriangle size={13} /> {level} short by {total} ({short.length} role{short.length === 1 ? '' : 's'})
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
// stay aligned. Interactive rows gain a leading grip cell — the old 3-column
// template mis-rendered 4-cell rows (roles wrapped under the header, the
// bug in the first screenshot pass).
const seatGrid = (interactive: boolean): string =>
  interactive
    ? 'grid-cols-[18px_minmax(110px,1.2fr)_minmax(64px,0.7fr)_minmax(150px,1.6fr)]'
    : 'grid-cols-[minmax(110px,1.2fr)_minmax(64px,0.7fr)_minmax(150px,1.6fr)]'

const dropHighlight: React.CSSProperties = { borderColor: 'var(--color-primary)' }

// SeatRow is both a drag source and a swap target (drop member → member).
function SeatRow({ slot, groupId, index, interactive, stripe, classNames }: {
  slot: SplitSlot
  groupId: number
  index: number
  interactive: boolean
  stripe: boolean
  classNames?: Record<string, string>
}): React.ReactElement {
  const id = `seat:${groupId}:${index}`
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
  return (
    <div
      ref={setRef}
      {...listeners}
      {...attributes}
      className={`grid ${seatGrid(interactive)} gap-3 items-center px-3 py-1.5 border border-transparent ${stripe ? 'bg-(--color-surface-2)/50' : ''} ${interactive ? 'cursor-grab touch-none active:cursor-grabbing' : ''} ${isDragging ? 'opacity-40' : ''}`}
      style={isOver && interactive ? { ...dropHighlight, backgroundColor: 'var(--color-surface-2)' } : undefined}
    >
      {interactive ? <GripVertical size={12} className="opacity-40" style={{ color: 'var(--color-muted-foreground)' }} /> : null}
      <span className="text-sm truncate" title={slot.rank ? `${slot.member} · ${slot.rank}` : slot.member} style={{ color: 'var(--color-foreground)' }}>
        {slot.member}
        {slot.rank ? <span className="text-[11px] opacity-60"> · {slot.rank}</span> : null}
      </span>
      <span className="text-xs" title={className ? slot.class : undefined} style={{ color: 'var(--color-muted-foreground)' }}>
        {className ?? slot.class ?? '—'}
      </span>
      <RoleBadge slot={slot} />
    </div>
  )
}

// OpenSeat is an insert target: an unoccupied seat in a not-full group.
// Each placeholder registers its own droppable id (open:<group>:<index>) —
// duplicate ids would collide in dnd-kit's registry.
function OpenSeat({ groupId, index, interactive }: {
  groupId: number
  index: number
  interactive: boolean
}): React.ReactElement {
  const id = `open:${groupId}:${index}`
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

function GroupCard({ group, interactive, classNames }: {
  group: { number: number; size: number; slots: SplitSlot[] }
  interactive: boolean
  classNames?: Record<string, string>
}): React.ReactElement {
  const openSeats = Math.max(0, group.size - group.slots.length)

  // Group shape summary: family counts in a fixed order so every card reads
  // the same — "2 tank · 1 heal · 2 sup · 1 dmg · 1 fill".
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
          Group {group.number}
        </span>
        <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
          {group.slots.length}/{group.size} seats
        </span>
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
              <SeatRow key={`${slot.member}-${i}`} slot={slot} groupId={group.number} index={i} interactive={interactive} stripe={i % 2 === 1} classNames={classNames} />
            ))}
          </>
        ) : (
          <div className="px-3 py-1.5 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            empty — drag members in
          </div>
        )}
        {interactive
          ? Array.from({ length: openSeats }, (_, i) => (
              <OpenSeat key={i} groupId={group.number} index={i} interactive={interactive} />
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
  const groups = report.groups ?? []
  const unassigned = report.unassigned ?? []
  const interactive = onEdit != null

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }))
  const [dragging, setDragging] = useState<DragRef | null>(null)

  function handleDragStart(e: DragStartEvent): void {
    setDragging(parseDragId(String(e.active.id)))
  }

  function handleDragEnd(e: DragEndEvent): void {
    setDragging(null)
    if (!onEdit || !e.over) return
    const from = parseDragId(String(e.active.id))
    if (!from) return
    const next = structuredClone(report)
    let done = false
    const overId = String(e.over.id)
    if (overId === 'benchlist') {
      done = applyBench(next, from)
    } else {
      const parts = overId.split(':')
      if (parts[0] === 'seat') {
        const to = parseDragId(overId)
        if (to) done = applySwap(next, from, to)
      } else if (parts[0] === 'open') {
        const g = Number(parts[1])
        if (Number.isFinite(g)) done = applyInsert(next, from, g)
      }
    }
    if (done) onEdit(finalize(next))
  }

  const draggedSlot = dragging?.kind === 'seat' ? findGroup(report, dragging.group)?.slots[dragging.index] : null
  const draggedEntry = dragging?.kind === 'bench' ? unassigned[dragging.index] : null

  const { setNodeRef: setBenchDropRef, isOver: benchOver } = useDroppable({ id: 'benchlist', disabled: !interactive })

  const body = (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3 text-sm flex-wrap" style={{ color: 'var(--color-muted-foreground)' }}>
        <Users size={14} />
        <span>
          Proposal for <span className="font-medium" style={{ color: 'var(--color-foreground)' }}>{report.encounter_name}</span>
          {' '}· {groups.length} group{groups.length === 1 ? '' : 's'} of ≤{report.group_size} ·{' '}
          {report.preference} seating
        </span>
        {interactive ? (
          <span className="text-xs opacity-80">
            drag members to swap · drop on an open seat to insert · drop here to bench
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

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-3 items-start">
        {groups.map((g) => (
          <GroupCard key={g.number} group={g} interactive={interactive} classNames={classNames} />
        ))}
      </div>

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

      <CoverageTable min={report.min ?? []} rec={report.rec ?? []} />
    </div>
  )

  if (!interactive) return body

  return (
    <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd} onDragCancel={() => setDragging(null)}>
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