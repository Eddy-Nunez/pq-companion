import React from 'react'
import { Filter, ExternalLink, SlidersHorizontal } from 'lucide-react'
import OverlayWindow from '../OverlayWindow'
import FilteredTimerView, { filterSummary } from './FilteredTimerView'
import { useFilteredTimers } from '../../hooks/useFilteredTimers'
import { useTimerAppearance } from '../../hooks/useTimerAppearance'

interface FilteredTimerPanelProps {
  defaultX?: number
  defaultY?: number
  defaultWidth?: number
  defaultHeight?: number
  snapGridSize?: number
  onLayoutChange?: (b: { x: number; y: number; width: number; height: number }) => void
}

// Dashboard-embedded Filtered Timers panel — same data and filter config as
// the popout window (FilteredTimerWindowPage), kept in sync via localStorage.
export default function FilteredTimerPanel({
  defaultX = 24,
  defaultY = 24,
  defaultWidth = 304,
  defaultHeight = 384,
  snapGridSize,
  onLayoutChange,
}: FilteredTimerPanelProps): React.ReactElement {
  const f = useFilteredTimers()
  const appearance = useTimerAppearance()
  const btn: React.CSSProperties = {
    background: 'none', border: 'none', cursor: 'pointer', padding: '1px 3px',
    color: 'var(--color-muted)', display: 'flex', alignItems: 'center',
  }

  return (
    <OverlayWindow
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 5, minWidth: 0 }}>
          <Filter size={13} style={{ color: '#38bdf8', flexShrink: 0 }} />
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {filterSummary(f.config)}
          </span>
        </span>
      }
      headerRight={
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <button
            onClick={f.toggleCollapsed}
            title={f.collapsed ? 'Show filters' : 'Hide filters'}
            style={{ ...btn, color: f.collapsed ? 'var(--color-muted)' : 'var(--color-foreground)' }}
          >
            <SlidersHorizontal size={12} />
          </button>
          {window.electron?.overlay && (
            <button
              onClick={() => window.electron.overlay.toggleFilteredTimer()}
              title="Pop out as floating overlay"
              style={btn}
            >
              <ExternalLink size={12} />
            </button>
          )}
        </div>
      }
      defaultWidth={defaultWidth}
      defaultHeight={defaultHeight}
      defaultX={defaultX}
      defaultY={defaultY}
      minWidth={240}
      minHeight={160}
      snapGridSize={snapGridSize}
      onLayoutChange={onLayoutChange}
    >
      <FilteredTimerView
        theme="panel"
        config={f.config}
        update={f.update}
        showFilters={!f.collapsed}
        timers={f.timers}
        groups={f.groups}
        activePlayer={f.activePlayer}
        roster={f.roster}
        zealConnected={f.zealConnected}
        appearance={appearance}
        rowsInteractive
      />
    </OverlayWindow>
  )
}
