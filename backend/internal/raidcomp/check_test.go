package raidcomp

import (
	"testing"
)

// aowFixture returns the seed-time aow encounter (comp rows transcribed from
// eqmon's raids/velious.yaml) for checker tests.
func aowFixture() *Encounter {
	e := SeedEncounters()[0]
	e.ID = "aow"
	return &e
}

// seedLeaves returns the starter taxonomy flattened — what an unmodified
// store would hand the checker.
func seedLeaves() []RoleLeaf {
	return LeavesFromRoles(SeedRoles())
}

func m(name string, class ClassCode) RosterMember {
	return RosterMember{Name: name, Class: class}
}

// fullRaid builds a roster that satisfies aow's min_comp exactly.
func fullRaid() []RosterMember {
	members := []RosterMember{
		m("WarA", CodeWarrior), m("WarB", CodeWarrior), m("WarC", CodeWarrior), // tank.defensive 3
		m("SkA", CodeShadowKnight), m("PalA", CodePaladin), // tank.snap 2
		m("ClrA", CodeCleric), m("ClrB", CodeCleric), m("ClrC", CodeCleric),
		m("ClrD", CodeCleric), m("ClrE", CodeCleric), // healer.ch_cleric 5
		m("DruA", CodeDruid), m("DruB", CodeDruid), // healer.ch_druid 2
		m("ShmA", CodeShaman), m("ShmB", CodeShaman), // slower 2
		m("EncA", CodeEnchanter), // debuffer.slows / cripple / mr
		m("RogA", CodeRogue),     // traps / lockpicker
	}
	for i := 0; i < 20; i++ {
		members = append(members, m("Dps"+string(rune('A'+i%26))+string(rune('0'+i/26)), CodeRogue)) // damage 20 (rogues are damage class)
	}
	return members
}

func TestCheck_ClassCodeMapping(t *testing.T) {
	cases := []struct {
		in   string
		want ClassCode
		ok   bool
	}{
		{"sk", CodeShadowKnight, true},
		{"SK", CodeShadowKnight, true},
		{"Shadow Knight", CodeShadowKnight, true},
		{"shadowknight", CodeShadowKnight, true},
		{"SHADOW KNIGHT", CodeShadowKnight, true},
		{"war", CodeWarrior, true},
		{"Wizard", CodeWizard, true},
		{"wiz", CodeWizard, true},
		{"beastlord", CodeBeastlord, true},
		{"", "", false},
		{"bard", CodeBard, true},
	}
	for _, c := range cases {
		got, ok := CodeForInput(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("CodeForInput(%q) = %q,%v want %q,%v", c.in, got, ok, c.want, c.ok)
		}
	}
	// Zeal class ids: 1=Warrior .. 15=Beastlord
	if code, ok := CodeForZealID(1); !ok || code != CodeWarrior {
		t.Errorf("CodeForZealID(1) = %q,%v", code, ok)
	}
	if code, ok := CodeForZealID(5); !ok || code != CodeShadowKnight {
		t.Errorf("CodeForZealID(5) = %q,%v", code, ok)
	}
	if _, ok := CodeForZealID(0); ok {
		t.Error("CodeForZealID(0) should be unknown")
	}
}

func TestCheck_FullRaidMeetsMin(t *testing.T) {
	rep := Check(seedLeaves(), aowFixture(), fullRaid())
	if rep.RosterTotal != 36 {
		t.Fatalf("RosterTotal = %d, want 36", rep.RosterTotal)
	}
	if !rep.Summary.Min.OK {
		t.Errorf("min should be OK, got %+v", rep.Summary.Min)
	}
	for _, row := range rep.Min {
		if row.Gap() > 0 {
			t.Errorf("row %s has gap %d (have %d need %d)", row.Path, row.Gap(), row.Have, row.Need)
		}
	}
}

func TestCheck_ShortRaidSurfacesGapsAndCandidates(t *testing.T) {
	// No warriors, no clerics, one shaman, one rogue — several gaps at MIN.
	members := []RosterMember{
		m("OnlyShm", CodeShaman),
		m("Rogue", CodeRogue),
	}
	rep := Check(seedLeaves(), aowFixture(), members)

	if rep.Summary.Min.OK {
		t.Error("min should not be OK")
	}
	byRow := map[string]RowReport{}
	for _, r := range rep.Min {
		byRow[r.Path] = r
	}
	// tank.defensive needs 3 warriors, has 0
	if r := byRow["tank.defensive"]; r.Need != 3 || r.Have != 0 || r.Gap() != 3 {
		t.Errorf("tank.defensive = %+v, want need=3 have=0 gap=3", r)
	}
	// healer.ch_cleric needs 5 clerics, has 0, and should list no candidates
	if r := byRow["healer.ch_cleric"]; len(r.Candidates) != 0 {
		t.Errorf("healer.ch_cleric candidates = %v, want none", r.Candidates)
	}
	// slower needs 2 shamans, has 1 -> gap 1, candidate OnlyShm listed
	if r := byRow["slower"]; r.Need != 2 || r.Have != 1 || r.Gap() != 1 {
		t.Errorf("slower = %+v, want need=2 have=1", r)
	} else if len(r.Candidates) != 1 || r.Candidates[0] != "OnlyShm" {
		t.Errorf("slower candidates = %v, want [OnlyShm]", r.Candidates)
	}
	// damage needs 20, has 1 (Rogue)
	if r := byRow["damage"]; r.Gap() != 19 {
		t.Errorf("damage gap = %d, want 19", r.Gap())
	}
}

func TestCheck_DamageCountsAllDamageClasses(t *testing.T) {
	members := []RosterMember{
		m("Rog", CodeRogue), m("Rng", CodeRanger), m("Mnk", CodeMonk),
		m("Mag", CodeMagician), m("Wiz", CodeWizard), m("Nec", CodeNecromancer),
		m("Bst", CodeBeastlord),
	}
	rep := Check(seedLeaves(), aowFixture(), members)
	for _, r := range rep.Min {
		if r.Path == "damage" {
			if r.Have != 7 {
				t.Errorf("damage have = %d, want 7 (all damage classes)", r.Have)
			}
		}
	}
}

func TestCheck_UnknownClassExcluded(t *testing.T) {
	members := []RosterMember{
		m("Mystery", ""),       // no code — excluded from counts and candidates
		m("WarA", CodeWarrior), // 1 warrior
	}
	rep := Check(seedLeaves(), aowFixture(), members)
	if rep.RosterTotal != 2 {
		t.Errorf("RosterTotal = %d, want 2", rep.RosterTotal)
	}
	if rep.RosterMapped != 1 {
		t.Errorf("RosterMapped = %d, want 1", rep.RosterMapped)
	}
	for _, r := range rep.Min {
		if r.Path == "tank.defensive" {
			if r.Have != 1 {
				t.Errorf("tank.defensive have = %d, want 1 (unknown excluded)", r.Have)
			}
			for _, c := range r.Candidates {
				if c == "Mystery" {
					t.Error("unknown-class member appeared in candidates")
				}
			}
		}
	}
}

func TestCheck_SubTypeFlatteningAndNeedLookup(t *testing.T) {
	// healer.ch_cleric counts only clerics; a druid must not satisfy it.
	members := []RosterMember{
		m("DruC", CodeDruid), m("DruD", CodeDruid), m("DruE", CodeDruid),
		m("DruF", CodeDruid), m("DruG", CodeDruid), m("DruH", CodeDruid),
	}
	rep := Check(seedLeaves(), aowFixture(), members)
	byRow := map[string]RowReport{}
	for _, r := range rep.Min {
		byRow[r.Path] = r
	}
	if r := byRow["healer.ch_cleric"]; r.Have != 0 || r.Need != 5 {
		t.Errorf("healer.ch_cleric = have %d need %d, want 0/5", r.Have, r.Need)
	}
	if r := byRow["healer.ch_druid"]; r.Have != 6 || r.Need != 2 {
		t.Errorf("healer.ch_druid = have %d need %d, want 6/2", r.Have, r.Need)
	}
	// rgc has min_count 0 — it must NOT appear in the report at all
	if _, ok := byRow["rgc"]; ok {
		t.Error("rgc (min 0) should be omitted from the report")
	}
}

func TestCheck_ZeroNeedRowsOmitted(t *testing.T) {
	e := &Encounter{ID: "mix", Name: "Mix", Zone: "Z", Status: StatusActive, Comps: []CompRow{
		{Role: "rgc", Min: 0, Rec: 0},        // both zero -> never shown
		{Role: "lockpicker", Min: 0, Rec: 2}, // min pass skips, rec pass shows
		{Role: "damage", Min: 5, Rec: 5},
	}}
	rep := Check(seedLeaves(), e, []RosterMember{m("Rog1", CodeRogue), m("Rog2", CodeRogue)})
	for _, r := range rep.Min {
		if r.Path == "rgc" || r.Path == "lockpicker" {
			t.Errorf("%s should be omitted from MIN (need 0), got %+v", r.Path, r)
		}
	}
	foundRecLockpicker := false
	for _, r := range rep.Rec {
		if r.Path == "lockpicker" {
			foundRecLockpicker = true
			if r.Need != 2 {
				t.Errorf("lockpicker rec need = %d, want 2", r.Need)
			}
		}
		if r.Path == "rgc" {
			t.Error("rgc should be omitted from REC too (need 0)")
		}
	}
	if !foundRecLockpicker {
		t.Error("lockpicker should appear in REC (need 2) despite min 0")
	}
	if rep.Summary.Min.Rows != 1 || rep.Summary.Min.GapRows != 1 {
		t.Errorf("min summary wrong: %+v (want 1 row, 1 gap: damage 5 vs 2)", rep.Summary.Min)
	}
}

func TestCheck_RecVsMin(t *testing.T) {
	// A full-min roster; compare REC-vs-MIN per leaf, by path (the two arrays
	// are no longer index-aligned: zero-need rows are filtered per level).
	rep := Check(seedLeaves(), aowFixture(), fullRaid())
	byMin := map[string]RowReport{}
	byRec := map[string]RowReport{}
	for _, r := range rep.Min {
		byMin[r.Path] = r
	}
	for _, r := range rep.Rec {
		byRec[r.Path] = r
	}
	for path, m := range byMin {
		rr, ok := byRec[path]
		if !ok {
			t.Errorf("%s present in MIN but missing from REC", path)
			continue
		}
		if rr.Need < m.Need {
			t.Errorf("rec need (%d) < min need (%d) for %s", rr.Need, m.Need, path)
		}
	}
}

func TestCheck_EmptyCompAndMissingRoles(t *testing.T) {
	e := &Encounter{ID: "bare", Name: "Bare", Zone: "Nowhere"}
	rep := Check(seedLeaves(), e, []RosterMember{m("WarA", CodeWarrior)})
	if rep.Summary.Min.Rows != 0 || rep.Summary.Min.OK {
		t.Errorf("empty comp: min should have 0 rows, got %+v", rep.Summary.Min)
	}
}

func TestLeaves_OrderAndCoverage(t *testing.T) {
	leaves := seedLeaves()
	seen := map[string]bool{}
	for _, l := range leaves {
		seen[l.Path()] = true
	}
	// all grouped sub-types present
	for _, want := range []string{"tank.defensive", "tank.snap", "healer.ch_cleric", "healer.ch_druid",
		"debuffer.slows", "debuffer.cripple", "debuffer.mr", "debuffer.dr", "debuffer.pr",
		"debuffer.fr", "debuffer.cr", "mind_wrack", "damage"} {
		if !seen[want] {
			t.Errorf("leaf %q missing from Leaves()", want)
		}
	}
	// role order preserved at the top level
	flat := []string{}
	last := ""
	for _, l := range leaves {
		if l.Role != last {
			flat = append(flat, l.Role)
			last = l.Role
		}
	}
	wantOrder := []string{"tank", "healer", "rgc", "lockpicker", "traps", "tracker",
		"coth", "slower", "puller", "debuffer", "mind_wrack", "damage"}
	if len(flat) != len(wantOrder) {
		t.Fatalf("top-level roles = %v, want %v", flat, wantOrder)
	}
	for i := range wantOrder {
		if flat[i] != wantOrder[i] {
			t.Errorf("role order[%d] = %s, want %s", i, flat[i], wantOrder[i])
		}
	}
}
