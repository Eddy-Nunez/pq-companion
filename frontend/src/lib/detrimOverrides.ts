import { getConfig, updateConfig } from '../services/api'

// Per-spell overrides for native Detrimental timers (config
// preferences.detrim_spell_overrides), keyed by spell name:
//   mute — keep the timer, skip the global "fading soon" alert for it
//   hide — never create the timer (the backend drops it at land time)
export type DetrimOverrideMode = 'mute' | 'hide'
export type DetrimOverrides = Record<string, DetrimOverrideMode>

// overrideFor looks a spell up case-insensitively, like the backend does.
export function overrideFor(
  overrides: DetrimOverrides | undefined,
  spellName: string,
): DetrimOverrideMode | undefined {
  if (!overrides) return undefined
  const want = spellName.toLowerCase()
  for (const [name, mode] of Object.entries(overrides)) {
    if (name.toLowerCase() === want) return mode
  }
  return undefined
}

// setDetrimOverride writes (or, with null, clears) one spell's override via a
// read-modify-write of the whole config, returning the new override map.
export async function setDetrimOverride(
  spellName: string,
  mode: DetrimOverrideMode | null,
): Promise<DetrimOverrides> {
  const cfg = await getConfig()
  const next: DetrimOverrides = {}
  for (const [name, m] of Object.entries(cfg.preferences?.detrim_spell_overrides ?? {})) {
    if (name.toLowerCase() !== spellName.toLowerCase()) next[name] = m as DetrimOverrideMode
  }
  if (mode) next[spellName] = mode
  const saved = await updateConfig({
    ...cfg,
    preferences: { ...cfg.preferences, detrim_spell_overrides: next },
  })
  return (saved.preferences?.detrim_spell_overrides ?? {}) as DetrimOverrides
}
