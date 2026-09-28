package db_test

import (
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

// Ring of Torden (17168) lost its loottable_entries/lootdrop_entries rows
// entirely in the 2026-09-27 quarm.db regen — Bastion of Thunder's key
// components moved to bothunder/encounters/ProgressionLoot.lua. Without
// scriptedLootByNPC, GetItemSources would report zero drop sources for it.
func TestGetItemSourcesIncludesScriptedLoot(t *testing.T) {
	d := openTestDB(t)
	src, err := d.GetItemSources(17168) // Ring of Torden
	if err != nil {
		t.Fatalf("GetItemSources(17168): %v", err)
	}
	found := false
	for _, drop := range src.Drops {
		if drop.Name == "Laef Windfall" {
			found = true
			if !drop.Scripted {
				t.Errorf("Laef Windfall source should be marked Scripted")
			}
		}
	}
	if !found {
		t.Errorf("expected Ring of Torden to list Laef Windfall as a scripted source, got %+v", src.Drops)
	}
}

// The Storms Esoteric Medallion (28780, the Krendic variant) is dropped both
// by the three Krendic minibosses (1-3 each) and by Gurebk, Lord of Krendic
// (5-8), all script-granted with no DB loot row.
func TestGetItemSourcesEsotericMedallionMultipleSources(t *testing.T) {
	d := openTestDB(t)
	src, err := d.GetItemSources(28780)
	if err != nil {
		t.Fatalf("GetItemSources(28780): %v", err)
	}
	names := map[string]bool{}
	for _, drop := range src.Drops {
		names[drop.Name] = true
	}
	for _, want := range []string{"Pendubk the Turbulent", "Solnebk the Unruly", "Gurebk, Lord of Krendic"} {
		if !names[want] {
			t.Errorf("expected %q among Esoteric Medallion sources, got %+v", want, src.Drops)
		}
	}
}

// GetNPCLoot must surface a "Script-granted loot" pool for an NPC whose only
// loot is script-granted, even when it also has (or lacks) a normal DB
// loottable.
func TestGetNPCLootIncludesScriptedDrop(t *testing.T) {
	d := openTestDB(t)
	loot, err := d.GetNPCLoot(209070) // Laef Windfall
	if err != nil {
		t.Fatalf("GetNPCLoot(209070): %v", err)
	}
	if loot == nil {
		t.Fatal("expected non-nil loot table for Laef Windfall")
	}
	var scriptedDrop *db.LootDrop
	for i := range loot.Drops {
		if loot.Drops[i].Name == "Script-granted loot" {
			scriptedDrop = &loot.Drops[i]
		}
	}
	if scriptedDrop == nil {
		t.Fatalf("expected a Script-granted loot pool, got drops: %+v", loot.Drops)
	}
	if len(scriptedDrop.Items) != 1 || scriptedDrop.Items[0].ItemID != 17168 {
		t.Errorf("scripted drop items = %+v, want [Ring of Torden]", scriptedDrop.Items)
	}
}

// Jeplak, Lord of Srerendi has a decoy pre-fight spawn id (210403,
// loottable_id=0) that never grants loot — only the real spawn id (210472)
// should carry the scripted medallion drop.
func TestScriptedLootExcludesJeplakDecoy(t *testing.T) {
	d := openTestDB(t)
	fakeLoot, err := d.GetNPCLoot(210403)
	if err != nil {
		t.Fatalf("GetNPCLoot(210403): %v", err)
	}
	if fakeLoot != nil {
		for _, drop := range fakeLoot.Drops {
			if drop.Name == "Script-granted loot" {
				t.Errorf("the fake pre-fight Jeplak (210403) should not carry scripted loot")
			}
		}
	}

	realLoot, err := d.GetNPCLoot(210472)
	if err != nil {
		t.Fatalf("GetNPCLoot(210472): %v", err)
	}
	if realLoot == nil {
		t.Fatal("expected non-nil loot table for the real Jeplak")
	}
	found := false
	for _, drop := range realLoot.Drops {
		if drop.Name == "Script-granted loot" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the real Jeplak (210472) to carry scripted loot, got %+v", realLoot.Drops)
	}
}
