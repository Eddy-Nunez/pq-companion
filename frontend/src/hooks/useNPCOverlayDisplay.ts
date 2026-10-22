import { useCallback, useEffect, useMemo, useState } from 'react'
import { getConfig } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'
import { resolveSectionOrder, type NPCSectionId } from '../lib/npcOverlayLayout'
import type { NPCOverlayTagStyle } from '../types/config'

export interface NPCOverlayDisplay {
  // Special-ability badge colour overrides (undefined = built-in colours).
  tagStyle: NPCOverlayTagStyle | undefined
  // Stats-view section order, always complete.
  sectionOrder: NPCSectionId[]
  // Special-ability codes to show first, in order.
  tagOrder: number[] | undefined
}

/**
 * Returns the user's NPC overlay display customisations — tag colours and
 * section/tag order — shared by both overlay surfaces. Re-reads on
 * `config:updated`, like useNPCOverlayHiddenItems. Built-in defaults until
 * config loads and on any error.
 */
export function useNPCOverlayDisplay(): NPCOverlayDisplay {
  const [tagStyle, setTagStyle] = useState<NPCOverlayTagStyle | undefined>()
  const [savedSectionOrder, setSavedSectionOrder] = useState<string[] | undefined>()
  const [tagOrder, setTagOrder] = useState<number[] | undefined>()

  const read = useCallback(() => {
    getConfig()
      .then((c) => {
        setTagStyle(c.preferences?.npc_overlay_tag_style)
        setSavedSectionOrder(c.preferences?.npc_overlay_section_order)
        setTagOrder(c.preferences?.npc_overlay_tag_order)
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

  const sectionOrder = useMemo(
    () => resolveSectionOrder(savedSectionOrder),
    [savedSectionOrder],
  )
  return { tagStyle, sectionOrder, tagOrder }
}
