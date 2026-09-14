/**
 * useRaidReadiness — shared data/check logic for every surface that shows
 * the raid composition check: the full RaidCheckPage, the dashboard panel,
 * and the popout overlay window. Centralized here so all three render the
 * same selection and report instead of drifting out of sync.
 *
 * There is deliberately no zone-based encounter auto-detection: multiple
 * encounters can share a zone (Kael has two), so a live zone can't pick an
 * encounter unambiguously. The user picks, the pick holds until they change
 * it, and it is relayed to every surface via the main process (see
 * pickEncounter / adoptSelection). The roster's live zone is display-only
 * (roster banner), and the backend still stamps it from the Zeal pipe.
 */
import { useCallback, useEffect, useRef, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { getRaidEncounters, getRaidRoster, checkRaidComp } from '../services/api'
import type { CheckReport, CheckRosterInput, RaidEncounter, RaidRosterSnapshot } from '../types/raid'

export interface RaidReadinessState {
  encounters: RaidEncounter[]
  roster: RaidRosterSnapshot | null
  selectedId: string
  // The UI-facing selector (Raid Composition page's dropdown — the only
  // picker). Publishes the pick so the dashboard panel and the popout
  // overlay mirror it.
  pickEncounter: (id: string) => void
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

  // selectedRef mirrors the latest selected id for callbacks that fire
  // outside a render (WS events, refresh) and would otherwise close over a
  // stale value.
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

  // Re-pull encounters + roster, then re-run the check for the current
  // selection so Refresh visibly updates the report too — not just the
  // roster banner.
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
    if (selectedRef.current) void runCheck(selectedRef.current)
  }, [runCheck])

  useEffect(() => {
    void refresh()
  }, [refresh])

  // adoptSelection applies a selection that arrived from another surface
  // (IPC relay or mount-time catch-up) and runs a check. Never re-publishes
  // — the sender already has it.
  const adoptSelection = useCallback(
    (id: string) => {
      if (!id || id === selectedRef.current) return
      setSelectedId(id)
      void runCheck(id)
    },
    [runCheck],
  )

  const pickEncounter = useCallback((id: string) => {
    setSelectedId(id)
    // The check page is the only picker; publish so the dashboard panel and
    // the popout overlay (separate windows, separate hook instances) mirror
    // the selection. Optional — plain-browser runs (smoke tests) have no
    // electron bridge and just stay local.
    void window.electron?.overlay?.setRaidSelection(id)
  }, [])

  // Mirror the Raid Composition page's picks across windows. The sender's
  // own instance ignores the echo (same id); every other surface adopts it.
  // Subscribes once — adoptSelection reads refs, not render state.
  useEffect(() => {
    const off = window.electron?.overlay?.onRaidSelectionChanged(adoptSelection)
    return off
  }, [adoptSelection])

  // Catch up on mount: a popout opened after a pick (or a re-mounted check
  // page) starts from the last selection the main process knows about.
  useEffect(() => {
    void window.electron?.overlay?.getRaidSelection().then(adoptSelection)
  }, [adoptSelection])

  // Auto-run a first check once data is loaded, when the knowledge base has
  // exactly one encounter — no ambiguity, so pre-selecting it is safe. With
  // more than one, the user picks (and the pick syncs everywhere).
  const autoRan = useRef(false)
  useEffect(() => {
    if (autoRan.current || busy || encounters.length !== 1) return
    autoRan.current = true
    setSelectedId(encounters[0].id)
    void runCheck(encounters[0].id)
  }, [encounters, busy, runCheck])

  // Live refresh: the backend broadcasts raid.roster when the roster or zone
  // CHANGES (change-deduped backend-side — Zeal re-sends MsgRaid every tick,
  // so a naive per-envelope broadcast would refetch 10x/sec). No payload —
  // same "trigger, don't carry state" shape as lockouts.snapshot — refresh()
  // re-fetches the roster and re-runs the check for the current selection.
  const handleWsMessage = useCallback(
    (msg: { type: string; data: unknown }) => {
      if (msg.type !== 'raid.roster') return
      void refresh()
    },
    [refresh],
  )
  useWebSocket(handleWsMessage)

  return { encounters, roster, selectedId, pickEncounter, report, busy, refreshing, error, refresh, runCheck }
}
