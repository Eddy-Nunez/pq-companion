package buffmod_test

import (
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/buffmod"
)

// TestFocusModifiersCategory pins the category each real focus spell lands in,
// so a tweak to CategoryOf can't silently move, say, Affliction Haste into Spell
// Haste.
func TestFocusModifiersCategory(t *testing.T) {
	d := openDB(t)
	tests := []struct {
		spellID int
		name    string
		want    string
	}{
		{2341, "Spell Haste III", "spell_haste"},
		{3529, "Speed of Solusek", "spell_haste"}, // detrimental-only but no duration gate
		{2360, "Affliction Haste I", "affliction_haste"},
		{2358, "Enhancement Haste II", "enhancement_haste"},
		{2351, "Summoning Haste I", "summoning_haste"},
		{2336, "Improved Damage I", "damage"},
		{2366, "Burning Affliction I", "dot_damage"},
		{2345, "Improved Healing I", "healing"},
		{2333, "Extended Enhancement I", "extended_enhancement"},
		{3843, "Timeburn", "extended_affliction"},
		{2348, "Extended Range I", "extended_range"},
		{2330, "Reagent Conservation I", "reagent_conservation"},
		{2342, "Mana Preservation I", "mana_preservation"},
		{3544, "Conservation of Mithaniel", "enhancement_efficiency"},
		{2363, "Affliction Efficiency I", "affliction_efficiency"},
		{2372, "Reanimation Efficiency I", "reanimation_efficiency"},
		{3085, "Chromosphere", ""}, // single-spell focus
		{3517, "Fury of Ro", "damage_fire"},
		{3519, "Anger of E`ci", "damage_cold"},
		{3522, "Anger of Druzzil", "damage_magic"},
		{3923, "Fury of Bertoxxulous", "damage_disease"},
		{2456, "Furious Bash II", "bash_hate"},
		{3549, "Innoruuk's Sarcasm", "aggro_reduction"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, err := d.GetSpell(tt.spellID)
			if err != nil {
				t.Fatalf("GetSpell(%d): %v", tt.spellID, err)
			}
			mods := buffmod.FocusModifiers(sp)
			if len(mods) == 0 {
				t.Fatalf("%s produced no modifiers", tt.name)
			}
			if got := mods[0].Category; got != tt.want {
				t.Errorf("%s category = %q, want %q (limits %+v)", tt.name, got, tt.want, mods[0].Limits)
			}
		})
	}
}

func TestCategoryLabels(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range buffmod.Categories {
		if seen[c.ID] {
			t.Errorf("duplicate category id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Label == "" || buffmod.CategoryLabel(c.ID) != c.Label {
			t.Errorf("category %q label mismatch", c.ID)
		}
	}
}

// TestEveryItemFocusIsCategorised walks every focus spell carried by an item in
// quarm.db. Each must land in a known category, or be a single-spell focus
// (SPA 139 whitelist) that deliberately has none — so a future dump that adds
// a focus with a new shape fails here instead of silently vanishing from the
// Spell Modifiers views.
func TestEveryItemFocusIsCategorised(t *testing.T) {
	d := openDB(t)
	ids, err := d.ItemFocusSpellIDs()
	if err != nil {
		t.Fatalf("ItemFocusSpellIDs: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("no item focus spells found")
	}
	for _, id := range ids {
		sp, err := d.GetSpell(id)
		if err != nil || sp == nil {
			continue
		}
		for _, m := range buffmod.FocusModifiers(sp) {
			if m.Category == "" {
				if len(m.Limits.IncludeSpells) == 0 {
					t.Errorf("%s (%d): SPA %d %d%% has no category and is not single-spell (limits %+v)",
						sp.Name, id, m.SPA, m.Percent, m.Limits)
				}
				continue
			}
			def, ok := buffmod.CategoryDefFor(m.Category)
			if !ok {
				t.Errorf("%s (%d): unknown category %q", sp.Name, id, m.Category)
			} else if def.SPA != m.SPA {
				t.Errorf("%s (%d): category %q is SPA %d but the focus is SPA %d", sp.Name, id, m.Category, def.SPA, m.SPA)
			}
		}
	}
}
