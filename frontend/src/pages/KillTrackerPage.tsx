import React from 'react'
import { Crosshair, ListOrdered, ScrollText } from 'lucide-react'
import { usePersistentState } from '../hooks/usePersistentState'
import KillLogTab from '../components/kills/KillLogTab'
import KillCountersTab from '../components/kills/KillCountersTab'

type Tab = 'log' | 'counters'
const TABS: readonly Tab[] = ['log', 'counters']

// Kill Tracker — two views over the same kill stream: the Kill Log (every
// kill the log shows, sortable by time/mob/zone) and Kill Counters (counts
// toward event milestones, e.g. the Bertoxxulous preset).
export default function KillTrackerPage(): React.ReactElement {
  const [tab, setTab] = usePersistentState<Tab>('pq-kill-tracker-tab', 'log', TABS)

  const tabBtn = (key: Tab, label: string, icon: React.ReactNode): React.ReactElement => {
    const active = tab === key
    return (
      <button
        onClick={() => setTab(key)}
        className="flex items-center gap-1.5 rounded-t px-3 py-1.5 text-xs font-medium whitespace-nowrap"
        style={{
          backgroundColor: active ? 'var(--color-surface)' : 'transparent',
          color: active ? 'var(--color-primary)' : 'var(--color-muted-foreground)',
          border: '1px solid', borderColor: active ? 'var(--color-border)' : 'transparent',
          borderBottom: active ? '1px solid var(--color-surface)' : '1px solid transparent',
        }}
      >
        {icon}
        {label}
      </button>
    )
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 px-4 pt-3 shrink-0">
        <Crosshair size={18} style={{ color: 'var(--color-primary)' }} />
        <span className="text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>Kill Tracker</span>
      </div>
      <div className="border-b shrink-0" style={{ borderColor: 'var(--color-border)' }}>
        <div className="px-3 pt-2 flex items-end gap-1" style={{ marginBottom: -1 }}>
          {tabBtn('log', 'Kill Log', <ScrollText size={12} />)}
          {tabBtn('counters', 'Kill Counters', <ListOrdered size={12} />)}
        </div>
      </div>
      <div className="flex-1 min-h-0 flex flex-col">
        {tab === 'log' ? <KillLogTab /> : <KillCountersTab />}
      </div>
    </div>
  )
}
