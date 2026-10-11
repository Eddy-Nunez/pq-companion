import { useCallback, useEffect, useMemo, useState } from 'react'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { useLiveRaidRoster } from './useLiveRaidRoster'
import { WSEvent } from '../lib/wsEvents'

/**
 * usePartyRoster — names of the people you're currently grouped/raided with,
 * from the Zeal pipe. Raid roster when in a raid (useLiveRaidRoster, every
 * member regardless of zone), otherwise in-zone groupmates from the
 * player:group_positions feed. The active player is always included.
 *
 * Group frames come with a ~2s heartbeat while grouped and Zeal emits nothing
 * once the group is gone, so a group list not refreshed for GROUP_STALE_MS is
 * dropped rather than shown forever.
 */

const GROUP_STALE_MS = 10_000

export interface PartyRoster {
  names: string[]
  zealConnected: boolean
  source: 'raid' | 'group' | 'solo'
}

export function usePartyRoster(activePlayer: string): PartyRoster {
  const { roster: raid } = useLiveRaidRoster()
  const [group, setGroup] = useState<{ names: string[]; at: number }>({ names: [], at: 0 })
  const [now, setNow] = useState(() => Date.now())

  const handle = useCallback((msg: WsMessage) => {
    if (msg.type !== WSEvent.PlayerGroupPositions) return
    const data = msg.data as { members?: { name?: string }[] } | null
    const names = (data?.members ?? [])
      .map((m) => m.name ?? '')
      .filter((n) => n !== '')
    setGroup({ names, at: Date.now() })
  }, [])
  useWebSocket(handle)

  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 5_000)
    return () => window.clearInterval(id)
  }, [])

  return useMemo(() => {
    const inRaid = !!raid?.in_raid && raid.members.length > 0
    const groupLive = group.names.length > 0 && now - group.at < GROUP_STALE_MS
    let names: string[] = []
    let source: PartyRoster['source'] = 'solo'
    if (inRaid) {
      names = raid!.members.map((m) => m.name)
      source = 'raid'
    } else if (groupLive) {
      names = group.names
      source = 'group'
    }
    if (activePlayer && !names.some((n) => n.toLowerCase() === activePlayer.toLowerCase())) {
      names = [...names, activePlayer]
    }
    return { names, zealConnected: !!raid?.zeal_connected, source }
  }, [raid, group, now, activePlayer])
}
