import React, { useCallback, useEffect, useState } from 'react'
import { PencilRuler, Plus, Trash2, Edit3, MapPin } from 'lucide-react'
import {
  getRaidTaxonomy,
  getRaidEncounters,
  createRaidEncounter,
  updateRaidEncounter,
  deleteRaidEncounter,
  searchZones,
} from '../services/api'
import type { RaidEncounter, RaidTaxonomy } from '../types/raid'
import type { Zone } from '../types/zone'
import EncounterForm from '../components/raids/EncounterForm'
import TaxonomyEditor from '../components/raids/TaxonomyEditor'
import { roleLabel, subLabel } from '../lib/raidLabels'

function asChip(s: string): React.ReactElement {
  return <span key={s} className="text-[11px] px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}>{s}</span>
}

export default function RaidEditorPage(): React.ReactElement {
  const [taxonomy, setTaxonomy] = useState<RaidTaxonomy | null>(null)
  const [encounters, setEncounters] = useState<RaidEncounter[]>([])
  const [zones, setZones] = useState<Zone[]>([])
  const [editing, setEditing] = useState<RaidEncounter | 'new' | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const load = useCallback(async (): Promise<void> => {
    try {
      const [tax, encRes, zoneRes] = await Promise.all([
        getRaidTaxonomy(),
        getRaidEncounters(),
        searchZones('', {}, 1000),
      ])
      setTaxonomy(tax)
      setEncounters(encRes.encounters)
      setZones(zoneRes.items ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  async function handleSubmit(enc: RaidEncounter): Promise<void> {
    setSaving(true)
    setError('')
    try {
      const isNew = editing === 'new' || !encounters.some((e) => e.id === enc.id)
      const saved = isNew ? await createRaidEncounter(enc) : await updateRaidEncounter(enc.id, enc)
      await load()
      setEditing(null)
      void saved
    } catch (err) {
      throw err // EncounterForm surfaces the message in its error banner
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: string): Promise<void> {
    setError('')
    try {
      await deleteRaidEncounter(id)
      setConfirmDelete(null)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setConfirmDelete(null)
    }
  }

  const active = encounters.filter((e) => e.status === 'active')
  const placeholder = encounters.filter((e) => e.status !== 'active')

  function EncounterList({ items }: { items: RaidEncounter[] }): React.ReactElement {
    if (items.length === 0) return <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>None</span>
    return (
      <div className="flex flex-col gap-1.5">
        {items.map((e) => (
          <div
            key={e.id}
            className="flex items-center justify-between gap-3 rounded-lg px-3 py-2"
            style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
          >
            <div className="flex flex-col gap-0.5 min-w-0">
              <div className="flex items-center gap-2">
                <span className="text-sm font-medium" style={{ color: 'var(--color-foreground)' }}>{e.name}</span>
                {e.id !== e.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') ? (
                  <span className="text-[11px] px-1.5 py-0.5 rounded" style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}>{e.id}</span>
                ) : null}
              </div>
              <div className="flex items-center gap-1.5 flex-wrap">
                {asChip(e.zone)}
                {asChip(`${e.comps.length} comp${e.comps.length === 1 ? '' : 's'}`)}
                {e.reqs && e.reqs.length > 0 ? asChip(`${e.reqs.length} req${e.reqs.length === 1 ? '' : 's'}`) : null}
                {e.source ? asChip(e.source) : null}
              </div>
              {e.notes ? (
                <span className="text-xs mt-0.5 line-clamp-2" style={{ color: 'var(--color-muted-foreground)' }}>{e.notes}</span>
              ) : null}
            </div>
            <div className="flex items-center gap-1 shrink-0">
              <button
                onClick={() => setEditing(e)}
                className="flex items-center gap-1 px-2 py-1 text-xs rounded"
                style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-foreground)' }}
              >
                <Edit3 size={13} /> Edit
              </button>
              {confirmDelete === e.id ? (
                <>
                  <button
                    onClick={() => void handleDelete(e.id)}
                    className="flex items-center gap-1 px-2 py-1 text-xs rounded font-medium"
                    style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}
                  >
                    Confirm
                  </button>
                  <button
                    onClick={() => setConfirmDelete(null)}
                    className="px-2 py-1 text-xs rounded"
                    style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
                  >
                    Keep
                  </button>
                </>
              ) : (
                <button
                  onClick={() => setConfirmDelete(e.id)}
                  className="flex items-center gap-1 px-2 py-1 text-xs rounded"
                  style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-danger)' }}
                >
                  <Trash2 size={13} /> Delete
                </button>
              )}
            </div>
          </div>
        ))}
      </div>
    )
  }

  if (editing !== null) {
    if (!taxonomy) {
      return <div className="px-6 py-4 text-sm" style={{ color: 'var(--color-muted-foreground)' }}>Loading…</div>
    }
    const existing = editing === 'new' ? null : editing
    return (
      <EncounterForm
        taxonomy={taxonomy}
        zones={zones}
        encounter={existing}
        onCancel={() => setEditing(null)}
        onSubmit={handleSubmit}
        saving={saving}
      />
    )
  }

  return (
    <div className="flex flex-col gap-4 px-6 py-4 overflow-auto" style={{ height: '100%' }}>
      <div className="flex items-center justify-between flex-wrap gap-3">
        <h1 className="text-lg font-semibold flex items-center gap-2" style={{ color: 'var(--color-foreground)' }}>
          <PencilRuler size={18} /> Raid Composition Editor
        </h1>
        <button
          onClick={() => setEditing('new')}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded font-medium"
          style={{ backgroundColor: 'var(--color-primary)', color: '#fff' }}
        >
          <Plus size={14} /> New Encounter
        </button>
      </div>

      {error ? (
        <div className="px-3 py-2 text-sm rounded" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          {error}
        </div>
      ) : null}

      {!taxonomy ? (
        <span className="text-sm" style={{ color: 'var(--color-muted-foreground)' }}>Loading…</span>
      ) : (
        <div className="flex flex-col gap-4">
          <section className="flex flex-col gap-1.5">
            <h2 className="text-sm font-semibold flex items-center gap-1.5" style={{ color: 'var(--color-success)' }}>
              Active · {active.length}
            </h2>
            <EncounterList items={active} />
          </section>

          {placeholder.length > 0 && (
            <section className="flex flex-col gap-1.5">
              <h2 className="text-sm font-semibold flex items-center gap-1.5" style={{ color: 'var(--color-muted-foreground)' }}>
                Placeholders · {placeholder.length}
              </h2>
              <EncounterList items={placeholder} />
            </section>
          )}

          <TaxonomyEditor classNames={taxonomy?.class_names ?? {}} />

          <section
            className="rounded-lg px-3 py-2 text-xs flex gap-2 items-start"
            style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
          >
            <MapPin size={13} className="mt-0.5 shrink-0" />
            <span>
              The role taxonomy below defines which classes cover which roles — edit it freely (the
              comp grid and the checker follow). Encounters carry their own staffing counts; a role you
              delete must first be cleared from any encounter comps using it. Zones come from the same
              game catalog the Zeal pipe reports.
            </span>
          </section>
          <span className="sr-only">{roleLabel('tank')} {subLabel('ch_cleric')}</span>
        </div>
      )}
    </div>
  )
}