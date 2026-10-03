import React, { useMemo } from 'react'
import { Users, UserX, AlertTriangle, ShieldCheck } from 'lucide-react'
import type { SplitReport, SplitCoverage, ProposedGroup } from '../../types/raid'

// GroupProposal renders the split proposal: proposed groups with their
// role-filled seats, unseated members with reasons, coverage rollups against
// the encounter's MIN/REC staffing, and any proposal warnings. It is a
// PROPOSAL view — nothing here mutates the raid or the roster; the group
// leader applies it in-game.
interface Props {
  report: SplitReport
}

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

const SLOT_COLS = 'grid-cols-[minmax(120px,1.2fr)_minmax(70px,0.8fr)_minmax(150px,1.4fr)]'

function GroupCard({ group }: { group: ProposedGroup }): React.ReactElement {
  return (
    <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
      <div
        className="flex items-center gap-2 px-3 py-1.5"
        style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
      >
        <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
          Group {group.number}
        </span>
        <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
          {group.slots.length}/{group.size} seats
        </span>
      </div>
      <div>
        <div
          className={`grid ${SLOT_COLS} gap-3 px-3 py-1 text-[11px] uppercase tracking-wide`}
          style={{ color: 'var(--color-muted-foreground)' }}
        >
          <span>Member</span>
          <span>Class</span>
          <span>Proposed role</span>
        </div>
        {group.slots.map((slot, i) => (
          <div key={`${slot.member}-${i}`} className={`grid ${SLOT_COLS} gap-3 items-center px-3 py-1.5 ${i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''}`}>
            <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
              {slot.member}
              {slot.rank ? <span className="text-[11px] opacity-60"> · {slot.rank}</span> : null}
            </span>
            <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
              {slot.class ?? '—'}
            </span>
            {slot.role ? (
              <span className="text-xs">
                <span style={{ color: 'var(--color-foreground)' }}>{slot.label || slot.path}</span>
                <span
                  className="ml-1.5 text-[10px] px-1 py-0.5 rounded"
                  style={{
                    backgroundColor: slot.level === 'min' ? 'var(--color-primary)' : 'var(--color-surface-2)',
                    color: slot.level === 'min' ? 'var(--color-primary-foreground, #fff)' : 'var(--color-muted-foreground)',
                  }}
                  title={slot.level === 'min' ? 'Fills a MIN (hard floor) slot' : 'Fills a REC (comfort) slot'}
                >
                  {slot.level}
                </span>
              </span>
            ) : (
              <span className="text-xs opacity-50" style={{ color: 'var(--color-muted-foreground)' }}>
                fill (no comp role)
              </span>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export default function GroupProposal({ report }: Props): React.ReactElement {
  // ?? [] guards: the backend's no-null contract makes these arrays always
  // present, but stale responses (or a mid-upgrade backend) could still send
  // null — render empty rather than crash (the null-classes lesson).
  const groups = report.groups ?? []
  const unassigned = report.unassigned ?? []
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-3 text-sm flex-wrap" style={{ color: 'var(--color-muted-foreground)' }}>
        <Users size={14} />
        <span>
          Proposal for <span className="font-medium" style={{ color: 'var(--color-foreground)' }}>{report.encounter_name}</span>
          {' '}· {groups.length} group{groups.length === 1 ? '' : 's'} of ≤{report.group_size} ·{' '}
          {report.preference} seating
        </span>
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
          <GroupCard key={g.number} group={g} />
        ))}
      </div>

      {unassigned.length > 0 ? (
        <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
          <div
            className="flex items-center gap-2 px-3 py-1.5"
            style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
          >
            <UserX size={13} style={{ color: 'var(--color-danger)' }} />
            <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
              Unseated ({unassigned.length})
            </span>
          </div>
          {unassigned.map((u, i) => (
            <div key={`${u.name}-${i}`} className="flex items-baseline gap-2 px-3 py-1.5 text-sm">
              <span style={{ color: 'var(--color-foreground)' }}>{u.name}</span>
              <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>{u.class ?? 'unclassed'}</span>
              <span className="text-xs" style={{ color: 'var(--color-danger)' }}>{u.reason}</span>
            </div>
          ))}
        </div>
      ) : null}

      <CoverageTable min={report.min ?? []} rec={report.rec ?? []} />
    </div>
  )
}
