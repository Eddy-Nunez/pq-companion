package db

import "strings"

// scriptedLootEntry is one item a Planes of Power encounter's quest Lua
// script grants directly via NPC:AddItem(...), with no
// loottable_entries/lootdrop_entries row backing it at all — invisible to
// every ordinary loot join. See scriptedLootByNPC for why this table exists
// and how it was derived.
type scriptedLootEntry struct {
	itemID    int
	chancePct float64
	minCount  int
	maxCount  int
}

// scriptedLootByNPC lists PoP loot granted purely by a quest Lua script
// (SecretsOTheP/quests), keyed by npc_types id. The EQMacEmu Sep 2026 PoP
// patch moved Bastion of Thunder's Ring/Symbol of Torden and gem/sphere
// components, and the three Plane of Storms Esoteric Medallion variants, out
// of the DB loot tables entirely and into these scripts (confirmed via the
// 2026-09-27 quarm.db regen: their loottable_entries/lootdrop_entries rows
// were deleted outright, not just probability-tuned). Without this table,
// GetItemSources/GetNPCLoot read these items as having NO known source at
// all — which is wrong, they're real, just script-granted — and
// buildPoPGated's safe "can't classify -> available" default then wrongly
// marks them obtainable pre-PoP-launch instead of PoP-gated.
//
// Verified against the actual script source in SecretsOTheP/quests, not
// guessed — every count/chance below traces to a literal AddItem/math.random
// call. There is no automated regen path for this table; re-derive by
// re-reading the cited files if the quest repo changes these encounters.
// Chance/count use the open-world (non-PvP-instance) branch where a script
// has one — the PvP instance variant is very slightly stingier and isn't
// modeled here, since this table only backs "where can I get this" UI, not
// exact drop-rate math.
//
//   - bothunder/encounters/ProgressionLoot.lua: GUARANTEED_LOOT (100% chance,
//     exact count) and COMPONENT_LOOT (25% chance, x1) tables.
//   - postorms/encounters/MiniBosses.lua: MiniSpawnEvent grants
//     math.random(1, 3) medallions per miniboss kill (100% chance of at
//     least 1, up to 3).
//   - postorms/encounters/{Gurebk,Jeplak,Neffiken}.lua: each lord's own
//     Spawn event grants 5 guaranteed medallions plus up to 3 more at
//     independent 50% rolls (5-8 total) — modeled here as the 5 guaranteed
//     (chance 100, count 5) since the bonus rolls aren't a fixed count.
//     Jeplak's medallion is on its REAL spawn id (210472, loottable_id
//     96975); the FAKE pre-fight decoy id (210403, loottable_id=0) never
//     grants loot and is deliberately excluded.
var scriptedLootByNPC = map[int][]scriptedLootEntry{
	// Bastion of Thunder key components — bothunder/encounters/ProgressionLoot.lua
	209070: {{itemID: 17168, chancePct: 100, minCount: 3, maxCount: 3}}, // Laef Windfall: Ring of Torden x3
	209071: {{itemID: 17168, chancePct: 100, minCount: 3, maxCount: 3}}, // Gaukr Sandstorm
	209072: {{itemID: 17168, chancePct: 100, minCount: 3, maxCount: 3}}, // Oreen Wavecrasher
	209082: {{itemID: 17168, chancePct: 100, minCount: 3, maxCount: 3}}, // Hreidar Lynhillig
	209016: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Brynju Thunderclap: Unadorned Symbol of Torden
	209059: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Auliffe Chaoswind
	209060: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Eindride Icestorm
	209061: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Kuanbyr Hailstorm
	209142: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Ekil Thundercall
	209146: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Hibdin Cyclone
	209147: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Jolur Sandstorm
	209148: {{itemID: 17169, chancePct: 100, minCount: 1, maxCount: 1}}, // Oljin Stormtide

	// Bastion of Thunder gem/sphere components (25% chance) — same script
	209001: {{itemID: 9421, chancePct: 25, minCount: 1, maxCount: 1}}, // Sandstorm Gem
	209004: {{itemID: 9421, chancePct: 25, minCount: 1, maxCount: 1}},
	209011: {{itemID: 9421, chancePct: 25, minCount: 1, maxCount: 1}},
	209007: {{itemID: 9422, chancePct: 25, minCount: 1, maxCount: 1}}, // Lightning Gem
	209009: {{itemID: 9422, chancePct: 25, minCount: 1, maxCount: 1}},
	209064: {{itemID: 9422, chancePct: 25, minCount: 1, maxCount: 1}},
	209014: {{itemID: 9423, chancePct: 25, minCount: 1, maxCount: 1}}, // Blizzard Gem
	209018: {{itemID: 9423, chancePct: 25, minCount: 1, maxCount: 1}},
	209055: {{itemID: 9423, chancePct: 25, minCount: 1, maxCount: 1}},
	209062: {{itemID: 9423, chancePct: 25, minCount: 1, maxCount: 1}},
	209000: {{itemID: 9424, chancePct: 25, minCount: 1, maxCount: 1}}, // Tornado Gem
	209002: {{itemID: 9424, chancePct: 25, minCount: 1, maxCount: 1}},
	209051: {{itemID: 9424, chancePct: 25, minCount: 1, maxCount: 1}},
	209162: {{itemID: 9424, chancePct: 25, minCount: 1, maxCount: 1}},
	209015: {{itemID: 9429, chancePct: 25, minCount: 1, maxCount: 1}}, // Sandstorm Sphere
	209040: {{itemID: 9429, chancePct: 25, minCount: 1, maxCount: 1}},
	209041: {{itemID: 9429, chancePct: 25, minCount: 1, maxCount: 1}},
	209013: {{itemID: 9430, chancePct: 25, minCount: 1, maxCount: 1}}, // Lightning Sphere
	209019: {{itemID: 9430, chancePct: 25, minCount: 1, maxCount: 1}},
	209020: {{itemID: 9430, chancePct: 25, minCount: 1, maxCount: 1}},
	209043: {{itemID: 9431, chancePct: 25, minCount: 1, maxCount: 1}}, // Blizzard Sphere
	209044: {{itemID: 9431, chancePct: 25, minCount: 1, maxCount: 1}},
	209133: {{itemID: 9431, chancePct: 25, minCount: 1, maxCount: 1}},
	209134: {{itemID: 9431, chancePct: 25, minCount: 1, maxCount: 1}},
	209161: {{itemID: 9431, chancePct: 25, minCount: 1, maxCount: 1}},
	209048: {{itemID: 9432, chancePct: 25, minCount: 1, maxCount: 1}}, // Tornado Sphere
	209050: {{itemID: 9432, chancePct: 25, minCount: 1, maxCount: 1}},
	209135: {{itemID: 9432, chancePct: 25, minCount: 1, maxCount: 1}},

	// Plane of Storms Esoteric Medallions (Bastion of Thunder access) —
	// postorms/encounters/MiniBosses.lua + {Gurebk,Jeplak,Neffiken}.lua
	210026: {{itemID: 28783, chancePct: 100, minCount: 1, maxCount: 3}}, // Laruken the Rigid: Kelek`Vor
	210027: {{itemID: 28783, chancePct: 100, minCount: 1, maxCount: 3}}, // Zertuken the Unyielding
	210028: {{itemID: 28765, chancePct: 100, minCount: 1, maxCount: 3}}, // Paruek the Strong: Srerendi
	210029: {{itemID: 28765, chancePct: 100, minCount: 1, maxCount: 3}}, // Faruek the Bold
	210032: {{itemID: 28780, chancePct: 100, minCount: 1, maxCount: 3}}, // Pendubk the Turbulent: Krendic
	210033: {{itemID: 28780, chancePct: 100, minCount: 1, maxCount: 3}}, // Solnebk the Unruly
	210332: {{itemID: 28780, chancePct: 100, minCount: 5, maxCount: 8}}, // Gurebk, Lord of Krendic
	210472: {{itemID: 28765, chancePct: 100, minCount: 5, maxCount: 8}}, // Jeplak, Lord of Srerendi (real id)
	210251: {{itemID: 28783, chancePct: 100, minCount: 5, maxCount: 8}}, // Neffiken, Lord of Kelek`Vor
}

// scriptedLootByItem is the reverse index of scriptedLootByNPC (item ->
// granting NPCs), built once on first use.
var scriptedLootByItem map[int][]int

func ensureScriptedLootByItem() {
	if scriptedLootByItem != nil {
		return
	}
	scriptedLootByItem = map[int][]int{}
	for npcID, entries := range scriptedLootByNPC {
		for _, e := range entries {
			scriptedLootByItem[e.itemID] = append(scriptedLootByItem[e.itemID], npcID)
		}
	}
}

// scriptedLootZone resolves the zone an npc_types id belongs to using
// Quarm's id-per-zone convention (id / 1000 == zone.zoneidnumber), the same
// fallback buildPoPGated already relies on for spawn2-less PoP NPCs. Every
// npc id in scriptedLootByNPC has been verified against this convention.
func (db *DB) scriptedLootZone(npcID int) (shortName, longName string) {
	_ = db.QueryRow(`SELECT short_name, long_name FROM zone WHERE zoneidnumber = ?`, npcID/1000).
		Scan(&shortName, &longName)
	return shortName, longName
}

// scriptedItemSourceNPCs returns ItemSourceNPC rows for every NPC that
// scriptedLootByNPC records as granting the given item, for GetItemSources
// to append to its normal DB-derived drop list.
func (db *DB) scriptedItemSourceNPCs(itemID int) ([]ItemSourceNPC, error) {
	ensureScriptedLootByItem()
	npcIDs := scriptedLootByItem[itemID]
	if len(npcIDs) == 0 {
		return nil, nil
	}
	var out []ItemSourceNPC
	for _, npcID := range npcIDs {
		var name string
		if err := db.QueryRow(`SELECT name FROM npc_types WHERE id = ?`, npcID).Scan(&name); err != nil {
			continue
		}
		zoneShort, zoneName := db.scriptedLootZone(npcID)
		var chance float64
		for _, e := range scriptedLootByNPC[npcID] {
			if e.itemID == itemID {
				chance = e.chancePct
				break
			}
		}
		display := strings.TrimSpace(strings.ReplaceAll(stripNPCDecoration(name), "_", " "))
		out = append(out, ItemSourceNPC{
			ID:            npcID,
			Name:          display,
			ZoneName:      zoneName,
			ZoneShortName: zoneShort,
			DropRate:      chance,
			Scripted:      true,
		})
	}
	return out, nil
}

// loadScriptedLootDrop builds a synthetic LootDrop group ("Script-granted
// loot") for an NPC's scriptedLootByNPC entries, for GetNPCLoot to append
// alongside its normal DB-derived Drops — the same "append a pool the DB
// join can't see" shape as loadLootdrop's zone-wide overlays. Returns nil,
// nil if the NPC has no scripted loot.
func (db *DB) loadScriptedLootDrop(npcID int) (*LootDrop, error) {
	entries := scriptedLootByNPC[npcID]
	if len(entries) == 0 {
		return nil, nil
	}
	drop := &LootDrop{
		Name:        "Script-granted loot",
		Multiplier:  1,
		Probability: 100,
	}
	for _, e := range entries {
		var name string
		var icon int
		if err := db.QueryRow(`SELECT Name, icon FROM items WHERE id = ?`, e.itemID).Scan(&name, &icon); err != nil {
			continue
		}
		// LootDropItem has no min/max range field (a normal lootdrop_entries
		// row is one chance + one multiplier), so a script's variable count
		// (e.g. 1-3 medallions) shows as its guaranteed floor; the full range
		// lives in scriptedLootByNPC's doc comment for anyone checking exact
		// numbers.
		drop.Items = append(drop.Items, LootDropItem{
			ItemID:     e.itemID,
			ItemName:   name,
			ItemIcon:   icon,
			Chance:     e.chancePct,
			Multiplier: e.minCount,
		})
	}
	return drop, nil
}
