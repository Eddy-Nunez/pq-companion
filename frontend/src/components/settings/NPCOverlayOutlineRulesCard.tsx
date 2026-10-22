import React, { useEffect, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, Plus, Trash2, X } from 'lucide-react'
import { listCharacters } from '../../services/api'
import { loadEnums } from '../../lib/enumsCache'
import { TAG_CATEGORIES, abilityCategory, type TagCategory } from '../../lib/npcTagStyle'
import {
  CASTER_CLASSES,
  OUTLINE_RULE_PRESETS,
  blankRule,
  normalizeRule,
  ruleFromPreset,
} from '../../lib/npcOutlineRules'
import type { EnumsCatalog } from '../../types/enums'
import type { NPCOutlineRule, NPCOutlineRuleSet } from '../../types/config'

interface Props {
  /** Global rules. */
  rules: NPCOutlineRule[]
  /** Per-character replacement lists. */
  sets: NPCOutlineRuleSet[]
  onChange: (rules: NPCOutlineRule[], sets: NPCOutlineRuleSet[]) => void
  /** Called when the "Editing" character changes, so a preview can follow. */
  onScopeChange?: (character: string) => void
}

const inputStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  color: 'var(--color-foreground)',
}

function Pill({ label, color, onRemove }: { label: string; color?: string; onRemove: () => void }): React.ReactElement {
  return (
    <span
      className="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-semibold"
      style={{
        backgroundColor: color ?? 'var(--color-surface)',
        color: color ? '#fff' : 'var(--color-foreground)',
        border: color ? undefined : '1px solid var(--color-border)',
      }}
    >
      {label}
      <button onClick={onRemove} title="Remove" className="opacity-70 hover:opacity-100">
        <X size={11} />
      </button>
    </span>
  )
}

function RuleEditor({
  rule,
  index,
  count,
  catalog,
  onChange,
  onMove,
  onDelete,
}: {
  rule: NPCOutlineRule
  index: number
  count: number
  catalog: EnumsCatalog | null
  onChange: (r: NPCOutlineRule) => void
  onMove: (delta: number) => void
  onDelete: () => void
}): React.ReactElement {
  const patch = (p: Partial<NPCOutlineRule>): void => onChange({ ...rule, ...p })
  const tagName = (c: number): string => catalog?.special_abilities[String(c)]?.name ?? `Ability ${c}`
  const clsName = (c: number): string => catalog?.npc_classes[String(c)] ?? `Class ${c}`

  const tagOptions = useMemo(() => {
    const byCat: Record<TagCategory, Array<{ code: number; name: string }>> = {
      attack: [],
      damage_immune: [],
      cc_immune: [],
      other: [],
    }
    for (const [code, meta] of Object.entries(catalog?.special_abilities ?? {})) {
      const c = Number(code)
      if (!rule.tags.includes(c)) byCat[abilityCategory(c)].push({ code: c, name: meta.name })
    }
    for (const list of Object.values(byCat)) list.sort((a, b) => a.name.localeCompare(b.name))
    return byCat
  }, [catalog, rule.tags])

  const classOptions = useMemo(
    () =>
      Object.entries(catalog?.npc_classes ?? {})
        .map(([id, name]) => ({ id: Number(id), name }))
        .filter((c) => !rule.classes.includes(c.id))
        .sort((a, b) => a.id - b.id),
    [catalog, rule.classes],
  )

  const iconBtn = 'rounded p-1 disabled:opacity-30'
  const missingCondition = rule.tags.length === 0 && rule.classes.length === 0

  return (
    <div
      className="rounded p-3"
      style={{
        backgroundColor: 'var(--color-surface-2)',
        border: '1px solid var(--color-border)',
        borderLeft: `4px solid ${rule.color}`,
        opacity: rule.enabled ? 1 : 0.6,
      }}
    >
      <div className="flex flex-wrap items-center gap-2">
        <input
          type="checkbox"
          checked={rule.enabled}
          onChange={(e) => patch({ enabled: e.target.checked })}
          title="Enabled"
        />
        <input
          type="text"
          value={rule.name}
          onChange={(e) => patch({ name: e.target.value })}
          placeholder="Label shown on the overlay"
          className="min-w-0 flex-1 rounded px-2 py-1 text-sm"
          style={inputStyle}
        />
        <input
          type="color"
          value={rule.color}
          onChange={(e) => patch({ color: e.target.value })}
          title="Outline color"
          className="h-7 w-9 cursor-pointer rounded"
          style={{ border: '1px solid var(--color-border)', padding: 1 }}
        />
        <label className="flex cursor-pointer items-center gap-1 text-xs" style={{ color: 'var(--color-foreground)' }}>
          <input type="checkbox" checked={rule.flash} onChange={(e) => patch({ flash: e.target.checked })} />
          Flash
        </label>
        <button onClick={() => onMove(-1)} disabled={index === 0} className={iconBtn} title="Higher priority" style={{ color: 'var(--color-muted)' }}>
          <ArrowUp size={14} />
        </button>
        <button onClick={() => onMove(1)} disabled={index === count - 1} className={iconBtn} title="Lower priority" style={{ color: 'var(--color-muted)' }}>
          <ArrowDown size={14} />
        </button>
        <button onClick={onDelete} className={iconBtn} title="Delete rule" style={{ color: 'var(--color-muted)' }}>
          <Trash2 size={14} />
        </button>
      </div>

      <div className="mt-2 flex flex-wrap items-center gap-1.5 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        <span>Target has</span>
        <select
          value={rule.match}
          onChange={(e) => patch({ match: e.target.value as NPCOutlineRule['match'] })}
          className="rounded px-1 py-0.5 text-xs"
          style={inputStyle}
        >
          <option value="any">any of</option>
          <option value="all">all of</option>
          <option value="none">none of</option>
        </select>
        {rule.tags.map((c) => (
          <Pill key={c} label={tagName(c)} onRemove={() => patch({ tags: rule.tags.filter((t) => t !== c) })} />
        ))}
        <select
          value=""
          onChange={(e) => {
            const c = Number(e.target.value)
            if (c) patch({ tags: [...rule.tags, c] })
          }}
          className="rounded px-1 py-0.5 text-xs"
          style={inputStyle}
        >
          <option value="">+ tag…</option>
          {TAG_CATEGORIES.map((cat) => (
            <optgroup key={cat.id} label={cat.label}>
              {tagOptions[cat.id].map((t) => (
                <option key={t.code} value={t.code}>
                  {t.name}
                </option>
              ))}
            </optgroup>
          ))}
        </select>
      </div>

      <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        <span>{rule.tags.length > 0 ? 'and class is' : 'Class is'}</span>
        {rule.classes.length === 0 && <span style={{ color: 'var(--color-muted)' }}>any</span>}
        {rule.classes.map((c) => (
          <Pill key={c} label={clsName(c)} onRemove={() => patch({ classes: rule.classes.filter((x) => x !== c) })} />
        ))}
        <select
          value=""
          onChange={(e) => {
            const c = Number(e.target.value)
            if (c) patch({ classes: [...rule.classes, c] })
          }}
          className="rounded px-1 py-0.5 text-xs"
          style={inputStyle}
        >
          <option value="">+ class…</option>
          {classOptions.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
        <button
          onClick={() => patch({ classes: Array.from(new Set([...rule.classes, ...CASTER_CLASSES])) })}
          className="text-xs"
          style={{ color: 'var(--color-primary)' }}
          title="Cleric, Druid, Shaman, Necromancer, Wizard, Magician, Enchanter"
        >
          + casters
        </button>
      </div>

      {missingCondition && (
        <p className="mt-1.5 text-xs" style={{ color: '#f59e0b' }}>
          Add at least one tag or class — this rule never matches yet.
        </p>
      )}
    </div>
  )
}

export default function NPCOverlayOutlineRulesCard({
  rules,
  sets,
  onChange,
  onScopeChange,
}: Props): React.ReactElement {
  const [catalog, setCatalog] = useState<EnumsCatalog | null>(null)
  const [characters, setCharacters] = useState<string[]>([])
  const [scope, setScope] = useState('') // '' = all characters (global list)

  useEffect(() => {
    loadEnums().then(setCatalog).catch(() => {})
    listCharacters()
      .then((r) => setCharacters(r.characters.map((c) => c.name)))
      .catch(() => {})
  }, [])

  const lc = scope.toLowerCase()
  const ownSet = scope !== '' ? sets.find((s) => s.character === lc) : undefined
  const editable = scope === '' || !!ownSet
  // The list the editor below reads and writes for the selected scope.
  const current = (scope === '' ? rules : (ownSet?.rules ?? rules)).map(normalizeRule)

  const write = (next: NPCOutlineRule[]): void => {
    if (scope === '') onChange(next, sets)
    else onChange(rules, sets.map((s) => (s.character === lc ? { ...s, rules: next } : s)))
  }

  const toggleOverride = (on: boolean): void => {
    if (on) onChange(rules, [...sets, { character: lc, rules: rules.map(normalizeRule) }])
    else onChange(rules, sets.filter((s) => s.character !== lc))
  }

  const update = (i: number, r: NPCOutlineRule): void => write(current.map((x, j) => (j === i ? r : x)))
  const move = (i: number, delta: number): void => {
    const next = [...current]
    const [r] = next.splice(i, 1)
    next.splice(i + delta, 0, r)
    write(next)
  }

  return (
    <section
      className="rounded-lg p-4"
      style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
    >
      <h2 className="mb-1 text-sm font-semibold uppercase tracking-wide" style={{ color: 'var(--color-muted)' }}>
        NPC Overlay Outline Rules
      </h2>
      <p className="mb-4 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Outline the NPC overlay in a color when the target matches — red when it can&rsquo;t be
        mezzed, flashing when it can&rsquo;t be slowed. Rules are checked top to bottom and the
        first match draws the outline; every matching rule&rsquo;s label shows on the overlay so you
        can tell why. A character can keep its own rules.
      </p>

      <div className="mb-3 flex flex-wrap items-center gap-3">
        <label className="flex items-center gap-2 text-xs" style={{ color: 'var(--color-foreground)' }}>
          Editing
          <select
            value={scope}
            onChange={(e) => {
              setScope(e.target.value)
              onScopeChange?.(e.target.value)
            }}
            className="rounded px-2 py-1 text-sm"
            style={{ ...inputStyle, backgroundColor: 'var(--color-surface-2)' }}
          >
            <option value="">All characters (default)</option>
            {characters.map((n) => (
              <option key={n} value={n}>
                {n}
                {sets.some((s) => s.character === n.toLowerCase()) ? ' — custom rules' : ''}
              </option>
            ))}
          </select>
        </label>
        {scope !== '' && (
          <label className="flex cursor-pointer items-center gap-2 text-xs" style={{ color: 'var(--color-foreground)' }}>
            <input type="checkbox" checked={!!ownSet} onChange={(e) => toggleOverride(e.target.checked)} />
            Use custom rules for {scope}
          </label>
        )}
      </div>

      {!editable ? (
        <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          {scope} uses the default rules ({rules.length}). Turn on custom rules to give them their own.
        </p>
      ) : (
        <>
          <div className="flex flex-col gap-2">
            {current.length === 0 && (
              <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
                No rules yet — the overlay is never outlined.
              </p>
            )}
            {current.map((r, i) => (
              <RuleEditor
                key={r.id}
                rule={r}
                index={i}
                count={current.length}
                catalog={catalog}
                onChange={(next) => update(i, next)}
                onMove={(d) => move(i, d)}
                onDelete={() => write(current.filter((_, j) => j !== i))}
              />
            ))}
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-2">
            <button
              onClick={() => write([...current, blankRule()])}
              className="flex items-center gap-1 rounded px-2 py-1 text-xs"
              style={{ ...inputStyle, backgroundColor: 'var(--color-surface-2)' }}
            >
              <Plus size={12} /> Add rule
            </button>
            <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
              or start from:
            </span>
            {OUTLINE_RULE_PRESETS.map((p, i) => (
              <button
                key={p.label}
                onClick={() => write([...current, ruleFromPreset(i)])}
                className="rounded px-2 py-0.5 text-xs"
                style={{ border: `1px solid ${p.rule.color}`, color: 'var(--color-foreground)' }}
              >
                {p.label}
              </button>
            ))}
          </div>
        </>
      )}
    </section>
  )
}
