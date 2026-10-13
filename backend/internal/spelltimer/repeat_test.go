package spelltimer

import (
	"testing"
	"time"
)

// A repeating custom timer restarts from its previous expiry (no drift) and
// stops cycling once its budget is spent.
func TestRepeatingCustomTimerRestartsFromExpiry(t *testing.T) {
	e := newTestEngine()
	// Started 125s ago with a 49s period: two whole cycles have elapsed.
	start := time.Now().Add(-125 * time.Second)
	e.StartExternal("Coirnav reinforcements", "custom", 49, 0, start, nil, 0, "", "", false, "", false)
	e.SetExternalRepeat("Coirnav reinforcements", "", 0)

	key := timerKey("Coirnav reinforcements", "")
	tm := e.timers[key]
	tm.expiryGraceUntil = time.Now().Add(-time.Second) // skip the 0:00 grace tick
	e.pruneExpired()

	got, ok := e.timers[key]
	if !ok {
		t.Fatal("repeating timer was dropped on expiry")
	}
	if want := start.Add(3 * 49 * time.Second); !got.ExpiresAt.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v (cycles anchored to the original start)", got.ExpiresAt, want)
	}
	if got.repeatsLeft != maxRepeatCycles-2 {
		t.Fatalf("repeatsLeft = %d, want %d", got.repeatsLeft, maxRepeatCycles-2)
	}

	// Budget exhausted: the next expiry drops it like any other timer.
	got.repeatsLeft = 0
	got.ExpiresAt = time.Now().Add(-time.Minute)
	got.expiryGraceUntil = time.Now().Add(-time.Second)
	e.pruneExpired()
	if _, ok := e.timers[key]; ok {
		t.Fatal("timer kept cycling after its repeat budget ran out")
	}
}

// Only non-stacked custom timers can repeat.
func TestSetExternalRepeatIgnoresOtherCategories(t *testing.T) {
	e := newTestEngine()
	e.StartExternal("AE Incoming", "debuff", 30, 0, time.Now(), nil, 0, "", "", false, "", false)
	e.SetExternalRepeat("AE Incoming", "", 5)
	if e.timers[timerKey("AE Incoming", "")].Repeat {
		t.Fatal("detrimental timer was marked repeating")
	}
}
