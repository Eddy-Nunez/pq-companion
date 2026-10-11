import { useCallback, useEffect, useState } from 'react'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { getOverlayNPCTarget } from '../services/api'
import { WSEvent } from '../lib/wsEvents'
import type { TargetState } from '../types/overlay'

/**
 * Name of the current target as the NPC overlay sees it (Zeal pipe target
 * first, log-inferred otherwise). '' when there's no target.
 */
export function useCurrentTargetName(): string {
  const [name, setName] = useState('')

  useEffect(() => {
    getOverlayNPCTarget()
      .then((s) => setName(s.has_target ? s.target_name ?? '' : ''))
      .catch(() => {})
  }, [])

  const handle = useCallback((msg: WsMessage) => {
    if (msg.type !== WSEvent.OverlayNPCTarget) return
    const s = msg.data as TargetState | null
    setName(s?.has_target ? s.target_name ?? '' : '')
  }, [])
  useWebSocket(handle)

  return name
}
