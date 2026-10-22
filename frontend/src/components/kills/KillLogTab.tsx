import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Skull, RefreshCw, Trash2, AlertCircle, X, ArrowUp, ArrowDown, Star } from 'lucide-react'
import { getKillMeta, listKills, clearKills, searchZones } from '../../services/api'
import type { KillEntry, KillScope } from '../../types/kills'
import { useWebSocket } from '../../hooks/useWebSocket'
import { WSEvent } from '../../lib/wsEvents'
import { useEscapeToClose } from '../../hooks/useEscapeToClose'
import MissingLogNotice from '../MissingLogNotice'
import BackfillLink from '../BackfillLink'
import { usePersistentState } from '../../hooks/usePersistentState'

// Seconds matter here — the Kill Log exists to time named/placeholder
// respawns, so the timestamp keeps them.
function formatTimestamp(unix: number): string {
  if (!unix) return ''
  return new Date(unix * 1000).toLocaleString([], {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function formatAgo(unix: number, now: number): string {
  const s = Math.max(0, Math.floor(now - unix))
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h ${m % 60}m ago`
  return `${Math.floor(h / 24)}d ago`
}

type View = 'list' | 'mob'
type ListSort = 'time' | 'target' | 'killer' | 'zone'
type MobSort = 'target' | 'count' | 'last' | 'zone'
type SortDir = 'asc' | 'desc'

// Time-window presets (seconds back from now); 0 = all time.
const WINDOWS: { label: string; secs: number }[] = [
  { label: 'Last hour', secs: 3600 },
  { label: 'Last 3 hours', secs: 3 * 3600 },
  { label: 'Last 24 hours', secs: 24 * 3600 },
  { label: 'Last 7 days', secs: 7 * 24 * 3600 },
  { label: 'All time', secs: 0 },
]

const DISPLAY_STEP = 500

interface MobRow {
  target: string
  named: boolean
  count: number
  first: number
  last: number
  zones: string[]
}

const inputStyle: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  border: '1px solid var(--color-border)',
  color: 'var(--color-foreground)',
}

const ghostBtn: React.CSSProperties = {
  backgroundColor: 'var(--color-surface-2)',
  color: 'var(--color-muted-foreground)',
  border: '1px solid var(--color-border)',
}

export default function KillLogTab(): React.ReactElement {
  const [characters, setCharacters] = useState<string[]>([])
  const [selectedChar, setSelectedChar] = useState('')
  const [metaLoaded, setMetaLoaded] = useState(false)
  const [zones, setZones] = useState<string[]>([])

  const [rows, setRows] = useState<KillEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const seqRef = useRef(0)
  const [search, setSearch] = useState('')
  const [zoneFilter, setZoneFilter] = useState('')
  const [scope, setScope] = useState<KillScope>('npc')
  const [windowSecs, setWindowSecs] = useState(0)
  const [view, setView] = usePersistentState<View>('pq-tab:kill-log-view', 'list', ['list', 'mob'])
  const [listSort, setListSort] = useState<ListSort>('time')
  const [mobSort, setMobSort] = useState<MobSort>('last')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [confirmClearOpen, setConfirmClearOpen] = useState(false)
  const [now, setNow] = useState(() => Date.now() / 1000)
  const navigate = useNavigate()

  // "x ago" in the by-mob view stays fresh without refetching.
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now() / 1000), 15000)
    return () => clearInterval(t)
  }, [])

  async function openZone(zoneName: string) {
    try {
      const r = await searchZones(zoneName)
      const exact = r.items.find((z) => z.long_name.toLowerCase() === zoneName.toLowerCase())
      const zone = exact ?? r.items[0]
      if (zone) navigate(`/zones?select=${zone.id}`)
    } catch { /* best-effort */ }
  }

  const loadMeta = useCallback(() => {
    getKillMeta(selectedChar || undefined)
      .then((m) => {
        const chars = m.characters ?? []
        setCharacters(chars)
        setZones(m.zones ?? [])
        setSelectedChar((cur) => {
          if (cur && chars.includes(cur)) return cur
          if (m.active && chars.includes(m.active)) return m.active
          return chars[0] ?? m.active ?? ''
        })
      })
      .catch(() => { /* best effort */ })
      .finally(() => setMetaLoaded(true))
  }, [selectedChar])

  const load = useCallback(() => {
    const seq = ++seqRef.current
    setLoading(true)
    setError(null)
    listKills({ character: selectedChar || undefined, search, zone: zoneFilter, scope, sort: 'desc', limit: 10000 })
      .then((r) => { if (seq === seqRef.current) setRows(r.kills) })
      .catch((e: Error) => { if (seq === seqRef.current) setError(e.message) })
      .finally(() => { if (seq === seqRef.current) setLoading(false) })
  }, [selectedChar, search, zoneFilter, scope])

  useEffect(() => { if (metaLoaded) load() }, [load, metaLoaded])
  useEffect(() => { loadMeta() }, [selectedChar]) // eslint-disable-line react-hooks/exhaustive-deps

  const reloadTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const onWs = useCallback((msg: { type: string }) => {
    if (msg.type !== WSEvent.KillsNew) return
    if (reloadTimer.current) clearTimeout(reloadTimer.current)
    reloadTimer.current = setTimeout(() => { load(); setNow(Date.now() / 1000) }, 500)
  }, [load])
  useWebSocket(onWs)

  // The time window is applied client-side so switching it is instant.
  const windowed = useMemo(() => {
    if (!windowSecs) return rows
    const from = now - windowSecs
    return rows.filter((r) => r.ts >= from)
  }, [rows, windowSecs, now])

  const [displayLimit, setDisplayLimit] = useState(DISPLAY_STEP)
  useEffect(() => { setDisplayLimit(DISPLAY_STEP) }, [selectedChar, search, zoneFilter, scope, windowSecs, view])

  const sortedRows = useMemo(() => {
    const dir = sortDir === 'asc' ? 1 : -1
    return [...windowed].sort((a, b) => {
      switch (listSort) {
        case 'time': return (a.ts - b.ts || a.id - b.id) * dir
        case 'target': return a.target.localeCompare(b.target) * dir || b.ts - a.ts
        case 'killer': return a.killer.localeCompare(b.killer) * dir || b.ts - a.ts
        case 'zone': return (a.zone || '').localeCompare(b.zone || '') * dir || b.ts - a.ts
        default: return 0
      }
    })
  }, [windowed, listSort, sortDir])

  const mobRows = useMemo(() => {
    const by = new Map<string, MobRow>()
    for (const r of windowed) {
      const key = r.target.toLowerCase()
      let m = by.get(key)
      if (!m) {
        m = { target: r.target, named: r.named, count: 0, first: r.ts, last: r.ts, zones: [] }
        by.set(key, m)
      }
      m.count++
      m.first = Math.min(m.first, r.ts)
      m.last = Math.max(m.last, r.ts)
      if (r.zone && !m.zones.includes(r.zone)) m.zones.push(r.zone)
    }
    const dir = sortDir === 'asc' ? 1 : -1
    return [...by.values()].sort((a, b) => {
      switch (mobSort) {
        case 'target': return a.target.localeCompare(b.target) * dir
        case 'count': return (a.count - b.count) * dir || b.last - a.last
        case 'last': return (a.last - b.last) * dir
        case 'zone': return (a.zones[0] ?? '').localeCompare(b.zones[0] ?? '') * dir
        default: return 0
      }
    })
  }, [windowed, mobSort, sortDir])

  function sortList(field: ListSort) {
    if (listSort === field) setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))
    else { setListSort(field); setSortDir(field === 'time' ? 'desc' : 'asc') }
  }
  function sortMob(field: MobSort) {
    if (mobSort === field) setSortDir((d) => (d === 'desc' ? 'asc' : 'desc'))
    else { setMobSort(field); setSortDir(field === 'target' || field === 'zone' ? 'asc' : 'desc') }
  }

  async function doClear() {
    setConfirmClearOpen(false)
    try {
      await clearKills(selectedChar || undefined)
      load()
      loadMeta()
    } catch (e) {
      setError((e as Error).message)
    }
  }

  const shown = view === 'list' ? sortedRows.length : mobRows.length
  const filtersActive = search || zoneFilter

  const zoneCell = (zone: string): React.ReactElement =>
    zone ? (
      <button
        onClick={() => openZone(zone)}
        className="py-1 truncate text-left hover:underline"
        style={{ color: 'var(--color-primary)', background: 'transparent', border: 'none', cursor: 'pointer' }}
        title="Open this zone in the Zone browser"
      >
        {zone}
      </button>
    ) : (
      <span className="py-1" style={{ color: 'var(--color-muted-foreground)' }}>—</span>
    )

  const nameCell = (target: string, named: boolean): React.ReactElement => (
    <span className="py-1 flex items-center gap-1 min-w-0" style={{ color: 'var(--color-foreground)', fontWeight: named ? 600 : 400 }}>
      {named && <Star size={10} style={{ color: '#f59e0b', flexShrink: 0 }} />}
      <span className="truncate">{target}</span>
    </span>
  )

  return (
    <>
      {/* Character tabs */}
      {metaLoaded && characters.length > 1 && (
        <div className="border-b shrink-0" style={{ borderColor: 'var(--color-border)' }}>
          <div className="px-3 pt-2 flex items-end gap-1 overflow-x-auto" style={{ marginBottom: -1 }}>
            {characters.map((name) => {
              const active = name === selectedChar
              return (
                <button key={name} onClick={() => setSelectedChar(name)}
                  className="rounded-t px-3 py-1.5 text-xs font-medium whitespace-nowrap"
                  style={{
                    backgroundColor: active ? 'var(--color-surface)' : 'transparent',
                    color: active ? 'var(--color-primary)' : 'var(--color-muted-foreground)',
                    border: '1px solid', borderColor: active ? 'var(--color-border)' : 'transparent',
                    borderBottom: active ? '1px solid var(--color-surface)' : '1px solid transparent',
                  }}>
                  {name}
                </button>
              )
            })}
          </div>
        </div>
      )}

      {/* Filters */}
      <div className="border-b px-4 py-2 shrink-0 flex items-center gap-2 flex-wrap" style={{ borderColor: 'var(--color-border)' }}>
        <input type="text" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search mob or killer…"
          className="text-xs rounded px-2 py-1 outline-none" style={{ ...inputStyle, minWidth: '13rem' }} />
        <select value={zoneFilter} onChange={(e) => setZoneFilter(e.target.value)}
          className="text-xs rounded px-2 py-1 outline-none" style={inputStyle}>
          <option value="">All zones</option>
          {zones.map((z) => <option key={z} value={z}>{z}</option>)}
        </select>
        <select value={scope} onChange={(e) => setScope(e.target.value as KillScope)}
          className="text-xs rounded px-2 py-1 outline-none" style={inputStyle}
          title="Named = mobs with a proper name (best-effort; some capitalized trash is included)">
          <option value="npc">All NPCs</option>
          <option value="named">Named only</option>
          <option value="all">NPCs + player deaths</option>
        </select>
        <select value={windowSecs} onChange={(e) => setWindowSecs(Number(e.target.value))}
          className="text-xs rounded px-2 py-1 outline-none" style={inputStyle}>
          {WINDOWS.map((w) => <option key={w.secs} value={w.secs}>{w.label}</option>)}
        </select>
        <div className="flex rounded overflow-hidden" style={{ border: '1px solid var(--color-border)' }}>
          {(['list', 'mob'] as View[]).map((v) => (
            <button key={v} onClick={() => setView(v)} className="text-xs px-2 py-1"
              style={{
                backgroundColor: view === v ? 'var(--color-primary)' : 'var(--color-surface-2)',
                color: view === v ? '#fff' : 'var(--color-muted-foreground)',
                border: 'none', cursor: 'pointer',
              }}>
              {v === 'list' ? 'Every kill' : 'By mob'}
            </button>
          ))}
        </div>
        {filtersActive && (
          <button onClick={() => { setSearch(''); setZoneFilter('') }}
            className="flex items-center gap-1.5 text-xs px-2 py-1 rounded" style={ghostBtn}>
            <X size={11} /> Clear filters
          </button>
        )}
        <span className="text-[11px]" style={{ color: 'var(--color-muted)' }}>
          {shown} {view === 'list' ? `kill${shown === 1 ? '' : 's'}` : `mob${shown === 1 ? '' : 's'}`}
        </span>
        <div className="ml-auto flex items-center gap-2">
          <BackfillLink />
          <button onClick={() => { load(); loadMeta() }} className="flex items-center gap-1.5 text-xs px-2 py-1 rounded" style={ghostBtn}>
            <RefreshCw size={11} /> Refresh
          </button>
          <button onClick={() => setConfirmClearOpen(true)} disabled={rows.length === 0}
            className="flex items-center gap-1.5 text-xs px-2 py-1 rounded disabled:opacity-50"
            style={{ ...ghostBtn, color: 'var(--color-danger)' }}>
            <Trash2 size={11} /> Clear all
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-4">
        <MissingLogNotice />
        {loading && rows.length === 0 && (
          <div className="flex items-center justify-center py-12">
            <RefreshCw size={18} className="animate-spin" style={{ color: 'var(--color-muted)' }} />
          </div>
        )}
        {error && !loading && (
          <div className="flex items-start gap-2 rounded p-3" style={{ backgroundColor: 'var(--color-surface-2)' }}>
            <AlertCircle size={14} style={{ color: 'var(--color-danger)' }} />
            <p className="text-xs" style={{ color: 'var(--color-danger)' }}>{error}</p>
          </div>
        )}
        {!loading && !error && shown === 0 && (
          <div className="flex flex-col items-center justify-center gap-2 py-12">
            <Skull size={32} style={{ color: 'var(--color-muted)' }} />
            <p className="text-sm" style={{ color: 'var(--color-muted-foreground)' }}>
              {rows.length === 0 ? 'No kills tracked yet.' : 'No kills match these filters.'}
            </p>
            {rows.length === 0 && (
              <p className="text-[11px] max-w-md text-center" style={{ color: 'var(--color-muted)' }}>
                Every kill your log shows is captured here as you play — yours, your group's, and any
                "has been slain by" line in range. To fill in past kills, use the
                <span className="font-medium"> Backfill</span> button above.
              </p>
            )}
          </div>
        )}

        {!error && shown > 0 && view === 'list' && (
          <div style={{ opacity: loading ? 0.6 : 1, transition: 'opacity 150ms' }}>
            <div className="grid gap-x-3 text-xs" style={{ gridTemplateColumns: 'auto minmax(0,1.4fr) minmax(0,1fr) minmax(0,1fr)', color: 'var(--color-muted-foreground)' }}>
              <SortHeader label="Time" active={listSort === 'time'} dir={sortDir} onClick={() => sortList('time')} />
              <SortHeader label="Mob" active={listSort === 'target'} dir={sortDir} onClick={() => sortList('target')} />
              <SortHeader label="Killed by" active={listSort === 'killer'} dir={sortDir} onClick={() => sortList('killer')} />
              <SortHeader label="Zone" active={listSort === 'zone'} dir={sortDir} onClick={() => sortList('zone')} />
              {sortedRows.slice(0, displayLimit).map((r) => (
                <React.Fragment key={r.id}>
                  <span className="py-1 tabular-nums whitespace-nowrap" style={{ color: 'var(--color-muted)' }}>{formatTimestamp(r.ts)}</span>
                  {r.is_npc ? nameCell(r.target, r.named) : (
                    <span className="py-1 truncate" style={{ color: 'var(--color-danger)' }} title="Player death">{r.target}</span>
                  )}
                  <span className="py-1 truncate">{r.killer || '—'}</span>
                  {zoneCell(r.zone)}
                </React.Fragment>
              ))}
            </div>
            {sortedRows.length > displayLimit && (
              <ShowMore shown={displayLimit} total={sortedRows.length} onMore={() => setDisplayLimit((l) => l + DISPLAY_STEP)} />
            )}
          </div>
        )}

        {!error && shown > 0 && view === 'mob' && (
          <div style={{ opacity: loading ? 0.6 : 1, transition: 'opacity 150ms' }}>
            <div className="grid gap-x-3 text-xs" style={{ gridTemplateColumns: 'minmax(0,1.4fr) auto auto auto minmax(0,1fr)', color: 'var(--color-muted-foreground)' }}>
              <SortHeader label="Mob" active={mobSort === 'target'} dir={sortDir} onClick={() => sortMob('target')} />
              <SortHeader label="Kills" active={mobSort === 'count'} dir={sortDir} onClick={() => sortMob('count')} />
              <span className="font-semibold border-b pb-1" style={{ color: 'var(--color-muted)', borderColor: 'var(--color-border)' }}>First</span>
              <SortHeader label="Last" active={mobSort === 'last'} dir={sortDir} onClick={() => sortMob('last')} />
              <SortHeader label="Zone" active={mobSort === 'zone'} dir={sortDir} onClick={() => sortMob('zone')} />
              {mobRows.slice(0, displayLimit).map((m) => (
                <React.Fragment key={m.target}>
                  {nameCell(m.target, m.named)}
                  <span className="py-1 tabular-nums text-right" style={{ color: 'var(--color-foreground)' }}>{m.count}</span>
                  <span className="py-1 tabular-nums whitespace-nowrap" style={{ color: 'var(--color-muted)' }}>{formatTimestamp(m.first)}</span>
                  <span className="py-1 tabular-nums whitespace-nowrap" title={formatTimestamp(m.last)}>
                    {formatTimestamp(m.last)}{' '}
                    <span style={{ color: 'var(--color-muted)' }}>({formatAgo(m.last, now)})</span>
                  </span>
                  <span className="py-1 truncate" title={m.zones.join(', ')}>
                    {m.zones.length > 1 ? `${m.zones[0]} +${m.zones.length - 1}` : zoneCell(m.zones[0] ?? '')}
                  </span>
                </React.Fragment>
              ))}
            </div>
            {mobRows.length > displayLimit && (
              <ShowMore shown={displayLimit} total={mobRows.length} onMore={() => setDisplayLimit((l) => l + DISPLAY_STEP)} />
            )}
          </div>
        )}

        {!error && shown > 0 && (
          <p className="mt-3 text-[11px]" style={{ color: 'var(--color-muted)' }}>
            Only kills your own log shows are listed. Kills far away in a big zone may not be logged.
          </p>
        )}
      </div>

      {confirmClearOpen && (
        <ConfirmModal
          title="Clear kill log?"
          body={`Delete all logged kills for ${selectedChar || 'the active character'}. Kill counters are not affected. This cannot be undone.`}
          onCancel={() => setConfirmClearOpen(false)}
          onConfirm={doClear}
        />
      )}
    </>
  )
}

function ShowMore({ shown, total, onMore }: { shown: number; total: number; onMore: () => void }): React.ReactElement {
  return (
    <div className="mt-3 flex items-center gap-3">
      <button onClick={onMore} className="text-xs px-2 py-1 rounded" style={ghostBtn}>
        Show {Math.min(DISPLAY_STEP, total - shown)} more
      </button>
      <span className="text-[11px]" style={{ color: 'var(--color-muted)' }}>Showing {shown} of {total}</span>
    </div>
  )
}

function SortHeader({
  label, active, dir, onClick,
}: {
  label: string
  active: boolean
  dir: SortDir
  onClick: () => void
}): React.ReactElement {
  return (
    <button onClick={onClick} className="font-semibold border-b pb-1 flex items-center gap-1"
      style={{ color: active ? 'var(--color-foreground)' : 'var(--color-muted)', borderColor: 'var(--color-border)', background: 'transparent', cursor: 'pointer' }}>
      <span>{label}</span>
      {active && (dir === 'asc' ? <ArrowUp size={10} /> : <ArrowDown size={10} />)}
    </button>
  )
}

function ConfirmModal({
  title, body, onCancel, onConfirm,
}: {
  title: string
  body: string
  onCancel: () => void
  onConfirm: () => void
}): React.ReactElement {
  useEscapeToClose(onCancel)
  return (
    <div onClick={onCancel} style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.6)', zIndex: 1000, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
      <div onClick={(e) => e.stopPropagation()} className="rounded-lg p-4 space-y-3" style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)', width: '100%', maxWidth: 420 }}>
        <div className="flex items-center gap-2">
          <AlertCircle size={16} style={{ color: 'var(--color-danger)' }} />
          <p className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>{title}</p>
        </div>
        <p className="text-xs leading-relaxed" style={{ color: 'var(--color-muted-foreground)' }}>{body}</p>
        <div className="flex justify-end gap-2 pt-1">
          <button onClick={onCancel} className="text-xs px-3 py-1.5 rounded font-medium" style={{ backgroundColor: 'var(--color-surface-2)', color: 'var(--color-foreground)', border: '1px solid var(--color-border)' }}>Cancel</button>
          <button onClick={onConfirm} className="text-xs px-3 py-1.5 rounded font-medium" style={{ backgroundColor: 'var(--color-danger)', color: '#fff', border: '1px solid transparent' }}>Clear all</button>
        </div>
      </div>
    </div>
  )
}
