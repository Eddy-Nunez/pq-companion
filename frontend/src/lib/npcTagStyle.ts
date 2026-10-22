// Special-ability badge colours for the NPC overlay. Both overlay surfaces
// (dashboard panel and popout window) and the Settings editor resolve colours
// here so a user's overrides apply everywhere.
//
// Default categories:
//   Yellow = special attacks (direct combat threat to the player)
//   Red    = damage/magic immunities (NPC can't be killed normally)
//   Orange = crowd-control immunities (NPC resists player CC tactics)
//   Gray   = passive/informational

import type React from 'react'
import type { NPCOverlayTagStyle } from '../types/config'

export type TagCategory = 'attack' | 'damage_immune' | 'cc_immune' | 'other'

export const TAG_CATEGORIES: Array<{ id: TagCategory; label: string }> = [
  { id: 'attack', label: 'Special attacks' },
  { id: 'damage_immune', label: 'Damage immunities' },
  { id: 'cc_immune', label: 'Crowd-control immunities' },
  { id: 'other', label: 'Other' },
]

export const DEFAULT_CATEGORY_COLORS: Record<TagCategory, string> = {
  attack: '#ca8a04',
  damage_immune: '#dc2626',
  cc_immune: '#f97316',
  other: '#6b7280',
}

// Dangerous melee specials: Summon, Enrage, Rampage, Area Rampage, Flurry,
// Triple Attack, Dual Wield.
const ATTACK_ABILITIES = new Set([1, 2, 3, 4, 5, 6, 7])
// Damage-blocking immunities: Melee, Magic, Non-Bane Melee, Non-Magical
// Melee, Harm-from-Client, Ranged Spells.
const DAMAGE_IMMUNE_ABILITIES = new Set([19, 20, 22, 23, 26, 35])
// Crowd-control immunities: Slow, Mez, Charm, Stun, Snare, Fear, Dispel,
// Fleeing, Aggro, Taunt, Pacify, Reverse Slow, Haste, Disarm, Riposte.
const CC_IMMUNE_ABILITIES = new Set([
  12, 13, 14, 15, 16, 17, 18, 21, 24, 28, 31, 50, 51, 52, 53,
])

export function abilityCategory(code: number): TagCategory {
  if (ATTACK_ABILITIES.has(code)) return 'attack'
  if (DAMAGE_IMMUNE_ABILITIES.has(code)) return 'damage_immune'
  if (CC_IMMUNE_ABILITIES.has(code)) return 'cc_immune'
  return 'other'
}

// An empty string means "use the default" — config maps can't delete keys
// through the PUT, so resetting stores '' rather than removing the entry.
export function categoryColor(
  category: TagCategory,
  style: NPCOverlayTagStyle | undefined,
): string {
  return style?.category_colors?.[category] || DEFAULT_CATEGORY_COLORS[category]
}

export interface ResolvedTagStyle {
  color: string
  emphasized: boolean
}

// Per-tag colour beats the category colour, which beats the default.
export function resolveTagStyle(
  code: number,
  style: NPCOverlayTagStyle | undefined,
): ResolvedTagStyle {
  return {
    color:
      style?.tag_colors?.[String(code)] ||
      categoryColor(abilityCategory(code), style),
    emphasized: style?.emphasized?.includes(code) ?? false,
  }
}

// Extra badge styling for emphasized tags: a bright ring plus a glow in the
// badge's own colour so it stands out from its neighbours.
export function emphasisStyle(color: string): React.CSSProperties {
  return {
    boxShadow: `0 0 0 1px #fff, 0 0 8px 2px ${color}`,
    fontWeight: 800,
  }
}
