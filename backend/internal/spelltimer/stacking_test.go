package spelltimer

import (
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
	"github.com/jasonsoprovich/pq-companion/backend/internal/ws"
)

func TestBuffOverwrites(t *testing.T) {
	database, err := db.Open("../../data/quarm.db")
	if err != nil {
		t.Skipf("quarm.db not available: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	spell := func(id int) *db.Spell {
		s, err := database.GetSpell(id)
		if err != nil || s == nil {
			t.Fatalf("GetSpell(%d): %v", id, err)
		}
		return s
	}

	tests := []struct {
		name     string
		newID    int
		oldID    int
		expected bool
	}{
		// Mana/WIS/INT line: slot 0 SE_CurrentMana conflicts.
		{"Tranquility over Koadic's", 3350, 2570, true},
		{"Koadic's over Tranquility", 2570, 3350, true},
		{"Tranquility over Voice of Quellious", 3350, 3360, true},
		// Clarity II has a CHA spacer in slot 0; mana regen in slot 1 conflicts.
		{"Tranquility over Clarity II", 3350, 1693, true},
		// HP line: slot 0 is an ignored stacking command, slot 1 max HP conflicts.
		{"Virtue over Aegolism", 3467, 1447, true},
		{"Ancient Aegolism over Aegolism", 2122, 1447, true},
		// Unrelated buffs stack.
		{"Tranquility beside Aegolism", 3350, 1447, false},
		{"Aegolism beside Tranquility", 1447, 3350, false},
		{"Blessing of Reverence beside Tranquility", 3472, 3350, false},
		{"Lunar Whispers beside Tranquility", 2980, 3350, false},
		{"same spell", 3350, 3350, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buffOverwrites(spell(tt.newID), spell(tt.oldID)); got != tt.expected {
				t.Errorf("buffOverwrites(%d, %d) = %v, want %v", tt.newID, tt.oldID, got, tt.expected)
			}
		})
	}
}

// Regression (issue #160): Tranquility landing on a player who already has
// Koadic's Endless Intellect replaces it in game with no worn-off line. The
// Koadic's timer must be dropped, or Live Buffs sums both buffs' WIS/INT.
// Unrelated buffs on the same player, and the same buff on someone else, stay.
func TestOnSpellLanded_OverwrittenBuffTimerRemoved(t *testing.T) {
	database, err := db.Open("../../data/quarm.db")
	if err != nil {
		t.Skipf("quarm.db not available: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	charCtx := func() (string, string, int) { return "/eq", "Mellisie", -1 }
	e := NewEngine(ws.NewHub(), database, charCtx,
		func() string { return scopeAnyone }, nil, nil, nil, nil, nil)

	land := func(at time.Time, kind logparser.SpellLandedKind, target string, id int, name string) {
		e.onSpellLanded(at, logparser.SpellLandedData{
			Kind:       kind,
			TargetName: target,
			Candidates: []logparser.SpellLandedCandidate{{SpellID: id, SpellName: name}},
		})
	}

	now := time.Now()
	land(now, logparser.SpellLandedKindYou, "", 1447, "Aegolism")
	land(now.Add(time.Second), logparser.SpellLandedKindYou, "", 2570, "Koadic's Endless Intellect")
	land(now.Add(2*time.Second), logparser.SpellLandedKindOther, "Otherguy", 2570, "Koadic's Endless Intellect")
	land(now.Add(3*time.Second), logparser.SpellLandedKindYou, "", 3350, "Tranquility")

	got := map[string]bool{}
	for _, tm := range e.GetState().Timers {
		got[tm.SpellName+"@"+tm.TargetName] = true
	}
	want := []string{
		"Aegolism@Mellisie",
		"Tranquility@Mellisie",
		"Koadic's Endless Intellect@Otherguy",
	}
	for _, k := range want {
		if !got[k] {
			t.Errorf("missing timer %s; have %v", k, got)
		}
	}
	if got["Koadic's Endless Intellect@Mellisie"] {
		t.Errorf("Koadic's on Mellisie should have been overwritten by Tranquility; have %v", got)
	}
	if len(got) != len(want) {
		t.Errorf("timer count = %d, want %d: %v", len(got), len(want), got)
	}
}
