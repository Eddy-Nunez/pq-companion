// Audio output device helpers for the "Alert output device" preference.
// Chromium lists output devices via enumerateDevices; 'default' (whatever
// Windows currently uses) is dropped since the app's own "System default"
// option already means that.

export interface OutputDevice {
  deviceId: string
  label: string
}

/** Lists the available audio output devices (empty when unsupported). */
export async function listOutputDevices(): Promise<OutputDevice[]> {
  const md = navigator.mediaDevices
  if (!md?.enumerateDevices) return []
  try {
    const all = await md.enumerateDevices()
    return all
      .filter((d) => d.kind === 'audiooutput' && d.deviceId && d.deviceId !== 'default')
      .map((d, i) => ({ deviceId: d.deviceId, label: d.label || `Output device ${i + 1}` }))
  } catch {
    return []
  }
}

/**
 * Resolves the saved device to a currently-present deviceId: by id first,
 * then by name (ids can change, e.g. after a driver reinstall). Returns ''
 * when the device isn't connected, so playback uses the system default
 * rather than going silent.
 */
export function resolveOutputDevice(
  devices: OutputDevice[],
  id: string,
  label: string,
): string {
  if (!id) return ''
  const hit =
    devices.find((d) => d.deviceId === id) ??
    (label ? devices.find((d) => d.label === label) : undefined)
  return hit ? hit.deviceId : ''
}
