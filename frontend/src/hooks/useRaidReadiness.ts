/**
 * useRaidReadiness — shared data/detection logic for every surface that shows
 * the raid composition check: the full RaidCheckPage, the dashboard panel,
 * and the popout overlay window. Centralized here so all three read the same
 * zone-detection and auto-run behavior instead of drifting out of sync.
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { getRaidEncounters, getRaidRoster, checkRaidComp } from '../services/api'
import type { CheckReport, CheckRosterInput, RaidEncounter, RaidRosterSnapshot } from '../types/raid'

// normalizeZone makes zone-name comparisons forgiving across Zeal's short
// names vs the knowledge base's display names (letters+digits only, lowercase).
function normalizeZone(s: string): string {
  return (s ?? '').toLowerCase().replace(/[^a-z0-9]+/g, '')
}

export interface RaidReadinessState {
  encounters: RaidEncounter[]
  // Encounters ordered by proximity to the live Zeal zone: exact zoneidnumber
  // matches first, then normalized-name matches, then the rest in knowledge-
  // base order. Stable sort — ties keep store order, so the dropdown's first
  // option is always the best zone match.
  orderedEncounters: RaidEncounter[]
  roster: RaidRosterSnapshot | null
  selectedId: string
  setSelectedId: (id: string) => void
  report: CheckReport | null
  busy: boolean
  // True while refresh() is in flight — the Refresh button spins on it.
  refreshing: boolean
  error: string
  refresh: () => Promise<void>
  // id defaults to the current selection; manualRoster overrides the live
  // roster for this one check (RaidCheckPage's manual-entry form).
  runCheck: (id?: string, manualRoster?: CheckRosterInput[]) => Promise<void>
}

export function useRaidReadiness(): RaidReadinessState {
  const [encounters, setEncounters] = useState<RaidEncounter[]>([])
  const [roster, setRoster] = useState<RaidRosterSnapshot | null>(null)
  const [selectedId, setSelectedId] = useState<string>('')
  const [report, setReport] = useState<CheckReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')

  const refresh = useCallback(async (): Promise<void> => {
    setRefreshing(true)
    try {
      const [encRes, rosterRes] = await Promise.all([getRaidEncounters(), getRaidRoster()])
      setEncounters(encRes.encounters)
      setRoster(rosterRes)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setRefreshing(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // Encounter ranking: 0 = exact zoneidnumber match with the live roster
  // (what Zeal is currently reporting), 1 = normalized-name match, 2 = no
  // match. Detection and the dropdown ordering share this so the top option
  // is always the best zone match.
  const rank = useCallback(
    (e: RaidEncounter): number => {
      if (!roster) return 2
      if (roster.zone_id && roster.zone_id > 0 && e.zone_id === roster.zone_id) {
        return 0
      }
      const zoneKey = normalizeZone(roster.zone ?? '')
      if (zoneKey && normalizeZone(e.zone) === zoneKey) return 1
      return 2
    },
    [roster],
  )

  const orderedEncounters = useMemo(() => {
    return encounters
      .map((e, i) => ({ e, i }))
      .sort((a, b) => rank(a.e) - rank(b.e) || a.i - b.i)
      .map((x) => x.e)
  }, [encounters, rank])

  // Encounter detection: pick the best zone-matching encounter (same ranking
  // the dropdown displays, so the pre-selection and the first option agree).
  // The dropdown/selection stays authoritative — this only ever proposes a
  // value, never fights a manual pick.
  useEffect(() => {
    if (!roster || orderedEncounters.length === 0) return
    const hit = orderedEncounters.find((e) => rank(e) < 2)
    if (hit) setSelectedId(hit.id)
  }, [roster, orderedEncounters, rank])

  // runCheck takes the encounter id explicitly rather than always reading
  // `selectedId` from closure — the auto-run effect below needs to check
  // against an id it just resolved this tick, before the corresponding
  // setSelectedId has re-rendered the component. selectedRef mirrors the
  // latest id for the WS-triggered re-check below, which fires outside any
  // render and would otherwise close over a stale value.
  const selectedRef = useRef(selectedId)
  selectedRef.current = selectedId

  const runCheck = useCallback(
    async (id: string = selectedRef.current, manualRoster?: CheckRosterInput[]): Promise<void> => {
      if (!id) {
        setError('Pick an encounter first')
        return
      }
      setBusy(true)
      setError('')
      try {
        const rep = await checkRaidComp({
          encounter_id: id,
          roster: manualRoster && manualRoster.length > 0 ? manualRoster : undefined,
        })
        setReport(rep)
      } catch (err) {
        setError(err instanceof Error ? err.message : String(err))
        setReport(null)
      } finally {
        setBusy(false)
      }
    },
    [],
  )

  // Auto-run a first check once data is loaded and an encounter is selectable.
  const autoRan = useRef(false)
  useEffect(() => {
    if (autoRan.current || busy || orderedEncounters.length === 0) return
    const id = selectedId || (orderedEncounters.length === 1 ? orderedEncounters[0].id : '')
    if (!id) return
    autoRan.current = true
    if (!selectedId) setSelectedId(id)
    void runCheck(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orderedEncounters, selectedId])

  // Live refresh: the backend broadcasts raid.roster on every MsgRaid update
  // and on pipe disconnect (no payload — same "trigger, don't carry state"
  // shape as lockouts.snapshot/keyring.snapshot). Re-fetch the roster and,
  // if an encounter is already selected, re-run the check against it so the
  // report tracks the live raid without the user hitting Refresh.
  const handleWsMessage = useCallback(
    (msg: { type: string; data: unknown }) => {
      if (msg.type !== 'raid.roster') return
      void refresh()
      if (selectedRef.current) void runCheck(selectedRef.current)
    },
    [refresh, runCheck],
  )
  useWebSocket(handleWsMessage)

  return { encounters, orderedEncounters, roster, selectedId, setSelectedId, report, busy, refreshing, error, refresh, runCheck }
}
