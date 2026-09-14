import React from 'react'
import { Radio } from 'lucide-react'
import type { RaidRosterSnapshot } from '../../types/raid'

// RosterStatusBanner reports the Zeal pipe / roster state the same way on
// every raid-composition surface (Raid Summary, Raid Composition Check):
// pipe not connected, connected-but-not-in-a-raid, or a live member count.

function StampTime({ ts }: { ts?: number }): React.ReactElement {
  if (!ts) return <span>—</span>
  return <span>{new Date(ts * 1000).toLocaleTimeString()}</span>
}

export default function RosterStatusBanner({
  roster,
  extraHint,
}: {
  roster: RaidRosterSnapshot | null
  // Extra sentence appended to the disconnected/not-in-raid states — the
  // Composition Check page points at its manual roster form, Raid Summary
  // has none.
  extraHint?: React.ReactNode
}): React.ReactElement {
  const liveCount = roster?.members.length ?? 0
  return (
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
              running, then hit <b>Refresh</b>.{extraHint ? <> {extraHint}</> : null}
            </span>
          </>
        ) : roster && !roster.in_raid ? (
          <>
            <Radio size={15} style={{ color: 'var(--color-primary)' }} />
            <span style={{ color: 'var(--color-foreground)' }}>
              Zeal is active but you&apos;re not in a raid.{extraHint ? <> {extraHint}</> : null}
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
  )
}
