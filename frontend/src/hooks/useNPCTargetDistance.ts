import { useCallback, useEffect, useState } from 'react'
import { getConfig, getOverlayNPCDistance } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'
import type { TargetDistance } from '../types/overlay'
import { useCharacterRanges } from './useCharacterRanges'

export interface NPCTargetDistance extends TargetDistance {
  // castRange / rangedRange are the ranges the readout is coloured against
  // (rangedRange 0 = none): worked out from the active character's gear, or
  // the typed values when the user set them manually.
  castRange: number
  rangedRange: number
}

const DEFAULT_CAST_RANGE = 200

/**
 * Live player→target distance from the Zeal pipe (overlay:npc_target_distance),
 * plus the cast/ranged ranges to colour it against. `has_descriptors` is false on
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
    manual: false,
  })
  const auto = useCharacterRanges()

  const readRanges = useCallback(() => {
    getConfig()
      .then((c) => {
        const cast = c.preferences?.npc_overlay_cast_range
        const ranged = c.preferences?.npc_overlay_ranged_range
        setRanges({
          castRange: cast && cast > 0 ? cast : DEFAULT_CAST_RANGE,
          rangedRange: ranged && ranged > 0 ? ranged : 0,
          manual: Boolean(c.preferences?.npc_overlay_ranges_manual),
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

  // Automatic unless set manually; the typed values are the fallback when the
  // character has no export to work from.
  if (!ranges.manual && auto?.has_export) {
    return { ...dist, castRange: auto.cast_range, rangedRange: auto.ranged_range }
  }
  return { ...dist, castRange: ranges.castRange, rangedRange: ranges.rangedRange }
}
