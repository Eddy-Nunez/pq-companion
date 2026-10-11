import { useCallback, useEffect, useMemo, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { useActivePlayerName } from './useActivePlayerName'
import { useHideOtherCharacterTimers, passesCharacterScope } from './useDisplayThresholds'
import { useFilteredTimerConfig } from './useFilteredTimerConfig'
import { usePartyRoster } from './usePartyRoster'
import { useCurrentTargetName } from './useCurrentTargetName'
import { getTimerState } from '../services/api'
import { WSEvent } from '../lib/wsEvents'
import { applyTimerFilter, type FilteredTimerConfig, type TimerGroup } from '../lib/timerFilter'
import type { ActiveTimer, TimerState } from '../types/timer'

/**
 * Everything the Filtered Timers overlay renders, shared by the dashboard
 * panel and the popout window: the live timer feed, the persisted filter
 * config, and the grouped result.
 *
 * Deliberately ignores the global per-category display thresholds — this
 * window has its own "under 1 min" filter, and its whole point is to show
 * the spells you picked regardless of how long they have left. The "hide
 * other characters' timers" setting still applies.
 */
export function useFilteredTimers(): {
  timers: ActiveTimer[] | null
  groups: TimerGroup[]
  config: FilteredTimerConfig
  update: (patch: Partial<FilteredTimerConfig>) => void
  collapsed: boolean
  toggleCollapsed: () => void
  activePlayer: string
  roster: string[]
  zealConnected: boolean
  wsState: ReturnType<typeof useWebSocket>
} {
  const [state, setState] = useState<TimerState | null>(null)
  const activePlayer = useActivePlayerName()
  const hideOtherCharacters = useHideOtherCharacterTimers()
  const { config, update, collapsed, toggleCollapsed } = useFilteredTimerConfig()
  const party = usePartyRoster(activePlayer)
  const currentTarget = useCurrentTargetName()

  useEffect(() => {
    getTimerState().then(setState).catch(() => {})
  }, [])

  const handleMessage = useCallback((msg: { type: string; data: unknown }) => {
    if (msg.type === WSEvent.OverlayTimers) setState(msg.data as TimerState)
  }, [])
  const wsState = useWebSocket(handleMessage)

  const timers = useMemo(
    () =>
      state === null
        ? null
        : (state.timers ?? []).filter((t) => passesCharacterScope(t, hideOtherCharacters, activePlayer)),
    [state, hideOtherCharacters, activePlayer],
  )

  const roster = useMemo(
    () => (config.showMissing ? party.names : []),
    [config.showMissing, party.names],
  )
  const groups = useMemo(
    () => applyTimerFilter(timers ?? [], config, { activePlayer, currentTarget, roster }),
    [timers, config, activePlayer, currentTarget, roster],
  )

  return {
    timers,
    groups,
    config,
    update,
    collapsed,
    toggleCollapsed,
    activePlayer,
    roster,
    zealConnected: party.zealConnected,
    wsState,
  }
}
