import React from 'react'
import { ShieldCheck, ShieldAlert, Users } from 'lucide-react'
import type { CheckReport, CheckRowReport, CheckSummaryAnswer } from '../../types/raid'
import { roleLabel, subLabel } from '../../lib/raidLabels'

// CompReport renders the structured MIN/REC composition check result with its
// own GUI — the EQMon behavior of copying a text report to the clipboard is
// replaced by this view (per the feature brief).

interface Props {
  report: CheckReport
}

function summaryPill(sum: CheckSummaryAnswer): React.ReactElement {
  if (sum.rows === 0) {
    return <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>no comp defined</span>
  }
  if (sum.ok) {
    return (
      <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-success)' }}>
        <ShieldCheck size={13} /> OK — every role met
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1 text-xs font-medium" style={{ color: 'var(--color-danger)' }}>
      <ShieldAlert size={13} />
      {sum.gap_rows} role{sum.gap_rows === 1 ? '' : 's'} short by {sum.gap_count}
    </span>
  )
}

function Row({ row, className }: { row: CheckRowReport; className: string }): React.ReactElement {
  const gap = row.need - row.have
  return (
    <div
      className={`grid grid-cols-[1fr_auto_auto_auto_1.4fr] gap-3 items-center px-3 py-1.5 ${className}`}
    >
      <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
        {row.label || (
          <>
            {roleLabel(row.role)}
            {row.sub_role ? <span className="opacity-70"> / {subLabel(row.sub_role)}</span> : null}
          </>
        )}
      </span>
      <span className="text-sm tabular-nums" style={{ color: 'var(--color-muted-foreground)' }}>
        {row.have}
        <span className="opacity-60">/{row.need}</span>
      </span>
      {gap > 0 ? (
        <span className="text-[11px] px-1.5 py-0.5 rounded font-medium" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          GAP {gap}
        </span>
      ) : (
        <span className="text-[11px] px-1.5 py-0.5 rounded font-medium" style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-success)' }}>
          OK
        </span>
      )}
      <span className="text-xs truncate" style={{ color: 'var(--color-muted-foreground)' }}>
        {gap > 0 && row.candidates ? (
          <>
            {row.candidates.join(', ')}
            {row.more_candidates ? <span className="opacity-60"> +{row.more_candidates} more</span> : null}
          </>
        ) : null}
      </span>
    </div>
  )
}

function Section({ title, subtitle, summary, rows }: {
  title: string
  subtitle: string
  summary: CheckSummaryAnswer
  rows: CheckRowReport[]
}): React.ReactElement {
  return (
    <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
      <div className="flex items-center justify-between px-3 py-2" style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}>
        <div className="flex items-center gap-2">
          <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>{title}</span>
          <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>{subtitle}</span>
        </div>
        {summaryPill(summary)}
      </div>
      {rows.length === 0 ? (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          This encounter has no {title.toLowerCase()} composition recorded.
        </div>
      ) : (
        <div>
          <div
            className="grid grid-cols-[1fr_auto_auto_auto_1.4fr] gap-3 px-3 py-1 text-[11px] uppercase tracking-wide"
            style={{ color: 'var(--color-muted-foreground)' }}
          >
            <span>Role</span>
            <span>Have / Need</span>
            <span>Status</span>
            <span>Class-eligible candidates</span>
            <span />
          </div>
          {rows.map((row, i) => (
            <Row key={row.path} row={row} className={i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''} />
          ))}
        </div>
      )}
    </div>
  )
}

export default function CompReport({ report }: Props): React.ReactElement {
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-muted-foreground)' }}>
        <Users size={14} />
        Raid: <span className="font-medium" style={{ color: 'var(--color-foreground)' }}>{report.roster_mapped}</span>
        /{report.roster_total} members classed
        {report.zone ? <> · zone: <span className="font-medium">{report.zone}</span></> : null}
        <span className="opacity-70">· candidates are class-eligible members, not assignments</span>
      </div>
      <Section
        title="MIN — Hard floor"
        subtitle="below this, don't pull"
        summary={report.summary.min}
        rows={report.min}
      />
      <Section
        title="REC — Recommended"
        subtitle="first kills / learning passes"
        summary={report.summary.rec}
        rows={report.rec}
      />
    </div>
  )
}
