package buffmod_test

import (
	"path/filepath"
	"testing"

	"github.com/jasonsoprovich/pq-companion/backend/internal/buffmod"
)

// Osui's fixture export has a non-ranged item (Mindpuncture, range 0) in the
// Range slot, so only the spell range comes from gear.
func TestComputeRangesOsui(t *testing.T) {
	requireTestdata(t, "Osui-Quarmy.txt")
	gameDB := openDB(t)
	eqPath := filepath.Join(repoRoot(t), "testdata")

	r, err := buffmod.ComputeRanges(eqPath, "Osui", gameDB)
	if err != nil {
		t.Fatalf("ComputeRanges: %v", err)
	}
	if !r.HasExport {
		t.Error("HasExport = false")
	}
	if r.RangedRange != 0 || r.RangedItem != "" {
		t.Errorf("ranged = %d (%q), want 0 for a non-ranged Range-slot item", r.RangedRange, r.RangedItem)
	}
	want := buffmod.BaseSpellRange * (100 + r.CastFocusPercent) / 100
	if r.CastRange != want {
		t.Errorf("cast range = %d, want %d for +%d%%", r.CastRange, want, r.CastFocusPercent)
	}
	t.Logf("Osui: cast %d (+%d%% %s), ranged %d", r.CastRange, r.CastFocusPercent, r.CastFocusItem, r.RangedRange)
}
