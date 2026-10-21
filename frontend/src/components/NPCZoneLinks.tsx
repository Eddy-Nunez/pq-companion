import React from 'react'
import { useNavigate } from 'react-router-dom'
import type { NPCZone } from '../types/npc'
import type { ItemSourceNPC } from '../types/item'

const INFERRED_TITLE =
  'No fixed spawn point (script/event spawned) — zone inferred from the NPC id'

// sourceNPCZones returns a source NPC's zones, falling back to its single
// zone_name/zone_short_name pair for responses that predate the zones list.
export function sourceNPCZones(npc: ItemSourceNPC): NPCZone[] {
  if (npc.zones && npc.zones.length > 0) return npc.zones
  if (!npc.zone_name) return []
  return [{ short_name: npc.zone_short_name, long_name: npc.zone_name }]
}

// zoneListLabel is the plain-text form ("A, B +2 more") for places that can't
// hold links, such as inside a list-row button.
export function zoneListLabel(zones: NPCZone[], max = 2): string {
  const shown = zones.slice(0, max).map((z) => z.long_name).join(', ')
  const extra = zones.length - max
  return extra > 0 ? `${shown} +${extra} more` : shown
}

// NPCZoneLinks renders an NPC's zones as links to the Zones page. Past `max`
// zones the rest collapse into a "+N more" with the full list on hover.
export function NPCZoneLinks({
  zones,
  max = 2,
  className = 'text-xs',
}: {
  zones: NPCZone[]
  max?: number
  className?: string
}): React.ReactElement | null {
  const navigate = useNavigate()
  if (zones.length === 0) return null
  const shown = zones.slice(0, max)
  const rest = zones.slice(max)
  return (
    <span className={`min-w-0 ${className}`} style={{ color: 'var(--color-muted)' }}>
      {shown.map((z, i) => (
        <React.Fragment key={z.short_name}>
          {i > 0 && ', '}
          <button
            onClick={() => navigate(`/zones?select=${z.short_name}`)}
            className="underline decoration-dotted"
            style={{ color: 'inherit' }}
            title={z.inferred ? INFERRED_TITLE : undefined}
          >
            {z.long_name}
            {z.inferred && '*'}
          </button>
        </React.Fragment>
      ))}
      {rest.length > 0 && (
        <span
          className="ml-1 whitespace-nowrap"
          title={rest.map((z) => z.long_name).join('\n')}
        >
          +{rest.length} more
        </span>
      )}
    </span>
  )
}
