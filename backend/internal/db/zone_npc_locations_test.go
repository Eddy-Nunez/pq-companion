package db_test

import (
	"testing"
)

// TestGetRaidTargetsByZonePoP verifies the Zones → Lockouts tab surfaces the
// Planes of Power raid bosses that gained npc_types.loot_lockout in the
// 2026-09-27 dump but never had raid_target=1 or (for the guild-instance
// encounters) any spawn2 row at all — see the GetRaidTargetsByZone doc
// comment. Saryrn is spawn2-backed (raid_target=0, loot_lockout>0); Fennin Ro
// is purely script-spawned (no spawn2 row anywhere).
func TestGetRaidTargetsByZonePoP(t *testing.T) {
	d := openTestDB(t)

	cases := []struct {
		zone string
		want string
	}{
		{"potorment", "Saryrn"},
		{"pofire", "Fennin Ro the Tyrant of Fire"},
		{"bothunder", "Agnarr the Storm Lord"},
		{"hohonora", "Freegan Haun"}, // Halls of Honor trial NPC, no spawn2
	}
	for _, c := range cases {
		t.Run(c.zone, func(t *testing.T) {
			targets, err := d.GetRaidTargetsByZone(c.zone)
			if err != nil {
				t.Fatalf("GetRaidTargetsByZone(%q): %v", c.zone, err)
			}
			found := false
			for _, tgt := range targets {
				if tgt.Name == c.want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("GetRaidTargetsByZone(%q) missing %q, got %+v", c.zone, c.want, targets)
			}
		})
	}
}

// TestGetRaidTargetsByZoneNoDuplicateNames guards against the script-spawned
// second query re-adding a boss under a different npc_types id after the
// primary spawn2-linked query (or a RaidTargetOverrides entry) already found
// it under a same-displayed-name row — e.g. Vallon Zek has both a spawn2-
// backed id (214316) and a script-spawned duplicate (214317).
func TestGetRaidTargetsByZoneNoDuplicateNames(t *testing.T) {
	d := openTestDB(t)
	targets, err := d.GetRaidTargetsByZone("potactics")
	if err != nil {
		t.Fatalf("GetRaidTargetsByZone(potactics): %v", err)
	}
	seen := map[string]int{}
	for _, tgt := range targets {
		seen[tgt.Name]++
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("boss %q appears %d times in potactics, want 1", name, n)
		}
	}
	if seen["Vallon Zek"] == 0 {
		t.Errorf("expected Vallon Zek in potactics, got %+v", targets)
	}
}

// TestGetRaidTargetsByZoneSsratempleUnchanged is a regression guard: the
// ssratemple RaidTargetOverrides (Emperor Ssraeshza, the Vyzh`dra pair, etc.)
// must still resolve correctly now that the script-spawned query could in
// principle also match them (it does, for some — the name-based dedup in
// `add` must prevent that from producing a duplicate or fighting the curated
// override's chosen id).
func TestGetRaidTargetsByZoneSsratempleUnchanged(t *testing.T) {
	d := openTestDB(t)
	targets, err := d.GetRaidTargetsByZone("ssratemple")
	if err != nil {
		t.Fatalf("GetRaidTargetsByZone(ssratemple): %v", err)
	}
	want := map[string]bool{
		"Emperor Ssraeshza":   false,
		"Vyzh`dra the Exiled": false,
		"Vyzh`dra the Cursed": false,
	}
	seen := map[string]int{}
	for _, tgt := range targets {
		seen[tgt.Name]++
		if _, ok := want[tgt.Name]; ok {
			want[tgt.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("ssratemple missing expected boss %q, got %+v", name, targets)
		}
	}
	for name, n := range seen {
		if n > 1 {
			t.Errorf("boss %q appears %d times in ssratemple, want 1", name, n)
		}
	}
}
