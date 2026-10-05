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

// ── weave integration (docs/raid-group-compositions-plan.md §5) ───────────

// shapesCohortComp is the cohort fixture with shapes attached.
func shapesCohortComp(id string, shapes ...Shape) *Encounter {
	e := cohortComp(id)
	e.Shapes = shapes
	return e
}

func TestSplit_Shapes_RequireCohortMode(t *testing.T) {
	enc := shapesCohortComp("c", Shape{ID: "healstack", Rows: []ShapeRow{{Role: "healer", Sub: "ch_cleric", Count: 2}}})
	for _, cohorts := range []int{0, 1} {
		_, err := Split(splitLeaves(t), enc, SplitRequest{
			Preference: SplitTrinity, Cohorts: cohorts, Shapes: []string{"healstack"},
		}, cohortRoster())
		if err == nil || !strings.Contains(err.Error(), "cohorts >= 2") {
			t.Errorf("cohorts=%d: err = %v, want cohort-mode rejection", cohorts, err)
		}
	}
}

func TestSplit_Shapes_UnknownID(t *testing.T) {
	enc := shapesCohortComp("c", Shape{ID: "healstack", Rows: []ShapeRow{{Role: "healer", Sub: "ch_cleric", Count: 2}}})
	_, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"healstack", "nope"},
	}, cohortRoster())
	if err == nil || !strings.Contains(err.Error(), "unknown shape \"nope\"") {
		t.Fatalf("err = %v, want unknown-shape rejection", err)
	}
}

func TestSplit_Shapes_BadDistribution(t *testing.T) {
	enc := shapesCohortComp("c", Shape{ID: "healstack", Rows: []ShapeRow{{Role: "healer", Sub: "ch_cleric", Count: 1}}})
	_, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"healstack"}, ShapeDistribution: "zigzag",
	}, cohortRoster())
	if err == nil || !strings.Contains(err.Error(), "shape_distribution") {
		t.Fatalf("err = %v, want distribution rejection", err)
	}
}

func TestSplit_Shapes_ReplicateSeatsShapeGroupsFirst(t *testing.T) {
	// healstack = 2 CH clerics + 1 defensive tank, replicated into both
	// raids of a 2-cohort split of 2 tanks / 4 clerics / 6 rogues. The
	// shape consumes one tank + two clerics per raid, claims group 1, and
	// its seats satisfy the template's MIN coverage.
	enc := shapesCohortComp("c",
		Shape{ID: "healstack", Rows: []ShapeRow{
			{Role: "healer", Sub: "ch_cleric", Count: 2},
			{Role: "tank", Sub: "defensive", Count: 1},
		}})
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"healstack"},
	}, cohortRoster())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(rep.Cohorts) != 2 {
		t.Fatalf("want 2 cohorts, got %d", len(rep.Cohorts))
	}
	for _, cr := range rep.Cohorts {
		if len(cr.Groups) != 2 {
			t.Fatalf("raid %d: want 2 groups (shape + trinity), got %d", cr.Number, len(cr.Groups))
		}
		g1 := cr.Groups[0]
		if g1.Number != 1 || g1.Shape != "healstack" {
			t.Errorf("raid %d group1 = #%d shape %q, want #1 healstack (shapes claim the first numbers)", cr.Number, g1.Number, g1.Shape)
		}
		// 2 clerics + 1 tank in the shape group; open seats NOT topped up.
		if len(g1.Slots) != 3 {
			t.Errorf("raid %d shape group has %d slots, want 3 (reserved open seats)", cr.Number, len(g1.Slots))
		}
		clerics, tanks := 0, 0
		for _, s := range g1.Slots {
			switch {
			case s.Path == "healer.ch_cleric":
				clerics++
			case s.Path == "tank.defensive":
				tanks++
			default:
				t.Errorf("raid %d shape group slot %s has unexpected path %q", cr.Number, s.Member, s.Path)
			}
		}
		if clerics != 2 || tanks != 1 {
			t.Errorf("raid %d shape group = %d clerics + %d tanks, want 2+1", cr.Number, clerics, tanks)
		}
		// Coverage attribution: shape seats satisfy the MIN needs they match.
		if c := cohortCoverage(t, cr, "healer.ch_cleric"); c.Placed != 2 {
			t.Errorf("raid %d ch_cleric MIN placed = %d, want 2 (shape credits)", cr.Number, c.Placed)
		}
		if c := cohortCoverage(t, cr, "tank.defensive"); c.Placed != 1 {
			t.Errorf("raid %d tank.defensive MIN placed = %d, want 1 (shape credit)", cr.Number, c.Placed)
		}
		if c := cohortCoverage(t, cr, "damage"); c.Placed != 3 {
			t.Errorf("raid %d damage MIN placed = %d, want 3 (trinity pass)", cr.Number, c.Placed)
		}
		// MIN fully covered → no MIN-shortfall warnings.
		for _, w := range cr.Warnings {
			if strings.Contains(w, "of MIN") {
				t.Errorf("raid %d: unexpected MIN shortfall warning %q", cr.Number, w)
			}
		}
	}
	// Shape members are consumed for the whole proposal: each raid's clerics
	// and tanks are exactly the shape's picks (name tie-break), and the 6
	// rogues split 3/3 by the trinity pass.
	raid1 := rep.Cohorts[0]
	shapeNames := map[string]bool{}
	for _, s := range raid1.Groups[0].Slots {
		shapeNames[s.Member] = true
	}
	for _, name := range []string{"ClericA", "ClericB", "TankA"} {
		if !shapeNames[name] {
			t.Errorf("raid 1 shape group missing %q (got %v)", name, shapeNames)
		}
	}
	if len(raid1.Groups[1].Slots) != 3 {
		t.Errorf("raid 1 trinity group has %d slots, want the 3 rogues", len(raid1.Groups[1].Slots))
	}
}

func TestSplit_Shapes_Distribute(t *testing.T) {
	// distribute: healstack → raid 1, caststack → raid 2; a third shape is
	// beyond the cohort count → ignored with a warning.
	enc := shapesCohortComp("c",
		Shape{ID: "healstack", Rows: []ShapeRow{{Role: "healer", Sub: "ch_cleric", Count: 2}}},
		Shape{ID: "caststack", Rows: []ShapeRow{{Role: "damage", Count: 2}}},
		Shape{ID: "ccstack", Rows: []ShapeRow{{Role: "damage", Count: 1}}})
	roster := append(cohortRoster(),
		RosterMember{Name: "Rogue07", Class: CodeRogue},
		RosterMember{Name: "Rogue08", Class: CodeRogue},
	)
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"healstack", "caststack", "ccstack"},
		ShapeDistribution: ShapeDistributionDistribute,
	}, roster)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if g := rep.Cohorts[0].Groups[0]; g.Shape != "healstack" {
		t.Errorf("raid 1 first group shape = %q, want healstack", g.Shape)
	}
	if g := rep.Cohorts[1].Groups[0]; g.Shape != "caststack" {
		t.Errorf("raid 2 first group shape = %q, want caststack", g.Shape)
	}
	for _, cr := range rep.Cohorts {
		for _, g := range cr.Groups {
			if g.Shape == "ccstack" {
				t.Errorf("raid %d: ccstack should have been ignored (distribute, 2 raids)", cr.Number)
			}
		}
	}
	ignored := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, `"ccstack" ignored`) {
			ignored = true
		}
	}
	if !ignored {
		t.Errorf("warnings %v, want the ignored-shape note", rep.Warnings)
	}
}

func TestSplit_Shapes_UnderfilledWarnsAndKeepsOpenSeats(t *testing.T) {
	// 4 clerics total, shape wants 3 per raid: raid 1 takes 3, raid 2 runs
	// short — the row warns and the open seats stay open (no trinity top-up).
	enc := shapesCohortComp("c", Shape{ID: "healstack", Rows: []ShapeRow{
		{Role: "healer", Sub: "ch_cleric", Count: 3},
	}})
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"healstack"},
	}, cohortRoster())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	raid2 := rep.Cohorts[1]
	warned := false
	for _, w := range raid2.Warnings {
		if strings.Contains(w, `shape "healstack": healer.ch_cleric 1/3`) {
			warned = true
		}
	}
	if !warned {
		t.Errorf("raid 2 warnings %v, want the 1/3 under-fill note", raid2.Warnings)
	}
	shapeSlots := 0
	for _, g := range raid2.Groups {
		if g.Shape == "healstack" {
			shapeSlots += len(g.Slots)
		}
	}
	if shapeSlots != 1 {
		t.Errorf("raid 2 shape seats = %d, want exactly the 1 picked cleric (open seats reserved)", shapeSlots)
	}
	// Raid 1 filled its 3/3 — no under-fill warning there.
	for _, w := range rep.Cohorts[0].Warnings {
		if strings.Contains(w, "1/3") {
			t.Errorf("raid 1: unexpected under-fill warning %q", w)
		}
	}
}

func TestSplit_Shapes_PinCollisionsAndOutOfRangeFallBack(t *testing.T) {
	// alpha pins group 1; beta also pins group 1 (collision → first free);
	// gamma pins group 5 (beyond a 3-member raid's budget of 1 → first
	// free). All degrade loudly, never silently drop the shape.
	enc := shapesCohortComp("c",
		Shape{ID: "alpha", GroupNumber: 1, Rows: []ShapeRow{{Role: "tank", Sub: "defensive", Count: 1}}},
		Shape{ID: "beta", GroupNumber: 1, Rows: []ShapeRow{{Role: "tank", Sub: "defensive", Count: 1}}},
		Shape{ID: "gamma", GroupNumber: 5, Rows: []ShapeRow{{Role: "damage", Count: 1}}})
	roster := []RosterMember{
		{Name: "TankA", Class: CodeWarrior}, {Name: "TankB", Class: CodeWarrior},
		{Name: "TankC", Class: CodeWarrior}, {Name: "TankD", Class: CodeWarrior},
		{Name: "Rogue01", Class: CodeRogue}, {Name: "Rogue02", Class: CodeRogue},
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, Cohorts: 2, Shapes: []string{"alpha", "beta", "gamma"},
	}, roster)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	cr := rep.Cohorts[0]
	wantShape := map[int]string{1: "alpha", 2: "beta", 3: "gamma"}
	for _, g := range cr.Groups {
		if g.Shape != wantShape[g.Number] {
			t.Errorf("raid 1 group %d shape = %q, want %q", g.Number, g.Shape, wantShape[g.Number])
		}
	}
	warnBeta, warnGamma := false, false
	for _, w := range cr.Warnings {
		if strings.Contains(w, `"beta" pinned to group 1 is unavailable`) {
			warnBeta = true
		}
		if strings.Contains(w, `"gamma" pinned to group 5 is unavailable`) {
			warnGamma = true
		}
	}
	if !warnBeta || !warnGamma {
		t.Errorf("raid 1 warnings %v, want pin-fallback notes for beta and gamma", cr.Warnings)
	}
}
