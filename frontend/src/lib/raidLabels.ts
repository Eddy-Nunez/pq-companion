// Human-readable labels for raid taxonomy keys. The internal ids (role,
// sub-role) stay stable — these map them to display text. The full taxonomy
// refactor will move labels into the user DB (editable); this map mirrors the
// seeded defaults for now.

const ROLE_LABELS: Record<string, string> = {
  tank: 'Tank',
  healer: 'Healer',
  rgc: 'Remove Greater Curse',
  lockpicker: 'Lockpicker',
  traps: 'Traps',
  tracker: 'Tracker',
  coth: 'Call of the Hero',
  slower: 'Slower',
  puller: 'Puller',
  debuffer: 'Debuffer',
  mind_wrack: 'Mind Wrack',
  damage: 'Damage',
}

const SUB_LABELS: Record<string, string> = {
  defensive: 'Defensive',
  snap: 'Snap Aggro',
  ch_cleric: 'CH Cleric',
  ch_druid: 'CH Druid',
  slows: 'Slows',
  cripple: 'Cripple',
  mr: 'MR',
  dr: 'Disease Resist',
  pr: 'Poison Resist',
  fr: 'Fire Resist',
  cr: 'Cold Resist',
}

export function titleCase(s: string): string {
  if (!s) return s
  return (
    s
      .split('_')
      .map((tok) => (tok.length <= 2 ? tok.toUpperCase() : tok.charAt(0).toUpperCase() + tok.slice(1)))
      .join(' ')
  )
}

export function roleLabel(role: string): string {
  return ROLE_LABELS[role] ?? titleCase(role)
}

export function subLabel(sub: string): string {
  return SUB_LABELS[sub] ?? titleCase(sub)
}
