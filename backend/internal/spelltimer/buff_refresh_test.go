package spelltimer

import (
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
)

// oldBuffRow inserts a buff timer that is well past dedupGraceWindow, keyed
// exactly as the engine would have keyed it on an earlier cast.
func oldBuffRow(e *Engine, key, name string, spellID int, target string) {
	e.timers[key] = &ActiveTimer{
		ID:         key,
		SpellName:  name,
		SpellID:    spellID,
		TargetName: target,
		Category:   CategoryBuff,
		CastAt:     time.Now().Add(-time.Minute),
		StartsAt:   time.Now().Add(-time.Minute),
		ExpiresAt:  time.Now().Add(time.Hour),
	}
}

func timerNames(e *Engine) []string {
	out := make([]string, 0, len(e.timers))
	for _, t := range e.timers {
		out = append(out, t.SpellName+"@"+t.TargetName)
	}
	return out
}

// Reported against v0.25.0: recasting Visions of Grandeur on a player
// sometimes left the old row running beside a new one. The earlier row carried
// a "#id:<spawn>" key suffix the refresh did not (or vice versa).
func TestOnSpellLanded_BuffRefreshReplacesDifferentlyKeyedRow(t *testing.T) {
	const vog, vogID = "Visions of Grandeur", 1710

	t.Run("old id-tagged row replaced", func(t *testing.T) {
		e := spawnIDEngine(t)
		oldBuffRow(e, timerKey(vog, "Ieaini"+targetIDKeySep+"101"), vog, vogID, "Ieaini")
		e.onSpellLanded(time.Now(), logparser.SpellLandedData{
			Kind: logparser.SpellLandedKindOther, SpellName: vog, TargetName: "Ieaini",
		})
		if len(e.timers) != 1 {
			t.Fatalf("want 1 timer, got %v", timerNames(e))
		}
	})

	t.Run("own cast with pipe target id no longer tags a buff", func(t *testing.T) {
		e := spawnIDEngine(t)
		for i := 0; i < 2; i++ {
			e.SetPipeTargetID(intPtr(101 + i)) // e.g. spawn id changed after a zone
			e.Handle(logparser.LogEvent{
				Type: logparser.EventSpellCast,
				Data: logparser.SpellCastData{SpellName: vog},
			})
			e.onSpellLanded(time.Now(), logparser.SpellLandedData{
				Kind: logparser.SpellLandedKindOther, SpellName: vog, TargetName: "Ieaini",
			})
		}
		if len(e.timers) != 1 {
			t.Fatalf("want 1 timer, got %v", keysOf(e.timers))
		}
	})

	t.Run("different recipients coexist", func(t *testing.T) {
		e := spawnIDEngine(t)
		for _, who := range []string{"Ieaini", "Rorshac"} {
			e.onSpellLanded(time.Now(), logparser.SpellLandedData{
				Kind: logparser.SpellLandedKindOther, SpellName: vog, TargetName: who,
			})
		}
		if len(e.timers) != 2 {
			t.Fatalf("want 2 timers, got %v", timerNames(e))
		}
	})
}

// Group Resist Magic landed by another caster resolves to the combined
// "Resist Magic" name; the player's own cast resolves to "Group Resist Magic".
// Both are the same buff on the same person and must refresh one row.
func TestOnSpellLanded_BuffRefreshAcrossAmbiguousGroupNames(t *testing.T) {
	t.Run("combined name then own cast", func(t *testing.T) {
		e := spawnIDEngine(t)
		oldBuffRow(e, timerKey("Resist Magic", "Sandrian"), "Resist Magic", 72, "Sandrian")
		e.onSpellLanded(time.Now(), logparser.SpellLandedData{
			Kind: logparser.SpellLandedKindOther, SpellName: "Group Resist Magic", TargetName: "Sandrian",
		})
		if len(e.timers) != 1 {
			t.Fatalf("want 1 timer, got %v", timerNames(e))
		}
		if _, ok := e.timers[timerKey("Group Resist Magic", "Sandrian")]; !ok {
			t.Errorf("expected the refreshed Group Resist Magic row, got %v", keysOf(e.timers))
		}
	})

	t.Run("own cast then combined name", func(t *testing.T) {
		e := spawnIDEngine(t)
		oldBuffRow(e, timerKey("Group Resist Magic", "Sandrian"), "Group Resist Magic", 72, "Sandrian")
		e.onSpellLanded(time.Now(), logparser.SpellLandedData{
			Kind:       logparser.SpellLandedKindOther,
			TargetName: "Sandrian",
			Candidates: []logparser.SpellLandedCandidate{
				{SpellID: 64, SpellName: "Resist Magic"},
				{SpellID: 72, SpellName: "Group Resist Magic"},
				{SpellID: 964, SpellName: "Resistance to Magic"},
				{SpellID: 3242, SpellName: "Guard of Druzzil"},
			},
		})
		if len(e.timers) != 1 {
			t.Fatalf("want 1 timer, got %v", timerNames(e))
		}
	})
}
