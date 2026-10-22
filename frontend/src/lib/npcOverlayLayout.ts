// Section and special-ability tag ordering for the NPC overlay's Stats view.
// Both overlay surfaces and the Settings editor resolve order here. Saved
// orders are partial-tolerant: unknown ids are dropped and anything missing
// (e.g. a section added in a later release) keeps its default slot at the end,
// so an empty/absent config renders exactly the built-in layout.

export type NPCSectionId =
  | 'identity'
  | 'combat'
  | 'behavior'
  | 'resists'
  | 'attributes'
  | 'special_abilities'
  | 'spells'
  | 'faction'

export const NPC_SECTIONS: Array<{ id: NPCSectionId; label: string }> = [
  { id: 'identity', label: 'Identity' },
  { id: 'combat', label: 'Combat' },
  { id: 'behavior', label: 'Behavior' },
  { id: 'resists', label: 'Resists' },
  { id: 'attributes', label: 'Attributes' },
  { id: 'special_abilities', label: 'Special Abilities' },
  { id: 'spells', label: 'Spells & Abilities' },
  { id: 'faction', label: 'Faction' },
]

const DEFAULT_SECTION_ORDER = NPC_SECTIONS.map((s) => s.id)

export function resolveSectionOrder(saved: string[] | undefined): NPCSectionId[] {
  const known = new Set<string>(DEFAULT_SECTION_ORDER)
  const out = (saved ?? []).filter(
    (id, i, arr): id is NPCSectionId => known.has(id) && arr.indexOf(id) === i,
  )
  for (const id of DEFAULT_SECTION_ORDER) if (!out.includes(id)) out.push(id)
  return out
}

// Sorts items by their code's position in the saved tag order; codes not in
// the list keep their incoming relative order after the listed ones.
export function sortByTagOrder<T extends { code: number }>(
  items: T[],
  order: number[] | undefined,
): T[] {
  if (!order || order.length === 0) return items
  const rank = new Map(order.map((code, i) => [code, i]))
  return items
    .map((item, i) => ({ item, i, r: rank.get(item.code) ?? order.length }))
    .sort((a, b) => a.r - b.r || a.i - b.i)
    .map((x) => x.item)
}
