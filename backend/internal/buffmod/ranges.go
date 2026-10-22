package buffmod

import (
	"fmt"
	"math"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
	"github.com/jasonsoprovich/pq-companion/backend/internal/zeal"
)

// BaseSpellRange is the range of most nukes and debuffs, which the NPC
// overlay's distance readout is coloured against.
const BaseSpellRange = 200

// itemTypeBow is items.itemtype for bows; their range adds the arrow's.
const itemTypeBow = 5

// Ranges is a character's effective spell and ranged-attack reach, worked
// out from their latest Quarmy export, for the NPC overlay's distance readout.
type Ranges struct {
	Character string `json:"character"`
	// HasExport is false when the character has no Quarmy export yet; the
	// values are then the defaults (200, no ranged weapon).
	HasExport bool `json:"has_export"`
	// CastRange is BaseSpellRange extended by the best worn Extended Range
	// focus (SPA 129). Foci carry their own limits (spell type, level), so
	// this is the reach of a typical nuke or debuff the focus applies to.
	CastRange        int    `json:"cast_range"`
	CastFocusPercent int    `json:"cast_focus_percent"`
	CastFocusItem    string `json:"cast_focus_item,omitempty"`
	CastFocusName    string `json:"cast_focus_name,omitempty"`
	// RangedRange is the Range-slot weapon's range, plus the arrow's for a
	// bow; 0 when nothing with a range is equipped there.
	RangedRange int    `json:"ranged_range"`
	RangedItem  string `json:"ranged_item,omitempty"`
	AmmoItem    string `json:"ammo_item,omitempty"`
}

// ComputeRanges returns charName's effective ranges from their latest Quarmy
// export.
func ComputeRanges(eqPath, charName string, gameDB *db.DB) (*Ranges, error) {
	mods, err := Compute(eqPath, charName, gameDB)
	if err != nil {
		return nil, fmt.Errorf("compute ranges: %w", err)
	}
	out := &Ranges{Character: charName, HasExport: true, CastRange: BaseSpellRange}

	// Extended Range foci don't stack: the best one applies.
	for _, m := range mods.Contributors {
		if m.SPA == SPAIncreaseRange && m.Percent > out.CastFocusPercent {
			out.CastFocusPercent = m.Percent
			out.CastFocusItem = m.SourceItemName
			out.CastFocusName = m.FocusSpellName
		}
	}
	out.CastRange = int(math.Round(float64(BaseSpellRange) * (100 + float64(out.CastFocusPercent)) / 100))

	q, err := zeal.ParseQuarmy(zeal.FindQuarmyFile(eqPath, charName), charName)
	if err != nil {
		return nil, fmt.Errorf("compute ranges: parse quarmy: %w", err)
	}
	var ranged, ammo *db.Item
	for _, e := range q.Inventory {
		if e.Location != "Range" && e.Location != "Ammo" {
			continue
		}
		item, err := gameDB.GetItem(e.ID)
		if err != nil || item == nil {
			continue
		}
		if e.Location == "Range" {
			ranged = item
		} else {
			ammo = item
		}
	}
	if ranged != nil && ranged.Range > 0 {
		out.RangedRange = ranged.Range
		out.RangedItem = ranged.Name
		if ranged.ItemType == itemTypeBow && ammo != nil && ammo.Range > 0 {
			out.RangedRange += ammo.Range
			out.AmmoItem = ammo.Name
		}
	}
	return out, nil
}
