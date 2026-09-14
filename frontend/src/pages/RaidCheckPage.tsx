import React, { useMemo, useState } from 'react'
import { RefreshCw, Play, ShieldCheck, Radio, UserRoundPlus, Trash2 } from 'lucide-react'
import { useRaidReadiness } from '../hooks/useRaidReadiness'
import type { CheckRosterInput } from '../types/raid'
import CompReport from '../components/raids/CompReport'
import EncounterNPCInfo from '../components/raids/EncounterNPCInfo'

const selectCls =
  'rounded px-2 py-1.5 text-sm outline-none border focus:ring-1 focus:ring-(--color-primary)'
const selectStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  borderColor: 'var(--color-border)',
  color: 'var(--color-foreground)',
}

function StampTime({ ts }: { ts?: number }): React.ReactElement {
  if (!ts) return <span>—</span>
  return <span>{new Date(ts * 1000).toLocaleTimeString()}</span>
}

export default function RaidCheckPage(): React.ReactElement {
  const {
    encounters, roster, selectedId, setSelectedId, detectedZone, report, busy, error, refresh, runCheck,
  } = useRaidReadiness()
  const [manualRows, setManualRows] = useState<CheckRosterInput[]>([])
  const [useManual, setUseManual] = useState(false)

  function addManualRow(): void {
    setManualRows((rows) => [...rows, { name: 'Tank', class: 'war' }])
  }

  function setManualRow(i: number, patch: Partial<CheckRosterInput>): void {
    setManualRows((rows) => rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  }

  const liveCount = roster?.members.length ?? 0
  const selectedEncounter = useMemo(
    () => encounters.find((e) => e.id === selectedId) ?? null,
    [encounters, selectedId],
  )

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
          onChange={(e) => setSelectedId(e.target.value)}
        >
          {encounters.length === 0 ? <option value="">No encounters</option> : null}
          {encounters.map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} — {e.zone}
              {e.status !== 'active' ? ' (placeholder)' : ''}
            </option>
          ))}
        </select>
        {useManual ? (
          <button
            onClick={() => void runCheck(selectedId, manualRows.length > 0 ? manualRows : undefined)}
            disabled={busy || !selectedId}
            className="flex items-center gap-1.5 px-3 py-1.5 text-sm rounded font-medium"
            style={{
              backgroundColor: busy ? 'var(--color-muted)' : 'var(--color-primary)',
              color: 'var(--color-primary-foreground, #fff)',
            }}
          >
            <Play size={14} /> {busy ? 'Checking…' : 'Check with manual roster'}
          </button>
        ) : null}
        <button
          onClick={() => void refresh()}
          className="flex items-center gap-1.5 px-2 py-1.5 text-sm rounded"
          style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-muted-foreground)' }}
        >
          <RefreshCw size={14} /> Refresh
        </button>
      </div>
      {!useManual ? (
        <p className="text-xs -mt-2" style={{ color: 'var(--color-muted-foreground)' }}>
          Composition updates automatically from the live Zeal roster as members join, leave, or change class —
          no need to re-run the check by hand.
        </p>
      ) : null}

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

      {selectedEncounter?.npc_id ? <EncounterNPCInfo npcId={selectedEncounter.npc_id} /> : null}
    </div>
  )
}
