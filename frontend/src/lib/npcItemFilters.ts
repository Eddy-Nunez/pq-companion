// Item-level filters for the NPC overlay. Section toggles (Settings → NPC
// Overlay Sections) hide whole sections; these keys hide individual chips and
// special-ability badges inside them. Keys are shared by both overlay surfaces
// and the Settings editor.

export type NPCItemSection =
  | 'identity'
  | 'combat'
  | 'behavior'
  | 'resists'
  | 'attributes'

export interface NPCItemGroup {
  section: NPCItemSection
  label: string
  items: Array<{ key: string; label: string }>
}

// Everything except special abilities, which are keyed by code and listed from
// the enum catalog (see abilityKey).
export const NPC_ITEM_GROUPS: NPCItemGroup[] = [
  {
    section: 'identity',
    label: 'Identity',
    items: [
      { key: 'identity.level', label: 'Level' },
      { key: 'identity.class', label: 'Class' },
      { key: 'identity.race', label: 'Race' },
      { key: 'identity.body', label: 'Body type' },
    ],
  },
  {
    section: 'combat',
    label: 'Combat',
    items: [
      { key: 'combat.hp', label: 'HP' },
      { key: 'combat.mana', label: 'Mana' },
      { key: 'combat.ac', label: 'AC' },
      { key: 'combat.dmg', label: 'Damage' },
      { key: 'combat.atk', label: 'Attacks per round' },
      { key: 'combat.speed', label: 'Run speed' },
      { key: 'combat.pbaoe', label: 'PBAoE XP warning' },
    ],
  },
  {
    section: 'behavior',
    label: 'Behavior',
    items: [
      { key: 'behavior.delay', label: 'Attack delay' },
      { key: 'behavior.aggro', label: 'Aggro radius' },
      { key: 'behavior.assist', label: 'Assist radius' },
    ],
  },
  {
    section: 'resists',
    label: 'Resists',
    items: [
      { key: 'resists.mr', label: 'Magic' },
      { key: 'resists.cr', label: 'Cold' },
      { key: 'resists.fr', label: 'Fire' },
      { key: 'resists.dr', label: 'Disease' },
      { key: 'resists.pr', label: 'Poison' },
    ],
  },
  {
    section: 'attributes',
    label: 'Attributes',
    items: [
      { key: 'attributes.str', label: 'STR' },
      { key: 'attributes.sta', label: 'STA' },
      { key: 'attributes.dex', label: 'DEX' },
      { key: 'attributes.agi', label: 'AGI' },
      { key: 'attributes.int', label: 'INT' },
      { key: 'attributes.wis', label: 'WIS' },
      { key: 'attributes.cha', label: 'CHA' },
    ],
  },
]

export const abilityKey = (code: number): string => `ability.${code}`

// Resolves the hidden-key list for a character: their own override when one
// exists (even an empty one), otherwise the global list.
export function resolveHiddenItems(
  global: string[] | undefined,
  byCharacter: Record<string, string[]> | undefined,
  character: string,
): string[] {
  const own = character ? byCharacter?.[character.toLowerCase()] : undefined
  return own ?? global ?? []
}
