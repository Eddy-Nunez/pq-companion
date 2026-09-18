/**
 * Formats a countdown in seconds for the buff/detrimental/target timer
 * overlays. Default is rounded-up minutes-only past 60s ("5m"); pass
 * showSeconds to get an "Mm Ss" breakdown ("4m 32s") instead.
 */
export function fmtRemaining(secs: number, showSeconds = false): string {
  if (secs <= 0) return '0s'
  if (secs < 60) return `${Math.ceil(secs)}s`
  if (!showSeconds) return `${Math.ceil(secs / 60)}m`
  const total = Math.ceil(secs)
  const m = Math.floor(total / 60)
  const s = total % 60
  return s > 0 ? `${m}m ${s}s` : `${m}m`
}
