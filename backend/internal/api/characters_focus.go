package api

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jasonsoprovich/pq-companion/backend/internal/buffmod"
	"github.com/jasonsoprovich/pq-companion/backend/internal/character"
	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
	"github.com/jasonsoprovich/pq-companion/backend/internal/db/enums"
	"github.com/jasonsoprovich/pq-companion/backend/internal/zeal"
)

// Classes (0-based) that don't cast spells, so "missing spell focus" hints would
// only be noise: Warrior, Monk, Rogue, Bard.
var nonCasterClasses = map[int]bool{0: true, 6: true, 7: true, 8: true}

// maxFocusUpgradeItems caps one category's upgrade list.
const maxFocusUpgradeItems = 60

// focusItem is one item a character could wear for a focus category.
type focusItem struct {
	ItemID       int            `json:"item_id"`
	Name         string         `json:"name"`
	Icon         int            `json:"icon"`
	ReqLevel     int            `json:"req_level"`
	NoDrop       bool           `json:"no_drop"`
	FocusSpellID int            `json:"focus_spell_id"`
	FocusName    string         `json:"focus_name"`
	Percent      int            `json:"percent"`
	Limits       buffmod.Limits `json:"limits"`
	// Equipped is true when the character is wearing this item now.
	Equipped bool `json:"equipped"`
	// IsUpgrade is true when the item's focus beats the best one currently worn
	// in this category (never set for categories where "higher" isn't "better").
	IsUpgrade bool `json:"is_upgrade"`
}

// missingFocusCategory is a category the character has no worn focus for but
// could get from at least one item usable by their class and level.
type missingFocusCategory struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Blurb     string `json:"blurb"`
	ItemCount int    `json:"item_count"`
}

// focusCandidates returns every item the character's class/race/level can use
// that carries a categorised focus, grouped by category.
func (h *charactersHandler) focusCandidates(char character.Character) (map[string][]focusItem, error) {
	cands, err := h.db.UpgradeCandidates(db.CandidateFilter{
		SlotMask:  0x7FFFFFFF, // every slot — a focus is on the item, not the slot
		ClassBit:  charClassBit(char),
		RaceBit:   enums.RaceBitForCharRace(char.Race),
		MaxLevel:  char.Level,
		FocusOnly: true,
	})
	if err != nil {
		return nil, err
	}
	spells := map[int][]buffmod.Modifier{}
	out := map[string][]focusItem{}
	for _, c := range cands {
		mods, ok := spells[c.FocusEffect]
		if !ok {
			if sp, err := h.db.GetSpell(c.FocusEffect); err == nil && sp != nil {
				mods = buffmod.FocusModifiers(sp)
			}
			spells[c.FocusEffect] = mods
		}
		// An item's category is its first categorised focus slot.
		for _, m := range mods {
			if m.Category == "" {
				continue
			}
			out[m.Category] = append(out[m.Category], focusItem{
				ItemID: c.ID, Name: c.Name, Icon: c.Icon, ReqLevel: c.ReqLevel,
				NoDrop: c.NoDrop == 0, FocusSpellID: c.FocusEffect, FocusName: c.FocusName,
				Percent: m.Percent, Limits: m.Limits,
			})
			break
		}
	}
	return out, nil
}

// missingFocusCategories lists the categories the character could use but has
// no worn contributor for, in display order. Empty for non-casting classes.
func (h *charactersHandler) missingFocusCategories(char character.Character, contributors []buffmod.Modifier) []missingFocusCategory {
	missing := []missingFocusCategory{}
	if nonCasterClasses[char.Class] {
		return missing
	}
	have := map[string]bool{}
	for _, m := range contributors {
		if m.Category != "" {
			have[m.Category] = true
		}
	}
	cands, err := h.focusCandidates(char)
	if err != nil {
		return missing
	}
	for _, def := range buffmod.Categories {
		if have[def.ID] || len(cands[def.ID]) == 0 {
			continue
		}
		missing = append(missing, missingFocusCategory{
			ID: def.ID, Label: def.Label, Blurb: def.Blurb, ItemCount: len(cands[def.ID]),
		})
	}
	return missing
}

type focusUpgradesResponse struct {
	Category string `json:"category"`
	Label    string `json:"label"`
	Blurb    string `json:"blurb"`
	SPA      int    `json:"spa"`
	// Current lists what the character wears in this category now.
	Current []buffmod.Modifier `json:"current"`
	// CurrentPercent is the best worn percent (0 when none) — only the best
	// focus in a category applies.
	CurrentPercent int         `json:"current_percent"`
	Items          []focusItem `json:"items"`
}

// focusUpgrades handles GET /api/characters/{id}/focus-upgrades?category=<id>:
// what the character wears for a focus category, plus every item their class and
// level can use for it, best first.
func (h *charactersHandler) focusUpgrades(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	char, ok, err := h.store.Get(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "character not found")
		return
	}
	catID := r.URL.Query().Get("category")
	var def *buffmod.CategoryDef
	for i := range buffmod.Categories {
		if buffmod.Categories[i].ID == catID {
			def = &buffmod.Categories[i]
		}
	}
	if def == nil {
		writeError(w, http.StatusBadRequest, "unknown category")
		return
	}

	// What's worn now (best effort — no export just means nothing is worn).
	current := []buffmod.Modifier{}
	cfg := h.mgr.Get()
	if cfg.EQPath != "" && zeal.FindQuarmyFile(cfg.EQPath, char.Name) != "" {
		if res, err := buffmod.Compute(cfg.EQPath, char.Name, h.db); err == nil {
			for _, m := range res.Contributors {
				if m.Category == catID {
					current = append(current, m)
				}
			}
		}
	}
	best := 0
	wornItems := map[int]bool{}
	for _, m := range current {
		if m.Percent > best {
			best = m.Percent
		}
		if m.SourceItemID > 0 {
			wornItems[m.SourceItemID] = true
		}
	}

	cands, err := h.focusCandidates(char)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := cands[catID]
	// For hate, a bigger number isn't an upgrade (it depends on tank vs. caster).
	comparable := catID != "spell_hate"
	for i := range items {
		items[i].Equipped = wornItems[items[i].ItemID]
		items[i].IsUpgrade = comparable && items[i].Percent > best
	}
	sort.SliceStable(items, func(a, b int) bool {
		if items[a].Percent != items[b].Percent {
			return items[a].Percent > items[b].Percent
		}
		if items[a].ReqLevel != items[b].ReqLevel {
			return items[a].ReqLevel < items[b].ReqLevel
		}
		return items[a].Name < items[b].Name
	})
	if len(items) > maxFocusUpgradeItems {
		items = items[:maxFocusUpgradeItems]
	}
	if items == nil {
		items = []focusItem{}
	}
	writeJSON(w, http.StatusOK, focusUpgradesResponse{
		Category: def.ID, Label: def.Label, Blurb: def.Blurb, SPA: def.SPA,
		Current: current, CurrentPercent: best, Items: items,
	})
}
