// Mirrors backend/internal/blockbuff (and the blockbuffViewDTO in
// internal/api/blockbuff.go). Blocks live in the game server and can only be
// changed with #blockbuff / #blockbuffif / #allowbuff; PQC keeps the list the
// player wants, tracks what the server last confirmed in the log, and shows
// the commands that close the gap.

export interface BlockedBuffEntry {
  spell_id: number
  /** 0 = always block; otherwise block only while this spell is on you. */
  if_spell_id: number
}

export type BlockedBuffStatus =
  | 'applied' // wanted and the server has it
  | 'needs_block' // wanted, server doesn't have it
  | 'needs_allow' // on the server but no longer wanted
  | 'unconfirmed' // wanted; server list never seen
  | 'server_only' // on the server, not wanted; list incomplete

export interface BlockedBuffRow extends BlockedBuffEntry {
  desired: boolean
  observed: boolean
  status: BlockedBuffStatus
  spell_name: string
  if_spell_name?: string
}

export interface BlockedBuffsView {
  character: string
  /** Unix seconds of the last full server listing; 0 = never seen. */
  synced_at: number
  rows: BlockedBuffRow[]
  /** In-game commands that bring the server in line with the wanted list. */
  commands: string[]
}
