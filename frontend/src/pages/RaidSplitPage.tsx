import React, { useMemo, useState } from 'react'
import { GitFork, Play, RefreshCw, Plus, Trash2, Lock } from 'lucide-react'
import { useRaidReadiness } from '../hooks/useRaidReadiness'
import { getRaidSplitPlan, getRaidTaxonomy, splitRaidComp } from '../services/api'
import type { SplitPlanReport, SplitPreference, SplitReport, SplitWildcard } from '../types/raid'
import GroupProposal from '../components/raids/GroupProposal'
import RosterStatusBanner from '../components/raids/RosterStatusBanner'

const PREFERENCES: { value: SplitPreference; label: string; hint: string }[] = [
  { value: 'trinity', label: 'Trinity', hint: 'Balanced groups: tank + healer + CC/debuff in every group, damage fills the rest.' },
  { value: 'focused', label: 'Focused classes', hint: 'Same-class members cluster into shared groups (e.g. clerics together for a CH chain).' },
  { value: 'curated', label: 'Curated', hint: 'Trinity seating plus your pin/cap rules (wildcards) below.' },
]

const selectCls =
  'rounded px-2 py-1.5 text-sm outline-none border focus:ring-1 focus:ring-(--color-primary)'
const selectStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  borderColor: 'var(--color-border)',
  color: 'var(--color-foreground)',
}

// WildcardRow is one curated-rule editor row. The shape intentionally maps
// 1:1 to the backend's Wildcard: pick a kind, then the kind's value, an
// optional group pin, and optional min/max bounds.
function WildcardRow({
  wc, onChange, onRemove,
}: {
  wc: SplitWildcard
  onChange: (patch: Partial<SplitWildcard>) => void
  onRemove: () => void
}): React.ReactElement {
  return (
    <div className="grid grid-cols-[auto_1fr_auto_auto_auto] gap-2 items-center">
      <select
        className={selectCls}
        style={selectStyle}
        value={wc.kind}
        onChange={(e) => onChange({ kind: e.target.value as SplitWildcard['kind'] })}
      >
        <option value="member">member</option>
        <option value="class">class</option>
        <option value="path">role</option>
        <option value="any">any</option>
      </select>
      {wc.kind === 'member' ? (
        <input
          className="rounded px-2 py-1 text-sm outline-none border"
          style={selectStyle}
          placeholder="exact roster name"
          value={wc.member ?? ''}
          onChange={(e) => onChange({ member: e.target.value })}
        />
      ) : wc.kind === 'any' ? (
        <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>any placement</span>
      ) : (
        <input
          className="rounded px-2 py-1 text-sm outline-none border"
          style={selectStyle}
          placeholder={wc.kind === 'class' ? 'class code (clr, sk…)' : 'role path (healer.ch_cleric)'}
          value={wc.value ?? ''}
          onChange={(e) => onChange({ value: e.target.value })}
        />
      )}
      <input
        className="rounded px-2 py-1 text-sm outline-none border w-16"
        style={selectStyle}
        type="number"
        min={0}
        max={12}
        placeholder="grp"
        title="Pin to group number (0 = any)"
        value={wc.group ?? 0}
        onChange={(e) => onChange({ group: Number(e.target.value) || 0 })}
      />
      <input
        className="rounded px-2 py-1 text-sm outline-none border w-20"
        style={selectStyle}
        type="number"
        min={0}
        placeholder="max"
        title="Cap: at most this many matching placements (0 = unlimited)"
        value={wc.max ?? 0}
        onChange={(e) => onChange({ max: Number(e.target.value) || 0 })}
      />
      <input
        className="rounded px-2 py-1 text-sm outline-none border w-20"
        style={selectStyle}
        type="number"
        min={0}
        placeholder="min"
        title="At least this many matching placements (advisory)"
        value={wc.min ?? 0}
        onChange={(e) => onChange({ min: Number(e.target.value) || 0 })}
      />
      <label className="flex items-center gap-1 text-xs" style={{ color: 'var(--color-muted-foreground)' }} title="Seat this member before anything else moves">
        <input type="checkbox" checked={wc.locked ?? false} onChange={(e) => onChange({ locked: e.target.checked })} />
        <Lock size={12} />
      </label>
      <button onClick={onRemove} className="px-1.5 rounded" style={{ color: 'var(--color-danger)' }} title="Remove rule">
        <Trash2 size={14} />
      </button>
    </div>
  )
}

export default function RaidSplitPage(): React.ReactElement {
  const { orderedEncounters, roster, selectedId, pickEncounter, refreshing, error, refresh } = useRaidReadiness()
  const [preference, setPreference] = useState<SplitPreference>('trinity')
  const [groupSize, setGroupSize] = useState(6)
  const [respectGroups, setRespectGroups] = useState(true)
  const [wildcards, setWildcards] = useState<SplitWildcard[]>([])
  const [report, setReport] = useState<SplitReport | null>(null)
  // adjusted = the user hand-edited the proposal via drag-and-drop since the
  // last generate; a regenerate replaces the whole report and resets it.
  const [adjusted, setAdjusted] = useState(false)
  // Class display names from the live taxonomy ("pal" → "Paladin") — the
  // proposal renders real class names instead of seed codes. Fetched once;
  // a failure just leaves the codes visible.
  const [classNames, setClassNames] = useState<Record<string, string>>({})
  // Cohort mode: split into N smaller raids (1 = single raid). The plan
  // endpoint suggests how many complete MIN comps the live roster can staff.
  const [cohorts, setCohorts] = useState(1)
  const [plan, setPlan] = useState<SplitPlanReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [splitError, setSplitError] = useState('')

  React.useEffect(() => {
    let cancelled = false
    getRaidTaxonomy()
      .then((tax) => {
        if (!cancelled) setClassNames(tax.class_names ?? {})
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  // Plan (cohort auto-suggest) follows the picked encounter.
  React.useEffect(() => {
    let cancelled = false
    setPlan(null)
    if (!selectedId) return
    getRaidSplitPlan(selectedId)
      .then((p) => {
        if (!cancelled) setPlan(p)
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [selectedId])

  const selectedEncounter = useMemo(
    () => orderedEncounters.find((e) => e.id === selectedId) ?? null,
    [orderedEncounters, selectedId],
  )

  async function runSplit(): Promise<void> {
    if (!selectedId) {
      setSplitError('Pick an encounter first')
      return
    }
    setBusy(true)
    setSplitError('')
    try {
      const rep = await splitRaidComp({
        encounter_id: selectedId,
        preference,
        group_size: groupSize,
        respect_existing_groups: respectGroups,
        wildcards: preference === 'curated' && wildcards.length > 0 ? wildcards : undefined,
        cohorts: cohorts > 1 ? cohorts : undefined,
      })
      setReport(rep)
      setAdjusted(false)
    } catch (err) {
      setSplitError(err instanceof Error ? err.message : String(err))
      setReport(null)
    } finally {
      setBusy(false)
    }
  }

  const prefHint = PREFERENCES.find((p) => p.value === preference)?.hint

  return (
    <div className="flex flex-col gap-4 px-6 py-4 overflow-auto" style={{ height: '100%' }}>
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-semibold flex items-center gap-2" style={{ color: 'var(--color-foreground)' }}>
          <GitFork size={18} /> Group Composition Proposal
        </h1>
        <select
          className={selectCls}
          style={selectStyle}
          value={selectedId}
          onChange={(e) => pickEncounter(e.target.value)}
        >
          {orderedEncounters.length === 0 ? <option value="">No encounters</option> : null}
          {orderedEncounters.map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} — {e.zone}
              {e.status !== 'active' ? ' (placeholder)' : ''}
            </option>
          ))}
        </select>
        <button
          onClick={() => void runSplit()}
          disabled={busy || !selectedId}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded font-medium"
          style={{
            backgroundColor: busy ? 'var(--color-muted)' : 'var(--color-primary)',
            color: 'var(--color-primary-foreground, #fff)',
          }}
        >
          <Play size={14} /> {busy ? 'Proposing…' : 'Generate proposal'}
        </button>
        {adjusted ? (
          <span
            className="text-[11px] px-1.5 py-0.5 rounded font-medium"
            style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
            title="Proposal hand-edited via drag-and-drop — regenerate to start over"
          >
            manually adjusted
          </span>
        ) : null}
        <button
          onClick={() => void refresh()}
          disabled={refreshing}
          className="flex items-center gap-1.5 px-2 py-1.5 text-sm rounded"
          style={{
            backgroundColor: 'var(--color-surface-2)',
            color: 'var(--color-foreground)',
            border: '1px solid var(--color-border)',
            cursor: refreshing ? 'wait' : 'pointer',
          }}
          title="Re-pull encounters and the live Zeal raid roster"
        >
          <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>

      {error ? (
        <div className="px-3 py-2 text-sm rounded" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          {error}
        </div>
      ) : null}

      <RosterStatusBanner roster={roster} extraHint="The proposal uses the live roster; class it via the Taxonomy Editor if members show unclassed." />

      {/* Proposal controls */}
      <div
        className="rounded-lg px-4 py-3 flex flex-col gap-2"
        style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}
      >
        <div className="flex items-center gap-3 flex-wrap">
          <select
            className={selectCls}
            style={selectStyle}
            value={preference}
            onChange={(e) => setPreference(e.target.value as SplitPreference)}
            title={prefHint}
          >
            {PREFERENCES.map((p) => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
          <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--color-foreground)' }}>
            group size
            <input
              className="rounded px-2 py-1 text-sm outline-none border w-14"
              style={selectStyle}
              type="number"
              min={1}
              max={12}
              value={groupSize}
              onChange={(e) => setGroupSize(Math.max(1, Math.min(12, Number(e.target.value) || 6)))}
            />
          </label>
          <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--color-foreground)' }} title="Split the roster into N smaller raids, each staffed against the full template">
            split into
            <select
              className={selectCls}
              style={selectStyle}
              value={cohorts}
              onChange={(e) => setCohorts(Number(e.target.value) || 1)}
            >
              {[1, 2, 3, 4, 5, 6].map((n) => (
                <option key={n} value={n}>{n} raid{n > 1 ? 's' : ''}</option>
              ))
              }
            </select>
          </label>
          <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--color-foreground)' }} title="Seat members into their live Zeal groups where possible">
            <input type="checkbox" checked={respectGroups} onChange={(e) => setRespectGroups(e.target.checked)} />
            keep live groups together
          </label>
        </div>
        {prefHint ? (
          <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>{prefHint}</p>
        ) : null}

        {cohorts > 1 && plan ? (
          <p className="text-xs" style={{ color: plan.max_cohorts >= cohorts ? 'var(--color-success)' : 'var(--color-danger)' }}>
            {plan.max_cohorts >= cohorts
              ? `live roster can staff ${plan.max_cohorts} complete raid${plan.max_cohorts === 1 ? '' : 's'}${plan.binding_path ? ` — capped by ${plan.leaves.find((l) => l.path === plan.binding_path)?.label ?? plan.binding_path}` : ''}`
              : `warning: the live roster only staffs ${plan.max_cohorts} complete raid${plan.max_cohorts === 1 ? '' : 's'}${plan.binding_path ? ` — capped by ${plan.leaves.find((l) => l.path === plan.binding_path)?.label ?? plan.binding_path}` : ''}; extra raids will show MIN gaps`}
          </p>
        ) : null}

        {preference === 'curated' ? (
          <div className="flex flex-col gap-1.5 pt-1">
            <div className="flex items-center gap-2">
              <span className="text-xs uppercase tracking-wide" style={{ color: 'var(--color-muted-foreground)' }}>
                Curated rules (wildcards)
              </span>
              <button
                onClick={() => setWildcards((w) => [...w, { kind: 'class', value: '', group: 0, max: 0, min: 0 }])}
                className="flex items-center gap-1 px-2 py-0.5 text-xs rounded"
                style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
              >
                <Plus size={12} /> Add rule
              </button>
            </div>
            {wildcards.length === 0 ? (
              <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
                No rules — curated behaves like trinity until you pin members or cap classes/roles.
              </span>
            ) : (
              wildcards.map((wc, i) => (
                <WildcardRow
                  key={i}
                  wc={wc}
                  onChange={(patch) => setWildcards((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))}
                  onRemove={() => setWildcards((rows) => rows.filter((_, idx) => idx !== i))}
                />
              ))
            )}
          </div>
        ) : null}
      </div>

      {splitError ? (
        <div className="px-3 py-2 text-sm rounded" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          {splitError}
        </div>
      ) : null}

      {report ? (
        <GroupProposal
          report={report}
          classNames={classNames}
          onEdit={(next) => {
            setReport(next)
            setAdjusted(true)
          }}
        />
      ) : (
        <div className="text-sm py-6 text-center" style={{ color: 'var(--color-muted-foreground)' }}>
          Pick an encounter and generate a proposal to see suggested groups.
        </div>
      )}

      {selectedEncounter && !report ? (
        <div className="text-xs text-center" style={{ color: 'var(--color-muted-foreground)' }}>
          Target: {selectedEncounter.name} — proposal fills its MIN floor first, then REC comfort slots.
        </div>
      ) : null}
    </div>
  )
}
