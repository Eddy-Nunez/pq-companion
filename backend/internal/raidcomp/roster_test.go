package raidcomp

import (
	"testing"
	"time"
)

// TestRosterStaleTransition pins the disband semantics: Zeal emits MsgRaid
// every main-loop tick while in a raid and NOTHING on disband (no "raid
// over" message exists), so staleness IS the disband signal. StaleChange
// must fire exactly once per fresh→stale transition, never at startup
// (nothing populated yet) and never while fresh.
func TestRosterStaleTransition(t *testing.T) {
	defer func(pn func() time.Time) { now = pn }(now)
	t0 := time.Unix(1_000_000, 0)
	now = func() time.Time { return t0 }

	r := NewRoster()
	if r.StaleChange() {
		t.Fatal("start-up (never populated) must not report a stale change")
	}
	if _, ok := r.Get(); ok {
		t.Fatal("start-up roster must be unseen")
	}

	// Raid forms: fresh roster, no change.
	r.Set(113, "kael", []Member{{Name: "Thud", Level: 60, Class: 1}})
	if r.StaleChange() {
		t.Fatal("fresh roster must not report a stale change")
	}
	snap, ok := r.Get()
	if !ok || len(snap.Members) != 1 {
		t.Fatalf("Get() = %+v, ok=%v; want 1 member", snap, ok)
	}

	// Raid disbands: the ticks stop. Past the window → one change, once.
	now = func() time.Time { return t0.Add(2 * rosterStaleAfter) }
	if !r.StaleChange() {
		t.Fatal("aged-out roster must report a stale change")
	}
	if r.StaleChange() {
		t.Fatal("stale change must fire only once per transition")
	}
	if _, ok := r.Get(); ok {
		t.Fatal("aged-out roster must report unseen from Get")
	}

	// Raid reforms: fresh again, no change; and a subsequent aging-out
	// reports once more.
	now = func() time.Time { return t0.Add(3 * rosterStaleAfter) }
	r.Set(113, "kael", []Member{{Name: "Thud", Level: 60, Class: 1}})
	if r.StaleChange() {
		t.Fatal("re-populated roster must not report a stale change")
	}
	now = func() time.Time { return t0.Add(10 * rosterStaleAfter) }
	if !r.StaleChange() {
		t.Fatal("second aging-out must report a stale change again")
	}
}

// TestRosterClearSuppressesStaleChange: Clear (pipe disconnect) already
// broadcasts on its own path, so it must arm lastStale — the watcher must
// not fire a duplicate change for the same emptying.
func TestRosterClearSuppressesStaleChange(t *testing.T) {
	defer func(pn func() time.Time) { now = pn }(now)
	t0 := time.Unix(1_000_000, 0)
	now = func() time.Time { return t0 }

	r := NewRoster()
	r.Set(113, "kael", []Member{{Name: "Thud"}})
	if r.StaleChange() {
		t.Fatal("fresh roster must not report a stale change")
	}
	r.Clear()
	if r.StaleChange() {
		t.Fatal("Clear must suppress the watcher's stale change (it broadcasts itself)")
	}
	if _, ok := r.Get(); ok {
		t.Fatal("cleared roster must be unseen")
	}
}
