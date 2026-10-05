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
}

// Categories lists every category in display order. Derived from the focus
// spells carried by items in quarm.db; single-spell foci (SPA 139 whitelist,
// e.g. Chromosphere) belong to no category.
var Categories = []CategoryDef{
	{"spell_haste", "Spell Haste", "Faster casting for all spells with a long enough cast time.", SPACastTime},
	{"affliction_haste", "Affliction Haste", "Faster casting for damage-over-time spells.", SPACastTime},
	{"enhancement_haste", "Enhancement Haste", "Faster casting for long-duration beneficial spells.", SPACastTime},
	{"summoning_haste", "Summoning Haste", "Faster casting for summoned pets.", SPACastTime},
	{"reanimation_haste", "Reanimation Haste", "Faster casting for necromancer undead pets.", SPACastTime},
	{"damage", "Spell Damage", "More damage from direct-damage spells.", SPAImprovedDamage},
	{"dot_damage", "Affliction Damage", "More damage from damage-over-time spells.", SPAImprovedDamage},
	{"healing", "Healing", "More healing from direct heals.", SPAImprovedHeal},
	{"resist_reduction", "Resist Reduction", "Lowers the target's resist against your spells.", SPAResistReduction},
	{"mana_preservation", "Mana Preservation", "Reduced mana cost.", SPAManaCost},
	{"enhancement_efficiency", "Enhancement Efficiency", "Reduced mana cost for beneficial spells.", SPAManaCost},
	{"affliction_efficiency", "Affliction Efficiency", "Reduced mana cost for damage-over-time spells.", SPAManaCost},
	{"summoning_efficiency", "Summoning Efficiency", "Reduced mana cost for summoned pets.", SPAManaCost},
	{"reanimation_efficiency", "Reanimation Efficiency", "Reduced mana cost for necromancer undead pets.", SPAManaCost},
	{"extended_enhancement", "Extended Enhancement", "Longer duration for beneficial spells.", SPADuration},
	{"extended_affliction", "Extended Affliction", "Longer duration for detrimental spells.", SPADuration},
	{"extended_range", "Extended Range", "Longer spell range.", SPAIncreaseRange},
	{"reagent_conservation", "Reagent Conservation", "Chance to not consume spell reagents.", SPAReagentConserve},
	{"spell_hate", "Spell Hate", "Changes the hate your spells generate.", SPASpellHate},
}

// SPA 137 target/effect codes that scope a focus to a pet-summoning line.
const (
	effectSummonPet = 33 // SE_SummonPet — mage/necro/beastlord pets
	effectNecPet    = 71 // SE_NecPet — necromancer undead pets

)

// CategoryOf returns the category ID for a focus modifier (its SPA plus
// limits), or "" when it is a single-spell focus or an unrecognised shape.
func CategoryOf(spa int, l Limits) string {
	switch spa {
	case SPAImprovedDamage:
		if !l.InstantOnly && l.MinDurationSec > 0 {
			return "dot_damage"
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
		return "spell_hate"
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
		mods[i].Category = CategoryOf(mods[i].SPA, mods[i].Limits)
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
