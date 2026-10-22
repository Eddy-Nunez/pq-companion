import { useCallback, useEffect, useMemo, useState } from 'react'
import { getConfig } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { useActivePlayerName } from './useActivePlayerName'
import { WSEvent } from '../lib/wsEvents'
import { resolveHiddenItems } from '../lib/npcItemFilters'
import { useNPCOverlayPreview } from '../lib/npcOverlayPreview'

/**
 * Returns the set of NPC overlay item keys hidden for the active character
 * (their per-character override if set, otherwise the global list). Re-reads on
 * `config:updated`, like useNPCOverlaySections. Empty until config loads, so
 * the overlay shows everything on first paint and on any error. Inside the
 * Settings preview it reads the unsaved preferences and preview character.
 */
export function useNPCOverlayHiddenItems(): Set<string> {
  const preview = useNPCOverlayPreview()
  const activeCharacter = useActivePlayerName()
  const [global, setGlobal] = useState<string[]>([])
  const [byChar, setByChar] = useState<Record<string, string[]>>({})

  const read = useCallback(() => {
    getConfig()
      .then((c) => {
        setGlobal(c.preferences?.npc_overlay_hidden_items ?? [])
        setByChar(c.preferences?.npc_overlay_hidden_items_by_character ?? {})
      })
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

  const g = preview ? preview.prefs.npc_overlay_hidden_items : global
  const bc = preview ? preview.prefs.npc_overlay_hidden_items_by_character : byChar
  const character = preview ? preview.character : activeCharacter
  return useMemo(
    () => new Set(resolveHiddenItems(g, bc, character)),
    [g, bc, character],
  )
}
