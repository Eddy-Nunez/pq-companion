import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Check, Copy, Plus, RefreshCw, Search, ShieldBan, Trash2, X } from 'lucide-react'
import {
  adoptBlockedBuffs,
  getBlockedBuffs,
  listCharacters,
  putBlockedBuffs,
  searchSpells,
  type Character,
} from '../services/api'
import type {
  BlockedBuffEntry,
  BlockedBuffRow,
  BlockedBuffStatus,
  BlockedBuffsView,
} from '../types/blockbuff'
import type { Spell } from '../types/spell'
import { useActiveCharacter } from '../contexts/ActiveCharacterContext'
import { useWebSocket } from '../hooks/useWebSocket'
import { useEscapeToClose } from '../hooks/useEscapeToClose'
import { SpellIcon } from '../components/Icon'
import SpellHoverCard from '../components/SpellHoverCard'

const STATUS_INFO: Record<BlockedBuffStatus, { label: string; color: string }> = {
  applied: { label: 'Applied on server', color: 'var(--color-success, #4ade80)' },
  needs_block: { label: 'Needs #blockbuff', color: 'var(--color-warning, #facc15)' },
  needs_allow: { label: 'Needs #allowbuff', color: 'var(--color-warning, #facc15)' },
  unconfirmed: { label: 'Not confirmed yet', color: 'var(--color-muted-foreground)' },
  server_only: { label: 'On server only', color: 'var(--color-muted-foreground)' },
}

function entryOf(r: BlockedBuffEntry): BlockedBuffEntry {
  return { spell_id: r.spell_id, if_spell_id: r.if_spell_id }
}

function sameEntry(a: BlockedBuffEntry, b: BlockedBuffEntry): boolean {
  return a.spell_id === b.spell_id && a.if_spell_id === b.if_spell_id
}

async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

// ── Spell search ─────────────────────────────────────────────────────────────

function SpellSearch({
  buffsOnly,
  onPick,
  autoFocus,
}: {
  /** Restrict to beneficial buffs — the only spells the server can block. */
  buffsOnly: boolean
  onPick: (s: Spell) => void
  autoFocus?: boolean
}): React.ReactElement {
  const [q, setQ] = useState('')
  const [results, setResults] = useState<Spell[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const term = q.trim()
    if (term.length < 2) {
      setResults([])
      return
    }
    let cancelled = false
    setLoading(true)
    const id = setTimeout(() => {
      searchSpells(term, 40, 0, -1, 0, 0, buffsOnly)
        .then((res) => {
          if (cancelled) return
          const items = res.items ?? []
          setResults(buffsOnly ? items.filter((s) => s.buff_duration > 0) : items)
        })
        .catch(() => { if (!cancelled) setResults([]) })
        .finally(() => { if (!cancelled) setLoading(false) })
    }, 250)
    return () => {
      cancelled = true
      clearTimeout(id)
    }
  }, [q, buffsOnly])

  return (
    <div className="space-y-2">
      <div
        className="flex items-center gap-2 rounded px-2.5 py-1.5"
        style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)' }}
      >
        <Search size={13} style={{ color: 'var(--color-muted)' }} />
        <input
          autoFocus={autoFocus}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={buffsOnly ? 'Search buffs (e.g. Spirit of Wolf)…' : 'Search spells…'}
          className="min-w-0 flex-1 bg-transparent text-xs outline-none"
          style={{ color: 'var(--color-foreground)' }}
        />
        {loading && <RefreshCw size={12} className="animate-spin" style={{ color: 'var(--color-muted)' }} />}
      </div>
      <div className="max-h-56 overflow-y-auto rounded" style={{ border: '1px solid var(--color-border)' }}>
        {results.length === 0 ? (
          <p className="px-3 py-4 text-center text-[11px]" style={{ color: 'var(--color-muted)' }}>
            {q.trim().length < 2 ? 'Type at least 2 characters.' : loading ? 'Searching…' : 'No matches.'}
          </p>
        ) : (
          results.map((s) => (
            <button
              key={s.id}
              onClick={() => onPick(s)}
              className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs transition-colors hover:bg-[var(--color-surface-2)]"
              style={{ color: 'var(--color-foreground)' }}
            >
              <SpellIcon id={s.new_icon} name={s.name} size={18} />
              <span className="min-w-0 flex-1 truncate">{s.name}</span>
              <span className="font-mono text-[10px]" style={{ color: 'var(--color-muted)' }}>#{s.id}</span>
            </button>
          ))
        )}
      </div>
    </div>
  )
}

// ── Add modal ────────────────────────────────────────────────────────────────

function AddBlockModal({
  onAdd,
  onClose,
}: {
  onAdd: (e: BlockedBuffEntry) => void
  onClose: () => void
}): React.ReactElement {
  useEscapeToClose(onClose)
  const [spell, setSpell] = useState<Spell | null>(null)
  const [conditional, setConditional] = useState(false)
  const [ifSpell, setIfSpell] = useState<Spell | null>(null)

  const canAdd = spell !== null && (!conditional || ifSpell !== null)

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4"
      style={{ backgroundColor: 'rgba(0,0,0,0.6)' }}
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        className="w-full max-w-md space-y-3 rounded-lg p-4"
        style={{ backgroundColor: 'var(--color-surface)', border: '1px solid var(--color-border)' }}
      >
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
            Block a buff
          </h3>
          <button onClick={onClose} title="Close" style={{ color: 'var(--color-muted)' }}>
            <X size={15} />
          </button>
        </div>

        {spell ? (
          <div
            className="flex items-center gap-2 rounded px-3 py-2 text-xs"
            style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)' }}
          >
            <SpellIcon id={spell.new_icon} name={spell.name} size={20} />
            <span className="flex-1 font-medium" style={{ color: 'var(--color-foreground)' }}>{spell.name}</span>
            <button className="text-[11px] underline" style={{ color: 'var(--color-muted-foreground)' }} onClick={() => setSpell(null)}>
              change
            </button>
          </div>
        ) : (
          <SpellSearch buffsOnly autoFocus onPick={setSpell} />
        )}

        <label className="flex cursor-pointer items-center gap-2 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          <input
            type="checkbox"
            checked={conditional}
            onChange={(e) => { setConditional(e.target.checked); if (!e.target.checked) setIfSpell(null) }}
          />
          Only while another spell is on me
          <span className="font-mono text-[10px]" style={{ color: 'var(--color-muted)' }}>(#blockbuffif)</span>
        </label>

        {conditional &&
          (ifSpell ? (
            <div
              className="flex items-center gap-2 rounded px-3 py-2 text-xs"
              style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)' }}
            >
              <span style={{ color: 'var(--color-muted)' }}>while</span>
              <SpellIcon id={ifSpell.new_icon} name={ifSpell.name} size={20} />
              <span className="flex-1 font-medium" style={{ color: 'var(--color-foreground)' }}>{ifSpell.name}</span>
              <button className="text-[11px] underline" style={{ color: 'var(--color-muted-foreground)' }} onClick={() => setIfSpell(null)}>
                change
              </button>
            </div>
          ) : (
            <SpellSearch buffsOnly={false} onPick={setIfSpell} />
          ))}

        <div className="flex justify-end gap-2 pt-1">
          <button
            onClick={onClose}
            className="rounded px-3 py-1.5 text-xs"
            style={{ color: 'var(--color-muted-foreground)', border: '1px solid var(--color-border)' }}
          >
            Cancel
          </button>
          <button
            disabled={!canAdd}
            onClick={() => spell && onAdd({ spell_id: spell.id, if_spell_id: conditional && ifSpell ? ifSpell.id : 0 })}
            className="rounded px-3 py-1.5 text-xs font-medium disabled:opacity-40"
            style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
          >
            Add block
          </button>
        </div>
      </div>
    </div>
  )
}

// ── Character sub-tabs ───────────────────────────────────────────────────────

function CharacterTabs({
  value,
  onChange,
  characters,
  active,
}: {
  value: string
  onChange: (next: string) => void
  characters: string[]
  active: string
}): React.ReactElement {
  return (
    <div
      className="flex shrink-0 items-center gap-1 overflow-x-auto border-b px-4"
      style={{ borderColor: 'var(--color-border)', backgroundColor: 'var(--color-surface)' }}
    >
      {characters.map((name) => {
        const isActive = name === value
        const isLogged = name === active
        return (
          <button
            key={name}
            onClick={() => onChange(name)}
            className="whitespace-nowrap px-3 py-2 text-xs font-medium transition-colors"
            style={{
              color: isActive ? 'var(--color-primary)' : 'var(--color-muted-foreground)',
              borderBottom: isActive ? '2px solid var(--color-primary)' : '2px solid transparent',
            }}
            title={isLogged ? `${name} (active character)` : name}
          >
            {name}
            {isLogged && (
              <span
                className="ml-1 text-[9px] uppercase tracking-wider"
                style={{ color: isActive ? 'var(--color-primary)' : 'var(--color-muted)' }}
              >
                ●
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}

// ── Page ─────────────────────────────────────────────────────────────────────

export default function CharacterBlockedBuffsPage(): React.ReactElement {
  const { active } = useActiveCharacter()
  const [characters, setCharacters] = useState<Character[]>([])
  const [viewed, setViewed] = useState('')
  const [view, setView] = useState<BlockedBuffsView | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [copied, setCopied] = useState<string | null>(null)
  const copiedTimer = useRef<number | null>(null)

  useEffect(() => {
    listCharacters()
      .then((res) => setCharacters(res.characters ?? []))
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [])

  const names = useMemo(() => characters.map((c) => c.name), [characters])

  // Default to the logged-in character, else the first one.
  useEffect(() => {
    if (viewed || names.length === 0) return
    setViewed(names.includes(active) ? active : names[0])
  }, [names, active, viewed])

  const load = useCallback((character: string) => {
    getBlockedBuffs(character)
      .then((v) => { setView(v); setError(null) })
      .catch((err: Error) => setError(err.message))
  }, [])

  useEffect(() => {
    setView(null)
    if (viewed) load(viewed)
  }, [viewed, load])

  // The backend broadcasts when a #blockbuff reply lands in the log.
  useWebSocket((msg) => {
    if (msg.type !== 'blockbuffs:updated' || !viewed) return
    const data = msg.data as { character?: string } | null
    if ((data?.character ?? '').toLowerCase() === viewed.toLowerCase()) load(viewed)
  })

  const desired = useMemo<BlockedBuffEntry[]>(
    () => (view?.rows ?? []).filter((r) => r.desired).map(entryOf),
    [view],
  )

  const save = useCallback(
    async (next: BlockedBuffEntry[]) => {
      if (!viewed) return
      setBusy(true)
      try {
        setView(await putBlockedBuffs(viewed, next))
        setError(null)
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Save failed')
      } finally {
        setBusy(false)
      }
    },
    [viewed],
  )

  const addEntry = (e: BlockedBuffEntry) => {
    setAdding(false)
    if (desired.some((d) => sameEntry(d, e))) return
    void save([...desired, e])
  }
  const removeEntry = (e: BlockedBuffEntry) => void save(desired.filter((d) => !sameEntry(d, e)))
  const keepEntry = (e: BlockedBuffEntry) => void save([...desired, e])

  const adopt = async () => {
    if (!viewed) return
    setBusy(true)
    try {
      setView(await adoptBlockedBuffs(viewed))
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Adopt failed')
    } finally {
      setBusy(false)
    }
  }

  const flashCopied = async (key: string, text: string) => {
    if (!(await copyText(text))) return
    setCopied(key)
    if (copiedTimer.current) window.clearTimeout(copiedTimer.current)
    copiedTimer.current = window.setTimeout(() => setCopied(null), 1500)
  }
  useEffect(() => () => { if (copiedTimer.current) window.clearTimeout(copiedTimer.current) }, [])

  const synced = (view?.synced_at ?? 0) > 0
  const rows = view?.rows ?? []
  const commands = view?.commands ?? []
  const hasServerRows = rows.some((r) => r.observed)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="shrink-0 px-6 pb-3 pt-5">
        <div className="flex items-center gap-2">
          <ShieldBan size={18} style={{ color: 'var(--color-primary)' }} />
          <h1 className="text-lg font-semibold" style={{ color: 'var(--color-foreground)' }}>
            Blocked Buffs
          </h1>
        </div>
        <p className="mt-1 max-w-3xl text-xs" style={{ color: 'var(--color-muted)' }}>
          Refuse specific beneficial buffs that other players cast on you. The server keeps this list
          itself, so PQC can’t write it directly: build your list here, then run the commands below
          in game. Self-casts, GMs and debuffs are never blocked.
        </p>
      </div>

      <CharacterTabs value={viewed} onChange={setViewed} characters={names} active={active} />

      <div className="min-h-0 flex-1 space-y-5 overflow-y-auto px-6 py-4">
        {error && (
          <p className="rounded px-3 py-2 text-xs" style={{ color: 'var(--color-danger, #f87171)', border: '1px solid var(--color-danger, #f87171)' }}>
            {error}
          </p>
        )}

        {loading || (viewed && !view && !error) ? (
          <p className="text-xs" style={{ color: 'var(--color-muted)' }}>Loading…</p>
        ) : !viewed ? (
          <p className="text-xs" style={{ color: 'var(--color-muted)' }}>No characters found yet.</p>
        ) : (
          <>
            {!synced && (
              <div
                className="rounded px-3 py-2 text-xs"
                style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)', color: 'var(--color-muted-foreground)' }}
              >
                PQC hasn’t seen {viewed}’s server list yet. Type{' '}
                <span className="font-mono" style={{ color: 'var(--color-foreground)' }}>#blockbuff</span> in game
                (no arguments) to list your current blocks — PQC reads the reply from your log and shows what’s
                already applied.
              </div>
            )}

            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
                  Block list
                </h2>
                <div className="flex items-center gap-2">
                  {synced && hasServerRows && (
                    <button
                      onClick={() => void adopt()}
                      disabled={busy}
                      className="rounded px-2.5 py-1 text-[11px] disabled:opacity-40"
                      style={{ color: 'var(--color-muted-foreground)', border: '1px solid var(--color-border)' }}
                      title="Replace your list with what the server currently has"
                    >
                      Use server list
                    </button>
                  )}
                  <button
                    onClick={() => setAdding(true)}
                    className="flex items-center gap-1 rounded px-2.5 py-1 text-[11px] font-medium"
                    style={{ backgroundColor: 'var(--color-primary)', color: 'var(--color-primary-foreground, #fff)' }}
                  >
                    <Plus size={12} /> Block a buff
                  </button>
                </div>
              </div>

              {rows.length === 0 ? (
                <p className="rounded px-3 py-6 text-center text-xs" style={{ color: 'var(--color-muted)', border: '1px dashed var(--color-border)' }}>
                  Nothing blocked yet.
                </p>
              ) : (
                <div className="overflow-hidden rounded" style={{ border: '1px solid var(--color-border)' }}>
                  {rows.map((r) => (
                    <BlockRow
                      key={`${r.spell_id}:${r.if_spell_id}`}
                      row={r}
                      busy={busy}
                      onRemove={() => removeEntry(r)}
                      onKeep={() => keepEntry(r)}
                    />
                  ))}
                </div>
              )}
            </section>

            <section className="space-y-2">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
                  Commands to run in game
                </h2>
                {commands.length > 0 && (
                  <button
                    onClick={() => void flashCopied('all', commands.join('\n'))}
                    className="flex items-center gap-1 rounded px-2.5 py-1 text-[11px]"
                    style={{ color: 'var(--color-muted-foreground)', border: '1px solid var(--color-border)' }}
                  >
                    {copied === 'all' ? <Check size={12} /> : <Copy size={12} />} Copy all
                  </button>
                )}
              </div>
              {commands.length === 0 ? (
                <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
                  {rows.length === 0
                    ? 'Add a block above to generate commands.'
                    : 'The server already matches your list. Nothing to run.'}
                </p>
              ) : (
                <>
                  <div className="overflow-hidden rounded" style={{ border: '1px solid var(--color-border)' }}>
                    {commands.map((c) => (
                      <div
                        key={c}
                        className="flex items-center justify-between gap-2 px-3 py-1.5"
                        style={{ borderBottom: '1px solid var(--color-border-subtle, var(--color-border))' }}
                      >
                        <code className="text-xs" style={{ color: 'var(--color-foreground)' }}>{c}</code>
                        <button
                          onClick={() => void flashCopied(c, c)}
                          title="Copy command"
                          style={{ color: 'var(--color-muted-foreground)' }}
                        >
                          {copied === c ? <Check size={13} /> : <Copy size={13} />}
                        </button>
                      </div>
                    ))}
                  </div>
                  <p className="text-[11px]" style={{ color: 'var(--color-muted)' }}>
                    Paste them into the EverQuest chat box one at a time. Once the server replies, PQC
                    picks it up from your log and the status above updates on its own.
                  </p>
                </>
              )}
            </section>
          </>
        )}
      </div>

      {adding && <AddBlockModal onAdd={addEntry} onClose={() => setAdding(false)} />}
    </div>
  )
}

function BlockRow({
  row,
  busy,
  onRemove,
  onKeep,
}: {
  row: BlockedBuffRow
  busy: boolean
  onRemove: () => void
  onKeep: () => void
}): React.ReactElement {
  const info = STATUS_INFO[row.status]
  return (
    <div
      className="flex items-center gap-3 px-3 py-2"
      style={{ borderBottom: '1px solid var(--color-border-subtle, var(--color-border))' }}
    >
      <SpellHoverCard spellId={row.spell_id}>
        <span className="flex min-w-0 flex-1 cursor-default items-center gap-2 text-xs" style={{ color: 'var(--color-foreground)' }}>
          <span className="truncate font-medium">{row.spell_name || `Spell ${row.spell_id}`}</span>
          <span className="font-mono text-[10px]" style={{ color: 'var(--color-muted)' }}>#{row.spell_id}</span>
        </span>
      </SpellHoverCard>
      <span className="w-48 truncate text-[11px]" style={{ color: 'var(--color-muted-foreground)' }}>
        {row.if_spell_id ? `only while ${row.if_spell_name || `#${row.if_spell_id}`}` : 'always'}
      </span>
      <span className="w-36 text-[11px] font-medium" style={{ color: info.color }}>
        {info.label}
      </span>
      {row.desired ? (
        <button onClick={onRemove} disabled={busy} title="Stop blocking" className="disabled:opacity-40" style={{ color: 'var(--color-muted-foreground)' }}>
          <Trash2 size={14} />
        </button>
      ) : (
        <button
          onClick={onKeep}
          disabled={busy}
          title="Add to your list so it stays blocked"
          className="rounded px-2 py-0.5 text-[10px] disabled:opacity-40"
          style={{ color: 'var(--color-muted-foreground)', border: '1px solid var(--color-border)' }}
        >
          Keep
        </button>
      )}
    </div>
  )
}
