import React from 'react'
import { ShieldAlert, ShieldCheck, ExternalLink, Circle } from 'lucide-react'
import { useRaidReadiness } from '../../hooks/useRaidReadiness'
import OverlayWindow from '../OverlayWindow'
import { deriveGapRows, RaidGapRow } from './raidReadinessShared'

interface RaidReadinessPanelProps {
  defaultX?: number
  defaultY?: number
  defaultWidth?: number
  defaultHeight?: number
  snapGridSize?: number
  onLayoutChange?: (b: { x: number; y: number; width: number; height: number }) => void
}

// RaidReadinessPanel is the dashboard-embedded view of the raid composition
// checker: just the MIN-level gap rows for whatever encounter got detected
// (or was last picked on the Raid Composition page) — "what are we still
// missing," not the full MIN+REC report. See RaidCheckPage for that.
export default function RaidReadinessPanel({
  defaultX = 24,
  defaultY = 24,
  defaultWidth = 280,
  defaultHeight = 260,
  snapGridSize,
  onLayoutChange,
}: RaidReadinessPanelProps): React.ReactElement {
  const { roster, report, encounters, selectedId } = useRaidReadiness()
  const encounter = encounters.find((e) => e.id === selectedId)
  const gapRows = deriveGapRows(report, 'min')
  const ok = report ? gapRows.length === 0 : null

  return (
    <OverlayWindow
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          {ok === false ? (
            <ShieldAlert size={13} style={{ color: '#ef4444' }} />
          ) : (
            <ShieldCheck size={13} style={{ color: '#a855f7' }} />
          )}
          Raid Readiness
        </span>
      }
      headerRight={
        window.electron?.overlay && (
          <button
            onClick={() => window.electron.overlay.toggleRaidReadiness()}
            title="Pop out as floating overlay"
            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '1px 3px', color: 'var(--color-muted)', display: 'flex', alignItems: 'center' }}
          >
            <ExternalLink size={12} />
          </button>
        )
      }
      defaultWidth={defaultWidth}
      defaultHeight={defaultHeight}
      defaultX={defaultX}
      defaultY={defaultY}
      minWidth={220}
      minHeight={140}
      snapGridSize={snapGridSize}
      onLayoutChange={onLayoutChange}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '6px 10px', fontSize: 11, borderBottom: '1px solid var(--color-border)', flexShrink: 0, backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted)' }}>
        <Circle size={10} style={{ color: roster?.in_raid ? '#22c55e' : '#6b7280' }} />
        {encounter ? `${encounter.name} — ${encounter.zone}` : 'No encounter selected'}
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto', display: 'flex', flexDirection: 'column' }}>
        {!report ? (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 8, color: 'var(--color-muted)', padding: 16 }}>
            <ShieldCheck size={28} style={{ opacity: 0.2 }} />
            <p style={{ fontSize: 12, margin: 0, textAlign: 'center' }}>
              Pick an encounter on the Raid Composition page
            </p>
          </div>
        ) : gapRows.length === 0 ? (
          <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 8, color: 'var(--color-muted)', padding: 16 }}>
            <ShieldCheck size={28} style={{ color: '#22c55e', opacity: 0.6 }} />
            <p style={{ fontSize: 12, margin: 0, textAlign: 'center' }}>Every MIN role is staffed.</p>
          </div>
        ) : (
          gapRows.map((row) => <RaidGapRow key={row.path} row={row} variant="panel" />)
        )}
      </div>
    </OverlayWindow>
  )
}
