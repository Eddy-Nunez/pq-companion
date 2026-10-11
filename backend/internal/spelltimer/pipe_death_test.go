package spelltimer

import (
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
)

// A pipe death removes exactly the tagged instance it names and leaves a
// same-named instance with a different spawn id running.
func TestHandlePipeDeath_RemovesExactSpawnOnly(t *testing.T) {
	e := spawnIDEngine(t)
	dead := slowKeyOn(e, 701)
	alive := slowKeyOn(e, 702)

	e.HandlePipeDeath(701, "a bloodguard")

	if _, ok := e.timers[dead]; ok {
		t.Error("expected #701 removed")
	}
	if _, ok := e.timers[alive]; !ok {
		t.Error("expected #702 kept")
	}
}

// Untagged timers fall back to the name match a log kill uses, except a
// charm timer; target-less detrimentals are not swept.
func TestHandlePipeDeath_UntaggedNameMatch(t *testing.T) {
	e := spawnIDEngine(t)
	now := time.Now()
	add := func(key, target string, cat Category, charm bool) {
		e.timers[key] = &ActiveTimer{
			ID: key, SpellName: key, TargetName: target, Category: cat,
			IsCharm: charm, CastAt: now, StartsAt: now,
			ExpiresAt: now.Add(time.Minute),
		}
	}
	add("Tashanian@Knight of Decay", "Knight of Decay", CategoryDebuff, false)
	add("Charm@Knight of Decay", "Knight of Decay", CategoryDebuff, true)
	add("Orphan@", "", CategoryDebuff, false)
	add("Tashanian@Priest of Decay", "Priest of Decay", CategoryDebuff, false)

	e.HandlePipeDeath(900, "Knight of Decay")

	if _, ok := e.timers["Tashanian@Knight of Decay"]; ok {
		t.Error("expected untagged same-name timer removed")
	}
	for _, k := range []string{"Charm@Knight of Decay", "Orphan@", "Tashanian@Priest of Decay"} {
		if _, ok := e.timers[k]; !ok {
			t.Errorf("expected %q kept", k)
		}
	}
}

// When the pipe reports the death, the log's ambiguous kill line for it
// must not flag the surviving same-named instance MaybeDead — whichever of
// the two arrives first.
func TestHandlePipeDeath_SuppressesMaybeDead(t *testing.T) {
	for _, pipeFirst := range []bool{true, false} {
		e := spawnIDEngine(t)
		slowKeyOn(e, 801)
		alive := slowKeyOn(e, 802)
		kill := func() {
			e.Handle(logparser.LogEvent{
				Type: logparser.EventKill,
				Data: logparser.KillData{Killer: "Groupmate", Target: "a bloodguard"},
			})
		}
		if pipeFirst {
			e.HandlePipeDeath(801, "a bloodguard")
			kill()
		} else {
			kill()
			e.HandlePipeDeath(801, "a bloodguard")
		}
		e.mu.Lock()
		e.resolvePendingKillsLocked(time.Now().Add(killEvidenceGrace + time.Second))
		e.mu.Unlock()

		if e.timers[alive] == nil || e.timers[alive].MaybeDead {
			t.Errorf("pipeFirst=%v: expected #802 kept and not MaybeDead", pipeFirst)
		}
		if len(e.timers) != 1 {
			t.Errorf("pipeFirst=%v: expected 1 timer, got %v", pipeFirst, keysOf(e.timers))
		}
	}
}
