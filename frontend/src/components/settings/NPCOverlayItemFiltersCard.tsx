import React, { useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { listCharacters } from '../../services/api'
import { loadEnums } from '../../lib/enumsCache'
import { NPC_ITEM_GROUPS, abilityKey } from '../../lib/npcItemFilters'
import type { EnumsCatalog } from '../../types/enums'

interface Props {
  /** Global hidden-item keys. */
  hidden: string[]
  /** Per-character overrides, keyed by lowercased name. */
  byCharacter: Record<string, string[]>
  onChange: (hidden: string[], byCharacter: Record<string, string[]>) => void
}

interface FilterGroup {
  id: string
  label: string
  items: Array<{ key: string; label: string; hint?: string }>
}

// Special abilities, bucketed so the long list is scannable: damaging specials
// (codes 1–7), "Immune to …" entries, and everything else.
function abilityGroups(catalog: EnumsCatalog | null): FilterGroup[] {
  const attacks: FilterGroup['items'] = []
  const immunities: FilterGroup['items'] = []
  const other: FilterGroup['items'] = []
  const entries = Object.entries(catalog?.special_abilities ?? {})
    .map(([code, meta]) => ({ code: Number(code), meta }))
    .sort((a, b) => a.code - b.code)
  for (const { code, meta } of entries) {
    const item = { key: abilityKey(code), label: meta.name, hint: meta.description || undefined }
    if (code >= 1 && code <= 7) attacks.push(item)
    else if (/^immune/i.test(meta.name)) immunities.push(item)
    else other.push(item)
  }
  return [
    { id: 'ability-attacks', label: 'Special abilities — attacks', items: attacks },
    { id: 'ability-immunities', label: 'Special abilities — immunities', items: immunities },
    { id: 'ability-other', label: 'Special abilities — other', items: other },
  ].filter((g) => g.items.length > 0)
}

export default function NPCOverlayItemFiltersCard({
  hidden,
  byCharacter,
  onChange,
}: Props): React.ReactElement {
  const [catalog, setCatalog] = useState<EnumsCatalog | null>(null)
  const [characters, setCharacters] = useState<string[]>([])
  const [scope, setScope] = useState('') // '' = all characters (global list)
  const [open, setOpen] = useState<Set<string>>(new Set())

  useEffect(() => {
    loadEnums().then(setCatalog).catch(() => {})
    listCharacters()
      .then((r) => setCharacters(r.characters.map((c) => c.name)))
      .catch(() => {})
  }, [])

  const groups = useMemo<FilterGroup[]>(
    () => [
      ...NPC_ITEM_GROUPS.map((g) => ({ id: g.section, label: g.label, items: g.items })),
      ...abilityGroups(catalog),
    ],
    [catalog],
  )

  const lc = scope.toLowerCase()
  const hasOverride = scope !== '' && byCharacter[lc] !== undefined
  // The list the editor below reads and writes for the selected scope.
  const current = scope === '' ? hidden : (byCharacter[lc] ?? hidden)
  const hiddenSet = new Set(current)
  const editable = scope === '' || hasOverride

  const write = (next: string[]): void => {
    if (scope === '') onChange(next, byCharacter)
    else onChange(hidden, { ...byCharacter, [lc]: next })
  }

  const setKeys = (keys: string[], show: boolean): void => {
    const next = new Set(current)
    for (const k of keys) {
      if (show) next.delete(k)
      else next.add(k)
    }
    write(Array.from(next))
  }

  const toggleOverride = (on: boolean): void => {
    if (on) {
      onChange(hidden, { ...byCharacter, [lc]: [...hidden] })
    } else {
      const rest = { ...byCharacter }
      delete rest[lc]
      onChange(hidden, rest)
    }
  }

  const toggleOpen = (id: string): void =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const linkStyle: React.CSSProperties = { color: 'var(--color-primary)' }

  return (
    <section
      className="rounded-lg p-4"
      style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
    >
      <h2
        className="mb-1 text-sm font-semibold uppercase tracking-wide"
        style={{ color: 'var(--color-muted)' }}
      >
        NPC Overlay Items
      </h2>
      <p className="mb-4 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Hide individual stats and special abilities you never care about, on both overlay
        surfaces. Unchecked items are hidden. A character can keep its own list — handy when a
        shaman and an enchanter care about different immunities.
      </p>

      <div className="mb-3 flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2 text-xs" style={{ color: 'var(--color-foreground)' }}>
          Editing
          <select
            value={scope}
            onChange={(e) => setScope(e.target.value)}
            className="rounded px-2 py-1 text-sm"
            style={{
              backgroundColor: 'var(--color-surface-2)',
              border: '1px solid var(--color-border)',
              color: 'var(--color-foreground)',
            }}
          >
            <option value="">All characters (default)</option>
            {characters.map((n) => (
              <option key={n} value={n}>
                {n}
                {byCharacter[n.toLowerCase()] !== undefined ? ' — custom list' : ''}
              </option>
            ))}
          </select>
        </label>
        {scope !== '' && (
          <label
            className="flex cursor-pointer items-center gap-2 text-xs"
            style={{ color: 'var(--color-foreground)' }}
          >
            <input
              type="checkbox"
              checked={hasOverride}
              onChange={(e) => toggleOverride(e.target.checked)}
            />
            Use a custom list for {scope}
          </label>
        )}
      </div>

      {!editable ? (
        <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          {scope} uses the default list ({hidden.length} hidden). Turn on a custom list to give
          them their own.
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          {groups.map((g) => {
            const isOpen = open.has(g.id)
            const keys = g.items.map((i) => i.key)
            const hiddenCount = keys.filter((k) => hiddenSet.has(k)).length
            return (
              <div
                key={g.id}
                className="rounded"
                style={{
                  backgroundColor: 'var(--color-surface-2)',
                  border: '1px solid var(--color-border)',
                }}
              >
                <button
                  onClick={() => toggleOpen(g.id)}
                  className="flex w-full items-center gap-2 px-3 py-2 text-left"
                >
                  {isOpen ? (
                    <ChevronDown size={13} style={{ color: 'var(--color-muted)' }} />
                  ) : (
                    <ChevronRight size={13} style={{ color: 'var(--color-muted)' }} />
                  )}
                  <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
                    {g.label}
                  </span>
                  <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                    {hiddenCount > 0 ? `${hiddenCount} hidden` : 'all shown'}
                  </span>
                </button>
                {isOpen && (
                  <div className="border-t px-3 py-2" style={{ borderColor: 'var(--color-border)' }}>
                    <div className="mb-2 flex gap-3 text-xs">
                      <button onClick={() => setKeys(keys, true)} style={linkStyle}>
                        Show all
                      </button>
                      <button onClick={() => setKeys(keys, false)} style={linkStyle}>
                        Hide all
                      </button>
                    </div>
                    <div className="grid gap-x-4 gap-y-1.5" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(14rem, 1fr))' }}>
                      {g.items.map((item) => (
                        <label
                          key={item.key}
                          className="flex cursor-pointer items-center gap-2"
                          title={item.hint}
                        >
                          <input
                            type="checkbox"
                            checked={!hiddenSet.has(item.key)}
                            onChange={(e) => setKeys([item.key], e.target.checked)}
                          />
                          <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
                            {item.label}
                          </span>
                        </label>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
