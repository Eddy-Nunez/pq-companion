package raidcomp

import (
	"errors"
	"testing"
)

// Persistence-focused tests for SaveEncounter — the write path import commit
// relies on. Coverage here targets what TestStore_CRUDRoundTrip's happy path
// doesn't: multi-table transaction atomicity (a failure mid-transaction must
// leave zero rows behind) and overwrite edge cases (children replacement,
// created_at preservation).

// TestStore_SaveEncounter_RollbackOnDuplicateComp forces a genuine
// mid-transaction failure: two comp rows sharing the same (role, sub) both
// target the same raid_encounter_comps PK, so the second INSERT violates the
// constraint after the parent row was already written inside the tx. The
// deferred rollback must discard ALL of it — no parent row, no children.
func TestStore_SaveEncounter_RollbackOnDuplicateComp(t *testing.T) {
	s := openTemp(t)
	e := &Encounter{
		ID:     "dup-comp",
		Name:   "Duplicate Comp",
		Zone:   "Kael Drakkel",
		Status: StatusActive,
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 1, Rec: 2},
			{Role: "damage", Min: 5, Rec: 6},
			{Role: "tank", Sub: "defensive", Min: 3, Rec: 4}, // dup path: PK violation mid-tx
		},
		Reqs:     []string{"some req"},
		Strategy: map[string]string{"notes": "should not survive"},
	}
	err := s.SaveEncounter(e)
	if err == nil {
		t.Fatal("expected duplicate-comp save to fail on the PK violation")
	}
	// Parent row must be gone (rollback, not just failed children).
	if _, err := s.GetEncounter("dup-comp"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("parent row survived the failed save: err = %v, want ErrNotFound", err)
	}
	// Belt and braces: no child rows either, at the SQL level.
	for _, table := range []string{"raid_encounter_comps", "raid_encounter_reqs", "raid_encounter_strategy"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE encounter_id = ?`, "dup-comp").Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s has %d orphaned rows after rollback, want 0", table, n)
		}
	}
}

// TestStore_SaveEncounter_UnknownRoleNoPartialWrites covers the pre-transaction
// guard: an unknown comp role must be rejected before ANY write happens, and
// an already-existing encounter must be left untouched by the failed save.
func TestStore_SaveEncounter_UnknownRoleNoPartialWrites(t *testing.T) {
	s := openTemp(t)
	good := &Encounter{
		ID: "vulak", Name: "Vulak'Aerr", Zone: "Kael Drakkel", Status: StatusActive,
		Comps: []CompRow{{Role: "damage", Min: 18, Rec: 22}},
	}
	if err := s.SaveEncounter(good); err != nil {
		t.Fatal(err)
	}
	before, err := s.GetEncounter("vulak")
	if err != nil {
		t.Fatal(err)
	}

	bad := *before
	bad.Comps = []CompRow{
		{Role: "damage", Min: 20, Rec: 24},
		{Role: "not_a_role", Min: 1, Rec: 1},
	}
	if err := s.SaveEncounter(&bad); err == nil {
		t.Fatal("expected unknown-role save to fail")
	}

	after, err := s.GetEncounter("vulak")
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Comps) != 1 || after.Comps[0].Min != 18 {
		t.Errorf("failed save mutated comps: %+v, want untouched min 18", after.Comps)
	}
	if after.UpdatedAt != before.UpdatedAt || after.CreatedAt != before.CreatedAt {
		t.Errorf("failed save bumped timestamps: created %d->%d updated %d->%d",
			before.CreatedAt, after.CreatedAt, before.UpdatedAt, after.UpdatedAt)
	}
}

// TestStore_SaveEncounter_OverwriteReplacesChildren pins the overwrite
// semantics import commit's "overwrite" choice depends on: children are
// replaced wholesale (shrunken comps/reqs/strategy must not leave stale
// rows behind), created_at is preserved, and updated_at advances.
func TestStore_SaveEncounter_OverwriteReplacesChildren(t *testing.T) {
	s := openTemp(t)
	v1 := &Encounter{
		ID: "tover", Name: "Overwrite Target", Zone: "Kael Drakkel", Status: StatusPlaceholder,
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 2, Rec: 3},
			{Role: "damage", Min: 18, Rec: 22},
		},
		Reqs:     []string{"req one", "req two"},
		Strategy: map[string]string{"pulling": "v1 pull", "notes": "v1 notes"},
	}
	if err := s.SaveEncounter(v1); err != nil {
		t.Fatal(err)
	}
	saved1, err := s.GetEncounter("tover")
	if err != nil {
		t.Fatal(err)
	}

	// Backdate updated_at so the bump assertion is deterministic even when
	// both saves land in the same wall-clock second.
	if _, err := s.db.Exec(`UPDATE raid_encounters SET updated_at = updated_at - 1000 WHERE id = 'tover'`); err != nil {
		t.Fatal(err)
	}

	v2 := &Encounter{
		ID: "tover", Name: "Overwrite Target", Zone: "Kael Drakkel", Status: StatusActive,
		// shrink comps AND change a survivor's counts
		Comps:    []CompRow{{Role: "damage", Min: 20, Rec: 24}},
		Reqs:     []string{"req three"},
		Strategy: map[string]string{"healing": "v2 healing"},
	}
	if err := s.SaveEncounter(v2); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetEncounter("tover")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Comps) != 1 || got.Comps[0].Path() != "damage" || got.Comps[0].Min != 20 {
		t.Errorf("comps not replaced wholesale: %+v", got.Comps)
	}
	if len(got.Reqs) != 1 || got.Reqs[0] != "req three" {
		t.Errorf("reqs not replaced: %v, want [req three]", got.Reqs)
	}
	if len(got.Strategy) != 1 || got.Strategy["healing"] != "v2 healing" {
		t.Errorf("strategy not replaced: %v", got.Strategy)
	}
	if got.CreatedAt != saved1.CreatedAt {
		t.Errorf("created_at changed on overwrite: %d -> %d", saved1.CreatedAt, got.CreatedAt)
	}
	if got.UpdatedAt <= saved1.UpdatedAt-1000 {
		t.Errorf("updated_at did not advance past backdated value: %d <= %d", got.UpdatedAt, saved1.UpdatedAt-1000)
	}
}
