package db_test

import (
	"slices"
	"testing"
)

// TestNPCZones covers a multi-zone NPC, a single-zone NPC, a spawn2-less
// (script-spawned) NPC resolved from its id, and an id resolved by neither.
func TestNPCZones(t *testing.T) {
	d := openTestDB(t)

	tests := []struct {
		name         string
		id           int
		wantShort    []string
		wantInferred bool
	}{
		{"multi-zone", 772, []string{"erudnext", "erudsxing", "qeynos"}, false},
		{"single zone", 32040, []string{"soldungb"}, false},
		{"script-spawned id fallback", 218414, []string{"poeartha"}, true},
		{"unknown id", 999999999, nil, false},
	}

	ids := make([]int, len(tests))
	for i, tc := range tests {
		ids[i] = tc.id
	}
	zones, err := d.NPCZones(ids)
	if err != nil {
		t.Fatalf("NPCZones: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, z := range zones[tc.id] {
				got = append(got, z.ShortName)
				if z.LongName == "" {
					t.Errorf("zone %q has empty LongName", z.ShortName)
				}
				if z.Inferred != tc.wantInferred {
					t.Errorf("zone %q Inferred: got %v, want %v", z.ShortName, z.Inferred, tc.wantInferred)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.wantShort) {
				t.Errorf("zones: got %v, want %v", got, tc.wantShort)
			}
		})
	}
}
