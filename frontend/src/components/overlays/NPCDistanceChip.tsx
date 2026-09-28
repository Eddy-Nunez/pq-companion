import React from 'react'
import type { NPCTargetDistance } from '../../hooks/useNPCTargetDistance'

// Range colours, fixed rather than theme tokens: the floating overlay window
// can't rely on the app's theme stylesheet (see NPCCasterSummarySection).
const IN_RANGE = '#4ade80'
const EDGE = '#facc15'
const OUT_OF_RANGE = '#f87171'
const MUTED = 'rgba(255,255,255,0.45)'

// The distance can move a few units between the server's check and ours, and
// model size also counts server-side, so the last 10% of a range is "edge".
const EDGE_FRACTION = 0.9

export function rangeColor(distance: number, range: number): string {
  if (distance > range) return OUT_OF_RANGE
  if (distance > range * EDGE_FRACTION) return EDGE
  return IN_RANGE
}

// inReach reports whether a spell with the given reach could land on a player
// standing `distance` away. False when either is unknown.
export function inReach(
  reach: number | undefined,
  distance: number | null | undefined,
): boolean {
  return reach != null && reach > 0 && distance != null && distance <= reach
}

function rangeLine(label: string, distance: number, range: number): string {
  const verdict =
    distance > range
      ? 'out of range'
      : distance > range * EDGE_FRACTION
        ? 'at the edge'
        : 'in range'
  return `${label} ${range}: ${verdict}`
}

// NPCDistanceChip shows the live player→target distance, coloured against the
// user's configured cast range. Renders nothing when this Zeal build doesn't
// report target info, and "n/a" when Zeal withheld the position (250+ away).
export default function NPCDistanceChip({
  dist,
}: {
  dist: NPCTargetDistance
}): React.ReactElement | null {
  if (!dist.has_descriptors) return null

  const base: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'baseline',
    gap: 4,
    borderRadius: 3,
    padding: '1px 6px',
    fontSize: 10,
    fontWeight: 600,
    backgroundColor: 'rgba(255,255,255,0.08)',
    fontVariantNumeric: 'tabular-nums',
  }

  if (dist.distance == null) {
    return (
      <span
        style={{ ...base, color: MUTED }}
        title="Target is 250+ units away. Zeal only reports a target's position within 250 units."
      >
        Dist n/a
      </span>
    )
  }

  const d = dist.distance
  const lines = [`Distance to target: ${d}`, rangeLine('Cast range', d, dist.castRange)]
  if (dist.rangedRange > 0) lines.push(rangeLine('Ranged', d, dist.rangedRange))
  lines.push(
    'Approximate: the server also counts model size, and range-extending ' +
      'focus effects and AAs are only included if you entered them in ' +
      'Settings.',
  )

  return (
    <span style={{ ...base, color: rangeColor(d, dist.castRange) }} title={lines.join('\n')}>
      Dist {d}
      {dist.rangedRange > 0 && (
        <span style={{ color: rangeColor(d, dist.rangedRange), fontWeight: 500 }}>
          · rng
        </span>
      )}
    </span>
  )
}
