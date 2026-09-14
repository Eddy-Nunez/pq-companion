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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useWebSocket } from './useWebSocket'
import { usePlayerPosition } from './usePlayerPosition'
import { getRaidEncounters, getRaidRoster, checkRaidComp, getConfig } from '../services/api'
import type { CheckReport, CheckRosterInput, RaidEncounter, RaidRosterSnapshot } from '../types/raid'
import type { Config } from '../types/config'

// normalizeZone makes zone-name comparisons forgiving across Zeal's short
// names vs the knowledge base's display names (letters+digits only, lowercase).
function normalizeZone(s: string): string {
  return (s ?? '').toLowerCase().replace(/[^a-z0-9]+/g, '')
}

export interface RaidReadinessState {
  encounters: RaidEncounter[]
  // Encounters ordered by proximity to the live Zeal zone: exact zoneidnumber
  // matches first, then normalized-name matches, then the rest in knowledge-
  // base order. Stable sort — ties keep store order. ORDERING ONLY: detection
  // was deliberately removed (multiple encounters can share a zone, and it
  // clobbered manual picks) — the dropdown's first option is just the best
  // zone match; nothing auto-selects.
  orderedEncounters: RaidEncounter[]
  roster: RaidRosterSnapshot | null
  selectedId: string
  // The UI-facing selector (Raid Composition page's dropdown — the only
  // picker). Publishes the pick so the dashboard panel and the popout
  // overlay mirror it, and runs the check immediately.
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

  // Encounter ranking: 0 = exact zoneidnumber match with the live roster
  // zone (what Zeal is currently reporting), 1 = normalized-name match,
  // 2 = no match. The dropdown displays this ordering; nothing auto-selects
  // from it — the user's pick stays authoritative.
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

  const orderedEncounters = useMemo(
    () =>
      encounters
        .map((e, i) => ({ e, i }))
        .sort((a, b) => rank(a.e) - rank(b.e) || a.i - b.i)
        .map((x) => x.e),
    [encounters, rank],
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

  // Live-zone change → reload. The backend heartbeats player:position (2s)
  // while the Zeal pipe is connected, so pos.zone tracks the character across
  // zoning. On a change, refresh() re-pulls encounters + roster and re-runs
  // the check — the dropdown re-sorts to put current-zone encounters on top
  // and the report follows the new zone context. Gated behind raids_enabled
  // (the flag that surfaces the raid UI at all): the hook can be mounted with
  // the flag off via a direct URL, and there's no reason to churn fetches on
  // every zoning for a feature that's switched off.
  const pos = usePlayerPosition()
  const [raidsEnabled, setRaidsEnabled] = useState(false)
  useEffect(() => {
    void getConfig()
      .then((c: Config) => setRaidsEnabled(Boolean(c.preferences?.raids_enabled)))
      .catch(() => setRaidsEnabled(false))
  }, [])
  const lastZoneRef = useRef<string | null>(null)
  useEffect(() => {
    const zone = pos?.zone ?? null
    const prev = lastZoneRef.current
    lastZoneRef.current = zone
    if (!raidsEnabled || !zone || prev === null || prev === zone) return
    void refresh()
  }, [pos?.zone, raidsEnabled, refresh])

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

  // The check page's dropdown is the only picker. Selecting sets state,
  // runs the check right away (this surface doesn't wait for an echo of its
  // own pick), and publishes so the dashboard panel and the popout overlay
  // (separate windows, separate hook instances) mirror the selection.
  // window.electron is optional — plain-browser runs (smoke tests) have no
  // electron bridge and just stay local.
  const pickEncounter = useCallback(
    (id: string) => {
      setSelectedId(id)
      void runCheck(id)
      void window.electron?.overlay?.setRaidSelection(id)
    },
    [runCheck],
  )

  // Mirror the Raid Composition page's picks across windows. The sender's
  // own instance already ran the check via pickEncounter above and ignores
  // the echo (same id); every other surface adopts it.
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

  return { encounters, orderedEncounters, roster, selectedId, pickEncounter, report, busy, refreshing, error, refresh, runCheck }
}
