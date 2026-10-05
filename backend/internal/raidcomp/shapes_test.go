package raidcomp

import (
	"strings"
	"testing"
)

// Persistence + validation coverage for group-composition shapes (the v1
// storage layer of docs/raid-group-compositions-plan.md). The weave consumes
// them in a later stage; here we pin: save/load round-trip, the no-null rule,
// overwrite replacement, and the validation rails.

func TestStore_ShapeRoundTrip(t *testing.T) {
	s := openTemp(t)
	e := &Encounter{
		ID: "shapes", Name: "Shapes", Zone: "Kael Drakkel", Status: StatusActive,
		Comps: []CompRow{{Role: "damage", Min: 2, Rec: 3}},
		Shapes: []Shape{
			{ID: "healstack", GroupNumber: 1, Rows: []ShapeRow{
				{Role: "healer", Sub: "ch_cleric", Count: 3},
				{Role: "damage", Count: 1},
			}},
			{ID: "casterstack", Rows: []ShapeRow{{Role: "damage", Count: 6}}},
		},
	}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetEncounter("shapes")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Shapes) != 2 {
		t.Fatalf("shapes = %d, want 2", len(got.Shapes))
	}
	h := got.Shapes[0]
	if h.ID != "healstack" || h.GroupNumber != 1 {
		t.Errorf("shape[0] = %+v, want healstack pinned to group 1", h)
	}
	if len(h.Rows) != 2 || h.Rows[0].Path() != "healer.ch_cleric" || h.Rows[0].Count != 3 || h.Rows[1].Count != 1 {
		t.Errorf("healstack rows = %+v, want ch_cleric x3 + damage x1 (in order)", h.Rows)
	}
	c := got.Shapes[1]
	if c.ID != "casterstack" || c.GroupNumber != 0 || len(c.Rows) != 1 || c.Rows[0].Count != 6 {
		t.Errorf("casterstack = %+v, want unpinned damage x6", c)
	}

	// Overwrite replaces shapes wholesale (children-replacement contract).
	e.Shapes = []Shape{{ID: "solo", Rows: []ShapeRow{{Role: "tank", Sub: "defensive", Count: 1}}}}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatalf("resave: %v", err)
	}
	got2, err := s.GetEncounter("shapes")
	if err != nil {
		t.Fatalf("reget: %v", err)
	}
	if len(got2.Shapes) != 1 || got2.Shapes[0].ID != "solo" || len(got2.Shapes[0].Rows) != 1 {
		t.Errorf("after overwrite shapes = %+v, want exactly [solo x1]", got2.Shapes)
	}
}

func TestStore_ShapeNoNullAndCascade(t *testing.T) {
	s := openTemp(t)
	e := &Encounter{ID: "plain", Name: "Plain", Zone: "Kael Drakkel", Status: StatusActive}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.GetEncounter("plain")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Shapes == nil {
		t.Fatal("shapes must be [] (the no-null rule), got nil")
	}
	if len(got.Shapes) != 0 {
		t.Errorf("shapes = %d, want 0", len(got.Shapes))
	}
	// Delete cascades to the shapes table.
	if err := s.SaveEncounter(&Encounter{
		ID: "g", Name: "G", Zone: "Kael Drakkel", Status: StatusActive,
		Shapes: []Shape{{ID: "s", Rows: []ShapeRow{{Role: "damage", Count: 1}}}},
	}); err != nil {
		t.Fatalf("save g: %v", err)
	}
	if err := s.DeleteEncounter("g"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM raid_group_shapes WHERE encounter_id = ?`, "g").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Errorf("raid_group_shapes left %d rows after encounter delete", n)
	}
}

func TestStore_ShapeValidation(t *testing.T) {
	s := openTemp(t)
	base := func(id string, shapes []Shape) *Encounter {
		return &Encounter{ID: id, Name: id, Zone: "Kael Drakkel", Status: StatusActive, Shapes: shapes}
	}
	cases := []struct {
		name string
		e    *Encounter
		want string // substring of the error; "" = success
	}{
		{"ok", base("ok", []Shape{{ID: "s", Rows: []ShapeRow{{Role: "damage", Count: 2}}}}), ""},
		{"empty id", base("b1", []Shape{{Rows: []ShapeRow{{Role: "damage", Count: 1}}}}), "shape id required"},
		{"dup id", base("b2", []Shape{
			{ID: "s", Rows: []ShapeRow{{Role: "damage", Count: 1}}},
			{ID: "s", Rows: []ShapeRow{{Role: "healer", Sub: "ch_cleric", Count: 1}}},
		}), "duplicate shape id"},
		{"no rows", base("b3", []Shape{{ID: "s"}}), "has no rows"},
		{"count 0", base("b4", []Shape{{ID: "s", Rows: []ShapeRow{{Role: "damage", Count: 0}}}}), "count must be >= 1"},
		{"group out of range", base("b5", []Shape{{ID: "s", GroupNumber: 13, Rows: []ShapeRow{{Role: "damage", Count: 1}}}}), "out of range"},
		{"unknown role", base("b6", []Shape{{ID: "s", Rows: []ShapeRow{{Role: "nope", Count: 1}}}}), "not in taxonomy"},
	}
	for _, tc := range cases {
		err := s.SaveEncounter(tc.e)
		if tc.want == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want containing %q", tc.name, err, tc.want)
		}
	}
}
