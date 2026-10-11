package db_test

import (
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

// TestGetZoneNPCSpawns checks spawn shares are normalized over the era-active
// entries of a group rather than read raw from spawnentry.chance.
func TestGetZoneNPCSpawns(t *testing.T) {
	d := openTestDB(t)

	cases := []struct {
		name string
		zone string
		npc  int
		// share is the NPC's expected share in one known group; it must fall
		// within the NPC's [MinPct, MaxPct] range across all its groups.
		share float64
	}{
		// Crushbone "low cents": pawn 20 / centurion 80, sums to 100.
		{"crushbone centurion", "crushbone", 58009, 80},
		{"crushbone pawn", "crushbone", 58001, 20},
		// Qeynos Hills group 2017: raw weights sum to 175; the Luclin+-era
		// active set sums to 120, so a 15-weight bat is 12.5%, not 15%.
		{"qeynos2 brown myotis bat", "qeynos2", 2017, 12.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spawns, err := d.GetZoneNPCSpawns(c.zone)
			if err != nil {
				t.Fatalf("GetZoneNPCSpawns(%q): %v", c.zone, err)
			}
			var got *db.ZoneNPCSpawn
			for i := range spawns {
				if spawns[i].NPCID == c.npc {
					got = &spawns[i]
				}
			}
			if got == nil {
				t.Fatalf("npc %d missing from %q", c.npc, c.zone)
			}
			if c.share < got.MinPct-0.01 || c.share > got.MaxPct+0.01 {
				t.Errorf("share %.2f outside [%.2f, %.2f]", c.share, got.MinPct, got.MaxPct)
			}
			if got.Points < 1 {
				t.Errorf("points = %d, want >= 1", got.Points)
			}
		})
	}

	// Group 2017's a_bat (2317) is gated to max_expansion 2.9 in every group
	// it's in, so in the PoP era it comes back as an other-era row, not a
	// share.
	spawns, err := d.GetZoneNPCSpawns("qeynos2")
	if err != nil {
		t.Fatalf("GetZoneNPCSpawns: %v", err)
	}
	sawGated := false
	for _, s := range spawns {
		if s.Inactive != "" {
			if s.Points != 0 || s.MaxPct != 0 {
				t.Errorf("inactive npc %d carries a share: %+v", s.NPCID, s)
			}
			if s.NPCID == 2317 {
				sawGated = true
				if s.Inactive != db.SpawnInactiveOtherEra {
					t.Errorf("npc 2317 inactive = %q, want %q", s.Inactive, db.SpawnInactiveOtherEra)
				}
			}
			continue
		}
		if s.MinPct <= 0 || s.MaxPct > 100 {
			t.Errorf("npc %d share out of range: [%.2f, %.2f]", s.NPCID, s.MinPct, s.MaxPct)
		}
		if s.NPCID == 2317 {
			t.Errorf("era-gated npc 2317 should be inactive, got %+v", s)
		}
	}
	if !sawGated {
		t.Error("era-gated npc 2317 missing from inactive rows")
	}
}

// TestGetVariantTraitsSpawnChanceNormalized checks the NPC overlay's variant
// hint uses the same normalized share as the Zones tab, not the raw weight.
func TestGetVariantTraitsSpawnChanceNormalized(t *testing.T) {
	d := openTestDB(t)
	traits, err := d.GetVariantTraits([]int{2017, 2317}, "qeynos2")
	if err != nil {
		t.Fatalf("GetVariantTraits: %v", err)
	}
	spawns, err := d.GetZoneNPCSpawns("qeynos2")
	if err != nil {
		t.Fatalf("GetZoneNPCSpawns: %v", err)
	}
	for _, s := range spawns {
		if s.NPCID == 2017 {
			want := float64(int(s.MaxPct*10+0.5)) / 10
			if traits[2017].SpawnChance != want {
				t.Errorf("2017 SpawnChance = %v, want %v (zones tab max share)", traits[2017].SpawnChance, want)
			}
		}
	}
	if traits[2317].SpawnChance != -1 {
		t.Errorf("other-era 2317 SpawnChance = %v, want -1", traits[2317].SpawnChance)
	}
}
