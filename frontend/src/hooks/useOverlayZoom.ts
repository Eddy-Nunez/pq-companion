import { useCallback, useEffect, useRef } from 'react'
import { getConfig, updateConfig } from '../services/api'
import { useWebSocket, type WsMessage } from './useWebSocket'
import { WSEvent } from '../lib/wsEvents'

// useOverlayZoom scales a popout overlay window by its own zoom factor, keyed
// by canonical overlay name (see lib/overlays.ts).
//
// This deliberately does NOT use Electron/Chromium zoom (applyZoom). Chromium
// keys zoom by origin, so in dev (every window is http://localhost:5173) one
// window's setZoomFactor rescales every other window — overlays fought each
// other (and the main window) and the last IPC to land won, which is why the
// Settings sliders "only worked sometimes". Instead the factor is applied as
// CSS `zoom` on this document's <html>, which is per-window by construction.
//
// Any Chromium zoom that still reaches this window (the main window's App Zoom
// leaking in through the shared origin, or a per-URL level persisted by older
// builds) is divided back out, so the overlay always renders at exactly its
// own factor. A Chromium zoom change resizes the viewport in CSS px, so the
// window `resize` event is the cue to recompute.
//
// Overlay roots must size with `100%`, not `100vw`/`100vh`: viewport units are
// multiplied by CSS zoom and would overflow the window.
//
// Refreshes on every `config:updated` WebSocket event so the Settings sliders
// preview live — the same approach as useTimerAppearance. An empty key (the
// trigger overlay, which has no slider) leaves the window untouched.
//
// Ctrl+scroll over the overlay steps its factor (macOS trackpad pinch arrives
// as the same ctrl+wheel event). The new size renders immediately and is saved
// on a short debounce, so the Settings slider and other windows follow.
export const OVERLAY_ZOOM_MIN = 0.5
export const OVERLAY_ZOOM_MAX = 2
const WHEEL_STEP = 0.05
const SAVE_DEBOUNCE_MS = 400

const clampFactor = (f: number): number =>
  Math.round(Math.max(OVERLAY_ZOOM_MIN, Math.min(OVERLAY_ZOOM_MAX, f)) * 100) / 100

export function useOverlayZoom(overlayKey: string): void {
  const factorRef = useRef(1)
  // Set while a Ctrl+scroll change is waiting to be saved, so a config refresh
  // landing in that window doesn't snap the overlay back to the old value.
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const render = useCallback(() => {
    if (!overlayKey) return
    const chromium = window.electron?.window?.getZoom?.() || 1
    const css = factorRef.current / chromium
    document.documentElement.style.zoom =
      Math.abs(css - 1) < 0.001 ? '' : String(css)
  }, [overlayKey])

  const apply = useCallback(() => {
    if (!overlayKey) return
    getConfig()
      .then((c) => {
        if (saveTimerRef.current) return
        const f = c.preferences.overlay_zoom_factors?.[overlayKey] ?? 1
        factorRef.current = f > 0 ? clampFactor(f) : 1
        render()
      })
      .catch(() => {})
  }, [overlayKey, render])

  useEffect(() => {
    apply()
  }, [apply])

  useEffect(() => {
    if (!overlayKey) return
    window.addEventListener('resize', render)
    return () => window.removeEventListener('resize', render)
  }, [overlayKey, render])

  useEffect(() => {
    if (!overlayKey) return
    const save = (): void => {
      const token = saveTimerRef.current
      // Read-modify-write so this only touches this overlay's own entry.
      getConfig()
        .then((c) =>
          updateConfig({
            ...c,
            preferences: {
              ...c.preferences,
              overlay_zoom_factors: {
                ...(c.preferences.overlay_zoom_factors ?? {}),
                [overlayKey]: factorRef.current,
              },
            },
          }),
        )
        .catch(() => {})
        .finally(() => {
          // A newer scroll may have queued another save meanwhile; leave its
          // marker in place.
          if (saveTimerRef.current === token) saveTimerRef.current = null
        })
    }
    const onWheel = (e: WheelEvent): void => {
      if (!e.ctrlKey || e.deltaY === 0) return
      // passive:false so this also stops Chromium's own ctrl+wheel handling.
      e.preventDefault()
      const next = clampFactor(
        factorRef.current + (e.deltaY < 0 ? WHEEL_STEP : -WHEEL_STEP),
      )
      if (next === factorRef.current) return
      factorRef.current = next
      render()
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current)
      saveTimerRef.current = setTimeout(save, SAVE_DEBOUNCE_MS)
    }
    window.addEventListener('wheel', onWheel, { passive: false })
    return () => {
      window.removeEventListener('wheel', onWheel)
      // Flush a pending change rather than drop it if the window goes away.
      if (saveTimerRef.current) {
        clearTimeout(saveTimerRef.current)
        save()
      }
    }
  }, [overlayKey, render])

  const handle = useCallback(
    (msg: WsMessage) => {
      if (msg.type !== WSEvent.ConfigUpdated) return
      apply()
    },
    [apply],
  )
  useWebSocket(handle)
}
