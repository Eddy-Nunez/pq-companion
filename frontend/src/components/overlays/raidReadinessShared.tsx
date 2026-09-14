import React from 'react'
import type { CheckReport, CheckRowReport } from '../../types/raid'

// deriveGapRows reduces a full CheckReport down to just the short (Have <
// Need) rows at one level, sorted worst-gap-first — this overlay is a "what
// are we still missing" glance, not the full MIN+REC report (that's
// RaidCheckPage/CompReport). MIN is the hard floor, so it's the level that
// actually matters for a go/no-go readiness HUD.
export function deriveGapRows(report: CheckReport | null, level: 'min' | 'rec' = 'min'): CheckRowReport[] {
  if (!report) return []
  const rows = level === 'min' ? report.min : report.rec
  return [...rows].filter((r) => r.need - r.have > 0).sort((a, b) => (b.need - b.have) - (a.need - a.have))
}

interface RaidGapRowProps {
  row: CheckRowReport
  variant: 'panel' | 'window'
}

// RaidGapRow renders one short-staffed role. Shared between the dashboard
// panel and the popout window, mirroring ZoneLockoutRow's panel/window split.
export function RaidGapRow({ row, variant }: RaidGapRowProps): React.ReactElement {
  const win = variant === 'window'
  const gap = row.need - row.have
  const border = win ? '1px solid rgba(255,255,255,0.08)' : '1px solid var(--color-border)'
  const textShadow = win ? '0 1px 2px rgba(0,0,0,0.9)' : undefined
  const nameColor = win ? 'rgba(255,255,255,0.92)' : 'var(--color-foreground)'
  const mutedColor = win ? 'rgba(255,255,255,0.4)' : 'var(--color-muted-foreground)'
  const gapBg = win ? 'rgba(239,68,68,0.85)' : 'var(--color-danger)'

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 8,
        padding: win ? '4px 8px' : '5px 10px',
        borderBottom: border,
      }}
    >
      <span
        style={{
          fontSize: win ? 11 : 12,
          color: nameColor,
          textShadow,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}
        title={row.candidates && row.candidates.length > 0 ? `Candidates: ${row.candidates.join(', ')}` : undefined}
      >
        {row.label || row.role}
      </span>
      <span style={{ display: 'flex', alignItems: 'center', gap: 6, flexShrink: 0 }}>
        <span style={{ fontSize: win ? 10 : 11, color: mutedColor, textShadow }}>
          {row.have}/{row.need}
        </span>
        <span
          style={{
            fontSize: win ? 10 : 10,
            fontWeight: 700,
            padding: '1px 5px',
            borderRadius: 3,
            backgroundColor: gapBg,
            color: '#fff',
          }}
        >
          GAP {gap}
        </span>
      </span>
    </div>
  )
}
