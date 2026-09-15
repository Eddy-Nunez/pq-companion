import React, { useMemo } from 'react'
import { ShieldCheck, ShieldAlert, Users } from 'lucide-react'
import type { CheckReport, CheckRowReport, CheckSummaryAnswer } from '../../types/raid'
import { roleLabel, subLabel } from '../../lib/raidLabels'

// CompReport renders the structured MIN/REC composition check result with its
// own GUI — the EQMon behavior of copying a text report to the clipboard is
// replaced by this view (per the feature brief).
//
// MIN and REC share one table (two columns) instead of two stacked sections:
// they're staffing floors for the *same* roles, so reading them side by side
// makes the gap between "can attempt" and "comfortable" immediate. Fixed
// pixel column widths (rather than `auto`) keep every row's numbers aligned
// to the header — each row is its own CSS grid, so `auto`-sized columns
// drift out of alignment the moment one row's content is wider than another.

interface Props {
  report: CheckReport
}

// One grid template shared by the header and every data row so columns line
// up regardless of each row's own content width (see file header comment).
const GRID_COLS = 'grid-cols-[minmax(140px,1.3fr)_76px_60px_76px_60px_minmax(140px,1.3fr)]'

function summaryPill(sum: CheckSummaryAnswer, label: string): React.ReactElement {
  if (sum.rows === 0) {
    return (
      <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        {label}: no comp defined
      </span>
    )
  }
  if (sum.ok) {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-success)' }}>
        <ShieldCheck size={13} /> {label} OK
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-danger)' }}>
      <ShieldAlert size={13} />
      {label} short by {sum.gap_count} ({sum.gap_rows} role{sum.gap_rows === 1 ? '' : 's'})
    </span>
  )
}

// MergedRow pairs one taxonomy leaf's MIN and REC assessment. Either side can
// be absent — a role required only at REC (min count 0) is a real case the
// backend supports (see raidcomp.Check), so this isn't just defensive code.
interface MergedRow {
  path: string
  role: string
  sub_role?: string
  label: string
  min?: CheckRowReport
  rec?: CheckRowReport
}

function mergeRows(min: CheckRowReport[], rec: CheckRowReport[]): MergedRow[] {
  const byPath = new Map<string, MergedRow>()
  const order: string[] = []
  for (const row of min) {
    byPath.set(row.path, { path: row.path, role: row.role, sub_role: row.sub_role, label: row.label, min: row })
    order.push(row.path)
  }
  for (const row of rec) {
    const existing = byPath.get(row.path)
    if (existing) {
      existing.rec = row
    } else {
      byPath.set(row.path, { path: row.path, role: row.role, sub_role: row.sub_role, label: row.label, rec: row })
      order.push(row.path)
    }
  }
  return order.map((p) => byPath.get(p)!)
}

function CountCell({ row }: { row?: CheckRowReport }): React.ReactElement {
  if (!row) {
    return <span className="text-sm justify-self-end opacity-40" style={{ color: 'var(--color-muted-foreground)' }}>—</span>
  }
  return (
    <span className="text-sm tabular-nums justify-self-end" style={{ color: 'var(--color-muted-foreground)' }}>
      {row.have}
      <span className="opacity-60">/{row.need}</span>
    </span>
  )
}

function StatusCell({ row }: { row?: CheckRowReport }): React.ReactElement {
  if (!row) {
    return <span className="justify-self-end" />
  }
  const gap = row.need - row.have
  if (gap > 0) {
    return (
      <span
        className="text-[11px] px-1.5 py-0.5 rounded font-medium justify-self-end"
        style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}
      >
        GAP {gap}
      </span>
    )
  }
  return (
    <span
      className="text-[11px] px-1.5 py-0.5 rounded font-medium justify-self-end"
      style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-success)' }}
    >
      OK
    </span>
  )
}

function Row({ row, className }: { row: MergedRow; className: string }): React.ReactElement {
  // Prefer whichever level is actually short-staffed for the candidates list
  // — REC is the wider ask, so check it first.
  const candidateSource =
    row.rec && row.rec.need > row.rec.have ? row.rec : row.min && row.min.need > row.min.have ? row.min : undefined
  return (
    <div className={`grid ${GRID_COLS} gap-3 items-center px-3 py-1.5 ${className}`}>
      <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
        {row.label || (
          <>
            {roleLabel(row.role)}
            {row.sub_role ? <span className="opacity-70"> / {subLabel(row.sub_role)}</span> : null}
          </>
        )}
      </span>
      <CountCell row={row.min} />
      <StatusCell row={row.min} />
      <CountCell row={row.rec} />
      <StatusCell row={row.rec} />
      <span className="text-xs truncate" style={{ color: 'var(--color-muted-foreground)' }}>
        {candidateSource?.candidates ? (
          <>
            {candidateSource.candidates.join(', ')}
            {candidateSource.more_candidates ? <span className="opacity-60"> +{candidateSource.more_candidates} more</span> : null}
          </>
        ) : null}
      </span>
    </div>
  )
}

export default function CompReport({ report }: Props): React.ReactElement {
  const rows = useMemo(() => mergeRows(report.min, report.rec), [report.min, report.rec])

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2 text-sm flex-wrap" style={{ color: 'var(--color-muted-foreground)' }}>
        <Users size={14} />
        Raid: <span className="font-medium" style={{ color: 'var(--color-foreground)' }}>{report.roster_mapped}</span>
        /{report.roster_total} members classed
        {report.zone ? <> · zone: <span className="font-medium">{report.zone}</span></> : null}
        <span className="opacity-70">· candidates are class-eligible members, not assignments</span>
      </div>

      <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
        <div
          className="flex items-center justify-between px-3 py-2 flex-wrap gap-1.5"
          style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
        >
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>Composition</span>
            <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
              MIN = hard floor, don&apos;t pull below it · REC = comfortable, first kills / learning passes
            </span>
          </div>
          <div className="flex items-center gap-3">
            {summaryPill(report.summary.min, 'MIN')}
            {summaryPill(report.summary.rec, 'REC')}
          </div>
        </div>

        {rows.length === 0 ? (
          <div className="px-3 py-3 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            This encounter has no composition recorded.
          </div>
        ) : (
          <div>
            <div
              className={`grid ${GRID_COLS} gap-3 px-3 py-1 text-[11px] uppercase tracking-wide`}
              style={{ color: 'var(--color-muted-foreground)' }}
            >
              <span>Role</span>
              <span className="justify-self-end">Min</span>
              <span className="justify-self-end">Status</span>
              <span className="justify-self-end">Rec</span>
              <span className="justify-self-end">Status</span>
              <span>Class-eligible candidates</span>
            </div>
            {rows.map((row, i) => (
              <Row key={row.path} row={row} className={i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
