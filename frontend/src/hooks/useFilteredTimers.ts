import { useCallback, useEffect, useMemo, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { useActivePlayerName } from './useActivePlayerName'
import {
  useDisplayThresholds,
  passesThreshold,
  useHideOtherCharacterTimers,
  passesCharacterScope,
} from './useDisplayThresholds'
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
 * Ignores the global / per-trigger display thresholds unless the window's
 * own "Use display thresholds" toggle is on — this window has its own
 * "under 1 min" filter, and its point is to show the spells you picked
 * regardless of how long they have left. The "hide other characters'
 * timers" setting always applies.
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
  const thresholds = useDisplayThresholds()
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

  // `scoped` feeds the missing list, which must ignore display thresholds;
  // `timers` is what the rows and dropdowns show.
  const scoped = useMemo(
    () =>
      state === null
        ? null
        : (state.timers ?? []).filter((t) => passesCharacterScope(t, hideOtherCharacters, activePlayer)),
    [state, hideOtherCharacters, activePlayer],
  )
  const timers = useMemo(
    () =>
      scoped === null || !config.respectThresholds
        ? scoped
        : scoped.filter((t) => passesThreshold(t, thresholds)),
    [scoped, config.respectThresholds, thresholds],
  )

  const roster = useMemo(
    () => (config.showMissing ? party.names : []),
    [config.showMissing, party.names],
  )
  const groups = useMemo(
    () => applyTimerFilter(timers ?? [], config, {
        activePlayer,
        currentTarget,
        roster,
        holders: scoped ?? [],
      }),
    [timers, scoped, config, activePlayer, currentTarget, roster],
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
