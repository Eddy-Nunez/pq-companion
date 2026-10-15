package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// petFocusItems mirrors petfocusItems[] in EQMacEmu Mob::HasSpellReagent:
// NoexpendReagent entries that are pet focus items, which the server skips
// when checking reagents — they're optional and only strengthen the pet.
var petFocusItems = map[int]bool{
	20508: true, 28144: true, 11571: true, 11569: true, 11567: true,
	11566: true, 11568: true, 6360: true, 6361: true, 6362: true, 6363: true,
}

// songInstruments are the NoexpendReagent1 values EQMacEmu
// Mob::HasSongInstrument treats as an instrument type (any instrument with
// that modifier satisfies it): Hand Drum, Wooden Flute, Lute, Horn.
var songInstruments = map[int]bool{13000: true, 13001: true, 13011: true, 13012: true}

// GetSpellComponents returns the items a spell needs at cast time. Bard songs
// (bard class level < 255) never consume components server-side, so for them
// only the instrument requirement from NoexpendReagent1 is reported.
func (db *DB) GetSpellComponents(spellID int) ([]SpellComponent, error) {
	var comp, counts, noexp [4]int
	var bardLevel int
	err := db.QueryRow(`
		SELECT components1, components2, components3, components4,
		       component_counts1, component_counts2, component_counts3, component_counts4,
		       NoexpendReagent1, NoexpendReagent2, NoexpendReagent3, NoexpendReagent4,
		       classes8
		FROM spells_new WHERE id = ?`, spellID,
	).Scan(&comp[0], &comp[1], &comp[2], &comp[3],
		&counts[0], &counts[1], &counts[2], &counts[3],
		&noexp[0], &noexp[1], &noexp[2], &noexp[3],
		&bardLevel)
	if errors.Is(err, sql.ErrNoRows) {
		return []SpellComponent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get spell components %d: %w", spellID, err)
	}

	var want []SpellComponent
	if bardLevel < 255 {
		if songInstruments[noexp[0]] {
			want = append(want, SpellComponent{ID: noexp[0], Count: 1, Kind: "instrument"})
		}
	} else {
		for i := range comp {
			if comp[i] <= 0 {
				continue
			}
			n := counts[i]
			if n < 1 {
				n = 1
			}
			want = append(want, SpellComponent{ID: comp[i], Count: n, Kind: "consumed"})
		}
		for _, id := range noexp {
			if id <= 0 {
				continue
			}
			kind := "required"
			if petFocusItems[id] {
				kind = "pet_focus"
			}
			want = append(want, SpellComponent{ID: id, Count: 1, Kind: kind})
		}
	}

	out := make([]SpellComponent, 0, len(want))
	for _, c := range want {
		err := db.QueryRow("SELECT name, icon FROM items WHERE id = ?", c.ID).
			Scan(&c.Name, &c.Icon)
		if errors.Is(err, sql.ErrNoRows) {
			continue // dangling item reference; nothing useful to show
		}
		if err != nil {
			return nil, fmt.Errorf("get spell component item %d: %w", c.ID, err)
		}
		out = append(out, c)
	}
	return out, nil
}
