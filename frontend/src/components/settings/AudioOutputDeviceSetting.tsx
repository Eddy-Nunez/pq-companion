import React, { useEffect, useState } from 'react'
import { Headphones } from 'lucide-react'
import { listOutputDevices, resolveOutputDevice } from '../../lib/audioDevices'
import type { OutputDevice } from '../../lib/audioDevices'
import {
  setAudioOutputDevice,
  speakTextForTest,
  stopTestPlayback,
} from '../../services/audio'

/**
 * AudioOutputDeviceSetting picks which speaker / headset every trigger and
 * alert plays on, independent of the Windows default device — e.g. alerts in
 * the headset with Discord while the game stays on speakers.
 */
export default function AudioOutputDeviceSetting({
  deviceId,
  deviceLabel,
  defaultVoice,
  onChange,
}: {
  deviceId: string
  deviceLabel: string
  defaultVoice: string
  onChange: (deviceId: string, deviceLabel: string) => void
}): React.ReactElement {
  const [devices, setDevices] = useState<OutputDevice[]>([])
  const [testing, setTesting] = useState(false)

  useEffect(() => {
    let cancelled = false
    const load = (): void => {
      listOutputDevices()
        .then((d) => { if (!cancelled) setDevices(d) })
        .catch(() => {})
    }
    load()
    navigator.mediaDevices?.addEventListener?.('devicechange', load)
    return () => {
      cancelled = true
      navigator.mediaDevices?.removeEventListener?.('devicechange', load)
    }
  }, [])

  const resolved = resolveOutputDevice(devices, deviceId, deviceLabel)
  // A saved device that isn't connected right now stays selectable (shown as
  // disconnected) so the choice isn't silently lost while it's unplugged.
  const missing = deviceId !== '' && resolved === '' && devices.length > 0

  const select = (id: string): void => {
    const label =
      devices.find((d) => d.deviceId === id)?.label ?? (id === deviceId ? deviceLabel : '')
    // Apply immediately so Test uses the new device before the prefs poll
    // picks up the saved value.
    setAudioOutputDevice(id)
    onChange(id, label)
  }

  const test = (): void => {
    if (testing) {
      stopTestPlayback()
      setTesting(false)
      return
    }
    setTesting(true)
    speakTextForTest('Trigger alerts will play here.', defaultVoice, 1.0, () => setTesting(false))
  }

  return (
    <div className="mt-4">
      <p className="text-sm" style={{ color: 'var(--color-foreground)' }}>
        Alert Output Device
      </p>
      <p className="text-xs" style={{ color: 'var(--color-muted-foreground)' }}>
        Where trigger sounds and spoken alerts play — e.g. your headset while
        the game stays on speakers. Built-in Windows voices are converted to
        audio first, so a new phrase can lag slightly the first time it&rsquo;s
        spoken. If the device is unplugged, alerts use the system default.
      </p>
      <div className="mt-2 flex items-center gap-2">
        <Headphones size={14} style={{ color: 'var(--color-muted)' }} />
        <select
          value={missing ? deviceId : resolved}
          onChange={(e) => select(e.target.value)}
          className="min-w-0 flex-1 rounded px-2 py-1 text-xs outline-none"
          style={{
            backgroundColor: 'var(--color-surface-2)',
            border: '1px solid var(--color-border)',
            color: 'var(--color-foreground)',
          }}
        >
          <option value="">System default</option>
          {missing && (
            <option value={deviceId}>
              {deviceLabel || 'Saved device'} (disconnected)
            </option>
          )}
          {devices.map((d) => (
            <option key={d.deviceId} value={d.deviceId}>
              {d.label}
            </option>
          ))}
        </select>
        <button
          type="button"
          onClick={test}
          className="shrink-0 rounded px-2 py-1 text-xs"
          style={{
            backgroundColor: 'var(--color-primary)',
            border: '1px solid var(--color-border)',
            color: 'var(--color-background)',
            cursor: 'pointer',
          }}
          title="Play a test alert on the selected device"
        >
          {testing ? 'Stop' : 'Test'}
        </button>
      </div>
    </div>
  )
}
