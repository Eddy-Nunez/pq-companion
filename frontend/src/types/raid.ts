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
  // npc_id links this encounter to its boss's npc_types row (quarm.db) so the
  // checker page can pull resists / HP / special abilities / signature spells
  // straight from the game database. 0 / undefined = not linked.
  npc_id?: number
  status: RaidStatus
  trigger?: string
  reqs?: string[]
  strategy?: Record<string, string>
  // Named group-composition templates (docs/raid-group-compositions-plan.md).
  shapes?: RaidShape[]
  source?: string
  notes?: string
  comps: RaidCompRow[]
  created_at: number
  updated_at: number
}

/** One raw taxonomy row as stored in user.db (used by the taxonomy editor). */
export interface RaidShapeRow {
  role: string
  sub_role?: string
  count: number
}

export interface RaidShape {
  shape_id: string
  group_number?: number
  rows: RaidShapeRow[]
}

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
  /** Comp paths absent from the local taxonomy — auto-provisionable. */
  missing_roles?: string[]
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

export interface RaidRoleProvisionResult {
  created: string[]
  present: string[]
}

// ── Group composition proposal (split) ───────────────────────────

export type SplitPreference = 'trinity'

export type CompLevelFill = 'min' | 'rec'

export interface SplitRequest {
  encounter_id: string
  preference?: SplitPreference
  group_size?: number
  respect_existing_groups?: boolean
  // Cohort mode: split the roster into N smaller raids (2..6), each staffed
  // against the full template. Omitted = single raid.
  cohorts?: number
  // Group-shape templates (docs/raid-group-compositions-plan.md): the
  // enabled shape ids (must exist on the encounter). Requires cohorts >= 2;
  // empty/omitted = pure trinity.
  shapes?: string[]
  // Per-raid shape placement in cohort mode: replicate (default) — every
  // raid fields every enabled shape; distribute — shape i applies to raid i.
  shape_distribution?: 'replicate' | 'distribute'
  roster?: CheckRosterInput[]
}

export interface SplitSlot {
  member: string
  class?: ClassCode
  role: string
  sub_role?: string
  path: string
  label: string
  level?: CompLevelFill
  group: number
  rank?: string
}

export interface ProposedGroup {
  number: number
  size: number
  slots: SplitSlot[]
  // Group-shape identifier this group was formed from (cohort mode with
  // enabled shapes); absent for trinity-formed groups.
  shape_id?: string
}

export interface SplitUnassigned {
  name: string
  class?: ClassCode
  reason: string
}

export interface SplitCoverage {
  path: string
  label: string
  need: number
  placed: number
}

export interface SplitReport {
  encounter_id: string
  encounter_name: string
  preference: SplitPreference
  group_size: number
  groups: ProposedGroup[]
  unassigned: SplitUnassigned[]
  min: SplitCoverage[]
  rec: SplitCoverage[]
  warnings?: string[]
  // Per-cohort reports (cohort mode). Single-raid responses carry exactly
  // one entry mirroring the legacy fields; cohort mode (>1 raids) is
  // authoritative here and leaves the legacy fields empty.
  cohorts: CohortReport[]
}

export interface CohortReport {
  number: number
  groups: ProposedGroup[]
  min: SplitCoverage[]
  rec: SplitCoverage[]
  roster_count: number
  warnings?: string[]
}

// ── cohort auto-suggest (split plan) ──────────────────────────

export interface SplitPlanLeaf {
  path: string
  label: string
  eligible: number
  min: number
  rec: number
}

export interface SplitPlanReport {
  encounter_id: string
  encounter_name: string
  roster_total: number
  roster_mapped: number
  max_cohorts: number
  binding_path?: string
  leaves: SplitPlanLeaf[]
}
