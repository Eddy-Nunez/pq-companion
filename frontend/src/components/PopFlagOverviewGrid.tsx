import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Check, ClipboardCopy, Download, RefreshCw } from 'lucide-react'
import { exportPopFlags, getPopFlagDataset, getPopFlagSummary } from '../services/api'
import type { PoPFlag, PoPSummaryCharacter, PoPSummaryResponse } from '../types/popflag'
import { zoneColor } from '../lib/popFlagKind'
import { useWebSocket } from '../hooks/useWebSocket'

interface Props {
  /** Switch the page to one character's checklist. */
  onSelectCharacter: (name: string) => void
}

const TIER_LABELS: Record<number, string> = { 5: 'Time' }
const tierName = (t: number): string => `Tier ${TIER_LABELS[t] ?? t}`

// summaryText renders one line per character, for pasting into Discord:
//   Astrael — 18/41 · T1: PoJ✓ PoD 2/3 PoI· · T2: ...
function summaryText(
  chars: PoPSummaryCharacter[],
  zones: PoPSummaryResponse['zones'],
): string {
  const tiers = Array.from(new Set(zones.map((z) => z.tier))).sort((a, b) => a - b)
  return chars
    .map((c) => {
      const parts = tiers.map((t) => {
        const cells = zones
          .filter((z) => z.tier === t)
          .map((z) => {
            const p = c.zones.find((x) => x.key === z.key)
            if (!p || p.done === 0) return `${z.label}·`
            if (p.done >= p.total) return `${z.label}✓`
            return `${z.label} ${p.done}/${p.total}`
          })
        return `T${t}: ${cells.join(' ')}`
      })
      return `${c.name} — ${c.done}/${c.total} · ${parts.join(' · ')}`
    })
    .join('\n')
}

function downloadJson(data: unknown, filename: string): void {
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

const btnStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  color: 'var(--color-muted-foreground)',
  border: '1px solid var(--color-border)',
}

export default function PopFlagOverviewGrid({ onSelectCharacter }: Props): React.ReactElement {
  const [summary, setSummary] = useState<PoPSummaryResponse | null>(null)
  const [dataset, setDataset] = useState<PoPFlag[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [copied, setCopied] = useState(false)

  const load = useCallback(() => {
    setLoading(true)
    setError(null)
    Promise.all([getPopFlagSummary(), getPopFlagDataset()])
      .then(([s, d]) => { setSummary(s); setDataset(d.flags) })
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { load() }, [load])

  // Any character's flags changing (Seer/#popflags commit) refreshes the grid.
  useWebSocket((msg) => {
    if (msg.type === 'popflag.snapshot') load()
  })

  const tierBands = useMemo(() => {
    const bands: Array<{ tier: number; count: number }> = []
    for (const z of summary?.zones ?? []) {
      const last = bands[bands.length - 1]
      if (last && last.tier === z.tier) last.count++
      else bands.push({ tier: z.tier, count: 1 })
    }
    return bands
  }, [summary])

  // zone key → tracked (non-optional, non-group) flags, for "missing" tooltips.
  const flagsByZone = useMemo(() => {
    const m = new Map<string, PoPFlag[]>()
    for (const f of dataset) {
      if (f.optional || f.group) continue
      if (!m.has(f.zone)) m.set(f.zone, [])
      m.get(f.zone)!.push(f)
    }
    return m
  }, [dataset])

  const onExport = useCallback(() => {
    exportPopFlags()
      .then((data) => {
        const day = new Date().toISOString().slice(0, 10)
        downloadJson(data, `pq-companion-popflags-${day}.json`)
      })
      .catch((err: Error) => setError(err.message))
  }, [])

  const onCopy = useCallback(() => {
    if (!summary) return
    navigator.clipboard
      .writeText(summaryText(summary.characters, summary.zones))
      .then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 1500)
      })
      .catch(() => {})
  }, [summary])

  if (loading && !summary) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <RefreshCw size={20} className="animate-spin" style={{ color: 'var(--color-muted)' }} />
      </div>
    )
  }

  const zones = summary?.zones ?? []
  const chars = summary?.characters ?? []

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div
        className="flex items-center gap-2 border-b px-4 py-2 shrink-0"
        style={{ borderColor: 'var(--color-border)' }}
      >
        <span className="text-[11px]" style={{ color: 'var(--color-muted)' }}>
          {chars.length} {chars.length === 1 ? 'character' : 'characters'} — click a name to open
          their checklist.
        </span>
        <div className="ml-auto flex items-center gap-2">
          <button
            onClick={onCopy}
            disabled={chars.length === 0}
            className="flex items-center gap-1.5 rounded px-2 py-1 text-xs"
            style={btnStyle}
            title="Copy a one-line-per-character text summary"
          >
            {copied ? <Check size={11} /> : <ClipboardCopy size={11} />}
            {copied ? 'Copied' : 'Copy summary'}
          </button>
          <button
            onClick={onExport}
            disabled={chars.length === 0}
            className="flex items-center gap-1.5 rounded px-2 py-1 text-xs"
            style={btnStyle}
            title="Download every character's progress as a JSON file"
          >
            <Download size={11} />
            Export JSON
          </button>
          <button
            onClick={load}
            className="flex items-center gap-1.5 rounded px-2 py-1 text-xs"
            style={btnStyle}
          >
            <RefreshCw size={11} />
            Refresh
          </button>
        </div>
      </div>

      {error && (
        <p className="px-4 py-2 text-[11px]" style={{ color: '#f87171' }}>{error}</p>
      )}

      {chars.length === 0 ? (
        <p className="px-4 py-6 text-sm" style={{ color: 'var(--color-muted)' }}>
          No characters yet. Add a character or sync flags from the game to see progress here.
        </p>
      ) : (
        <div className="min-h-0 flex-1 overflow-auto p-4">
          <table className="border-separate border-spacing-0 text-xs">
            <thead>
              <tr>
                <th
                  className="sticky left-0 top-0 z-20"
                  style={{ backgroundColor: 'var(--color-surface)' }}
                />
                {bandsHeader(tierBands)}
                <th
                  className="sticky top-0 z-10"
                  style={{ backgroundColor: 'var(--color-surface)' }}
                />
              </tr>
              <tr>
                <th
                  className="sticky left-0 z-20 px-3 py-1.5 text-left text-[10px] uppercase tracking-wider"
                  style={{
                    top: 20,
                    backgroundColor: 'var(--color-surface)',
                    color: 'var(--color-muted)',
                  }}
                >
                  Character
                </th>
                {zones.map((z) => (
                  <th
                    key={z.key}
                    className="sticky z-10 px-1.5 py-1.5 text-center text-[10px] font-semibold uppercase"
                    style={{
                      top: 20,
                      backgroundColor: 'var(--color-surface)',
                      color: zoneColor(z.key),
                    }}
                    title={z.key}
                  >
                    {z.label}
                  </th>
                ))}
                <th
                  className="sticky z-10 px-3 py-1.5 text-right text-[10px] uppercase tracking-wider"
                  style={{
                    top: 20,
                    backgroundColor: 'var(--color-surface)',
                    color: 'var(--color-muted)',
                  }}
                >
                  Total
                </th>
              </tr>
            </thead>
            <tbody>
              {chars.map((c) => {
                const doneIds = new Set(c.flags.filter((f) => f.done).map((f) => f.id))
                return (
                  <tr key={c.name}>
                    <td
                      className="sticky left-0 z-10 px-3 py-1"
                      style={{ backgroundColor: 'var(--color-surface)' }}
                    >
                      <button
                        onClick={() => onSelectCharacter(c.name)}
                        className="text-left text-sm font-medium hover:underline"
                        style={{ color: 'var(--color-foreground)' }}
                      >
                        {c.name}
                      </button>
                    </td>
                    {zones.map((z) => {
                      const p = c.zones.find((x) => x.key === z.key)
                      const done = p?.done ?? 0
                      const total = p?.total ?? 0
                      const complete = total > 0 && done >= total
                      const partial = done > 0 && !complete
                      const missing = (flagsByZone.get(z.key) ?? [])
                        .filter((f) => !doneIds.has(f.id))
                        .map((f) => f.label)
                      const title = complete
                        ? `${z.label} ${done}/${total} — complete`
                        : `${z.label} ${done}/${total} — missing: ${missing.join(', ')}`
                      return (
                        <td key={z.key} className="px-1 py-1 text-center">
                          <span
                            title={title}
                            className="inline-flex h-5 w-5 items-center justify-center rounded text-[9px] font-semibold"
                            style={{
                              backgroundColor: complete
                                ? '#34b97b'
                                : partial
                                  ? '#3b9ae8'
                                  : 'var(--color-surface-2)',
                              color: complete || partial ? '#fff' : 'var(--color-muted)',
                            }}
                          >
                            {partial ? done : ''}
                          </span>
                        </td>
                      )
                    })}
                    <td
                      className="px-3 py-1 text-right tabular-nums"
                      style={{ color: 'var(--color-muted-foreground)' }}
                    >
                      {c.done}/{c.total}
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
          <div
            className="mt-3 flex items-center gap-4 text-[10px]"
            style={{ color: 'var(--color-muted)' }}
          >
            <Legend color="#34b97b" label="Zone complete" />
            <Legend color="#3b9ae8" label="In progress (flags done)" />
            <Legend color="var(--color-surface-2)" label="Not started" />
          </div>
        </div>
      )}
    </div>
  )
}

function bandsHeader(bands: Array<{ tier: number; count: number }>): React.ReactNode {
  return bands.map((b) => (
    <th
      key={b.tier}
      colSpan={b.count}
      className="sticky top-0 z-10 border-l px-1.5 py-1 text-center text-[10px] uppercase tracking-wider"
      style={{
        backgroundColor: 'var(--color-surface)',
        borderColor: 'var(--color-border)',
        color: 'var(--color-muted)',
      }}
    >
      {tierName(b.tier)}
    </th>
  ))
}

function Legend({ color, label }: { color: string; label: string }): React.ReactElement {
  return (
    <span className="flex items-center gap-1.5">
      <span className="inline-block h-3 w-3 rounded" style={{ backgroundColor: color }} />
      {label}
    </span>
  )
}
