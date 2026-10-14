import React, { useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, LayoutGrid, List } from 'lucide-react'
import { usePersistentState } from '../../hooks/usePersistentState'
import type { RaidRosterMember, ClassCode } from '../../types/raid'

// Members section of the Raid Summary page: the roster either as one flat
// table or split into per-group cards (Zeal reports group "1".."12", "0" =
// ungrouped). Column headings sort in both views — in group view the sort
// applies within each card and is shared across all of them.

export type MemberSortKey = 'name' | 'level' | 'class' | 'group' | 'rank'
type SortDir = 'asc' | 'desc'
type ViewMode = 'groups' | 'list'

const VIEW_MODES: readonly ViewMode[] = ['groups', 'list']

// Ungrouped members ("0" / missing) sort after group 12.
const UNGROUPED = 99

function groupNumber(m: RaidRosterMember): number {
  const n = parseInt(m.group ?? '', 10)
  return Number.isFinite(n) && n > 0 ? n : UNGROUPED
}

function groupLabel(m: RaidRosterMember): string {
  const n = groupNumber(m)
  return n === UNGROUPED ? 'Ungrouped' : String(n)
}

function rankWeight(m: RaidRosterMember): number {
  if (m.rank === 'Raid Leader') return 0
  if (m.rank === 'Group Leader') return 1
  return 2
}

interface Props {
  members: RaidRosterMember[]
  classOrder: ClassCode[]
  classLabel: (code: ClassCode) => string
}

export default function RaidMembersView({ members, classOrder, classLabel }: Props): React.ReactElement {
  const [view, setView] = usePersistentState<ViewMode>('raidSummary.membersView', 'groups', VIEW_MODES)
  const [sortKey, setSortKey] = useState<MemberSortKey>('class')
  const [sortDir, setSortDir] = useState<SortDir>('asc')

  const onSort = (key: MemberSortKey): void => {
    if (key === sortKey) {
      setSortDir(sortDir === 'asc' ? 'desc' : 'asc')
    } else {
      setSortKey(key)
      // Levels read most naturally highest-first; everything else A→Z / 1→12.
      setSortDir(key === 'level' ? 'desc' : 'asc')
    }
  }

  const sorted = useMemo(() => {
    const classRank = (m: RaidRosterMember): number => {
      const i = m.code ? classOrder.indexOf(m.code as ClassCode) : -1
      return i === -1 ? classOrder.length : i
    }
    const primary = (a: RaidRosterMember, b: RaidRosterMember): number => {
      switch (sortKey) {
        case 'name':
          return a.name.localeCompare(b.name)
        case 'level':
          return (a.level ?? 0) - (b.level ?? 0)
        case 'class':
          return classRank(a) - classRank(b)
        case 'group':
          return groupNumber(a) - groupNumber(b)
        case 'rank':
          return rankWeight(a) - rankWeight(b)
      }
    }
    const dir = sortDir === 'asc' ? 1 : -1
    return [...members].sort(
      (a, b) => dir * primary(a, b) || classRank(a) - classRank(b) || a.name.localeCompare(b.name),
    )
  }, [members, sortKey, sortDir, classOrder])

  const groups = useMemo(() => {
    const byGroup = new Map<number, RaidRosterMember[]>()
    for (const m of sorted) {
      const g = groupNumber(m)
      const list = byGroup.get(g)
      if (list) list.push(m)
      else byGroup.set(g, [m])
    }
    return [...byGroup.entries()].sort((a, b) => a[0] - b[0])
  }, [sorted])

  return (
    <div
      className="rounded-lg flex flex-col shrink-0"
      style={{
        border: '1px solid var(--color-border)',
        backgroundColor: 'var(--color-surface)',
        // A full 72-member raid scrolls within its own box in list view; group
        // cards lay out in a grid and just extend the page.
        maxHeight: view === 'list' ? '28rem' : undefined,
      }}
    >
      <div
        className="px-3 py-2 flex items-center gap-2 shrink-0"
        style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
      >
        <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
          Members
        </span>
        <div className="ml-auto flex items-center rounded overflow-hidden" style={{ border: '1px solid var(--color-border)' }}>
          <ViewButton active={view === 'groups'} onClick={() => setView('groups')} icon={<LayoutGrid size={13} />} label="Groups" />
          <ViewButton active={view === 'list'} onClick={() => setView('list')} icon={<List size={13} />} label="List" />
        </div>
      </div>

      {members.length === 0 ? (
        <div className="px-3 py-3 text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
          No members yet.
        </div>
      ) : view === 'list' ? (
        <div className="overflow-y-auto min-h-0">
          <MemberTable
            members={sorted}
            showGroup
            sortKey={sortKey}
            sortDir={sortDir}
            onSort={onSort}
            classLabel={classLabel}
            stickyHeader
          />
        </div>
      ) : (
        <div className="grid gap-2 p-2" style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))' }}>
          {groups.map(([g, list]) => (
            <div
              key={g}
              className="rounded-md flex flex-col"
              style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}
            >
              <div
                className="px-3 py-1.5 flex items-center justify-between"
                style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
              >
                <span className="text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--color-primary)' }}>
                  {g === UNGROUPED ? 'Ungrouped' : `Group ${g}`}
                </span>
                <span className="text-xs tabular-nums" style={{ color: 'var(--color-muted-foreground)' }}>
                  {g === UNGROUPED ? list.length : `${list.length}/6`}
                </span>
              </div>
              <MemberTable
                members={list}
                showGroup={false}
                sortKey={sortKey}
                sortDir={sortDir}
                onSort={onSort}
                classLabel={classLabel}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function ViewButton({
  active,
  onClick,
  icon,
  label,
}: {
  active: boolean
  onClick: () => void
  icon: React.ReactNode
  label: string
}): React.ReactElement {
  return (
    <button
      onClick={onClick}
      className="flex items-center gap-1 px-2 py-1 text-xs"
      style={{
        backgroundColor: active ? 'var(--color-primary)' : 'transparent',
        color: active ? 'var(--color-primary-foreground)' : 'var(--color-muted-foreground)',
      }}
    >
      {icon}
      {label}
    </button>
  )
}

interface TableProps {
  members: RaidRosterMember[]
  showGroup: boolean
  sortKey: MemberSortKey
  sortDir: SortDir
  onSort: (key: MemberSortKey) => void
  classLabel: (code: ClassCode) => string
  stickyHeader?: boolean
}

function MemberTable({
  members,
  showGroup,
  sortKey,
  sortDir,
  onSort,
  classLabel,
  stickyHeader,
}: TableProps): React.ReactElement {
  const cols = showGroup ? 'grid-cols-[1.4fr_60px_1fr_1fr_1fr]' : 'grid-cols-[1.4fr_44px_1fr_1fr]'
  const header = (key: MemberSortKey, label: string, alignEnd = false): React.ReactElement => {
    const active = key === sortKey
    return (
      <button
        onClick={() => onSort(key)}
        className={`flex items-center gap-0.5 uppercase tracking-wide hover:text-(--color-foreground) ${alignEnd ? 'justify-self-end' : ''}`}
        style={{ color: active ? 'var(--color-primary)' : undefined }}
      >
        {label}
        {active && (sortDir === 'asc' ? <ArrowUp size={10} /> : <ArrowDown size={10} />)}
      </button>
    )
  }
  return (
    <>
      <div
        className={`grid ${cols} gap-3 px-3 py-1 text-[11px] ${stickyHeader ? 'sticky top-0' : ''}`}
        style={{ color: 'var(--color-muted-foreground)', backgroundColor: 'var(--color-surface)' }}
      >
        {header('name', 'Name')}
        {header('level', 'Level', true)}
        {header('class', 'Class')}
        {showGroup && header('group', 'Group')}
        {header('rank', 'Rank')}
      </div>
      {members.map((m, i) => (
        <div
          key={`${m.name}-${i}`}
          className={`grid ${cols} gap-3 items-center px-3 py-1.5 ${i % 2 === 1 ? 'bg-(--color-surface-2)/50' : ''}`}
        >
          <span className="text-sm truncate" style={{ color: 'var(--color-foreground)' }}>{m.name}</span>
          <span className="text-sm tabular-nums justify-self-end" style={{ color: 'var(--color-muted-foreground)' }}>{m.level ?? '—'}</span>
          <span className="text-sm truncate" style={{ color: m.code ? 'var(--color-foreground)' : 'var(--color-danger)' }}>
            {m.code ? classLabel(m.code as ClassCode) : 'Unknown'}
          </span>
          {showGroup && (
            <span className="text-sm truncate" style={{ color: 'var(--color-muted-foreground)' }}>{groupLabel(m)}</span>
          )}
          <span className="text-sm truncate" style={{ color: 'var(--color-muted-foreground)' }}>{m.rank || '—'}</span>
        </div>
      ))}
    </>
  )
}
