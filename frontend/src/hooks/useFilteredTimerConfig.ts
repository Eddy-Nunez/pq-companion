import { useCallback, useEffect, useState } from 'react'
import {
  DEFAULT_FILTERED_TIMER_CONFIG,
  type FilteredTimerConfig,
} from '../lib/timerFilter'

/**
 * Filter settings for the Filtered Timers overlay, persisted per viewer in
 * localStorage. The dashboard panel and the popout window are separate
 * webContents sharing one origin, so a `storage` listener keeps both in sync
 * — same pattern as useBuffSortMode.
 */

const STORAGE_KEY = 'pq-filtered-timer-config'
const COLLAPSED_KEY = 'pq-filtered-timer-filters-collapsed'

function readStored(): FilteredTimerConfig {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT_FILTERED_TIMER_CONFIG
    const v = JSON.parse(raw) as Partial<FilteredTimerConfig>
    return {
      type: v.type === 'detrimental' ? 'detrimental' : 'beneficial',
      viewBy: v.viewBy === 'target' ? 'target' : 'spell',
      target: typeof v.target === 'string' ? v.target : '',
      onlySelected: v.onlySelected === true,
      selectedSpells: Array.isArray(v.selectedSpells)
        ? v.selectedSpells.filter((s): s is string => typeof s === 'string')
        : [],
      underOneMin: v.underOneMin === true,
      showMissing: v.showMissing === true,
      respectThresholds: v.respectThresholds === true,
    }
  } catch {
    return DEFAULT_FILTERED_TIMER_CONFIG
  }
}

function readCollapsed(): boolean {
  try {
    return localStorage.getItem(COLLAPSED_KEY) === '1'
  } catch {
    return false
  }
}

export function useFilteredTimerConfig(): {
  config: FilteredTimerConfig
  update: (patch: Partial<FilteredTimerConfig>) => void
  collapsed: boolean
  toggleCollapsed: () => void
} {
  const [config, setConfig] = useState<FilteredTimerConfig>(() => readStored())
  const [collapsed, setCollapsed] = useState<boolean>(() => readCollapsed())

  useEffect(() => {
    const onStorage = (e: StorageEvent): void => {
      if (e.key === STORAGE_KEY) setConfig(readStored())
      else if (e.key === COLLAPSED_KEY) setCollapsed(readCollapsed())
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const update = useCallback((patch: Partial<FilteredTimerConfig>) => {
    setConfig((prev) => {
      const next = { ...prev, ...patch }
      try { localStorage.setItem(STORAGE_KEY, JSON.stringify(next)) } catch { /* noop */ }
      return next
    })
  }, [])

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev
      try { localStorage.setItem(COLLAPSED_KEY, next ? '1' : '0') } catch { /* noop */ }
      return next
    })
  }, [])

  return { config, update, collapsed, toggleCollapsed }
}
