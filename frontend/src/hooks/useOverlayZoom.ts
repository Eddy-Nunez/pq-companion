import { useCallback, useEffect, useRef } from 'react'
import { getConfig } from '../services/api'
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
export function useOverlayZoom(overlayKey: string): void {
  const factorRef = useRef(1)

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
        const f = c.preferences.overlay_zoom_factors?.[overlayKey] ?? 1
        factorRef.current = f > 0 ? Math.max(0.5, Math.min(2, f)) : 1
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

  const handle = useCallback(
    (msg: WsMessage) => {
      if (msg.type !== WSEvent.ConfigUpdated) return
      apply()
    },
    [apply],
  )
  useWebSocket(handle)
}
