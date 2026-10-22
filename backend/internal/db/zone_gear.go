package db

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ZoneItemSource is one NPC in a zone that drops an item, for the upgrade
// finder's by-zone view ("kill this to get that"). NPCID is 0 for a zone-wide
// loot pool, which isn't tied to any one NPC — NPCName then holds the pool's
// label. Chance is the effective per-kill drop rate in percent, matching
// GetZoneDrops and the item page's "Dropped by" rate.
type ZoneItemSource struct {
	NPCID   int     `json:"npc_id"`
	NPCName string  `json:"npc_name"`
	Chance  float64 `json:"chance"`
}

// ZoneItemSources returns every item that drops in a zone, keyed by canonical
// item id (the id UpgradeCandidates reports), with the NPCs that drop it
// sorted by best chance first. It covers three kinds of drop:
//
//   - NPCs that spawn in the zone per spawn2 (the GetZoneDrops set),
//   - NPCs with no spawn2 row at all, homed to the zone by the
//     zoneidnumber*1000+n id convention — script-spawned raid bosses (PoTimeB,
//     etc.) are invisible to a spawn2-only join, and they're exactly the
//     targets a raid wants this view for (same fallback buildPoPGated uses),
//   - zone-wide loot pools (zoneLootOverlays, e.g. Vex Thal's shared drops).
func (db *DB) ZoneItemSources(shortName string) (map[int][]ZoneItemSource, error) {
	var zoneID int
	err := db.QueryRow(`SELECT zoneidnumber FROM zone WHERE short_name = ? LIMIT 1`, shortName).Scan(&zoneID)
	if errors.Is(err, sql.ErrNoRows) {
		return map[int][]ZoneItemSource{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("zone item sources %q: %w", shortName, err)
	}
	idMin, idMax := zoneID*1000, zoneID*1000+999

	rows, err := db.Query(`
		SELECT lde.item_id, n.id, n.name,
		       MAX(CAST(lte.probability AS REAL) * lde.chance / 100.0) AS chance
		FROM npc_types n
		JOIN loottable_entries lte ON lte.loottable_id = n.loottable_id
		JOIN lootdrop_entries lde ON lde.lootdrop_id = lte.lootdrop_id
		WHERE n.loottable_id > 0 AND (
		  n.id IN (
		    SELECT DISTINCT se.npcID
		    FROM spawnentry se
		    JOIN spawn2 s2 ON s2.spawngroupID = se.spawngroupID
		    WHERE s2.zone = ?
		  ) OR (
		    n.id BETWEEN ? AND ? AND NOT EXISTS (
		      SELECT 1 FROM spawnentry se
		      JOIN spawn2 s2 ON s2.spawngroupID = se.spawngroupID
		      WHERE se.npcID = n.id
		    )
		  )
		)
		GROUP BY lde.item_id, n.id`, shortName, idMin, idMax)
	if err != nil {
		return nil, fmt.Errorf("zone item sources %q: %w", shortName, err)
	}
	defer rows.Close()

	// Duplicate npc_types rows for one boss (variants, placeholder-decorated
	// names) collapse to one display name per item, keeping the best chance.
	type key struct {
		item int
		name string
	}
	best := map[key]ZoneItemSource{}
	add := func(itemID int, src ZoneItemSource) {
		if itemID <= 0 || src.Chance <= 0 {
			return
		}
		k := key{db.CanonicalItemID(itemID), src.NPCName}
		if cur, ok := best[k]; !ok || src.Chance > cur.Chance {
			best[k] = src
		}
	}
	for rows.Next() {
		var itemID, npcID int
		var name string
		var chance float64
		if err := rows.Scan(&itemID, &npcID, &name, &chance); err != nil {
			return nil, fmt.Errorf("zone item sources scan: %w", err)
		}
		display := strings.TrimSpace(strings.ReplaceAll(stripNPCDecoration(name), "_", " "))
		add(itemID, ZoneItemSource{NPCID: npcID, NPCName: display, Chance: chance})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if o := zoneOverlayFor(idMin); o != nil {
		ld, err := db.loadLootdrop(o.lootdropID)
		if err != nil {
			return nil, err
		}
		if ld != nil {
			for _, it := range ld.Items {
				add(it.ItemID, ZoneItemSource{NPCName: o.label, Chance: it.Chance})
			}
		}
	}

	out := make(map[int][]ZoneItemSource)
	for k, src := range best {
		src.Chance = float64(int(src.Chance*100+0.5)) / 100
		out[k.item] = append(out[k.item], src)
	}
	for _, list := range out {
		sort.Slice(list, func(i, j int) bool {
			if list[i].Chance != list[j].Chance {
				return list[i].Chance > list[j].Chance
			}
			return list[i].NPCName < list[j].NPCName
		})
	}
	return out, nil
}
