package spelltimer

import (
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/db"
	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
	"github.com/jasonsoprovich/pq-companion/backend/internal/ws"
)

// spawnIDEngine builds a DB-backed engine (needed so StartExternal can
// resolve spellID to a DB spell name for keyTargetTokenLocked's comparison
// against lastCastSpell). Mirrors ownershipEngine in clicky_ownership_test.go.
func spawnIDEngine(t *testing.T) *Engine {
	t.Helper()
	database, err := db.Open("../../data/quarm.db")
	if err != nil {
		t.Skipf("quarm.db not available: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	charCtx := func() (string, string, int) { return "/eq", "Osui", -1 }
	return NewEngine(ws.NewHub(), database, charCtx,
		func() string { return scopeAnyone }, nil, nil, nil, nil, nil)
}

func intPtr(i int) *int { return &i }

const (
	shasAdvantageName = "Sha's Advantage" // BST slow (50-65%)
	shasAdvantageID   = 2942
)

// keyTargetTokenLocked is the core of the fix for the issue Grimrose (SoS)
// reported against the built-in Slows pack: slowing two identically-named
// mobs only showed one countdown, because the trigger-driven timer key
// namespaces purely by captured mob name (see timerKey), and the combat log
// line that name comes from carries no spawn id at all. Exercised directly
// here (rather than only through StartExternal) so each condition of the
// "confidently my own recent cast" test is isolated.
func TestKeyTargetTokenLocked(t *testing.T) {
	spell := &db.Spell{Name: shasAdvantageName}
	now := time.Now()

	cases := []struct {
		name       string
		target     string
		spell      *db.Spell
		castSpell  string
		castAt     time.Time
		castTarget int
		at         time.Time
		want       string
	}{
		{
			name:       "matching recent self-cast appends spawn id",
			target:     "a gnoll",
			spell:      spell,
			castSpell:  shasAdvantageName,
			castAt:     now,
			castTarget: 101,
			at:         now.Add(time.Second),
			want:       "a gnoll#id:101",
		},
		{
			name:   "no captured target passes through untouched",
			target: "",
			spell:  spell,
			want:   "",
		},
		{
			name:   "no spell (spellID <= 0 or unknown) passes through untouched",
			target: "a gnoll",
			spell:  nil,
			want:   "a gnoll",
		},
		{
			name:       "no live target id at cast time falls back to name",
			target:     "a gnoll",
			spell:      spell,
			castSpell:  shasAdvantageName,
			castAt:     now,
			castTarget: 0,
			at:         now.Add(time.Second),
			want:       "a gnoll",
		},
		{
			name:       "different spell name (someone else's cast, or unrelated) falls back",
			target:     "a gnoll",
			spell:      spell,
			castSpell:  "Some Other Spell",
			castAt:     now,
			castTarget: 101,
			at:         now.Add(time.Second),
			want:       "a gnoll",
		},
		{
			name:       "outside the correlation window falls back",
			target:     "a gnoll",
			spell:      spell,
			castSpell:  shasAdvantageName,
			castAt:     now,
			castTarget: 101,
			at:         now.Add(castSelfSlowWindow + time.Second),
			want:       "a gnoll",
		},
		{
			name:       "landed line before the cast (stale/negative delta) falls back",
			target:     "a gnoll",
			spell:      spell,
			castSpell:  shasAdvantageName,
			castAt:     now,
			castTarget: 101,
			at:         now.Add(-time.Second),
			want:       "a gnoll",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Engine{
				lastCastSpell:    tc.castSpell,
				lastCastAt:       tc.castAt,
				lastCastTargetID: tc.castTarget,
			}
			got := e.keyTargetTokenLocked(tc.spell, tc.target, tc.at)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// End-to-end wiring check: a real StartExternal call, fed by the same
// SetPipeTargetID + EventSpellCast path production code uses, must produce
// a timer keyed with the spawn-id suffix while leaving the displayed
// TargetName as the plain mob name.
func TestStartExternal_UsesSpawnIDInKeyNotDisplay(t *testing.T) {
	e := spawnIDEngine(t)
	now := time.Now()

	e.SetPipeTargetID(intPtr(101))
	e.Handle(logparser.LogEvent{
		Type: logparser.EventSpellCast,
		Data: logparser.SpellCastData{SpellName: shasAdvantageName},
	})
	e.StartExternal("BST slow (50-65%)", "debuff", 192, 0, now.Add(time.Second),
		nil, shasAdvantageID, "a gnoll", "#ff0000", false, "", false)

	wantKey := timerKey("BST slow (50-65%)", "a gnoll"+targetIDKeySep+"101")
	timer, ok := e.timers[wantKey]
	if !ok {
		t.Fatalf("expected timer at key %q, got keys: %v", wantKey, keysOf(e.timers))
	}
	if timer.TargetName != "a gnoll" {
		t.Errorf("displayed TargetName should stay the plain mob name, got %q", timer.TargetName)
	}
}

// Without a live target id at cast time (Zeal disconnected, an NPC-cast
// slow with no "You begin casting" line on this client, or a slow cast by
// someone else in the raid), the key falls back to the plain target name
// exactly as before this feature existed — two same-named mobs still
// collide into one row. This is the documented, unavoidable remainder of
// LIMITATIONS.md §1.3: those cases have nothing on this client to
// correlate a spawn id against.
func TestStartExternal_NoSpawnIDFallsBackToNameCollision(t *testing.T) {
	e := spawnIDEngine(t)
	now := time.Now()

	e.StartExternal("BST slow (50-65%)", "debuff", 192, 0, now,
		nil, shasAdvantageID, "a gnoll", "#ff0000", false, "", false)
	e.StartExternal("BST slow (50-65%)", "debuff", 192, 0, now.Add(time.Second),
		nil, shasAdvantageID, "a gnoll", "#ff0000", false, "", false)

	if len(e.timers) != 1 {
		t.Fatalf("expected the pre-existing name-only collision without a spawn id, got %d rows: %v",
			len(e.timers), keysOf(e.timers))
	}
}
