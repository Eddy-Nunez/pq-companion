import { useEffect } from 'react'
import { getConfig } from '../services/api'
import {
  setMasterVolume,
  setDefaultTTSVoice,
  setRepeatAudioCooldownMs,
  setTTSRate,
  setAudioOutputDevice,
} from '../services/audio'
import { listOutputDevices, resolveOutputDevice } from '../lib/audioDevices'

const POLL_INTERVAL = 3000

/**
 * Polls the user config and pushes the audio-related preferences into the
 * audio service so subsequent playSound / speakText calls (and their test
 * variants) pick them up:
 *   - Preferences.master_volume scales every playback on top of each
 *     action's per-trigger volume.
 *   - Preferences.default_tts_voice is the voice used by any TTS alert
 *     whose own voice field is empty ("App default").
 *   - Preferences.tts_rate sets the global speaking rate for every TTS
 *     alert (1.0 = normal, higher = faster).
 *   - Preferences.trigger_audio_cooldown_secs rate-limits repeat audio from
 *     the same trigger (consumed in useAudioEngine). 0 = off.
 *   - Preferences.audio_output_device_id / _label pick the output device
 *     for every alert. Re-resolved on Windows' devicechange so unplugging the
 *     headset falls back to the default device and plugging it back in
 *     restores the route.
 *
 * Mounted only inside MainWindowLayout — overlay windows don't fire alerts.
 */
export function useAudioPrefs(): void {
  useEffect(() => {
    let cancelled = false
    let wantedId = ''
    let wantedLabel = ''
    const applyDevice = (): void => {
      if (!wantedId) {
        setAudioOutputDevice('')
        return
      }
      listOutputDevices()
        .then((devices) => {
          if (!cancelled) setAudioOutputDevice(resolveOutputDevice(devices, wantedId, wantedLabel))
        })
        .catch(() => {})
    }
    navigator.mediaDevices?.addEventListener?.('devicechange', applyDevice)
    const fetch = (): void => {
      getConfig()
        .then((c) => {
          if (cancelled) return
          const pct = c.preferences?.master_volume
          // Treat missing/invalid as 100% so a transient backend hiccup
          // doesn't silently mute alerts.
          const value = typeof pct === 'number' && Number.isFinite(pct) ? pct : 100
          setMasterVolume(value / 100)
          setDefaultTTSVoice(c.preferences?.default_tts_voice ?? '')
          const rate = c.preferences?.tts_rate
          // Missing/invalid/zero → normal speed (1.0), not the 0.5 clamp floor.
          setTTSRate(typeof rate === 'number' && rate > 0 ? rate : 1.0)
          const cd = c.preferences?.trigger_audio_cooldown_secs
          setRepeatAudioCooldownMs(
            typeof cd === 'number' && Number.isFinite(cd) ? cd * 1000 : 0,
          )
          const id = c.preferences?.audio_output_device_id ?? ''
          const label = c.preferences?.audio_output_device_label ?? ''
          if (id !== wantedId || label !== wantedLabel) {
            wantedId = id
            wantedLabel = label
            applyDevice()
          }
        })
        .catch(() => {})
    }
    fetch()
    const id = setInterval(fetch, POLL_INTERVAL)
    return () => {
      cancelled = true
      clearInterval(id)
      navigator.mediaDevices?.removeEventListener?.('devicechange', applyDevice)
    }
  }, [])
}
