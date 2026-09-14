import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { RefreshCw, Play, ShieldCheck, Radio, UserRoundPlus, Trash2 } from 'lucide-react'
import {
  getRaidEncounters,
  getRaidRoster,
  checkRaidComp,
} from '../services/api'
import type {
  CheckReport,
  CheckRosterInput,
  RaidEncounter,
  RaidRosterSnapshot,
} from '../types/raid'
import CompReport from '../components/raids/CompReport'

const selectCls =
  'rounded px-2 py-1.5 text-sm outline-none border focus:ring-1 focus:ring-(--color-primary)'
const selectStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  borderColor: 'var(--color-border)',
  color: 'var(--color-foreground)',
}

// normalizeZone makes zone-name comparisons forgiving across Zeal's short
// names vs the knowledge base's display names (letters+digits only, lowercase).
function normalizeZone(s: string): string {
  return (s ?? '').toLowerCase().replace(/[^a-z0-9]+/g, '')
}

function StampTime({ ts }: { ts?: number }): React.ReactElement {
  if (!ts) return <span>—</span>
  return <span>{new Date(ts * 1000).toLocaleTimeString()}</span>
}

export default function RaidCheckPage(): React.ReactElement {
  const [encounters, setEncounters] = useState<RaidEncounter[]>([])
  const [roster, setRoster] = useState<RaidRosterSnapshot | null>(null)
  const [selectedId, setSelectedId] = useState<string>('')
  const [detectedZone, setDetectedZone] = useState<string>('')
  const [report, setReport] = useState<CheckReport | null>(null)
  const [manualRows, setManualRows] = useState<CheckRosterInput[]>([])
  const [useManual, setUseManual] = useState(false)
  const [busy, setBusy] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')
  // Set when the user picks an encounter in the dropdown; detection then stops
  // auto-selecting so polling can't clobber the user's choice.
  const userPickedRef = useRef(false)

  // Re-pulls the encounter list and the live roster snapshot from the backend,
  // which is what re-evaluates the Zeal connectivity state (disconnected /
  // connected-but-not-in-raid / live roster). Always enabled — safe to smash
  // at any time; the spinner is the only feedback it needs.
  const refresh = useCallback(async (): Promise<void> => {
    setRefreshing(true)
    try {
      const [encRes, rosterRes] = await Promise.all([getRaidEncounters(), getRaidRoster()])
      setEncounters(encRes.encounters)
      setRoster(rosterRes)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // Poll the roster while the page is open: the live raid state (Zeal
  // connectivity, in-raid, current zone) only arrives through this endpoint,
  // so without polling a raid joined after page load would never be
  // detected until a manual Refresh.
  useEffect(() => {
    const t = setInterval(() => {
      void refresh()
    }, 5000)
    return () => clearInterval(t)
  }, [refresh])

  // Encounter ranking + detection share one notion of "how close is this
  // encounter to the live zone": 0 = exact zoneidnumber match, 1 =
  // normalized-name match, 2 = no match. Stable-sorting the dropdown by that
  // rank keeps the current zone's encounters as the first options while
  // preserving the knowledge-base order everywhere else.
  const rank = useCallback(
    (e: RaidEncounter): number => {
      if (!roster) return 2
      if (roster.zone_id && roster.zone_id > 0 && e.zone_id === roster.zone_id) {
        return 0
      }
      const zoneKey = normalizeZone(roster.zone ?? '')
      if (zoneKey && normalizeZone(e.zone) === zoneKey) return 1
      return 2
    },
    [roster],
  )

  // Dropdown order: current-zone encounters first (rank 0, then 1), rest in
  // knowledge-base order. Stable sort — encounters within a rank keep their
  // store order, so ties are deterministic.
  const orderedEncounters = useMemo(() => {
    return encounters
      .map((e, i) => ({ e, i }))
      .sort((a, b) => rank(a.e) - rank(b.e) || a.i - b.i)
      .map((x) => x.e)
  }, [encounters, rank])

  // Encounter detection: prefer an exact zoneidnumber match with the live
  // roster (what Zeal reports); fall back to normalized-name comparison for
  // encounters without a resolved zone id. The dropdown stays authoritative.
  useEffect(() => {
    if (!roster || encounters.length === 0) return
    if (userPickedRef.current) return
    const hit = orderedEncounters.find((e) => rank(e) < 2)
    if (hit) {
      setSelectedId(hit.id)
      setDetectedZone(roster.zone ?? String(roster.zone_id ?? ''))
    }
  }, [roster, encounters, orderedEncounters, rank])

  async function runCheck(): Promise<void> {
    if (!selectedId) {
      setError('Pick an encounter first')
      return
    }
    setBusy(true)
    setError('')
    try {
      // When a manual roster is entered it replaces the live one entirely.
      const rep = await checkRaidComp({
        encounter_id: selectedId,
        roster: useManual && manualRows.length > 0 ? manualRows : undefined,
      })
      setReport(rep)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
      setReport(null)
    } finally {
      setBusy(false)
    }
  }

  // Auto-run a first check once data is loaded and an encounter is selectable.
  const autoRan = useMemo(() => ({ done: false }), [])
  useEffect(() => {
    if (!autoRan.done && !busy && (selectedId || encounters.length === 1) && encounters.length > 0) {
      autoRan.done = true
      setSelectedId(selectedId || orderedEncounters[0].id)
      void runCheck()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [encounters, selectedId, orderedEncounters])

  function addManualRow(): void {
    setManualRows((rows) => [...rows, { name: 'Tank', class: 'war' }])
  }

  function setManualRow(i: number, patch: Partial<CheckRosterInput>): void {
    setManualRows((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  }

  const liveCount = roster?.members.length ?? 0

  return (
    <div className="flex flex-col gap-4 px-6 py-4 overflow-auto" style={{ height: '100%' }}>
      <div className="flex items-center gap-3 flex-wrap">
        <h1 className="text-lg font-semibold flex items-center gap-2" style={{ color: 'var(--color-foreground)' }}>
          <ShieldCheck size={18} /> Raid Composition Check
        </h1>
        <select
          className={selectCls}
          style={selectStyle}
          value={selectedId}
          onChange={(e) => {
            userPickedRef.current = true
            setSelectedId(e.target.value)
          }}
        >
          {encounters.length === 0 ? <option value="">No encounters</option> : null}
          {orderedEncounters.map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} — {e.zone}
              {e.status !== 'active' ? ' (placeholder)' : ''}
            </option>
          ))}
        </select>
        <button
          onClick={() => void runCheck()}
          disabled={busy || !selectedId}
          className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded font-medium"
          style={{ backgroundColor: busy ? 'var(--color-muted)' : 'var(--color-primary)', color: '#fff' }}
        >
          <Play size={14} /> {busy ? 'Checking…' : 'Check composition'}
        </button>
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
          title="Re-check encounters and the live Zeal raid roster"
        >
          <RefreshCw size={14} className={refreshing ? 'animate-spin' : ''} /> Refresh
        </button>
      </div>

      {error ? (
        <div className="px-3 py-2 text-sm rounded" style={{ backgroundColor: 'var(--color-danger)', color: '#fff' }}>
          {error}
        </div>
      ) : null}

      {detectedZone ? (
        <div
          className="px-3 py-2 text-sm rounded flex items-center gap-2"
          style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-foreground)', border: '1px solid var(--color-primary)' }}
        >
          <Radio size={14} style={{ color: 'var(--color-primary)' }} />
          Detected encounter for zone &quot;{detectedZone}&quot; — change it above if wrong.
        </div>
      ) : null}

      {/* Roster source card */}
      <div
        className="rounded-lg px-4 py-3 flex items-center justify-between flex-wrap gap-3"
        style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}
      >
        <div className="flex items-center gap-2 text-sm">
          {roster && !roster.zeal_connected ? (
            <>
              <Radio size={15} style={{ color: 'var(--color-danger)' }} />
              <span style={{ color: 'var(--color-foreground)' }}>
                There&apos;s a problem with the Zeal connection — make sure Zeal is installed and
                running, then hit <b>Refresh</b>. You can still use the manual roster form below.
              </span>
            </>
          ) : roster && !roster.in_raid ? (
            <>
              <Radio size={15} style={{ color: 'var(--color-primary)' }} />
              <span style={{ color: 'var(--color-foreground)' }}>
                Zeal is active but you&apos;re not in a raid — you can still use the manual roster
                form below.
              </span>
            </>
          ) : (
            <>
              <Radio size={15} style={{ color: 'var(--color-primary)' }} />
              <span style={{ color: 'var(--color-foreground)' }}>
                Live roster (Zeal pipe) — <b>{liveCount}</b> members
                {roster?.zone ? <> in <b>{roster.zone}</b></> : null}
                {' '}· updated <StampTime ts={roster?.updated_at} />
              </span>
            </>
          )}
        </div>
      </div>

      {/* Manual roster editor */}
      <div
        className="rounded-lg px-4 py-3 flex flex-col gap-2"
        style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}
      >
        <div className="flex items-center gap-2">
          <label className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--color-foreground)' }}>
            <input
              type="checkbox"
              checked={useManual}
              onChange={(e) => setUseManual(e.target.checked)}
            />
            Use manually entered roster instead of the live one
          </label>
          <button
            onClick={addManualRow}
            className="flex items-center gap-1.5 px-2 py-1 text-xs rounded"
            style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
          >
            <UserRoundPlus size={13} /> Add member
          </button>
        </div>
        {manualRows.length > 0 ? (
          <div className="flex flex-col gap-1.5 max-h-48 overflow-auto">
            {manualRows.map((row, i) => (
              <div key={i} className="grid grid-cols-[1fr_1.2fr_auto] gap-2">
                <input
                  className="rounded px-2 py-1 text-sm outline-none border"
                  style={selectStyle}
                  value={row.name}
                  onChange={(e) => setManualRow(i, { name: e.target.value })}
                  placeholder="Member name"
                />
                <input
                  className="rounded px-2 py-1 text-sm outline-none border"
                  style={selectStyle}
                  value={row.class}
                  onChange={(e) => setManualRow(i, { class: e.target.value })}
                  placeholder="class code or name (e.g. war, Shadow Knight)"
                />
                <button
                  onClick={() => setManualRows((rows) => rows.filter((_, idx) => idx !== i))}
                  className="px-2 rounded"
                  style={{ color: 'var(--color-danger)' }}
                  title="Remove member"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        ) : (
          <span className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            No manual members yet — add rows for each raider, or skip and use the live roster.
          </span>
        )}
      </div>

      {report ? (
        <CompReport report={report} />
      ) : (
        <div className="text-sm py-6 text-center" style={{ color: 'var(--color-muted-foreground)' }}>
          Pick an encounter and run a check to see its MIN / REC composition report.
        </div>
      )}
    </div>
  )
}