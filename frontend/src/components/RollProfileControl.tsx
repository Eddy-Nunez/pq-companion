import React, { useEffect, useState } from 'react'
import {
  Plus, Trash2, ArrowUp, ArrowDown, Check, Layers, Info, ChevronDown,
  ChevronRight, Save,
} from 'lucide-react'
import type { RollProfile, ProfileScheme, ProfileTier, SavedRollProfile } from '../types/rolls'
import type { Config } from '../types/config'
import { getConfig, saveConfigChanges } from '../services/api'
import {
  ROLL_PROFILE_PRESETS,
  presetIdFor,
  profilesEqual,
  normalizeProfile,
} from '../lib/rollProfilePresets'

// A starter shown when the user switches a simple profile to Custom so there
// is something to edit rather than an empty form.
const CUSTOM_STARTER: RollProfile = {
  mode: 'tiered',
  scheme: 'suffix',
  divisor: 100,
  tiers: [
    { match: 11, label: 'Main' },
    { match: 22, label: 'Alt' },
  ],
}

function seedDraft(profile: RollProfile): RollProfile {
  return profile.mode === 'tiered' ? normalizeProfile(profile) : { ...CUSTOM_STARTER }
}

// Saved custom profiles are addressed in the dropdown as "saved:<name>".
const SAVED_PREFIX = 'saved:'

function toProfile(s: SavedRollProfile): RollProfile {
  return normalizeProfile({ mode: s.mode, scheme: s.scheme, divisor: s.divisor, tiers: s.tiers })
}

/** selectionFor resolves the dropdown id for a live profile: a built-in
 *  preset, else a saved custom with identical tiers, else plain 'custom'. */
function selectionFor(profile: RollProfile, saved: SavedRollProfile[]): string {
  const id = presetIdFor(profile)
  if (id !== 'custom') return id
  const hit = saved.find((s) => profilesEqual(toProfile(s), profile))
  return hit ? SAVED_PREFIX + hit.name : 'custom'
}

/**
 * RollProfileControl renders the grouping-profile selector (Simple / presets /
 * Custom) plus, when Custom is selected, an inline tier editor. It owns the
 * edit draft and commits via onChange; the parent passes the live profile.
 */
export default function RollProfileControl({
  profile,
  onChange,
}: {
  profile: RollProfile
  onChange: (p: RollProfile) => void
}): React.ReactElement {
  const [config, setConfig] = useState<Config | null>(null)
  const saved = config?.preferences.roll_tracker_saved_profiles ?? []
  const [selected, setSelected] = useState(() => presetIdFor(profile))
  const [draft, setDraft] = useState<RollProfile>(() => seedDraft(profile))
  const [name, setName] = useState('')
  const [infoOpen, setInfoOpen] = useState(false)

  useEffect(() => {
    getConfig().then(setConfig).catch(() => {})
  }, [])

  // Re-sync when the live profile changes from outside (another window, a WS
  // broadcast), unless the user is mid-edit on a matching custom draft.
  useEffect(() => {
    const id = selectionFor(profile, saved)
    setSelected(id)
    if (id.startsWith(SAVED_PREFIX)) setName(id.slice(SAVED_PREFIX.length))
    if (id !== 'custom' || !profilesEqual(profile, draft)) {
      setDraft(seedDraft(profile))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [profile])

  // When the saved library loads or changes, label a live profile that
  // matches a saved entry by that entry's name. Selection only — the draft
  // is left alone so a save doesn't flash the old tiers back before the
  // new live profile echoes in.
  useEffect(() => {
    const id = selectionFor(profile, saved)
    if (id.startsWith(SAVED_PREFIX) && profilesEqual(profile, draft)) {
      setSelected(id)
      setName(id.slice(SAVED_PREFIX.length))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config])

  const handleSelect = (id: string): void => {
    setSelected(id)
    if (id === 'custom') {
      setDraft(seedDraft(profile))
      setName('')
      return
    }
    if (id.startsWith(SAVED_PREFIX)) {
      const entry = saved.find((s) => SAVED_PREFIX + s.name === id)
      if (!entry) return
      const p = toProfile(entry)
      setName(entry.name)
      setDraft(p)
      onChange(p)
      return
    }
    const preset = ROLL_PROFILE_PRESETS.find((p) => p.id === id)
    if (preset) onChange(preset.profile)
  }

  // Persist the saved-profile library. An empty list is sent as [] (not
  // dropped) so deleting the last entry actually clears it server-side.
  const writeSaved = (list: SavedRollProfile[]): void => {
    if (!config) return
    const next: Config = {
      ...config,
      preferences: { ...config.preferences, roll_tracker_saved_profiles: list },
    }
    setConfig(next)
    saveConfigChanges(next, config).then(setConfig).catch(() => {})
  }

  const savedName = selected.startsWith(SAVED_PREFIX)
    ? selected.slice(SAVED_PREFIX.length)
    : null

  // Save upserts by name; editing a saved profile and renaming it replaces
  // that entry rather than leaving the old name behind. Saving also applies.
  const handleSave = (): void => {
    const trimmed = name.trim()
    if (!trimmed || !valid) return
    const p = normalizeProfile(draft)
    const entry: SavedRollProfile = { name: trimmed, ...p }
    const rest = saved.filter((s) => s.name !== trimmed && s.name !== savedName)
    writeSaved([...rest, entry].sort((a, b) => a.name.localeCompare(b.name)))
    setSelected(SAVED_PREFIX + trimmed)
    if (!profilesEqual(p, profile)) onChange(p)
  }

  const handleDelete = (): void => {
    if (savedName === null) return
    writeSaved(saved.filter((s) => s.name !== savedName))
    setSelected('custom')
    setName('')
  }

  const editing = selected === 'custom' || savedName !== null
  const selectedPreset = ROLL_PROFILE_PRESETS.find((p) => p.id === selected)
  const tiers = draft.tiers ?? []

  const patchTier = (idx: number, patch: Partial<ProfileTier>): void => {
    setDraft((d) => ({
      ...d,
      tiers: (d.tiers ?? []).map((t, i) => (i === idx ? { ...t, ...patch } : t)),
    }))
  }
  const addTier = (): void => {
    setDraft((d) => ({ ...d, tiers: [...(d.tiers ?? []), { match: 0, label: '' }] }))
  }
  const removeTier = (idx: number): void => {
    setDraft((d) => ({ ...d, tiers: (d.tiers ?? []).filter((_, i) => i !== idx) }))
  }
  const moveTier = (idx: number, dir: -1 | 1): void => {
    setDraft((d) => {
      const next = [...(d.tiers ?? [])]
      const j = idx + dir
      if (j < 0 || j >= next.length) return d
      ;[next[idx], next[j]] = [next[j], next[idx]]
      return { ...d, tiers: next }
    })
  }

  // Client-side guard mirroring the backend Validate(): non-empty distinct
  // labels, at least one tier, distinct match values.
  const matches = tiers.map((t) => t.match)
  const valid =
    tiers.length > 0 &&
    tiers.every((t) => t.label.trim() !== '') &&
    new Set(matches).size === matches.length
  const dirty = !profilesEqual(draft, profile)

  const matchHint =
    draft.scheme === 'suffix'
      ? 'Match = the last digits (max % divisor): 11, 22, 33…'
      : 'Match = the exact /random number: 111, 222…'

  return (
    <div className="flex flex-col gap-2">
      <label className="flex items-center gap-2 text-xs" style={{ color: 'var(--color-muted)' }}>
        <Layers size={13} style={{ color: 'var(--color-primary)' }} />
        <span>Grouping</span>
        <select
          value={selected}
          onChange={(e) => handleSelect(e.target.value)}
          className="rounded px-2 py-1 text-xs"
          style={{
            border: '1px solid var(--color-border)',
            backgroundColor: 'var(--color-surface)',
            color: 'var(--color-foreground)',
          }}
        >
          {ROLL_PROFILE_PRESETS.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
          {saved.length > 0 && (
            <optgroup label="Saved">
              {saved.map((s) => (
                <option key={s.name} value={SAVED_PREFIX + s.name}>
                  {s.name}
                </option>
              ))}
            </optgroup>
          )}
          <option value="custom">Custom…</option>
        </select>
        {!editing && selectedPreset && (
          <button
            onClick={() => setInfoOpen((v) => !v)}
            className="flex items-center gap-1 rounded px-1.5 py-0.5 transition-colors hover:bg-(--color-surface-3)"
            style={{ color: 'var(--color-muted)' }}
            title="How this grouping works"
          >
            <Info size={12} />
            {infoOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
            <span>How it works</span>
          </button>
        )}
      </label>

      {!editing && selectedPreset && infoOpen && (
        <div
          className="flex flex-col gap-2 rounded-lg border p-3 text-xs"
          style={{ borderColor: 'var(--color-border)', backgroundColor: 'var(--color-surface)' }}
        >
          <p style={{ color: 'var(--color-foreground)' }}>{selectedPreset.how}</p>
          <div className="flex flex-col gap-1">
            <span
              className="text-[10px] font-semibold uppercase tracking-wider"
              style={{ color: 'var(--color-muted)' }}
            >
              Examples
            </span>
            {selectedPreset.examples.map((ex, i) => (
              <div key={i} className="flex gap-1.5" style={{ color: 'var(--color-muted)' }}>
                <span aria-hidden>•</span>
                <span>{ex}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {editing && (
        <div
          className="flex flex-col gap-2 rounded-lg border p-3"
          style={{ borderColor: 'var(--color-border)', backgroundColor: 'var(--color-surface)' }}
        >
          {/* Scheme + divisor */}
          <div className="flex flex-wrap items-center gap-3 text-xs">
            <div
              className="flex items-center overflow-hidden rounded"
              style={{ border: '1px solid var(--color-border)' }}
            >
              {(['suffix', 'exact'] as ProfileScheme[]).map((s) => (
                <button
                  key={s}
                  onClick={() => setDraft((d) => ({ ...d, scheme: s }))}
                  className="px-2.5 py-1 transition-colors"
                  style={{
                    backgroundColor: draft.scheme === s ? 'var(--color-primary)' : 'transparent',
                    color: draft.scheme === s ? 'white' : 'var(--color-foreground)',
                  }}
                  title={
                    s === 'suffix'
                      ? 'Tier from the last digits; leading digit groups items (1xx / 2xx)'
                      : 'Tier from the exact number; one item at a time'
                  }
                >
                  {s === 'suffix' ? 'Suffix (1xx)' : 'Exact'}
                </button>
              ))}
            </div>
            {draft.scheme === 'suffix' && (
              <label className="flex items-center gap-1" style={{ color: 'var(--color-muted)' }}>
                <span>Item divisor</span>
                <input
                  type="number"
                  min={1}
                  value={draft.divisor ?? 100}
                  onChange={(e) =>
                    setDraft((d) => ({ ...d, divisor: parseInt(e.target.value, 10) || 100 }))
                  }
                  className="w-16 rounded bg-transparent px-1 py-0.5 text-right tabular-nums font-mono outline-none"
                  style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
                  title="max / divisor = which item; max % divisor = which tier"
                />
              </label>
            )}
          </div>

          {/* Tier rows, best → worst */}
          <div className="flex flex-col gap-1.5">
            {tiers.map((t, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <span
                  className="w-5 text-center text-[11px] tabular-nums"
                  style={{ color: 'var(--color-muted)' }}
                  title="Priority — top wins"
                >
                  {idx + 1}
                </span>
                <input
                  value={t.label}
                  onChange={(e) => patchTier(idx, { label: e.target.value })}
                  placeholder="Tier name"
                  className="flex-1 rounded bg-transparent px-2 py-1 text-xs outline-none"
                  style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
                />
                <input
                  type="number"
                  value={t.match}
                  onChange={(e) => patchTier(idx, { match: parseInt(e.target.value, 10) || 0 })}
                  className="w-20 rounded bg-transparent px-2 py-1 text-right text-xs tabular-nums font-mono outline-none"
                  style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
                  title={matchHint}
                />
                <button
                  onClick={() => moveTier(idx, -1)}
                  disabled={idx === 0}
                  className="rounded p-1 transition-colors hover:bg-(--color-surface-3) disabled:opacity-30"
                  style={{ color: 'var(--color-muted)' }}
                  title="Move up (higher priority)"
                >
                  <ArrowUp size={12} />
                </button>
                <button
                  onClick={() => moveTier(idx, 1)}
                  disabled={idx === tiers.length - 1}
                  className="rounded p-1 transition-colors hover:bg-(--color-surface-3) disabled:opacity-30"
                  style={{ color: 'var(--color-muted)' }}
                  title="Move down (lower priority)"
                >
                  <ArrowDown size={12} />
                </button>
                <button
                  onClick={() => removeTier(idx)}
                  className="rounded p-1 transition-colors hover:bg-(--color-surface-3)"
                  style={{ color: 'var(--color-muted)' }}
                  title="Remove tier"
                >
                  <Trash2 size={12} />
                </button>
              </div>
            ))}
          </div>

          <div className="flex items-center justify-between gap-2">
            <button
              onClick={addTier}
              className="flex items-center gap-1 rounded px-2 py-1 text-xs transition-colors hover:bg-(--color-surface-3)"
              style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
            >
              <Plus size={12} /> Add tier
            </button>
            <div className="flex items-center gap-2">
              <span className="text-[11px]" style={{ color: 'var(--color-muted)' }}>
                {matchHint}
              </span>
              <button
                onClick={() => valid && onChange(normalizeProfile(draft))}
                disabled={!valid || !dirty}
                className="flex items-center gap-1 rounded px-2.5 py-1 text-xs transition-colors disabled:opacity-30"
                style={{ backgroundColor: 'var(--color-primary)', color: 'white' }}
                title={valid ? 'Apply this grouping' : 'Every tier needs a unique number and a name'}
              >
                <Check size={12} /> Apply
              </button>
            </div>
          </div>

          {/* Save to the library so the setup survives switching presets */}
          <div
            className="flex items-center gap-2 border-t pt-2"
            style={{ borderColor: 'var(--color-border)' }}
          >
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSave()}
              placeholder="Name this grouping to save it"
              maxLength={40}
              className="flex-1 rounded bg-transparent px-2 py-1 text-xs outline-none"
              style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
            />
            <button
              onClick={handleSave}
              disabled={!valid || !name.trim() || !config}
              className="flex items-center gap-1 rounded px-2.5 py-1 text-xs transition-colors hover:bg-(--color-surface-3) disabled:opacity-30"
              style={{ border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
              title="Save (and apply) this grouping so it stays in the Grouping list"
            >
              <Save size={12} /> {savedName !== null ? 'Save' : 'Save as'}
            </button>
            {savedName !== null && (
              <button
                onClick={handleDelete}
                className="rounded p-1 transition-colors hover:bg-(--color-surface-3)"
                style={{ color: 'var(--color-muted)' }}
                title={`Delete saved grouping "${savedName}"`}
              >
                <Trash2 size={12} />
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
