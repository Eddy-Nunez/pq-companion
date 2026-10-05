import React, { useMemo, useState } from 'react'
import { GitFork, Play, RefreshCw } from 'lucide-react'
import { useRaidReadiness } from '../hooks/useRaidReadiness'
import { getRaidSplitPlan, getRaidTaxonomy, splitRaidComp } from '../services/api'
import type { SplitPlanReport, SplitReport } from '../types/raid'
import GroupProposal from '../components/raids/GroupProposal'
import RosterStatusBanner from '../components/raids/RosterStatusBanner'

const selectCls =
  'rounded px-2 py-1.5 text-sm outline-none border focus:ring-1 focus:ring-(--color-primary)'
const selectStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  borderColor: 'var(--color-border)',
  color: 'var(--color-foreground)',
}

export default function RaidSplitPage(): React.ReactElement {
  const { orderedEncounters, roster, selectedId, pickEncounter, refreshing, error, refresh } = useRaidReadiness()
  const [groupSize, setGroupSize] = useState(6)
  const [respectGroups, setRespectGroups] = useState(true)
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
  // Group-shape templates (docs/raid-group-compositions-plan.md): explicit
  // opt-in per generate — checked ids travel with the request, unchecked
  // (the default) means pure trinity. Works for single-raid and cohort
  // proposals alike; the placement select is cohort-only (it has no
  // meaning for one raid).
  const [shapeIds, setShapeIds] = useState<Set<string>>(new Set())
  const [shapeDist, setShapeDist] = useState<'replicate' | 'distribute'>('replicate')
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
    setShapeIds(new Set()) // shape selection is per-encounter; default unchecked
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
      const withShapes = shapeIds.size > 0
      const rep = await splitRaidComp({
        encounter_id: selectedId,
        group_size: groupSize,
        respect_existing_groups: respectGroups,
        cohorts: cohorts > 1 ? cohorts : undefined,
        shapes: withShapes ? Array.from(shapeIds) : undefined,
        // Placement only means something when splitting into multiple raids.
        shape_distribution: withShapes && cohorts > 1 ? shapeDist : undefined,
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
          <label className="flex items-center gap-1.5 text-sm" style={{ color: cohorts > 1 ? 'var(--color-muted-foreground)' : 'var(--color-foreground)' }} title={cohorts > 1 ? 'Always on when splitting into multiple raids — each raid keeps its live groups intact' : 'Seat members into their live Zeal groups where possible'}>
            <input type="checkbox" checked={cohorts > 1 ? true : respectGroups} disabled={cohorts > 1} onChange={(e) => setRespectGroups(e.target.checked)} />
            keep live groups together{cohorts > 1 ? ' (always)' : ''}
          </label>
        </div>
        <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          Trinity seating: tank + healer + support first in every group, damage fills the rest.
        </p>

        {(selectedEncounter?.shapes?.length ?? 0) > 0 ? (
          <div className="flex items-center gap-3 flex-wrap">
            <span className="text-sm" style={{ color: 'var(--color-foreground)' }}>
              group compositions
            </span>
            {selectedEncounter!.shapes!.map((sh) => (
              <label
                key={sh.shape_id}
                className="flex items-center gap-1.5 text-sm"
                style={{ color: shapeIds.has(sh.shape_id) ? 'var(--color-foreground)' : 'var(--color-muted-foreground)' }}
                title={`Group template: ${sh.rows.map((r) => `${r.role}${r.sub_role ? '.' + r.sub_role : ''} ×${r.count}`).join(', ')}${sh.group_number ? ` — pinned to group ${sh.group_number}` : ''}`}
              >
                <input
                  type="checkbox"
                  checked={shapeIds.has(sh.shape_id)}
                  onChange={(e) =>
                    setShapeIds((prev) => {
                      const next = new Set(prev)
                      if (e.target.checked) next.add(sh.shape_id)
                      else next.delete(sh.shape_id)
                      return next
                    })
                  }
                />
                {sh.shape_id}
              </label>
            ))}
            {cohorts > 1 ? (
              <label
                className="flex items-center gap-1.5 text-sm"
                style={{ color: 'var(--color-muted-foreground)' }}
                title="replicate — every raid fields every enabled shape · distribute — shape 1 to raid 1, shape 2 to raid 2 (shapes beyond the raid count are ignored)"
              >
                placement
                <select
                  className={selectCls}
                  style={selectStyle}
                  value={shapeDist}
                  onChange={(e) => setShapeDist(e.target.value as 'replicate' | 'distribute')}
                >
                  <option value="replicate">replicate</option>
                  <option value="distribute">distribute</option>
                </select>
              </label>
            ) : null}
          </div>
        ) : null}

        {cohorts > 1 && plan ? (
          <p className="text-xs" style={{ color: plan.max_cohorts >= cohorts ? 'var(--color-success)' : 'var(--color-danger)' }}>
            {plan.max_cohorts >= cohorts
              ? `live roster can staff ${plan.max_cohorts} complete raid${plan.max_cohorts === 1 ? '' : 's'}${plan.binding_path ? ` — capped by ${plan.leaves.find((l) => l.path === plan.binding_path)?.label ?? plan.binding_path}` : ''}`
              : `warning: the live roster only staffs ${plan.max_cohorts} complete raid${plan.max_cohorts === 1 ? '' : 's'}${plan.binding_path ? ` — capped by ${plan.leaves.find((l) => l.path === plan.binding_path)?.label ?? plan.binding_path}` : ''}; extra raids will show MIN gaps`}
          </p>
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
