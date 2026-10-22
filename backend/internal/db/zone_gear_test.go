package db_test

import (
	"testing"
)

func TestZoneItemSources(t *testing.T) {
	d := openTestDB(t)

	tests := []struct {
		name    string
		zone    string
		itemID  int
		wantNPC string
	}{
		// Quarm is script-spawned (no spawn2 row) — only the id-convention
		// fallback finds his drops.
		{"script-spawned boss", "potimeb", 21993, "Quarm"},
		// Vex Thal's zone-wide pool isn't on any NPC's loottable.
		{"zone-wide pool", "vexthal", 28895, "Vex Thal zone-wide loot"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.ZoneItemSources(tc.zone)
			if err != nil {
				t.Fatalf("ZoneItemSources(%q): %v", tc.zone, err)
			}
			srcs := got[d.CanonicalItemID(tc.itemID)]
			found := false
			for _, s := range srcs {
				if s.Chance <= 0 {
					t.Errorf("item %d source %q has non-positive chance %v", tc.itemID, s.NPCName, s.Chance)
				}
				if s.NPCName == tc.wantNPC {
					found = true
				}
			}
			if !found {
				t.Fatalf("item %d in %s: want source %q, got %+v", tc.itemID, tc.zone, tc.wantNPC, srcs)
			}
		})
	}

	t.Run("unknown zone", func(t *testing.T) {
		got, err := d.ZoneItemSources("not_a_zone")
		if err != nil || len(got) != 0 {
			t.Fatalf("want empty, got %d items, err %v", len(got), err)
		}
	})
}
