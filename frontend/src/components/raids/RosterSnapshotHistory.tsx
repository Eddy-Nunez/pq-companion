import React, { useState } from 'react'
import { Check, Eye, History, Pencil, Trash2, X } from 'lucide-react'
import { ConfirmModal } from '../ConfirmModal'
import type { RaidRosterHistoryEntry } from '../../types/raid'

// Saved raid roster snapshots — attendance + group composition history.
// Rows are summaries (no members); "View" loads the full snapshot into the
// Raid Summary dashboard above.

export function formatSnapshotTime(unix: number): string {
  return new Date(unix * 1000).toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

interface Props {
  snapshots: RaidRosterHistoryEntry[]
  viewingId: number | null
  onView: (id: number) => void
  onRename: (id: number, label: string) => Promise<void>
  onDelete: (id: number) => Promise<void>
}

export default function RosterSnapshotHistory({
  snapshots,
  viewingId,
  onView,
  onRename,
  onDelete,
}: Props): React.ReactElement {
  const [editingId, setEditingId] = useState<number | null>(null)
  const [draft, setDraft] = useState('')
  const [pendingDelete, setPendingDelete] = useState<RaidRosterHistoryEntry | null>(null)

  const startEdit = (s: RaidRosterHistoryEntry): void => {
    setEditingId(s.id)
    setDraft(s.label)
  }
  const commitEdit = async (): Promise<void> => {
    if (editingId === null) return
    await onRename(editingId, draft)
    setEditingId(null)
  }

  return (
    <div
      className="rounded-lg flex flex-col shrink-0"
      style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)', maxHeight: '20rem' }}
    >
      <div
        className="px-3 py-2 text-sm font-semibold shrink-0 flex items-center gap-2"
        style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)', color: 'var(--color-foreground)' }}
      >
        <History size={14} /> Snapshot history
        <span className="text-xs font-normal" style={{ color: 'var(--color-muted-foreground)' }}>
          ({snapshots.length})
        </span>
      </div>
      <div className="overflow-y-auto min-h-0">
        {snapshots.length === 0 ? (
          <div className="px-3 py-3 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
            No snapshots yet. Use <span className="font-semibold">Snapshot</span> while in a raid to save
            who attended and how groups were set up.
          </div>
        ) : (
          snapshots.map((s, i) => {
            const viewing = s.id === viewingId
            return (
              <div
                key={s.id}
                className={`grid grid-cols-[170px_1fr_110px_70px_auto] gap-3 items-center px-3 py-1.5 ${i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''}`}
                style={viewing ? { boxShadow: 'inset 3px 0 0 var(--color-primary)' } : undefined}
              >
                <span className="text-sm tabular-nums" style={{ color: 'var(--color-foreground)' }}>
                  {formatSnapshotTime(s.taken_at)}
                </span>
                {editingId === s.id ? (
                  <div className="flex items-center gap-1 min-w-0">
                    <input
                      autoFocus
                      value={draft}
                      maxLength={200}
                      onChange={(e) => setDraft(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') void commitEdit()
                        if (e.key === 'Escape') setEditingId(null)
                      }}
                      placeholder="Label (e.g. Plane of Fear clear)"
                      className="flex-1 min-w-0 px-2 py-0.5 text-sm rounded"
                      style={{ backgroundColor: 'var(--color-surface-2)', border: '1px solid var(--color-border)', color: 'var(--color-foreground)' }}
                    />
                    <IconButton title="Save" onClick={() => void commitEdit()} icon={<Check size={13} />} />
                    <IconButton title="Cancel" onClick={() => setEditingId(null)} icon={<X size={13} />} />
                  </div>
                ) : (
                  <span
                    className="text-sm truncate"
                    style={{ color: s.label ? 'var(--color-foreground)' : 'var(--color-muted-foreground)' }}
                  >
                    {s.label || 'Untitled'}
                  </span>
                )}
                <span className="text-sm truncate" style={{ color: 'var(--color-muted-foreground)' }}>
                  {s.zone || '—'}
                </span>
                <span className="text-sm tabular-nums" style={{ color: 'var(--color-muted-foreground)' }}>
                  {s.member_count} chars
                </span>
                <div className="flex items-center gap-1 justify-self-end">
                  <IconButton
                    title={viewing ? 'Currently viewing' : 'View snapshot'}
                    onClick={() => onView(s.id)}
                    icon={<Eye size={13} />}
                    active={viewing}
                  />
                  <IconButton title="Rename" onClick={() => startEdit(s)} icon={<Pencil size={13} />} />
                  <IconButton title="Delete" onClick={() => setPendingDelete(s)} icon={<Trash2 size={13} />} danger />
                </div>
              </div>
            )
          })
        )}
      </div>
      {pendingDelete && (
        <ConfirmModal
          title="Delete snapshot?"
          message={
            <>
              Delete the snapshot from {formatSnapshotTime(pendingDelete.taken_at)}
              {pendingDelete.label ? ` (${pendingDelete.label})` : ''}? This can&apos;t be undone.
            </>
          }
          confirmLabel="Delete"
          tone="danger"
          onCancel={() => setPendingDelete(null)}
          onConfirm={() => {
            const id = pendingDelete.id
            setPendingDelete(null)
            void onDelete(id)
          }}
        />
      )}
    </div>
  )
}

function IconButton({
  title,
  onClick,
  icon,
  danger,
  active,
}: {
  title: string
  onClick: () => void
  icon: React.ReactNode
  danger?: boolean
  active?: boolean
}): React.ReactElement {
  return (
    <button
      title={title}
      onClick={onClick}
      className="p-1 rounded hover:bg-(--color-surface-2)"
      style={{
        color: active ? 'var(--color-primary)' : danger ? 'var(--color-danger)' : 'var(--color-muted-foreground)',
      }}
    >
      {icon}
    </button>
  )
}
