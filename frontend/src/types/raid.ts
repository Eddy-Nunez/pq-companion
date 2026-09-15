// Types for the raid knowledge base + composition checker (mirror the Go
// structs in backend/internal/raidcomp). Class codes are EQMon's taxonomy
// codes ("war", "sk", "clr", ...) resolved in the backend from Zeal class ids
// or free-text names.

export type ClassCode = string

export interface RaidTaxonomyRoleSub {
  label: string
  classes: ClassCode[]
}

export interface RaidTaxonomyRole {
  label: string
  classes?: ClassCode[]
  sub_roles?: Record<string, RaidTaxonomyRoleSub>
}

export interface RaidTaxonomy {
  role_order: string[]
  roles: Record<string, RaidTaxonomyRole>
  class_names: Record<string, string>
  strategy_order: string[]
}

export type RaidStatus = 'active' | 'placeholder'
export type CompLevel = 'min' | 'rec'

export interface RaidCompRow {
  role: string
  sub_role?: string
  min: number
  rec: number
}

export interface RaidEncounter {
  id: string
  name: string
  zone: string
  zone_id?: number
  status: RaidStatus
  trigger?: string
  reqs?: string[]
  strategy?: Record<string, string>
  source?: string
  notes?: string
  comps: RaidCompRow[]
  created_at: number
  updated_at: number
}

/** One raw taxonomy row as stored in user.db (used by the taxonomy editor). */
export interface RaidRole {
  role: string
  sub_role?: string
  label: string
  classes: ClassCode[]
  position?: number
}

export interface RaidRosterMember {
  name: string
  level?: number
  class?: number // Zeal 1-indexed class id
  code?: ClassCode
  group?: string
  rank?: string
}

export interface RaidRosterSnapshot {
  zeal_connected: boolean // Zeal pipe currently connected
  in_raid: boolean // a raid roster with members has arrived
  updated_at?: number
  zone_id?: number
  zone?: string
  members: RaidRosterMember[]
}

/** One member of a manually-entered roster (class = code or class name). */
export interface CheckRosterInput {
  name: string
  class: string
}

export interface CheckRowReport {
  path: string
  role: string
  sub_role?: string
  label: string // live taxonomy label, e.g. "Tank / Defensive"
  need: number
  have: number
  candidates?: string[]
  more_candidates?: number
}

export interface CheckSummaryAnswer {
  ok: boolean
  rows: number
  gap_rows: number
  gap_count: number
}

export interface CheckReport {
  encounter_id: string
  encounter_name: string
  zone?: string
  zone_id?: number
  roster_total: number
  roster_mapped: number
  members?: RaidRosterMember[]
  min: CheckRowReport[]
  rec: CheckRowReport[]
  summary: { min: CheckSummaryAnswer; rec: CheckSummaryAnswer }
}

export interface CheckRequest {
  encounter_id: string
  roster?: CheckRosterInput[]
}

// ── Pack export/import ───────────────────────────────────────────────────────
// Portable JSON packs carrying encounters (with comps/reqs/strategy) between
// users. Shapes mirror the Go structs in backend/internal/api/raidpacks.go.

export interface RaidPack {
  kind: string
  version: number
  pack_name: string
  description?: string
  exported_at: number
  encounters: RaidEncounter[]
}

export interface RaidImportPreviewItem {
  encounter: RaidEncounter
  exists: boolean
  errors?: string[]
  warnings?: string[]
}

export interface RaidImportPreview {
  pack_name: string
  encounters: RaidImportPreviewItem[]
}

export interface RaidImportCommitItem {
  encounter: RaidEncounter
  overwrite: boolean
}

export interface RaidImportCommitRequest {
  pack_name?: string
  encounters: RaidImportCommitItem[]
}

export interface RaidImportCommitResult {
  saved: string[]
  skipped: string[]
  failed?: Record<string, string>
}
