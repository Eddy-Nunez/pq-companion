package buffmod

import "github.com/jasonsoprovich/pq-companion/backend/internal/db"

// CategoryDef is one named kind of spell focus (what players call "Spell
// Haste", "Affliction Efficiency", …). A focus SPA alone is too coarse — SPA 127
// covers Spell Haste, Affliction Haste, Enhancement Haste and the Summoning /
// Reanimation variants — so a category is the SPA plus the limits that scope it.
// Only the best focus within a category matters, which is what the Spell
// Modifiers "missing" and "upgrades" views are built on.
type CategoryDef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Blurb string `json:"blurb"`
	SPA   int    `json:"spa"`
	// NoMissingHint keeps a category out of the "missing focus types" list: it
	// is real gear but not something a spellcaster is expected to want.
	NoMissingHint bool `json:"-"`
	// Rolls marks foci whose percent is a per-cast random roll of 1..N% rather
	// than a fixed value (every focus but spell haste and range, per the wiki).
	Rolls bool `json:"rolls"`
}

// Categories lists every category in display order. Derived from the focus
// spells carried by items in quarm.db; single-spell foci (SPA 139 whitelist,
// e.g. Chromosphere) belong to no category.
var Categories = []CategoryDef{
	{ID: "spell_haste", Label: "Spell Haste", Blurb: "Faster casting for all spells with a long enough cast time.", SPA: SPACastTime},
	{ID: "affliction_haste", Label: "Affliction Haste", Blurb: "Faster casting for damage-over-time and debuff spells.", SPA: SPACastTime},
	{ID: "enhancement_haste", Label: "Enhancement Haste", Blurb: "Faster casting for long-duration beneficial spells.", SPA: SPACastTime},
	{ID: "summoning_haste", Label: "Summoning Haste", Blurb: "Faster casting for summoned pets.", SPA: SPACastTime},
	{ID: "reanimation_haste", Label: "Reanimation Haste", Blurb: "Faster casting for necromancer undead pets.", SPA: SPACastTime},
	{ID: "damage", Label: "Improved Damage", Blurb: "More damage from direct-damage spells of any resist type.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "damage_magic", Label: "Magic Damage", Blurb: "More damage from direct magic-resist spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "damage_fire", Label: "Fire Damage", Blurb: "More damage from direct fire spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "damage_cold", Label: "Cold Damage", Blurb: "More damage from direct cold spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "damage_poison", Label: "Poison Damage", Blurb: "More damage from direct poison spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "damage_disease", Label: "Disease Damage", Blurb: "More damage from direct disease spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "dot_damage", Label: "Affliction Damage", Blurb: "More damage from damage-over-time spells.", SPA: SPAImprovedDamage, Rolls: true},
	{ID: "healing", Label: "Improved Healing", Blurb: "More healing from direct heals (complete heals excluded for clerics, druids and shamans).", SPA: SPAImprovedHeal, Rolls: true},
	{ID: "resist_reduction", Label: "Resist Reduction", Blurb: "Lowers the target's resist against your spells.", SPA: SPAResistReduction},
	{ID: "mana_preservation", Label: "Mana Preservation", Blurb: "Reduced mana cost.", SPA: SPAManaCost, Rolls: true},
	{ID: "enhancement_efficiency", Label: "Enhancement Efficiency", Blurb: "Reduced mana cost for beneficial spells.", SPA: SPAManaCost, Rolls: true},
	{ID: "affliction_efficiency", Label: "Affliction Efficiency", Blurb: "Reduced mana cost for damage-over-time spells.", SPA: SPAManaCost, Rolls: true},
	{ID: "summoning_efficiency", Label: "Summoning Efficiency", Blurb: "Reduced mana cost for summoned pets.", SPA: SPAManaCost, Rolls: true},
	{ID: "reanimation_efficiency", Label: "Reanimation Efficiency", Blurb: "Reduced mana cost for necromancer undead pets.", SPA: SPAManaCost, Rolls: true},
	{ID: "extended_enhancement", Label: "Extended Enhancement", Blurb: "Longer duration for beneficial spells.", SPA: SPADuration, Rolls: true},
	{ID: "extended_affliction", Label: "Extended Affliction", Blurb: "Longer duration for detrimental spells.", SPA: SPADuration, Rolls: true},
	{ID: "extended_range", Label: "Extended Range", Blurb: "Longer spell range.", SPA: SPAIncreaseRange},
	{ID: "reagent_conservation", Label: "Reagent Conservation", Blurb: "Chance to not consume spell reagents.", SPA: SPAReagentConserve, Rolls: true},
	{ID: "aggro_reduction", Label: "Spell Aggro Reduction", Blurb: "Less hate from the spells you cast.", SPA: SPASpellHate},
	{ID: "bash_hate", Label: "Bash Hate", Blurb: "More hate from shield bashes (tank gear).", SPA: SPASpellHate, NoMissingHint: true},
}

// damageResistCategory maps a SPA 135 resist type to its school-limited damage
// category (the wiki's "Anger/Fury/Wrath of Ro/E`ci/Druzzil…" lines).
var damageResistCategory = map[int]string{
	1: "damage_magic", 2: "damage_fire", 3: "damage_cold", 4: "damage_poison", 5: "damage_disease",
}

// SPA 137 target/effect codes that scope a focus to a pet-summoning line.
const (
	effectSummonPet = 33 // SE_SummonPet — mage/necro/beastlord pets
	effectNecPet    = 71 // SE_NecPet — necromancer undead pets

)

// CategoryOf returns the category ID for a focus modifier (its SPA plus
// limits), or "" when it is a single-spell focus or an unrecognised shape.
func CategoryOf(spa, percent int, l Limits) string {
	switch spa {
	case SPAImprovedDamage:
		if !l.InstantOnly && l.MinDurationSec > 0 {
			return "dot_damage"
		}
		if id, ok := damageResistCategory[l.ResistType]; ok {
			return id
		}
		return "damage"
	case SPAImprovedHeal:
		return "healing"
	case SPAResistReduction:
		return "resist_reduction"
	case SPACastTime:
		switch {
		case hasInt(l.IncludeEffects, effectSummonPet):
			return "summoning_haste"
		case hasInt(l.IncludeEffects, effectNecPet):
			return "reanimation_haste"
		case l.MinDurationSec > 0 && l.SpellType == SpellTypeDetrimental:
			return "affliction_haste"
		case l.MinDurationSec > 0 && l.SpellType == SpellTypeBeneficial:
			return "enhancement_haste"
		}
		return "spell_haste"
	case SPADuration:
		if l.SpellType == SpellTypeBeneficial {
			return "extended_enhancement"
		}
		return "extended_affliction"
	case SPAIncreaseRange:
		return "extended_range"
	case SPASpellHate:
		// Negative = the real spell aggro reduction; positive foci in the data
		// are the "Furious Bash" shield-bash hate line (warrior/paladin/SK).
		if percent < 0 {
			return "aggro_reduction"
		}
		return "bash_hate"
	case SPAReagentConserve:
		return "reagent_conservation"
	case SPAManaCost:
		switch {
		case len(l.IncludeSpells) > 0:
			return "" // single-spell focus
		case hasInt(l.IncludeEffects, effectSummonPet):
			return "summoning_efficiency"
		case hasInt(l.IncludeEffects, effectNecPet):
			return "reanimation_efficiency"
		case l.MinDurationSec > 0 && l.SpellType == SpellTypeDetrimental:
			return "affliction_efficiency"
		case l.SpellType == SpellTypeBeneficial:
			return "enhancement_efficiency"
		}
		return "mana_preservation"
	}
	return ""
}

// CategoryDefFor returns a category's definition and whether the id is known.
func CategoryDefFor(id string) (CategoryDef, bool) {
	for _, c := range Categories {
		if c.ID == id {
			return c, true
		}
	}
	return CategoryDef{}, false
}

// CategoryLabel returns a category's display label ("" for an unknown id).
func CategoryLabel(id string) string {
	for _, c := range Categories {
		if c.ID == id {
			return c.Label
		}
	}
	return ""
}

// FocusModifiers parses a focus spell into its modifiers, each tagged with its
// category. It is the exported form of parseFocusSpell for callers (the focus
// upgrade browser) that need the same limit parsing for items they don't wear.
func FocusModifiers(s *db.Spell) []Modifier {
	mods := parseFocusSpell(s)
	for i := range mods {
		mods[i].Category = CategoryOf(mods[i].SPA, mods[i].Percent, mods[i].Limits)
	}
	return mods
}

func hasInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
