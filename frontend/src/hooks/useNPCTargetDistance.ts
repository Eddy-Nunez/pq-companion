import { useCallback, useEffect, useState } from 'react'
import { getConfig, getOverlayNPCDistance } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'
import type { TargetDistance } from '../types/overlay'

export interface NPCTargetDistance extends TargetDistance {
  // castRange / rangedRange are the user's configured ranges the readout is
  // coloured against (rangedRange 0 = not configured).
  castRange: number
  rangedRange: number
}

const DEFAULT_CAST_RANGE = 200

/**
 * Live player→target distance from the Zeal pipe (overlay:npc_target_distance),
 * plus the user's configured cast/ranged ranges. `has_descriptors` is false on
 * Zeal builds that don't report target info — callers hide the readout then.
 * `distance` is null when Zeal withheld the target position (250+ units away).
 */
export function useNPCTargetDistance(): NPCTargetDistance {
  const [dist, setDist] = useState<TargetDistance>({
    distance: null,
    has_descriptors: false,
  })
  const [ranges, setRanges] = useState({
    castRange: DEFAULT_CAST_RANGE,
    rangedRange: 0,
  })

  const readRanges = useCallback(() => {
    getConfig()
      .then((c) => {
        const cast = c.preferences?.npc_overlay_cast_range
        const ranged = c.preferences?.npc_overlay_ranged_range
        setRanges({
          castRange: cast && cast > 0 ? cast : DEFAULT_CAST_RANGE,
          rangedRange: ranged && ranged > 0 ? ranged : 0,
        })
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    readRanges()
    getOverlayNPCDistance()
      .then(setDist)
      .catch(() => {})
  }, [readRanges])

  const handle = useCallback(
    (msg: WsMessage) => {
      if (msg.type === WSEvent.OverlayNPCTargetDistance) {
        setDist(msg.data as TargetDistance)
      } else if (msg.type === WSEvent.ConfigUpdated) {
        readRanges()
      }
    },
    [readRanges],
  )
  useWebSocket(handle)

  return { ...dist, ...ranges }
}
