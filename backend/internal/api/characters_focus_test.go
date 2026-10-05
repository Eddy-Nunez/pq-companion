package api

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/buffmod"
	"github.com/jasonsoprovich/pq-companion/backend/internal/character"
	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
)

func openFocusTestDB(t *testing.T) *db.DB {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	d, err := db.Open(filepath.Join(filepath.Dir(file), "..", "..", "data", "quarm.db"))
	if err != nil {
		t.Skipf("quarm.db not available: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestMissingFocusCategories(t *testing.T) {
	h := &charactersHandler{db: openFocusTestDB(t)}
	enchanter := character.Character{Class: 13, Race: 1, Level: 60}

	ids := func(m []missingFocusCategory) map[string]bool {
		out := map[string]bool{}
		for _, c := range m {
			out[c.ID] = true
		}
		return out
	}

	// Nothing worn: spell haste is available to an enchanter, so it's missing.
	none := ids(h.missingFocusCategories(enchanter, nil))
	if !none["spell_haste"] {
		t.Errorf("enchanter with no focus should be missing spell_haste, got %v", none)
	}

	// Wearing a spell-haste focus removes just that category.
	worn := []buffmod.Modifier{{SPA: buffmod.SPACastTime, Percent: 15, Category: "spell_haste"}}
	got := ids(h.missingFocusCategories(enchanter, worn))
	if got["spell_haste"] {
		t.Errorf("spell_haste should no longer be missing once worn")
	}
	if len(got) != len(none)-1 {
		t.Errorf("only spell_haste should drop out: before %d, after %d", len(none), len(got))
	}

	// Non-casters get no hints.
	warrior := character.Character{Class: 0, Race: 1, Level: 60}
	if m := h.missingFocusCategories(warrior, nil); len(m) != 0 {
		t.Errorf("warrior should get no missing-focus hints, got %v", m)
	}
}

func TestFocusCandidates_ClassAndLevelFiltered(t *testing.T) {
	h := &charactersHandler{db: openFocusTestDB(t)}
	lowLevel, err := h.focusCandidates(character.Character{Class: 13, Race: 1, Level: 10})
	if err != nil {
		t.Fatal(err)
	}
	high, err := h.focusCandidates(character.Character{Class: 13, Race: 1, Level: 60})
	if err != nil {
		t.Fatal(err)
	}
	count := func(m map[string][]focusItem) int {
		n := 0
		for _, v := range m {
			n += len(v)
		}
		return n
	}
	if count(high) <= count(lowLevel) {
		t.Errorf("level 60 should see more focus items than level 10: %d vs %d", count(high), count(lowLevel))
	}
	for cat, items := range high {
		for _, it := range items {
			if it.ReqLevel > 60 {
				t.Errorf("%s: %s requires level %d > 60", cat, it.Name, it.ReqLevel)
			}
		}
	}
}
