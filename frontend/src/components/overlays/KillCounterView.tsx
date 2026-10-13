/**
 * KillCounterView — body of the Kill Counter overlay, shared by the dashboard
 * panel (KillCounterPanel) and the popout window (KillCounterWindowPage).
 * Shows every enabled counter, grouped, with a milestone-ticked progress bar,
 * the next milestone, and the event clock when one is running.
 *
 * The `theme` prop picks the colour set: 'panel' uses the app's CSS tokens,
 * 'overlay' uses the translucent-white palette every popout overlay uses.
 */
import React, { useMemo } from 'react'
import { Minus, Plus, RotateCcw } from 'lucide-react'
import { fmtClock, nextMilestone, useNowSecs } from '../../hooks/useKillCounters'
import type { KillCounter, KillCounterAction } from '../../types/kills'

export type KillCounterTheme = 'panel' | 'overlay'

interface Palette {
  fg: string
  muted: string
  border: string
  track: string
}

const PALETTES: Record<KillCounterTheme, Palette> = {
  panel: {
    fg: 'var(--color-foreground)',
    muted: 'var(--color-muted)',
    border: 'var(--color-border)',
    track: 'var(--color-surface-2)',
  },
  overlay: {
    fg: 'rgba(255,255,255,1)',
    muted: 'rgba(255,255,255,0.55)',
    border: 'rgba(255,255,255,0.1)',
    track: 'rgba(255,255,255,0.08)',
  },
}

const ACCENT = '#f59e0b'
const DONE = '#22c55e'
const URGENT = '#ef4444'

interface KillCounterViewProps {
  theme: KillCounterTheme
  counters: KillCounter[]
  skewSecs: number
  // When set, +/−/reset buttons render on each counter.
  onAction?: (id: string, body: KillCounterAction) => void
  hideEmptyState?: boolean
}

export default function KillCounterView({
  theme,
  counters,
  skewSecs,
  onAction,
  hideEmptyState,
}: KillCounterViewProps): React.ReactElement {
  const pal = PALETTES[theme]
  const enabled = useMemo(() => counters.filter((c) => c.def.enabled), [counters])
  const anyClock = enabled.some((c) => c.def.clock && c.clock_started_at > 0)
  const now = useNowSecs(anyClock, skewSecs)

  // Preserve backend order; a group's header renders before its first member.
  const rows: Array<{ header?: string; counter: KillCounter }> = []
  let lastGroup: string | undefined
  for (const c of enabled) {
    const g = c.def.group || undefined
    rows.push({ header: g && g !== lastGroup ? g : undefined, counter: c })
    lastGroup = g
  }

  if (enabled.length === 0) {
    if (hideEmptyState) return <div />
    return (
      <div style={{ padding: 12, fontSize: 12, color: pal.muted, textAlign: 'center' }}>
        No kill counters enabled. Add one on the Kill Tracker page.
      </div>
    )
  }

  return (
    <div style={{ flex: 1, overflowY: 'auto', padding: '6px 8px', display: 'flex', flexDirection: 'column', gap: 8 }}>
      {rows.map(({ header, counter }) => (
        <React.Fragment key={counter.def.id}>
          {header && (
            <div
              style={{
                fontSize: 10,
                fontWeight: 700,
                letterSpacing: 0.6,
                textTransform: 'uppercase',
                color: pal.muted,
                marginTop: 2,
              }}
            >
              {header}
            </div>
          )}
          <CounterRow counter={counter} pal={pal} now={now} onAction={onAction} />
        </React.Fragment>
      ))}
    </div>
  )
}

function CounterRow({
  counter: c,
  pal,
  now,
  onAction,
}: {
  counter: KillCounter
  pal: Palette
  now: number
  onAction?: (id: string, body: KillCounterAction) => void
}): React.ReactElement {
  const milestones = c.def.milestones ?? []
  const goal = c.def.goal || (milestones.length ? milestones[milestones.length - 1].at : 0)
  const done = goal > 0 && c.count >= goal
  const next = nextMilestone(c)
  const pct = goal > 0 ? Math.min(100, (c.count / goal) * 100) : 0

  let clockText: string | null = null
  let clockUrgent = false
  if (c.def.clock && c.clock_started_at > 0) {
    const left = c.clock_started_at + c.def.clock.duration_secs - now
    clockText = left > 0 ? fmtClock(left) : 'expired'
    clockUrgent = left < 300
  }

  const btn: React.CSSProperties = {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: 18,
    height: 18,
    borderRadius: 3,
    border: `1px solid ${pal.border}`,
    background: 'transparent',
    color: pal.muted,
    cursor: 'pointer',
    padding: 0,
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
        <span
          style={{
            flex: 1,
            minWidth: 0,
            fontSize: 12,
            fontWeight: 600,
            color: pal.fg,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {c.def.name}
        </span>
        {onAction && (
          <span style={{ display: 'flex', gap: 3, alignSelf: 'center' }}>
            <button style={btn} title="Subtract one" onClick={() => onAction(c.def.id, { action: 'adjust', value: -1 })}>
              <Minus size={10} />
            </button>
            <button style={btn} title="Add one" onClick={() => onAction(c.def.id, { action: 'adjust', value: 1 })}>
              <Plus size={10} />
            </button>
            <button
              style={btn}
              title="Reset count and clock"
              onClick={() => {
                if (window.confirm(`Reset "${c.def.name}" to 0?`)) onAction(c.def.id, { action: 'reset' })
              }}
            >
              <RotateCcw size={10} />
            </button>
          </span>
        )}
        <span
          style={{
            fontSize: 16,
            fontWeight: 700,
            fontVariantNumeric: 'tabular-nums',
            color: done ? DONE : pal.fg,
          }}
        >
          {c.count}
          {goal > 0 && <span style={{ fontSize: 11, fontWeight: 500, color: pal.muted }}> / {goal}</span>}
        </span>
      </div>

      {goal > 0 && (
        <div style={{ position: 'relative', height: 6, borderRadius: 3, background: pal.track, overflow: 'hidden' }}>
          <div
            style={{
              position: 'absolute',
              inset: 0,
              width: `${pct}%`,
              background: done ? DONE : ACCENT,
              transition: 'width 0.3s ease',
            }}
          />
          {milestones
            .filter((m) => m.at < goal)
            .map((m) => (
              <div
                key={m.at}
                title={`${m.at}: ${m.label}`}
                style={{
                  position: 'absolute',
                  top: 0,
                  bottom: 0,
                  left: `${(m.at / goal) * 100}%`,
                  width: 1,
                  background: m.at <= c.count ? 'rgba(0,0,0,0.45)' : pal.muted,
                  opacity: 0.8,
                }}
              />
            ))}
        </div>
      )}

      <div style={{ display: 'flex', gap: 8, fontSize: 11, color: pal.muted, minWidth: 0 }}>
        <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {next ? (
            <>
              Next: <span style={{ color: pal.fg }}>{next.label || next.at}</span> at {next.at}
              {' · '}
              <span style={{ color: next.at - c.count <= 2 ? ACCENT : pal.muted, fontWeight: next.at - c.count <= 2 ? 700 : 400 }}>
                {next.at - c.count} to go
              </span>
            </>
          ) : done ? (
            <span style={{ color: DONE }}>Complete</span>
          ) : null}
        </span>
        {clockText && (
          <span
            title={c.def.clock?.label}
            style={{ fontVariantNumeric: 'tabular-nums', color: clockUrgent ? URGENT : pal.fg, fontWeight: 600 }}
          >
            {clockText}
          </span>
        )}
      </div>
    </div>
  )
}
