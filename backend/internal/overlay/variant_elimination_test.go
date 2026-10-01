package overlay

import (
	"testing"
	"time"

	"github.com/jasonsoprovich/pq-companion/backend/internal/logparser"
)

const (
	kaasName  = "Kaas Thox Xi Aten Ha Ra"
	kaasNorth = 158437 // loottable 12519
	kaasSouth = 158464 // loottable 96732
)

// vexThalTracker returns a DB-backed tracker standing in Vex Thal (zone 158).
func vexThalTracker(t *testing.T) *NPCTracker {
	t.Helper()
	tr := newRealDBTracker(t)
	tr.SetPipePlayerSnapshot(158, 0, 0, 0)
	if tr.pipeZoneShort == "" {
		t.Skip("vexthal zone not resolvable")
	}
	return tr
}

func killEvent(name string) logparser.LogEvent {
	return logparser.LogEvent{
		Type: logparser.EventKill,
		Data: logparser.KillData{Killer: "You", Target: name},
	}
}

func targetKaas(tr *NPCTracker) TargetState {
	tr.clearTarget()
	tr.SetPipeTarget(kaasName)
	return tr.GetState()
}

func TestVariantElimination_DeadVariantResolvesSibling(t *testing.T) {
	tr := vexThalTracker(t)
	if st := targetKaas(tr); len(st.Variants) != 2 {
		t.Fatalf("baseline: want 2 variants, got %d", len(st.Variants))
	}

	tr.markVariantDead(tr.pipeZoneShort, "Kaas_Thox_Xi_Aten_Ha_Ra", kaasNorth)

	st := targetKaas(tr)
	if len(st.Variants) != 0 || st.NPCData == nil || st.NPCData.ID != kaasSouth {
		t.Fatalf("want sibling %d resolved alone, got npc=%v variants=%d", kaasSouth, npcID(st.NPCData), len(st.Variants))
	}
}

func TestVariantElimination_NeverEmptiesCandidateSet(t *testing.T) {
	tr := vexThalTracker(t)
	tr.markVariantDead(tr.pipeZoneShort, "Kaas_Thox_Xi_Aten_Ha_Ra", kaasNorth)
	tr.markVariantDead(tr.pipeZoneShort, "Kaas_Thox_Xi_Aten_Ha_Ra", kaasSouth)
	if st := targetKaas(tr); len(st.Variants) != 2 {
		t.Fatalf("all dead must fall back to the full set, got %d variants", len(st.Variants))
	}
}

func TestVariantElimination_ExpiredDeathIgnored(t *testing.T) {
	tr := vexThalTracker(t)
	tr.markVariantDead(tr.pipeZoneShort, "Kaas_Thox_Xi_Aten_Ha_Ra", kaasNorth)
	key := eliminationKey(tr.pipeZoneShort, "Kaas_Thox_Xi_Aten_Ha_Ra")
	tr.deadVariants[key][kaasNorth] = time.Now().Add(-time.Minute)
	if st := targetKaas(tr); len(st.Variants) != 2 {
		t.Fatalf("expired death must not filter, got %d variants", len(st.Variants))
	}
}

func TestVariantElimination_LootIdentifiesDeadVariant(t *testing.T) {
	cases := []struct {
		name     string
		loot     string
		wantNPC  int // 0 = still ambiguous
		wantVars int
	}{
		// Only in loottable 12519 (north), so north died and south remains.
		{"north-exclusive item", "--Bob has looted a Belt of Mastery.--", kaasSouth, 0},
		// Only in loottable 96732 (south), so south died and north remains.
		{"south-exclusive item", "--Bob has looted a Mask of Piety.--", kaasNorth, 0},
		// In both tables: no evidence either way.
		{"shared item", "--Bob has looted a Ancient: Lifebane.--", 0, 2},
		{"not a loot line", "You say, 'hello'", 0, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tr := vexThalTracker(t)
			tr.Handle(killEvent(kaasName)) // no resolved target: goes pending
			tr.HandleLootLine(time.Now(), c.loot)

			st := targetKaas(tr)
			if c.wantNPC == 0 {
				if len(st.Variants) != c.wantVars {
					t.Fatalf("want %d variants, got %d", c.wantVars, len(st.Variants))
				}
				return
			}
			if len(st.Variants) != 0 || npcID(st.NPCData) != c.wantNPC {
				t.Fatalf("want npc %d alone, got npc=%d variants=%d", c.wantNPC, npcID(st.NPCData), len(st.Variants))
			}
		})
	}
}

func TestVariantElimination_LootWithoutKillDoesNothing(t *testing.T) {
	tr := vexThalTracker(t)
	tr.HandleLootLine(time.Now(), "--Bob has looted a Belt of Mastery.--")
	if st := targetKaas(tr); len(st.Variants) != 2 {
		t.Fatalf("loot with no preceding kill must not eliminate, got %d variants", len(st.Variants))
	}
}

func TestVariantElimination_OrdinaryMobKillIsIgnored(t *testing.T) {
	tr := vexThalTracker(t)
	tr.Handle(killEvent("a rotting corpse"))
	if len(tr.pendingKills) != 0 || len(tr.deadVariants) != 0 {
		t.Fatalf("ordinary kill recorded state: pending=%v dead=%v", tr.pendingKills, tr.deadVariants)
	}
}
