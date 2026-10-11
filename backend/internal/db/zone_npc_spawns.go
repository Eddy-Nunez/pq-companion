package db

import (
	"fmt"
	"strings"
)

// currentExpansion is the era the live server runs, used to decide which
// expansion-gated spawn rows are active. Quarm is in Planes of Power.
const currentExpansion = popExpansion

// activeContentClause mirrors EQMacEmu's ContentFilterCriteria::apply for the
// current era: a row is loaded only when its min/max_expansion window covers
// the current expansion (-1 = unbounded) and it carries no content flags (the
// server enables none that we can see, so a flagged row never loads).
func activeContentClause(alias string) string {
	return strings.NewReplacer("@", alias).Replace(fmt.Sprintf(`
		(@.min_expansion <= %[1]d OR @.min_expansion = -1)
		AND (@.max_expansion >= %[1]d OR @.max_expansion = -1)
		AND (@.content_flags IS NULL OR @.content_flags = '')
		AND (@.content_flags_disabled IS NULL OR @.content_flags_disabled = '')`,
		currentExpansion))
}

// Reasons a zone NPC has no spawn share (ZoneNPCSpawn.Inactive).
const (
	// SpawnInactiveOtherEra: every spawn entry or point for the NPC is
	// expansion-gated out of the current era (typically a pre-Luclin model
	// variant that was swapped for a newer row).
	SpawnInactiveOtherEra = "other_era"
	// SpawnInactiveZeroWeight: the NPC is in an active group but with chance
	// 0, so the server's roll never picks it.
	SpawnInactiveZeroWeight = "zero_weight"
)

// ZoneNPCSpawn is how likely an NPC is to be the one a spawn point rolls, in
// one zone. This is a different number from any loot chance: it's the NPC's
// share of its spawn group, not a drop rate.
type ZoneNPCSpawn struct {
	NPCID int `json:"npc_id"`
	// MinPct and MaxPct are the NPC's lowest and highest share across the
	// spawn groups it belongs to in the zone (equal when it is in one group).
	MinPct float64 `json:"min_pct"`
	MaxPct float64 `json:"max_pct"`
	// Points is how many spawn points in the zone can roll this NPC.
	Points int `json:"points"`
	// Inactive is set (to a SpawnInactive* reason) when the NPC is in the
	// zone's spawn data but can't currently spawn; the other fields are zero.
	Inactive string `json:"inactive,omitempty"`
}

// npcSpawnShares computes spawn shares for the current era.
//
// The server picks a group member with a weighted roll over the eligible
// entries' chance values (SpawnGroup::GetNPCType), so a share is
// chance / sum(eligible chances) — raw chance alone is wrong for the ~200
// groups whose weights don't sum to 100, which is mostly groups mixing
// era-gated variants (a Classic and a Luclin row for the same slot).
// Day/night-gated entries (mintime/maxtime) are counted as always competing,
// so shares in those groups are an all-day average.
//
// zone scopes to one zone's spawn points ("" = every zone). ids, when
// non-empty, limits the result to those NPCs.
func (db *DB) npcSpawnShares(zone string, ids []int) ([]ZoneNPCSpawn, error) {
	var ptsWhere []string
	var args []any
	if zone != "" {
		ptsWhere = append(ptsWhere, "s2.zone = ?")
		args = append(args, zone)
	}
	idFilter := ""
	if len(ids) > 0 {
		idFilter = strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		ptsWhere = append(ptsWhere,
			"s2.spawngroupID IN (SELECT spawngroupID FROM spawnentry WHERE npcID IN ("+idFilter+"))")
		for _, id := range ids {
			args = append(args, id)
		}
	}
	ptsWhere = append(ptsWhere, activeContentClause("s2"))

	final := "e.chance > 0"
	if idFilter != "" {
		final += " AND e.npcID IN (" + idFilter + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}

	q := `
		WITH pts AS (
			SELECT s2.id, s2.spawngroupID
			FROM spawn2 s2
			WHERE ` + strings.Join(ptsWhere, " AND ") + `
		),
		ents AS (
			SELECT se.spawngroupID, se.npcID, se.chance
			FROM spawnentry se
			WHERE se.spawngroupID IN (SELECT spawngroupID FROM pts)
			  AND ` + activeContentClause("se") + `
		),
		tot AS (
			SELECT spawngroupID, SUM(chance) AS t
			FROM ents GROUP BY spawngroupID HAVING t > 0
		)
		SELECT e.npcID,
		       MIN(e.chance * 100.0 / tot.t),
		       MAX(e.chance * 100.0 / tot.t),
		       COUNT(DISTINCT p.id)
		FROM ents e
		JOIN tot ON tot.spawngroupID = e.spawngroupID
		JOIN pts p ON p.spawngroupID = e.spawngroupID
		WHERE ` + final + `
		GROUP BY e.npcID`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("npc spawn shares: %w", err)
	}
	defer rows.Close()

	out := []ZoneNPCSpawn{}
	for rows.Next() {
		var s ZoneNPCSpawn
		if err := rows.Scan(&s.NPCID, &s.MinPct, &s.MaxPct, &s.Points); err != nil {
			return nil, fmt.Errorf("scan npc spawn share: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetZoneNPCSpawns returns spawn shares for every NPC in the zone's spawn
// data: a share for each NPC a spawn point can roll in the current era, and
// an Inactive row for each one that is listed but can't spawn.
func (db *DB) GetZoneNPCSpawns(shortName string) ([]ZoneNPCSpawn, error) {
	out, err := db.npcSpawnShares(shortName, nil)
	if err != nil {
		return nil, fmt.Errorf("get zone npc spawns %q: %w", shortName, err)
	}
	active := make(map[int]bool, len(out))
	for _, s := range out {
		active[s.NPCID] = true
	}

	// Every NPC the zone's spawn chain references in any era, and whether any
	// of its entries is live now — live-but-unrolled means zero weight.
	rows, err := db.Query(`
		SELECT se.npcID,
		       MAX(CASE WHEN `+activeContentClause("se")+` AND `+activeContentClause("s2")+`
		                THEN 1 ELSE 0 END)
		FROM spawnentry se
		JOIN spawn2 s2 ON s2.spawngroupID = se.spawngroupID
		WHERE s2.zone = ?
		GROUP BY se.npcID`, shortName)
	if err != nil {
		return nil, fmt.Errorf("get zone inactive npcs %q: %w", shortName, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, live int
		if err := rows.Scan(&id, &live); err != nil {
			return nil, fmt.Errorf("scan zone inactive npc: %w", err)
		}
		if active[id] {
			continue
		}
		reason := SpawnInactiveOtherEra
		if live == 1 {
			reason = SpawnInactiveZeroWeight
		}
		out = append(out, ZoneNPCSpawn{NPCID: id, Inactive: reason})
	}
	return out, rows.Err()
}
