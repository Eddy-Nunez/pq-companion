import { useCallback, useEffect, useMemo, useState } from 'react'
import { getConfig } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { useActivePlayerName } from './useActivePlayerName'
import { WSEvent } from '../lib/wsEvents'
import { matchOutlineRules, resolveOutlineRules } from '../lib/npcOutlineRules'
import { useNPCOverlayPreview } from '../lib/npcOverlayPreview'
import type { NPCOutlineRule, NPCOutlineRuleSet } from '../types/config'
import type { TargetState } from '../types/overlay'

/**
 * Returns the outline rules the current NPC target matches, in priority order
 * (the first draws the outline), using the active character's own rule list
 * when they have one. Re-reads rules on `config:updated`. Empty for no target,
 * player targets, and until config loads. Inside the Settings preview it
 * reads the unsaved preferences and preview character.
 */
export function useNPCOutlineMatches(target: TargetState | null): NPCOutlineRule[] {
  const preview = useNPCOverlayPreview()
  const activeCharacter = useActivePlayerName()
  const [global, setGlobal] = useState<NPCOutlineRule[]>([])
  const [sets, setSets] = useState<NPCOutlineRuleSet[]>([])

  const read = useCallback(() => {
    getConfig()
      .then((c) => {
        setGlobal(c.preferences?.npc_overlay_outline_rules ?? [])
        setSets(c.preferences?.npc_overlay_outline_rule_sets ?? [])
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

  const g = preview ? preview.prefs.npc_overlay_outline_rules : global
  const s = preview ? preview.prefs.npc_overlay_outline_rule_sets : sets
  const character = preview ? preview.character : activeCharacter
  return useMemo(() => {
    const npc = target?.has_target ? target.npc_data : undefined
    // Zeal target type 0/3 = PC / PC corpse: never outline players.
    if (!npc || target?.target_type === 0 || target?.target_type === 3) return []
    const rules = resolveOutlineRules(g, s, character)
    if (rules.length === 0) return []
    const abilities = new Set(
      (target?.special_abilities ?? []).filter((a) => a.value !== 0).map((a) => a.code),
    )
    return matchOutlineRules(rules, {
      abilities,
      classId: target?.live_class ?? npc.class,
    })
  }, [target, g, s, character])
}
