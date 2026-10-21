package db

import (
	"fmt"
	"strings"
)

// NPCZone is one zone an NPC appears in.
type NPCZone struct {
	ShortName string `json:"short_name"`
	LongName  string `json:"long_name"`
	// Inferred is true when the NPC has no spawn2 placement at all (quest-
	// or event-spawned by a zone script) and the zone comes from Quarm's
	// id-per-zone convention (npc id / 1000 == zone.zoneidnumber) instead.
	// The convention holds for ~97% of placed NPCs, so it is a strong hint
	// but not a guarantee.
	Inferred bool `json:"inferred,omitempty"`
}

// npcZonesChunk bounds the IN (...) list per query, well under SQLite's
// bound-parameter limit.
const npcZonesChunk = 500

// NPCZones returns every zone each of the given NPCs spawns in, ordered by
// zone long name. NPCs with no spawn2 rows fall back to the zone their id
// encodes (marked Inferred). NPCs resolved by neither are absent from the
// map.
func (db *DB) NPCZones(npcIDs []int) (map[int][]NPCZone, error) {
	out := make(map[int][]NPCZone, len(npcIDs))
	for start := 0; start < len(npcIDs); start += npcZonesChunk {
		end := min(start+npcZonesChunk, len(npcIDs))
		if err := db.npcSpawnZones(npcIDs[start:end], out); err != nil {
			return nil, err
		}
	}

	var missing []int
	seen := make(map[int]bool, len(npcIDs))
	for _, id := range npcIDs {
		if _, ok := out[id]; !ok && !seen[id] {
			missing = append(missing, id)
		}
		seen[id] = true
	}
	for start := 0; start < len(missing); start += npcZonesChunk {
		end := min(start+npcZonesChunk, len(missing))
		if err := db.npcIDZones(missing[start:end], out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// FillNPCZones sets Zones on each NPC in place.
func (db *DB) FillNPCZones(npcs []NPC) error {
	ids := make([]int, len(npcs))
	for i, n := range npcs {
		ids[i] = n.ID
	}
	zones, err := db.NPCZones(ids)
	if err != nil {
		return err
	}
	for i := range npcs {
		npcs[i].Zones = zones[npcs[i].ID]
	}
	return nil
}

func (db *DB) npcSpawnZones(ids []int, out map[int][]NPCZone) error {
	if len(ids) == 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := db.Query(`
		SELECT DISTINCT se.npcID, s2.zone, COALESCE(z.long_name, s2.zone) AS long_name
		FROM spawnentry se
		JOIN spawn2 s2 ON s2.spawngroupID = se.spawngroupID
		LEFT JOIN zone z ON z.short_name = s2.zone
		WHERE se.npcID IN (`+ph+`)
		ORDER BY se.npcID, long_name`, args...)
	if err != nil {
		return fmt.Errorf("npc spawn zones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var z NPCZone
		if err := rows.Scan(&id, &z.ShortName, &z.LongName); err != nil {
			return fmt.Errorf("scan npc spawn zone: %w", err)
		}
		out[id] = append(out[id], z)
	}
	return rows.Err()
}

func (db *DB) npcIDZones(ids []int, out map[int][]NPCZone) error {
	byZoneID := make(map[int][]int, len(ids))
	var args []any
	for _, id := range ids {
		zid := id / 1000
		if _, ok := byZoneID[zid]; !ok {
			args = append(args, zid)
		}
		byZoneID[zid] = append(byZoneID[zid], id)
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")
	rows, err := db.Query(`
		SELECT zoneidnumber, short_name, long_name FROM zone
		WHERE zoneidnumber IN (`+ph+`)`, args...)
	if err != nil {
		return fmt.Errorf("npc id zones: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var zid int
		z := NPCZone{Inferred: true}
		if err := rows.Scan(&zid, &z.ShortName, &z.LongName); err != nil {
			return fmt.Errorf("scan npc id zone: %w", err)
		}
		for _, id := range byZoneID[zid] {
			out[id] = []NPCZone{z}
		}
	}
	return rows.Err()
}
