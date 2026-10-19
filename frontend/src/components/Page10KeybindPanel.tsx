import React, { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, Keyboard, Save, Undo2, X } from 'lucide-react'
import { updatePage10Keybinds } from '../services/api'
import type { KeyBind, MacroButton, Page10Binds } from '../types/zeal'
import { formatKeybind, keybindFromEvent } from '../lib/eqKeybind'
import { ConfirmModal } from './ConfirmModal'

// Page10KeybindPanel shows and edits Zeal's Page 10 social keybinds
// (SocialPage10Slot1-12 in eqclient.ini) under the Page 10 macro grid.
//
// Saved on its own, not with the macros: the binds live in a different file
// (eqclient.ini, not <Char>_pq.proj.ini), and one Save writing two files would
// leave the user guessing which half failed.

interface Props {
  character: string
  binds: Page10Binds
  // Page 10's macros, so each row can say which social the key fires.
  buttons: MacroButton[]
  onSaved: (b: Page10Binds) => void
}

type Field = 'primary' | 'alternate'

function cloneSocial(s: KeyBind[]): KeyBind[] {
  return s.map((k) => ({ ...k }))
}

export function Page10KeybindPanel({
  character,
  binds,
  buttons,
  onSaved,
}: Props): React.ReactElement {
  const [draft, setDraft] = useState<KeyBind[]>(() => cloneSocial(binds.social))
  const [capturing, setCapturing] = useState<{ slot: number; field: Field } | null>(null)
  const [captureError, setCaptureError] = useState<string | null>(null)
  const [confirming, setConfirming] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // A fresh read (Refresh, a save, another character) replaces the draft.
  useEffect(() => {
    setDraft(cloneSocial(binds.social))
    setCapturing(null)
    setCaptureError(null)
    setError(null)
  }, [binds])

  const dirty = draft.some((k, i) => {
    const o = binds.social[i]
    return !o || k.primary !== o.primary || k.alternate !== o.alternate
  })

  // Clashes are computed against the draft, not the file: the other binds in
  // the section come from disk, the Page 10 ones from what is on screen now.
  const clashes = useMemo(() => {
    const others = new Map<string, string[]>()
    for (const [key, names] of Object.entries(binds.bound_to ?? {})) {
      const rest = names.filter((n) => !/^SocialPage10Slot\d+(_2)?$/i.test(n))
      if (rest.length) others.set(key, rest)
    }
    const page10 = new Map<string, string[]>()
    for (const k of draft) {
      for (const [field, v] of [['primary', k.primary], ['alternate', k.alternate]] as const) {
        if (!v) continue
        const key = v.toUpperCase()
        page10.set(key, [...(page10.get(key) ?? []), `${k.slot}${field === 'alternate' ? ' (alt)' : ''}`])
      }
    }
    return (v: string, self: string): string[] => {
      if (!v) return []
      const key = v.toUpperCase()
      const sameKey = (page10.get(key) ?? []).filter((s) => s !== self).map((s) => `Button ${s}`)
      return [...sameKey, ...(others.get(key) ?? [])]
    }
  }, [binds.bound_to, draft])

  // Window-level listener while capturing, so the key lands wherever focus is.
  useEffect(() => {
    if (!capturing) return
    const onKey = (e: KeyboardEvent): void => {
      e.preventDefault()
      e.stopPropagation()
      if (e.code === 'Escape') {
        setCapturing(null)
        setCaptureError(null)
        return
      }
      if (e.code === 'Backspace' || e.code === 'Delete') {
        setValue(capturing.slot, capturing.field, '')
        return
      }
      const r = keybindFromEvent(e)
      if (!r) return
      if (!r.ok) {
        setCaptureError(r.reason)
        return
      }
      setValue(capturing.slot, capturing.field, r.value)
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [capturing])

  function setValue(slot: number, field: Field, value: string): void {
    setDraft((prev) => prev.map((k) => (k.slot === slot ? { ...k, [field]: value } : k)))
    setCapturing(null)
    setCaptureError(null)
  }

  function save(): void {
    setSaving(true)
    setError(null)
    const changed = draft.filter((k, i) => {
      const o = binds.social[i]
      return !o || k.primary !== o.primary || k.alternate !== o.alternate
    })
    updatePage10Keybinds(character, changed, binds.modified_at)
      .then(onSaved)
      .catch((err: Error) => setError(err.message))
      .finally(() => {
        setSaving(false)
        setConfirming(false)
      })
  }

  const nameOf = (slot: number): string => {
    const b = buttons.find((x) => x.page === 10 && x.button === slot)
    return b?.name || ''
  }

  const keyCell = (k: KeyBind, field: Field): React.ReactElement => {
    const v = k[field]
    const active = capturing?.slot === k.slot && capturing.field === field
    const self = `${k.slot}${field === 'alternate' ? ' (alt)' : ''}`
    const clash = clashes(v, self)
    return (
      <div className="flex items-center gap-1">
        <button
          onClick={(e) => {
            // Drop focus, or the Space/Enter being captured would also
            // "click" this button again.
            e.currentTarget.blur()
            setCaptureError(null)
            setCapturing(active ? null : { slot: k.slot, field })
          }}
          className="min-w-[72px] rounded border px-2 py-0.5 text-left font-mono text-[11px]"
          style={{
            backgroundColor: active ? 'var(--color-surface-2)' : 'var(--color-surface)',
            borderColor: active
              ? 'var(--color-primary)'
              : clash.length
                ? 'var(--color-warning, #f59e0b)'
                : 'var(--color-border)',
            color: v ? 'var(--color-foreground)' : 'var(--color-muted)',
          }}
          title={
            clash.length
              ? `Also bound to: ${clash.join(', ')}`
              : 'Click, then press the key. Backspace clears, Esc cancels.'
          }
        >
          {active ? 'Press a key…' : v ? formatKeybind(v) : '—'}
        </button>
        {v && !active && (
          <button
            onClick={() => setValue(k.slot, field, '')}
            title="Clear"
            style={{ color: 'var(--color-muted)' }}
          >
            <X size={11} />
          </button>
        )}
      </div>
    )
  }

  return (
    <div
      className="mt-4 rounded-lg border p-3"
      style={{ borderColor: 'var(--color-border)', backgroundColor: 'var(--color-surface)', maxWidth: 560 }}
    >
      <div className="mb-2 flex items-center gap-2">
        <Keyboard size={14} style={{ color: 'var(--color-primary)' }} />
        <span className="text-xs font-semibold" style={{ color: 'var(--color-foreground)' }}>
          Page 10 keybinds
        </span>
        <span className="text-[10px]" style={{ color: 'var(--color-muted)' }}>
          {binds.per_character
            ? `Per-character (Zeal) · [${binds.section}]`
            : `Shared by all characters · [${binds.section}]`}
        </span>
        <div className="ml-auto flex items-center gap-1">
          <button
            onClick={() => setDraft(cloneSocial(binds.social))}
            disabled={!dirty || saving}
            className="flex items-center gap-1 rounded px-2 py-0.5 text-[11px] disabled:opacity-40"
            style={{ color: 'var(--color-muted-foreground)', border: '1px solid var(--color-border)' }}
          >
            <Undo2 size={11} /> Revert
          </button>
          <button
            onClick={() => setConfirming(true)}
            disabled={!dirty || saving}
            className="flex items-center gap-1 rounded px-2 py-0.5 text-[11px] font-medium disabled:opacity-40"
            style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
          >
            <Save size={11} className={saving ? 'animate-pulse' : ''} /> Save keybinds
          </button>
        </div>
      </div>

      <p className="mb-2 text-[11px]" style={{ color: 'var(--color-muted)' }}>
        Zeal&rsquo;s SocialPage10 binds fire these buttons from anywhere, without the Actions
        window open. {!binds.per_character && (
          <>Zeal&rsquo;s per-character keybinds option is off, so these apply to every character.</>
        )}
      </p>

      {captureError && (
        <p className="mb-2 text-[11px]" style={{ color: 'var(--color-warning, #f59e0b)' }}>
          {captureError}
        </p>
      )}
      {error && (
        <p className="mb-2 text-[11px]" style={{ color: 'var(--color-danger, #ef4444)' }}>
          {error}
        </p>
      )}

      <div
        className="grid items-center gap-x-3 gap-y-1 text-[11px]"
        style={{ gridTemplateColumns: 'auto 1fr auto auto' }}
      >
        <span style={{ color: 'var(--color-muted)' }}>Button</span>
        <span style={{ color: 'var(--color-muted)' }}>Social</span>
        <span style={{ color: 'var(--color-muted)' }}>Key</span>
        <span style={{ color: 'var(--color-muted)' }}>Alternate</span>
        {draft.map((k) => (
          <React.Fragment key={k.slot}>
            <span className="font-mono" style={{ color: 'var(--color-muted)' }}>{k.slot}</span>
            <span
              className="truncate"
              style={{
                color: nameOf(k.slot) ? 'var(--color-foreground)' : 'var(--color-muted)',
                fontStyle: nameOf(k.slot) ? 'normal' : 'italic',
              }}
            >
              {nameOf(k.slot) || 'Empty'}
            </span>
            {keyCell(k, 'primary')}
            {keyCell(k, 'alternate')}
          </React.Fragment>
        ))}
      </div>

      {confirming && (
        <ConfirmModal
          title="Save Page 10 keybinds?"
          confirmLabel="Save"
          onConfirm={save}
          onCancel={() => setConfirming(false)}
          message={
            <div className="flex flex-col gap-2">
              <p>
                Write the changed Page 10 binds to <code>[{binds.section}]</code> in{' '}
                <code>eqclient.ini</code>? Only those lines are touched, and your EQ config
                files are backed up first.
              </p>
              <p className="flex gap-1.5" style={{ color: 'var(--color-warning, #f59e0b)' }}>
                <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                <span>
                  Save with EverQuest closed. The game writes its own keybinds back to
                  eqclient.ini when it exits, which would undo changes made while it runs.
                  {binds.per_character && ` ${character}'s binds load when ${character} enters the world.`}
                </span>
              </p>
            </div>
          }
        />
      )}
    </div>
  )
}
