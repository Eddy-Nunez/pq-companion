/**
 * useRaidReadiness — shared data/detection logic for every surface that shows
 * the raid composition check: the full RaidCheckPage, the dashboard panel,
 * and the popout overlay window. Centralized here so all three read the same
 * zone-detection and auto-run behavior instead of drifting out of sync.
 */
import { useCallback, useEffect, useRef, useState } from 'react'
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
  roster: RaidRosterSnapshot | null
  selectedId: string
  setSelectedId: (id: string) => void
  detectedZone: string
  report: CheckReport | null
  busy: boolean
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
  const [detectedZone, setDetectedZone] = useState<string>('')
  const [report, setReport] = useState<CheckReport | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const refresh = useCallback(async (): Promise<void> => {
    try {
      const [encRes, rosterRes] = await Promise.all([getRaidEncounters(), getRaidRoster()])
      setEncounters(encRes.encounters)
      setRoster(rosterRes)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // Encounter detection: prefer an exact zoneidnumber match with the live
  // roster (what Zeal reports); fall back to normalized-name comparison for
  // encounters without a resolved zone id. The dropdown/selection stays
  // authoritative — this only ever proposes a value, never fights a manual pick.
  useEffect(() => {
    if (!roster || encounters.length === 0) return
    if (roster.zone_id && roster.zone_id > 0) {
      const hit = encounters.find((e) => e.zone_id === roster.zone_id)
      if (hit) {
        setSelectedId(hit.id)
        setDetectedZone(roster.zone ?? String(roster.zone_id))
        return
      }
    }
    if (roster.zone === undefined) return
    const zoneKey = normalizeZone(roster.zone)
    if (!zoneKey) return
    const hit = encounters.find((e) => normalizeZone(e.zone) === zoneKey)
    if (hit) {
      setSelectedId(hit.id)
      setDetectedZone(roster.zone)
    }
  }, [roster, encounters])

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

  // Auto-run against the live Zeal roster whenever the selected encounter
  // changes (including the very first selection) or the encounter/roster
  // data is reloaded — no manual "Check composition" click required. The
  // Zeal pipe already tells the app the raid's composition and members, so
  // the checker should just reflect that live, the same way the NPC overlay
  // reflects the live target. The explicit "Check composition" button (in
  // RaidCheckPage) stays only for applying a manually-entered roster, which
  // this auto-run intentionally doesn't touch.
  useEffect(() => {
    if (busy || encounters.length === 0) return
    const id = selectedId || (encounters.length === 1 ? encounters[0].id : '')
    if (!id) return
    if (!selectedId) setSelectedId(id)
    void runCheck(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [encounters, selectedId])

  // Live refresh: the backend broadcasts raid.roster on every MsgRaid update
  // and on pipe disconnect (no payload — same "trigger, don't carry state"
  // shape as lockouts.snapshot/keyring.snapshot). Re-fetching here updates
  // `encounters`/`roster` state, which retriggers the auto-run effect above
  // so the report tracks the live raid without the user hitting Refresh.
  const handleWsMessage = useCallback(
    (msg: { type: string; data: unknown }) => {
      if (msg.type !== 'raid.roster') return
      void refresh()
    },
    [refresh],
  )
  useWebSocket(handleWsMessage)

  return { encounters, roster, selectedId, setSelectedId, detectedZone, report, busy, error, refresh, runCheck }
}
