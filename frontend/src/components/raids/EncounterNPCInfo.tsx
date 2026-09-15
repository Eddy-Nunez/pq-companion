import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { Skull, Database } from 'lucide-react'
import { getNPC, getNPCSpells } from '../../services/api'
import { className, bodyTypeName, npcLevelLabel, npcRunSpeedPct, npcSpecialAbilities } from '../../lib/npcHelpers'
import { ResistChip } from '../ResistChip'
import NPCCasterSummarySection from '../overlays/NPCCasterSummarySection'
import { DEFAULT_NPC_OVERLAY_SECTIONS } from '../../types/config'
import type { NPC, NPCCasterSummary } from '../../types/npc'
import type { SpecialAbility } from '../../types/overlay'

// EncounterNPCInfo renders the boss's live game-database stats (resists, HP,
// special abilities, signature spells) below the composition report — pulled
// from the NPC linked in the Raid Editor, so raiders can see fight info
// without leaving the composition check page.

// Danger melee specials (Summon, Enrage, Rampage, Area Rampage, Flurry, Triple
// Attack, Dual Wield) and hard immunities — mirrors the NPC overlay's badge
// coloring so the same abilities read the same way everywhere in the app.
const DANGER_ABILITIES = new Set([1, 2, 3, 4, 5, 6, 7])
const IMMUNE_ABILITIES = new Set([12, 13, 14, 15, 16, 17, 18, 19, 20, 24, 26, 31])

function abilityBadgeColor(code: number): string {
  if (DANGER_ABILITIES.has(code)) return '#dc2626'
  if (IMMUNE_ABILITIES.has(code)) return '#f97316'
  return '#6b7280'
}

function AbilityBadge({ ability }: { ability: SpecialAbility }): React.ReactElement {
  return (
    <span
      className="rounded px-1.5 py-0.5 text-[10px] font-semibold text-white"
      style={{ backgroundColor: abilityBadgeColor(ability.code) }}
    >
      {ability.name || `Ability ${ability.code}`}
    </span>
  )
}

function Stat({ label, value, color }: { label: string; value: string | number; color?: string }): React.ReactElement {
  return (
    <div className="flex flex-col items-center rounded px-2 py-1" style={{ backgroundColor: 'var(--color-surface-2)', minWidth: '3.25rem' }}>
      <span className="text-[9px] font-semibold uppercase tracking-wider" style={{ color: 'var(--color-muted-foreground)' }}>{label}</span>
      <span className="text-xs font-semibold tabular-nums" style={{ color: color ?? 'var(--color-foreground)' }}>{value}</span>
    </div>
  )
}

export default function EncounterNPCInfo({ npcId }: { npcId: number }): React.ReactElement | null {
  const [npc, setNpc] = useState<NPC | null>(null)
  const [casterSummary, setCasterSummary] = useState<NPCCasterSummary | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError(false)
    setNpc(null)
    setCasterSummary(undefined)
    Promise.all([getNPC(npcId), getNPCSpells(npcId).catch(() => null)])
      .then(([n, spells]) => {
        if (cancelled) return
        setNpc(n)
        setCasterSummary(spells?.summary)
      })
      .catch(() => { if (!cancelled) setError(true) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [npcId])

  if (loading) {
    return (
      <div className="rounded-lg px-4 py-3 text-sm" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)', color: 'var(--color-muted-foreground)' }}>
        Loading NPC info…
      </div>
    )
  }
  if (error || !npc) {
    return (
      <div className="rounded-lg px-4 py-3 text-sm" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)', color: 'var(--color-muted-foreground)' }}>
        Couldn&apos;t load the linked NPC (id {npcId}).
      </div>
    )
  }

  const abilities = npcSpecialAbilities(npc)

  return (
    <div className="rounded-lg overflow-hidden" style={{ border: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface)' }}>
      <div
        className="flex items-center justify-between px-3 py-2"
        style={{ borderBottom: '1px solid var(--color-border)', backgroundColor: 'var(--color-surface-2)' }}
      >
        <span className="flex items-center gap-1.5 text-sm font-semibold" style={{ color: 'var(--color-foreground)' }}>
          <Skull size={14} /> {npc.name.replace(/_/g, ' ')}
          <span className="font-normal opacity-70">— fight info</span>
        </span>
        <Link
          to={`/npcs?select=${npc.id}`}
          className="flex items-center gap-1 text-[11px]"
          style={{ color: 'var(--color-primary)' }}
        >
          <Database size={12} /> Open in NPC database
        </Link>
      </div>

      <div className="flex flex-col gap-2.5 px-3 py-3">
        <div className="flex flex-wrap gap-1.5">
          <Stat label="Level" value={npcLevelLabel(npc)} color="var(--color-primary)" />
          <Stat label="Class" value={className(npc.class)} />
          <Stat label="Body" value={bodyTypeName(npc.body_type)} />
          <Stat label="HP" value={npc.hp.toLocaleString()} color="#22c55e" />
          {npc.mana > 0 && <Stat label="Mana" value={npc.mana.toLocaleString()} color="#3b82f6" />}
          <Stat label="AC" value={npc.ac} />
          <Stat label="Min DMG" value={npc.min_dmg} color="#ef4444" />
          <Stat label="Max DMG" value={npc.max_dmg} color="#ef4444" />
          <Stat label="Atk/Rd" value={npc.attack_count < 0 ? 'default' : npc.attack_count} />
          <Stat label="Speed" value={`${npcRunSpeedPct(npc.run_speed)}%`} />
          {npc.raid_target === 1 && (
            <span className="flex items-center rounded px-2 py-1 text-[10px] font-semibold text-white" style={{ backgroundColor: '#7c3aed' }}>
              RAID TARGET
            </span>
          )}
        </div>

        <div>
          <p className="mb-1 text-[9px] font-semibold uppercase tracking-widest" style={{ color: 'var(--color-muted-foreground)' }}>Resists</p>
          <div className="flex flex-wrap gap-1.5">
            <ResistChip type="magic" value={npc.mr} />
            <ResistChip type="cold" value={npc.cr} />
            <ResistChip type="fire" value={npc.fr} />
            <ResistChip type="disease" value={npc.dr} />
            <ResistChip type="poison" value={npc.pr} />
          </div>
        </div>

        {abilities.length > 0 && (
          <div>
            <p className="mb-1 text-[9px] font-semibold uppercase tracking-widest" style={{ color: 'var(--color-muted-foreground)' }}>Special Abilities</p>
            <div className="flex flex-wrap gap-1">
              {abilities.filter((a) => a.value !== 0).map((a) => (
                <AbilityBadge key={a.code} ability={a} />
              ))}
            </div>
          </div>
        )}

        {casterSummary && (
          <NPCCasterSummarySection
            summary={casterSummary}
            sections={DEFAULT_NPC_OVERLAY_SECTIONS}
            theme={{
              heading: 'var(--color-muted-foreground)',
              muted: 'var(--color-muted-foreground)',
              chipBg: 'var(--color-surface-2)',
              chipText: 'var(--color-foreground)',
            }}
          />
        )}
      </div>
    </div>
  )
}
