package db

import (
	"fmt"
	"strings"
)

// VariantTraits holds the npc_types columns and spawn weighting that help a
// player tell same-name rows apart but that the shared NPC model doesn't
// carry: the body texture and gender (what the client renders) and how likely
// the row is to be the one a spawn point rolls.
type VariantTraits struct {
	Texture int
	Gender  int
	// SpawnChance is the highest spawnentry.chance the row has in the zone
	// (0-100), or -1 when it has no spawnentry in that zone (script-spawned
	// rows, or an unknown zone).
	SpawnChance int
}

// GetVariantTraits returns VariantTraits for the given npc_types ids. zone is
// the zone short name used to scope the spawn chance; empty means unscoped.
// Ids with no npc_types row are absent from the result.
func (db *DB) GetVariantTraits(ids []int, zone string) (map[int]VariantTraits, error) {
	out := make(map[int]VariantTraits, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	ph := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(
		fmt.Sprintf("SELECT id, texture, gender FROM npc_types WHERE id IN (%s)", strings.Join(ph, ",")),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("get variant traits: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		t := VariantTraits{SpawnChance: -1}
		if err := rows.Scan(&id, &t.Texture, &t.Gender); err != nil {
			return nil, fmt.Errorf("scan variant traits: %w", err)
		}
		out[id] = t
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate variant traits: %w", err)
	}

	q := fmt.Sprintf(`
		SELECT se.npcID, MAX(se.chance)
		FROM spawnentry se
		WHERE se.npcID IN (%s)`, strings.Join(ph, ","))
	qargs := args
	if zone != "" {
		q += ` AND se.spawngroupID IN (SELECT spawngroupID FROM spawn2 WHERE zone = ?)`
		qargs = append(append([]any{}, args...), zone)
	}
	q += ` GROUP BY se.npcID`
	crow, err := db.Query(q, qargs...)
	if err != nil {
		return nil, fmt.Errorf("get variant spawn chance: %w", err)
	}
	defer crow.Close()
	for crow.Next() {
		var id, chance int
		if err := crow.Scan(&id, &chance); err != nil {
			return nil, fmt.Errorf("scan variant spawn chance: %w", err)
		}
		if t, ok := out[id]; ok {
			t.SpawnChance = chance
			out[id] = t
		}
	}
	return out, crow.Err()
}
