/**
 * useLiveRaidRoster — the raw Zeal raid roster snapshot, kept live over the
 * WebSocket. Split out from useRaidReadiness (which also drives the
 * MIN/REC composition checker) so surfaces that only need "who's in the
 * raid right now" — like the Raid Summary dashboard — don't pull in
 * encounter/check state they don't use.
 */
import { useCallback, useEffect, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { getRaidRoster } from '../services/api'
import type { RaidRosterSnapshot } from '../types/raid'

export interface LiveRaidRosterState {
  roster: RaidRosterSnapshot | null
  error: string
  refresh: () => Promise<void>
}

export function useLiveRaidRoster(): LiveRaidRosterState {
  const [roster, setRoster] = useState<RaidRosterSnapshot | null>(null)
  const [error, setError] = useState('')

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const res = await getRaidRoster()
      setRoster(res)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // The backend broadcasts raid.roster on every MsgRaid update and on pipe
  // disconnect (no payload — same "trigger, don't carry state" shape as
  // lockouts.snapshot/keyring.snapshot).
  const handleWsMessage = useCallback(
    (msg: { type: string; data: unknown }) => {
      if (msg.type !== 'raid.roster') return
      void refresh()
    },
    [refresh],
  )
  useWebSocket(handleWsMessage)

  return { roster, error, refresh }
}
