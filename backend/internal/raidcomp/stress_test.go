package raidcomp

import (
	"fmt"
	"os"
	"sync"
	"testing"
)

// Concurrency stress tests for the Store write path. These are NOT part of
// the normal suite (they are slow and hammer a single SQLite connection):
// every test skips unless RAIDCOMP_STRESS=1. Run explicitly with the race
// detector:
//
//	RAIDCOMP_STRESS=1 go test -race ./internal/raidcomp/ -run Stress -v
//
// What we're probing:
//   - lost updates / torn writes when many goroutines save the SAME encounter
//     id concurrently (children must exactly match exactly ONE writer)
//   - SQLITE_BUSY / intermittent failures under concurrent distinct writes
//   - no orphaned children when Delete races Save on the same id
//   - taxonomy writes racing encounter writes (Leaves() is read inside
//     SaveEncounter's validation)
//
// All stores here run on temp DBs — nothing touches user data.

func stressEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("RAIDCOMP_STRESS") == "" {
		t.Skip("set RAIDCOMP_STRESS=1 to run concurrency stress tests")
	}
}

func stressEnc(id string, variant int) *Encounter {
	return &Encounter{
		ID:     id,
		Name:   fmt.Sprintf("Stress %s v%d", id, variant),
		Zone:   "Kael Drakkel",
		Status: StatusActive,
		// variant is embedded in the comp counts so a torn/merged write is
		// detectable: the survivor's comps must match ONE variant exactly.
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: variant, Rec: variant},
			{Role: "damage", Min: variant + 100, Rec: variant + 100},
		},
		Reqs:     []string{fmt.Sprintf("req-%d", variant)},
		Strategy: map[string]string{"notes": fmt.Sprintf("v%d", variant)},
	}
}

// matchesVariant reports whether the stored encounter's children correspond
// exactly to the given writer variant (all-or-nothing per writer).
func matchesVariant(t *testing.T, s *Store, id string, variant int) error {
	t.Helper()
	got, err := s.GetEncounter(id)
	if err != nil {
		return fmt.Errorf("load: %w", err)
	}
	if len(got.Comps) != 2 {
		return fmt.Errorf("comps = %d rows, want 2", len(got.Comps))
	}
	byPath := map[string]CompRow{}
	for _, c := range got.Comps {
		byPath[c.Path()] = c
	}
	td := byPath["tank.defensive"]
	dm := byPath["damage"]
	if td.Min != variant || dm.Min != variant+100 {
		return fmt.Errorf("torn write: tank=%d damage=%d (variant %d would be %d/%d)",
			td.Min, dm.Min, variant, variant, variant+100)
	}
	if len(got.Reqs) != 1 || got.Reqs[0] != fmt.Sprintf("req-%d", variant) {
		return fmt.Errorf("reqs = %v, want [req-%d]", got.Reqs, variant)
	}
	if got.Strategy["notes"] != fmt.Sprintf("v%d", variant) {
		return fmt.Errorf("strategy = %v, want v%d", got.Strategy, variant)
	}
	return nil
}

// TestStress_ConcurrentSameID hammers the same encounter id from many
// goroutines at once. Every write must land atomically: after the storm the
// stored children must match exactly one writer's variant, and no failure
// may leave partial rows.
func TestStress_ConcurrentSameID(t *testing.T) {
	stressEnabled(t)
	s := openTemp(t)
	const workers = 24
	const rounds = 15

	var mu sync.Mutex
	errs := map[string]int{}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				v := w*rounds + r
				if err := s.SaveEncounter(stressEnc("stress-same", v)); err != nil {
					mu.Lock()
					errs[err.Error()]++
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()

	if len(errs) > 0 {
		t.Errorf("%d failed writes across variants: %v", len(errs), errs)
	}
	// The survivor must be one whole variant, not a blend.
	var matched bool
	var lastErr error
	for v := 0; v < workers*rounds; v++ {
		if err := matchesVariant(t, s, "stress-same", v); err == nil {
			matched = true
			break
		} else {
			lastErr = err
		}
	}
	if !matched {
		t.Errorf("no writer variant survived intact; last mismatch: %v", lastErr)
	}
}

// TestStress_ConcurrentDistinctIDs writes distinct encounters concurrently
// (the import-commit shape: per-encounter transactions in parallel). All
// writes must succeed (single-connection WAL + busy_timeout should absorb
// contention) and every encounter must come back whole.
func TestStress_ConcurrentDistinctIDs(t *testing.T) {
	stressEnabled(t)
	s := openTemp(t)
	const workers = 24
	const rounds = 10

	errs := make([]error, workers*rounds)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				i := w*rounds + r
				id := fmt.Sprintf("stress-distinct-%03d", i)
				errs[i] = s.SaveEncounter(stressEnc(id, i))
			}
		}(w)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("write %d failed: %v", i, err)
		}
	}
	encs, err := s.ListEncounters()
	if err != nil {
		t.Fatal(err)
	}
	if len(encs) < workers*rounds {
		t.Errorf("listed %d encounters, want >= %d", len(encs), workers*rounds)
	}
	for i := 0; i < workers*rounds; i += 37 { // spot-check every 37th
		id := fmt.Sprintf("stress-distinct-%03d", i)
		if err := matchesVariant(t, s, id, i); err != nil {
			t.Errorf("%s not whole: %v", id, err)
		}
	}
}

// TestStress_DeleteRacesSave runs Save and Delete against the same id
// concurrently. The endpoint never lets those interleave for one id, but the
// store must be safe regardless: afterwards the id either doesn't exist or
// exists WHOLE — and no comp/req/strategy row may survive without its parent.
func TestStress_DeleteRacesSave(t *testing.T) {
	stressEnabled(t)
	s := openTemp(t)
	const workers = 16
	const rounds = 20

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				if w%2 == 0 {
					_ = s.SaveEncounter(stressEnc("stress-delrace", w*rounds+r))
				} else {
					_ = s.DeleteEncounter("stress-delrace")
				}
			}
		}(w)
	}
	wg.Wait()

	// No orphans, whatever the final state.
	for _, table := range []string{"raid_encounter_comps", "raid_encounter_reqs", "raid_encounter_strategy"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table + ` WHERE encounter_id = 'stress-delrace'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		_, parentErr := s.GetEncounter("stress-delrace")
		if n > 0 && parentErr != nil {
			t.Errorf("%s has %d orphaned rows after delete/save race", table, n)
		}
	}
	if err := matchesVariant(t, s, "stress-delrace", -1); err != nil {
		// Not an error per se — need it to match SOME variant if it exists.
		var matched bool
		for v := 0; v < workers*rounds && !matched; v++ {
			matched = matchesVariant(t, s, "stress-delrace", v) == nil
		}
		exists := false
		if _, err := s.GetEncounter("stress-delrace"); err == nil {
			exists = true
		}
		if exists && !matched {
			t.Errorf("encounter exists but matches no writer variant")
		}
	}
}

// TestStress_RolesRaceEncounters mutates the taxonomy while encounters are
// being saved (SaveEncounter reads Leaves() for validation). Writers must
// either succeed cleanly or fail validation — never corrupt state.
func TestStress_RolesRaceEncounters(t *testing.T) {
	stressEnabled(t)
	s := openTemp(t)
	const rounds = 40

	var wg sync.WaitGroup
	// Taxonomy churn: add/remove a role other writers don't reference.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := 0; r < rounds; r++ {
			_ = s.SaveRole(Role{Role: "stressrole", Label: "Stress", Classes: []ClassCode{"bst"}})
			_ = s.DeleteRole("stressrole", "")
		}
	}()
	// Encounter writers use only seeded roles, so their validation outcome
	// must ALWAYS be success regardless of the churn.
	var mu sync.Mutex
	var writeErrs []error
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				if err := s.SaveEncounter(stressEnc(fmt.Sprintf("stress-roles-%d-%d", w, r), w*rounds+r)); err != nil {
					mu.Lock()
					writeErrs = append(writeErrs, err)
					mu.Unlock()
				}
			}
		}(w)
	}
	wg.Wait()

	for _, err := range writeErrs {
		t.Errorf("encounter write failed during taxonomy churn: %v", err)
	}
}

// TestStress_LargePack saves one encounter with a very wide comp set (every
// seeded leaf duplicated across sub-roles is not possible, so we instead use
// many reqs + strategy + all seeded leaves) and times it as a smoke signal
// for pathological per-row behavior on import of big packs.
func TestStress_LargePack(t *testing.T) {
	stressEnabled(t)
	s := openTemp(t)
	leaves, err := s.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) == 0 {
		t.Fatal("no taxonomy leaves seeded")
	}
	comps := make([]CompRow, 0, len(leaves))
	for _, l := range leaves {
		comps = append(comps, CompRow{Role: l.Role, Sub: l.Sub, Min: 1, Rec: 2})
	}
	reqs := make([]string, 500)
	for i := range reqs {
		reqs[i] = fmt.Sprintf("requirement line %d with some text", i)
	}
	e := &Encounter{
		ID: "stress-large", Name: "Large Pack", Zone: "Kael Drakkel",
		Status: StatusActive, Comps: comps, Reqs: reqs,
		Strategy: map[string]string{"notes": "big", "pulling": "big"},
	}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatalf("large save failed: %v", err)
	}
	if err := matchesVariant(t, s, "stress-large", -1); err != nil {
		// variant -1 is meaningless here; just require the row to load whole
		got, gerr := s.GetEncounter("stress-large")
		if gerr != nil {
			t.Fatalf("reload failed: %v", gerr)
		}
		if len(got.Reqs) != len(reqs) {
			t.Errorf("reqs = %d, want %d", len(got.Reqs), len(reqs))
		}
		if len(got.Comps) != len(comps) {
			t.Errorf("comps = %d, want %d", len(got.Comps), len(comps))
		}
	}
}
