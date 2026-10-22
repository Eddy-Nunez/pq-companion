// Outline rules for the NPC overlay: colour the window border when the target
// matches a rule (e.g. red when it can't be mezzed). Both overlay surfaces and
// the Settings editor share this. Matching reads static DB data (special
// abilities, class), so unlike faction it can't go stale.

import type { NPCOutlineRule, NPCOutlineRuleSet } from '../types/config'

// Pure-caster NPC classes: Cleric, Druid, Shaman, Necromancer, Wizard,
// Magician, Enchanter.
export const CASTER_CLASSES = [2, 6, 10, 11, 12, 13, 14]

export interface OutlineTarget {
  // Special-ability codes the target has (value != 0).
  abilities: Set<number>
  // NPC class id, or undefined when unknown (e.g. a player target).
  classId?: number
}

export function ruleMatches(rule: NPCOutlineRule, t: OutlineTarget): boolean {
  const tags = rule.tags ?? []
  const classes = rule.classes ?? []
  if (!rule.enabled || (tags.length === 0 && classes.length === 0)) return false
  if (tags.length > 0) {
    const has = (c: number): boolean => t.abilities.has(c)
    const ok =
      rule.match === 'all'
        ? tags.every(has)
        : rule.match === 'none'
          ? !tags.some(has)
          : tags.some(has)
    if (!ok) return false
  }
  if (classes.length > 0) {
    if (t.classId == null || !classes.includes(t.classId)) return false
  }
  return true
}

// All matching rules in priority order; the first one draws the outline.
export function matchOutlineRules(
  rules: NPCOutlineRule[],
  t: OutlineTarget,
): NPCOutlineRule[] {
  return rules.filter((r) => ruleMatches(r, t))
}

// Fills list fields a round-trip may have turned into null (Go encodes empty
// slices as null) so callers can use them directly.
export function normalizeRule(r: NPCOutlineRule): NPCOutlineRule {
  return { ...r, match: r.match || 'any', tags: r.tags ?? [], classes: r.classes ?? [] }
}

// A character's own rule list when one exists (even empty), else the global.
export function resolveOutlineRules(
  global: NPCOutlineRule[] | undefined,
  sets: NPCOutlineRuleSet[] | undefined,
  character: string,
): NPCOutlineRule[] {
  const lc = character.toLowerCase()
  const own = lc ? sets?.find((s) => s.character === lc) : undefined
  return own?.rules ?? global ?? []
}

const newId = (): string =>
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `r${Date.now().toString(36)}${Math.random().toString(36).slice(2, 8)}`

export function blankRule(): NPCOutlineRule {
  return {
    id: newId(),
    name: 'New rule',
    enabled: true,
    match: 'any',
    tags: [],
    classes: [],
    color: '#ef4444',
    flash: false,
  }
}

// Starting points for common roles; the user can edit any of them after.
export const OUTLINE_RULE_PRESETS: Array<{
  label: string
  rule: Omit<NPCOutlineRule, 'id' | 'enabled'>
}> = [
  { label: "Can't mez", rule: { name: 'NO MEZ', match: 'any', tags: [13], classes: [], color: '#ef4444', flash: false } },
  { label: "Can't charm", rule: { name: 'NO CHARM', match: 'any', tags: [14], classes: [], color: '#a855f7', flash: false } },
  { label: "Can't slow", rule: { name: 'NO SLOW', match: 'any', tags: [12, 50], classes: [], color: '#ef4444', flash: true } },
  { label: 'Rampage', rule: { name: 'RAMPAGE', match: 'any', tags: [3, 4], classes: [], color: '#f97316', flash: false } },
  { label: 'Summons', rule: { name: 'SUMMONS', match: 'any', tags: [1], classes: [], color: '#eab308', flash: false } },
  { label: 'Enrages', rule: { name: 'ENRAGE', match: 'any', tags: [2], classes: [], color: '#f97316', flash: false } },
  { label: 'Belly caster', rule: { name: 'BELLY CASTER', match: 'any', tags: [26], classes: [], color: '#06b6d4', flash: false } },
  { label: 'Snarable', rule: { name: 'SNARABLE', match: 'none', tags: [16], classes: [], color: '#22c55e', flash: false } },
  { label: 'Caster class', rule: { name: 'CASTER', match: 'any', tags: [], classes: CASTER_CLASSES, color: '#3b82f6', flash: false } },
]

export function ruleFromPreset(i: number): NPCOutlineRule {
  return { ...OUTLINE_RULE_PRESETS[i].rule, id: newId(), enabled: true }
}
