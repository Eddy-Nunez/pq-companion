import { useCallback, useEffect, useState } from 'react'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'
import { getConfig } from '../services/api'
import { setDetrimOverride, type DetrimOverrideMode, type DetrimOverrides } from '../lib/detrimOverrides'

/**
 * The user's per-spell Detrimental timer overrides (mute alert / hide timer),
 * kept in sync with config across windows via the config:updated event.
 * `setOverride` updates local state immediately and rolls back if the save fails.
 */
export function useDetrimSpellOverrides(): {
  overrides: DetrimOverrides
  setOverride: (spellName: string, mode: DetrimOverrideMode | null) => Promise<void>
} {
  const [overrides, setOverrides] = useState<DetrimOverrides>({})

  const load = useCallback(() => {
    getConfig()
      .then((c) => setOverrides((c.preferences?.detrim_spell_overrides ?? {}) as DetrimOverrides))
      .catch(() => {})
  }, [])

  useEffect(load, [load])

  useWebSocket(
    useCallback((msg: WsMessage) => {
      if (msg.type === WSEvent.ConfigUpdated) load()
    }, [load]),
  )

  const setOverride = useCallback(
    async (spellName: string, mode: DetrimOverrideMode | null) => {
      const previous = overrides
      const optimistic: DetrimOverrides = {}
      for (const [n, m] of Object.entries(previous)) {
        if (n.toLowerCase() !== spellName.toLowerCase()) optimistic[n] = m
      }
      if (mode) optimistic[spellName] = mode
      setOverrides(optimistic)
      try {
        setOverrides(await setDetrimOverride(spellName, mode))
      } catch {
        setOverrides(previous)
      }
    },
    [overrides],
  )

  return { overrides, setOverride }
}
