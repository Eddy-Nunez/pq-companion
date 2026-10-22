import { useCallback, useEffect, useState } from 'react'
import { getConfig } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'
import type { NPCOverlayTagStyle } from '../types/config'

/**
 * Returns the user's special-ability badge colour overrides for the NPC
 * overlay. Re-reads on `config:updated`, like useNPCOverlayHiddenItems.
 * Undefined until config loads (and on error), which resolves to the
 * built-in category colours.
 */
export function useNPCOverlayTagStyle(): NPCOverlayTagStyle | undefined {
  const [style, setStyle] = useState<NPCOverlayTagStyle | undefined>()

  const read = useCallback(() => {
    getConfig()
      .then((c) => setStyle(c.preferences?.npc_overlay_tag_style))
      .catch(() => {})
  }, [])

  useEffect(() => {
    read()
  }, [read])

  const handle = useCallback(
    (msg: WsMessage) => {
      if (msg.type !== WSEvent.ConfigUpdated) return
      read()
    },
    [read],
  )
  useWebSocket(handle)

  return style
}
