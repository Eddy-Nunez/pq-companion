import { useCallback, useState } from 'react'
import type { NPCOverlaySurface } from './useNPCOverlaySections'

export type NPCOverlayView = 'stats' | 'loot' | 'timers'

const VIEWS: readonly NPCOverlayView[] = ['stats', 'loot', 'timers']

const storageKey = (surface: NPCOverlaySurface): string =>
  `npcOverlay:view:${surface}`

function loadView(surface: NPCOverlaySurface): NPCOverlayView {
  try {
    const v = localStorage.getItem(storageKey(surface))
    return VIEWS.includes(v as NPCOverlayView) ? (v as NPCOverlayView) : 'stats'
  } catch {
    return 'stats'
  }
}

/**
 * The NPC overlay's active tab (Stats / Loot / Timers), remembered per
 * surface across app restarts so the overlay reopens where it was left.
 */
export function useNPCOverlayView(
  surface: NPCOverlaySurface,
): [NPCOverlayView, (v: NPCOverlayView) => void] {
  const [view, setViewState] = useState<NPCOverlayView>(() => loadView(surface))
  const setView = useCallback(
    (v: NPCOverlayView) => {
      setViewState(v)
      try { localStorage.setItem(storageKey(surface), v) } catch { /* ignore */ }
    },
    [surface],
  )
  return [view, setView]
}
