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
}

// GetZoneNPCSpawns returns spawn chances for every NPC that a spawn point in
// the zone can roll in the current era.
//
// The server picks a group member with a weighted roll over the eligible
// entries' chance values (SpawnGroup::GetNPCType), so a share is
// chance / sum(eligible chances) — raw chance alone is wrong for the ~200
// groups whose weights don't sum to 100, which is mostly groups mixing
// era-gated variants (a Classic and a Luclin row for the same slot).
// Day/night-gated entries (mintime/maxtime) are counted as always competing,
// so shares in those groups are an all-day average.
func (db *DB) GetZoneNPCSpawns(shortName string) ([]ZoneNPCSpawn, error) {
	q := `
		WITH pts AS (
			SELECT s2.id, s2.spawngroupID
			FROM spawn2 s2
			WHERE s2.zone = ? AND ` + activeContentClause("s2") + `
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
		WHERE e.chance > 0
		GROUP BY e.npcID`
	rows, err := db.Query(q, shortName)
	if err != nil {
		return nil, fmt.Errorf("get zone npc spawns %q: %w", shortName, err)
	}
	defer rows.Close()

	out := []ZoneNPCSpawn{}
	for rows.Next() {
		var s ZoneNPCSpawn
		if err := rows.Scan(&s.NPCID, &s.MinPct, &s.MaxPct, &s.Points); err != nil {
			return nil, fmt.Errorf("scan zone npc spawn: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
