import React, { useCallback, useEffect, useState } from 'react'
import { Plus, PencilRuler, Trash2, Save, X } from 'lucide-react'
import { getRaidRoles, saveRaidRole, deleteRaidRole } from '../../services/api'
import type { RaidRole } from '../../types/raid'

// TaxonomyEditor — user-editable raid role taxonomy. Rows are flat
// (role / sub_role / label / class codes); the checker and the encounter
// comp grid read the same store, so edits apply immediately. Deleting a role
// that an encounter comp still uses is rejected by the backend.

const inputCls =
  'w-full rounded px-2 py-1 text-sm outline-none border focus:ring-1 focus:ring-(--color-primary)'
const inputStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  borderColor: 'var(--color-border)',
  color: 'var(--color-foreground)',
}

function parseClasses(s: string): string[] {
  return s
    .split(/[\s,]+/)
    .map((c) => c.trim())
    .filter(Boolean)
}

interface Props {
  classNames: Record<string, string>
}

export default function TaxonomyEditor({ classNames }: Props): React.ReactElement {
  const [roles, setRoles] = useState<RaidRole[]>([])
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<RaidRole | null>(null)
  const [form, setForm] = useState({ role: '', sub: '', label: '', classes: '' })
  const [busy, setBusy] = useState(false)

  const load = useCallback(async (): Promise<void> => {
    try {
      const res = await getRaidRoles()
      setRoles(res.roles)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  function startEdit(r: RaidRole): void {
    setEditing(r)
    setForm({ role: r.role, sub: r.sub_role ?? '', label: r.label, classes: r.classes.join(', ') })
    setError('')
  }

  function resetForm(): void {
    setEditing(null)
    setForm({ role: '', sub: '', label: '', classes: '' })
    setError('')
  }

  async function handleSave(e: React.FormEvent): Promise<void> {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await saveRaidRole({
        role: form.role.trim(),
        sub_role: form.sub.trim() || undefined,
        label: form.label.trim(),
        classes: parseClasses(form.classes),
        position: editing?.position,
      })
      resetForm()
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  async function handleDelete(r: RaidRole): Promise<void> {
    setError('')
    try {
      await deleteRaidRole(r.role, r.sub_role)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <section
      className="rounded-lg px-4 py-3 flex flex-col gap-2"
      style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}
    >
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold flex items-center gap-1.5" style={{ color: 'var(--color-foreground)' }}>
          <PencilRuler size={14} /> Role Taxonomy
        </h2>
        <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
          {roles.length} rows · defines the comp grid + checker
        </span>
      </div>

      {error ? (
        <div className="px-3 py-2 text-sm rounded" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          {error}
        </div>
      ) : null}

      {/* add / edit form */}
      <form
        onSubmit={(e) => void handleSave(e)}
        className="grid grid-cols-[1fr_1fr_1.2fr_1fr_auto_auto] gap-2 items-end"
      >
        <label className="flex flex-col gap-0.5">
          <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>Role id</span>
          <input className={inputCls} style={inputStyle} value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value })} placeholder="puller" />
        </label>
        <label className="flex flex-col gap-0.5">
          <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>Sub-role (optional)</span>
          <input className={inputCls} style={inputStyle} value={form.sub} onChange={(e) => setForm({ ...form, sub: e.target.value })} placeholder="defensive" />
        </label>
        <label className="flex flex-col gap-0.5">
          <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>Label</span>
          <input className={inputCls} style={inputStyle} value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })} placeholder="Remove Greater Curse" />
        </label>
        <label className="flex flex-col gap-0.5">
          <span className="text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>Classes (codes)</span>
          <input className={inputCls} style={inputStyle} value={form.classes} onChange={(e) => setForm({ ...form, classes: e.target.value })} placeholder="war, sk" />
        </label>
        <button
          type="submit"
          disabled={busy}
          className="flex items-center gap-1 px-2.5 py-1.5 text-xs rounded font-medium justify-center"
          style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
        >
          {editing ? <Save size={13} /> : <Plus size={13} />} {editing ? 'Update' : 'Add'}
        </button>
        {editing ? (
          <button
            type="button"
            onClick={resetForm}
            className="flex items-center gap-1 px-2.5 py-1.5 text-xs rounded justify-center"
            style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
          >
            <X size={13} /> Cancel
          </button>
        ) : null}
      </form>

      {/* rows */}
      <div className="flex flex-col gap-1 max-h-64 overflow-auto">
        {roles.map((r) => (
          <div
            key={`${r.role}\x00${r.sub_role ?? ''}`}
            className="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded"
            style={{ backgroundColor: 'var(--color-surface-2)' }}
          >
            <div className="flex items-center gap-2 min-w-0">
              <span className="text-sm font-medium" style={{ color: 'var(--color-foreground)' }}>{r.label}</span>
              <span className="text-[11px] truncate" style={{ color: 'var(--color-muted-foreground)' }}>
                {r.role}{r.sub_role ? ` / ${r.sub_role}` : ''}
              </span>
            </div>
            <div className="flex items-center gap-1 shrink-0">
              {r.classes.map((c) => (
                <span key={c} className="text-[11px] px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface)', color: 'var(--color-muted-foreground)' }}>
                  {classNames?.[c] ?? c}
                </span>
              ))}
              <button onClick={() => startEdit(r)} className="px-1.5 py-0.5 rounded" style={{ color: 'var(--color-foreground)' }} title="Edit">
                <PencilRuler size={13} />
              </button>
              <button onClick={() => void handleDelete(r)} className="px-1.5 py-0.5 rounded" style={{ color: 'var(--color-danger)' }} title="Delete">
                <Trash2 size={13} />
              </button>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
