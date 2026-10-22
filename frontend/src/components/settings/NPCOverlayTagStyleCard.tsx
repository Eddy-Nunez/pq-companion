import React, { useEffect, useMemo, useState } from 'react'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { loadEnums } from '../../lib/enumsCache'
import {
  TAG_CATEGORIES,
  abilityCategory,
  categoryColor,
  emphasisStyle,
  resolveTagStyle,
  type TagCategory,
} from '../../lib/npcTagStyle'
import type { EnumsCatalog } from '../../types/enums'
import type { NPCOverlayTagStyle } from '../../types/config'

interface Props {
  style: NPCOverlayTagStyle
  onChange: (style: NPCOverlayTagStyle) => void
}

interface TagEntry {
  code: number
  name: string
  hint?: string
}

// Hex for <input type="color">, which only accepts #rrggbb.
const toHex = (c: string): string => (/^#[0-9a-f]{6}$/i.test(c) ? c : '#6b7280')

function Badge({ label, color, emphasized }: { label: string; color: string; emphasized: boolean }): React.ReactElement {
  return (
    <span
      className="rounded px-1.5 py-0.5 text-[10px] font-semibold text-white"
      style={{ backgroundColor: color, whiteSpace: 'nowrap', ...(emphasized ? emphasisStyle(color) : null) }}
    >
      {label}
    </span>
  )
}

export default function NPCOverlayTagStyleCard({ style, onChange }: Props): React.ReactElement {
  const [catalog, setCatalog] = useState<EnumsCatalog | null>(null)
  const [open, setOpen] = useState<Set<TagCategory>>(new Set())

  useEffect(() => {
    loadEnums().then(setCatalog).catch(() => {})
  }, [])

  const byCategory = useMemo(() => {
    const out: Record<TagCategory, TagEntry[]> = {
      attack: [],
      damage_immune: [],
      cc_immune: [],
      other: [],
    }
    const entries = Object.entries(catalog?.special_abilities ?? {})
      .map(([code, meta]) => ({ code: Number(code), name: meta.name, hint: meta.description || undefined }))
      .sort((a, b) => a.code - b.code)
    for (const e of entries) out[abilityCategory(e.code)].push(e)
    return out
  }, [catalog])

  const emphasized = new Set(style.emphasized ?? [])

  // Colour maps store '' to reset — the config PUT can't delete map keys.
  const setCategoryColor = (cat: TagCategory, color: string): void =>
    onChange({ ...style, category_colors: { ...style.category_colors, [cat]: color } })

  const setTagColor = (code: number, color: string): void =>
    onChange({ ...style, tag_colors: { ...style.tag_colors, [String(code)]: color } })

  const setEmphasized = (code: number, on: boolean): void => {
    const next = new Set(emphasized)
    if (on) next.add(code)
    else next.delete(code)
    onChange({ ...style, emphasized: Array.from(next).sort((a, b) => a - b) })
  }

  const resetCategoryTags = (cat: TagCategory): void => {
    const tagColors = { ...style.tag_colors }
    for (const t of byCategory[cat]) {
      if (tagColors[String(t.code)]) tagColors[String(t.code)] = ''
    }
    onChange({ ...style, tag_colors: tagColors })
  }

  const resetAll = (): void => {
    const blank = (m: Record<string, string> | undefined): Record<string, string> =>
      Object.fromEntries(Object.keys(m ?? {}).map((k) => [k, '']))
    onChange({
      category_colors: blank(style.category_colors),
      tag_colors: blank(style.tag_colors),
      emphasized: [],
    })
  }

  const toggleOpen = (id: TagCategory): void =>
    setOpen((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const linkStyle: React.CSSProperties = { color: 'var(--color-primary)' }
  const colorInputClass = 'h-6 w-8 shrink-0 cursor-pointer rounded'
  const colorInputStyle: React.CSSProperties = { border: '1px solid var(--color-border)', padding: 1 }

  return (
    <section
      className="rounded-lg p-4"
      style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
    >
      <div className="mb-1 flex items-center justify-between">
        <h2 className="text-sm font-semibold uppercase tracking-wide" style={{ color: 'var(--color-muted)' }}>
          NPC Overlay Tag Colors
        </h2>
        <button onClick={resetAll} className="text-xs" style={linkStyle}>
          Reset all to defaults
        </button>
      </div>
      <p className="mb-4 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Recolor special-ability tags on both overlay surfaces. Set a whole category at once, or
        open a category to color individual tags. Emphasized tags get a glow so the ones you care
        about most — Immune to Charm, Immune to Mez — jump out.
      </p>

      <div className="flex flex-col gap-2">
        {TAG_CATEGORIES.map(({ id, label }) => {
          const tags = byCategory[id]
          const isOpen = open.has(id)
          const catColor = categoryColor(id, style)
          const catCustom = !!style.category_colors?.[id]
          const customCount = tags.filter(
            (t) => !!style.tag_colors?.[String(t.code)] || emphasized.has(t.code),
          ).length
          return (
            <div
              key={id}
              className="rounded"
              style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)' }}
            >
              <div className="flex items-center gap-2 px-3 py-2">
                <button onClick={() => toggleOpen(id)} className="flex flex-1 items-center gap-2 text-left">
                  {isOpen ? (
                    <ChevronDown size={13} style={{ color: 'var(--color-muted)' }} />
                  ) : (
                    <ChevronRight size={13} style={{ color: 'var(--color-muted)' }} />
                  )}
                  <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
                    {label}
                  </span>
                  <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                    {customCount > 0 ? `${customCount} customized` : `${tags.length} tags`}
                  </span>
                </button>
                {catCustom && (
                  <button onClick={() => setCategoryColor(id, '')} className="text-xs" style={linkStyle}>
                    Default
                  </button>
                )}
                <input
                  type="color"
                  value={toHex(catColor)}
                  onChange={(e) => setCategoryColor(id, e.target.value)}
                  title={`Color for all ${label.toLowerCase()}`}
                  className={colorInputClass}
                  style={colorInputStyle}
                />
              </div>
              {isOpen && (
                <div className="border-t px-3 py-2" style={{ borderColor: 'var(--color-border)' }}>
                  {customCount > 0 && (
                    <div className="mb-2 text-xs">
                      <button onClick={() => resetCategoryTags(id)} style={linkStyle}>
                        Clear per-tag colors
                      </button>
                    </div>
                  )}
                  <div
                    className="grid gap-x-4 gap-y-1.5"
                    style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(18rem, 1fr))' }}
                  >
                    {tags.map((t) => {
                      const resolved = resolveTagStyle(t.code, style)
                      const custom = !!style.tag_colors?.[String(t.code)]
                      return (
                        <div key={t.code} className="flex items-center gap-2" title={t.hint}>
                          <input
                            type="color"
                            value={toHex(resolved.color)}
                            onChange={(e) => setTagColor(t.code, e.target.value)}
                            className={colorInputClass}
                            style={colorInputStyle}
                          />
                          <span className="min-w-0 flex-1 truncate">
                            <Badge label={t.name} color={resolved.color} emphasized={resolved.emphasized} />
                          </span>
                          {custom && (
                            <button onClick={() => setTagColor(t.code, '')} className="text-xs" style={linkStyle}>
                              Default
                            </button>
                          )}
                          <label
                            className="flex cursor-pointer items-center gap-1 text-xs"
                            style={{ color: 'var(--color-muted-foreground)' }}
                          >
                            <input
                              type="checkbox"
                              checked={emphasized.has(t.code)}
                              onChange={(e) => setEmphasized(t.code, e.target.checked)}
                            />
                            Emphasize
                          </label>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </section>
  )
}
