import React, { useState } from 'react'
import { Plus, X } from 'lucide-react'
import SpellSearchPicker from '../SpellSearchPicker'

type Mode = 'mute' | 'hide'

interface DetrimSpellOverridesEditorProps {
  value: Record<string, Mode> | undefined
  onChange: (next: Record<string, Mode>) => void
}

const MODE_LABEL: Record<Mode, string> = {
  mute: 'Mute alert (still show timer)',
  hide: 'Hide timer (never show)',
}

/**
 * Settings editor for preferences.detrim_spell_overrides: spells that should
 * skip the default Detrimental fading-soon alert, or not show a timer at all.
 * Rows can also be added straight from a Detrimental timer's row buttons.
 */
export default function DetrimSpellOverridesEditor({
  value,
  onChange,
}: DetrimSpellOverridesEditorProps): React.ReactElement {
  const [picking, setPicking] = useState(false)
  const entries = Object.entries(value ?? {}).sort(([a], [b]) => a.localeCompare(b))

  const set = (name: string, mode: Mode): void => onChange({ ...(value ?? {}), [name]: mode })
  const remove = (name: string): void => {
    const next = { ...(value ?? {}) }
    delete next[name]
    onChange(next)
  }

  return (
    <div className="mt-4">
      <p className="mb-1 text-sm font-medium" style={{ color: 'var(--color-foreground)' }}>
        Per-spell exceptions
      </p>
      <p className="mb-2 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Silence the alert for a spell you don't need to hear (e.g. Tashanian), or hide its
        timer entirely. You can also set these from the bell / eye buttons on a timer row in
        the Detrimental overlay.
      </p>

      {entries.length === 0 ? (
        <p className="mb-2 text-xs italic" style={{ color: 'var(--color-muted)' }}>
          No exceptions — every detrimental uses the alert above.
        </p>
      ) : (
        <div className="mb-2 flex flex-col gap-1">
          {entries.map(([name, mode]) => (
            <div
              key={name}
              className="flex items-center gap-2 rounded border px-2 py-1 text-xs"
              style={{ borderColor: 'var(--color-border)', backgroundColor: 'var(--color-surface)' }}
            >
              <span className="flex-1 truncate" style={{ color: 'var(--color-foreground)' }}>{name}</span>
              <select
                value={mode}
                onChange={(e) => set(name, e.target.value as Mode)}
                className="rounded border px-1 py-0.5 text-xs"
                style={{
                  borderColor: 'var(--color-border)',
                  backgroundColor: 'var(--color-surface-2)',
                  color: 'var(--color-foreground)',
                }}
              >
                {(Object.keys(MODE_LABEL) as Mode[]).map((m) => (
                  <option key={m} value={m}>{MODE_LABEL[m]}</option>
                ))}
              </select>
              <button
                onClick={() => remove(name)}
                title={`Remove ${name} exception`}
                style={{ color: 'var(--color-muted)' }}
              >
                <X size={13} />
              </button>
            </div>
          ))}
        </div>
      )}

      <button
        onClick={() => setPicking(true)}
        className="flex items-center gap-1 rounded border px-2 py-1 text-xs"
        style={{
          borderColor: 'var(--color-border)',
          backgroundColor: 'var(--color-surface-2)',
          color: 'var(--color-muted-foreground)',
        }}
      >
        <Plus size={11} /> Add spell
      </button>

      {picking && (
        <SpellSearchPicker
          onPick={(spell) => {
            set(spell.name, 'mute')
            setPicking(false)
          }}
          onClose={() => setPicking(false)}
        />
      )}
    </div>
  )
}
