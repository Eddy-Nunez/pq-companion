package db_test

import (
	"reflect"
	"testing"
)

func TestGetSpellComponents(t *testing.T) {
	d := openTestDB(t)

	type comp struct {
		ID    int
		Count int
		Kind  string
	}
	tests := []struct {
		name    string
		spellID int
		want    []comp
	}{
		{"Rune V consumes a Peridot", 1689, []comp{{10028, 1, "consumed"}}},
		{"Gate needs nothing", 36, nil},
		{"Purify Mana consumes multiple slots and counts", 696, []comp{{10035, 4, "consumed"}, {16965, 1, "consumed"}}},
		{"Flame Lick needs Fire Beetle Eye, not consumed", 239, []comp{{10307, 1, "required"}}},
		{"Elementalkin: Earth pet focus is optional", 58, []comp{{10015, 1, "consumed"}, {6361, 1, "pet_focus"}}},
		{"Bard song reports instrument only", 1449, []comp{{13012, 1, "instrument"}}},
		{"Bard song with no NoexpendReagent needs nothing", 702, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.GetSpellComponents(tt.spellID)
			if err != nil {
				t.Fatalf("GetSpellComponents: %v", err)
			}
			var gotC []comp
			for _, c := range got {
				if c.Name == "" {
					t.Errorf("component %d has empty name", c.ID)
				}
				gotC = append(gotC, comp{c.ID, c.Count, c.Kind})
			}
			if !reflect.DeepEqual(gotC, tt.want) {
				t.Errorf("got %+v, want %+v", gotC, tt.want)
			}
		})
	}
}

// TestGetItem_ScrollEffect verifies scroll items expose the spell they teach
// so the item page can link to it.
func TestGetItem_ScrollEffect(t *testing.T) {
	d := openTestDB(t)
	it, err := d.GetItem(19377) // Spell: Rune V
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if it.ScrollEffect != 1689 || it.ScrollName != "Rune V" {
		t.Errorf("got scroll %d %q, want 1689 \"Rune V\"", it.ScrollEffect, it.ScrollName)
	}
}
