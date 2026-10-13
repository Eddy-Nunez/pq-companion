import React from 'react'
import { Crosshair, ExternalLink } from 'lucide-react'
import OverlayWindow from '../OverlayWindow'
import KillCounterView from './KillCounterView'
import { useKillCounters } from '../../hooks/useKillCounters'

interface KillCounterPanelProps {
  defaultX?: number
  defaultY?: number
  defaultWidth?: number
  defaultHeight?: number
  snapGridSize?: number
  onLayoutChange?: (b: { x: number; y: number; width: number; height: number }) => void
}

// Dashboard-embedded Kill Counter panel — same live state as the popout
// window (KillCounterWindowPage), both fed by killcounters:state broadcasts.
export default function KillCounterPanel({
  defaultX = 24,
  defaultY = 24,
  defaultWidth = 304,
  defaultHeight = 240,
  snapGridSize,
  onLayoutChange,
}: KillCounterPanelProps): React.ReactElement {
  const k = useKillCounters()
  const btn: React.CSSProperties = {
    background: 'none', border: 'none', cursor: 'pointer', padding: '1px 3px',
    color: 'var(--color-muted)', display: 'flex', alignItems: 'center',
  }

  return (
    <OverlayWindow
      title={
        <span style={{ display: 'flex', alignItems: 'center', gap: 5 }}>
          <Crosshair size={13} style={{ color: '#f59e0b' }} />
          Kill Counters
        </span>
      }
      headerRight={
        window.electron?.overlay ? (
          <button
            onClick={() => window.electron.overlay.toggleKillCounter()}
            title="Pop out as floating overlay"
            style={btn}
          >
            <ExternalLink size={12} />
          </button>
        ) : undefined
      }
      defaultWidth={defaultWidth}
      defaultHeight={defaultHeight}
      defaultX={defaultX}
      defaultY={defaultY}
      minWidth={220}
      minHeight={120}
      snapGridSize={snapGridSize}
      onLayoutChange={onLayoutChange}
    >
      <KillCounterView theme="panel" counters={k.counters} skewSecs={k.skewSecs} onAction={k.act} />
    </OverlayWindow>
  )
}
