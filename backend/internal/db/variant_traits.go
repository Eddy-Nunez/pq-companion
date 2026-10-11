package db

import (
	"fmt"
	"math"
	"strings"
)

// VariantTraits holds the npc_types columns and spawn weighting that help a
// player tell same-name rows apart but that the shared NPC model doesn't
// carry: the body texture and gender (what the client renders) and how likely
// the row is to be the one a spawn point rolls.
type VariantTraits struct {
	Texture int
	Gender  int
	// SpawnChance is the row's highest spawn-group share in the zone, in
	// percent and normalized the same way as the Zones tab (see
	// npcSpawnShares), or -1 when no current-era spawn point there can roll
	// it (script-spawned rows, other-era variants, or an unknown zone).
	SpawnChance float64
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

	shares, err := db.npcSpawnShares(zone, ids)
	if err != nil {
		return nil, fmt.Errorf("get variant spawn chance: %w", err)
	}
	for _, sh := range shares {
		if t, ok := out[sh.NPCID]; ok {
			// One decimal is plenty for a hint and keeps 1/3-style shares
			// from rendering as 33.333333%.
			t.SpawnChance = math.Round(sh.MaxPct*10) / 10
			out[sh.NPCID] = t
		}
	}
	return out, nil
}
