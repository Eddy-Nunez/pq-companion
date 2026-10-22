// Live preview support for the NPC overlay. The Settings preview wraps the
// overlay components in this context so the display hooks read the unsaved,
// in-memory preferences (and a chosen character) instead of the saved config.
// Outside the preview the context is null and the hooks behave as normal.

import { createContext, useContext } from 'react'
import type { Preferences } from '../types/config'

export interface NPCOverlayPreview {
  prefs: Preferences
  // Character whose per-character lists apply; '' = the default lists.
  character: string
}

export const NPCOverlayPreviewContext = createContext<NPCOverlayPreview | null>(null)

export function useNPCOverlayPreview(): NPCOverlayPreview | null {
  return useContext(NPCOverlayPreviewContext)
}
