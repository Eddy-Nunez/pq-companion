package raidcomp

import (
	"fmt"
	"strings"
	"testing"
)

// splitFixture builds a standard 24-member raid roster with group numbers.
func splitFixture() []RosterMember {
	type row struct {
		name  string
		code  ClassCode
		group string
	}
	rows := []row{
		{"Bonce", CodeWarrior, "1"}, {"Kavija", CodeWarrior, "1"}, {"Ironmule", CodeWarrior, "2"},
		{"Dorfstick", CodeShadowKnight, "2"}, {"Holyheal", CodeCleric, "3"}, {"Cureall", CodeCleric, "3"},
		{"Bigheal", CodeCleric, "4"}, {"Tickleheal", CodeCleric, "4"}, {"Leafblower", CodeDruid, "5"},
		{"Dogcaller", CodeBeastlord, "5"}, {"Slowmo", CodeShaman, "5"}, {"Slowtwo", CodeShaman, "6"},
		{"Encheese", CodeEnchanter, "6"}, {"Mezzes", CodeEnchanter, "7"}, {"Tricky", CodeRogue, "7"},
		{"Stabby", CodeRogue, "7"}, {"Pointy", CodeRanger, "8"}, {"Nuker", CodeWizard, "8"},
		{"Elemental", CodeMagician, "8"}, {"Bonedaddy", CodeNecromancer, "9"}, {"Chains", CodeMonk, "9"},
		{"Pulltwo", CodeMonk, "9"}, {"Twangy", CodeBard, "10"}, {"Crowdctl", CodeEnchanter, "10"},
	}
	members := make([]RosterMember, 0, len(rows))
	for _, r := range rows {
		members = append(members, RosterMember{Name: r.name, Class: r.code, Group: r.group})
	}
	return members
}

// splitEncounter returns a small encounter comp: 2 defensive tanks, 2 snap,
// 5 CH clerics, 1 druid healer, 2 slowers, 20 damage — close to the seed.
func splitEncounter(id string) *Encounter {
	return &Encounter{
		ID: id, Name: "Test Encounter", Zone: "kael", Status: StatusActive,
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 2, Rec: 3},
			{Role: "tank", Sub: "snap", Min: 1, Rec: 2},
			{Role: "healer", Sub: "ch_cleric", Min: 4, Rec: 6},
			{Role: "healer", Sub: "ch_druid", Min: 1, Rec: 2},
			{Role: "slower", Min: 2, Rec: 2},
			{Role: "damage", Min: 12, Rec: 18},
		},
	}
}

func splitLeaves(t *testing.T) []RoleLeaf {
	t.Helper()
	roles := SeedRoles()
	return LeavesFromRoles(roles)
}

func slotByMember(rep *SplitReport, name string) *Slot {
	for _, g := range rep.Groups {
		for i := range g.Slots {
			if g.Slots[i].Member == name {
				return &g.Slots[i]
			}
		}
	}
	return nil
}

func findGroup(rep *SplitReport, n int) *ProposedGroup {
	for i := range rep.Groups {
		if rep.Groups[i].Number == n {
			return &rep.Groups[i]
		}
	}
	return nil
}

func TestSplit_TrinitySeatsEveryoneAndFillsMin(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{
		{Role: "tank", Sub: "defensive", Min: 2, Rec: 2},
		{Role: "healer", Sub: "ch_cleric", Min: 2, Rec: 3},
		{Role: "slower", Min: 1, Rec: 1},
		{Role: "damage", Min: 9, Rec: 12},
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitTrinity, GroupSize: 6}, splitFixture())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(rep.Groups) == 0 {
		t.Fatal("no groups in report")
	}
	total := 0
	for _, g := range rep.Groups {
		if len(g.Slots) > g.Size {
			t.Errorf("group %d has %d slots > size %d", g.Number, len(g.Slots), g.Size)
		}
		total += len(g.Slots)
	}
	// 24 roster members, all classed and seatable: every member seated.
	if total != 24 {
		t.Errorf("seated %d members, want 24", total)
	}
	if len(rep.Unassigned) != 0 {
		t.Errorf("unexpected unassigned: %+v", rep.Unassigned)
	}
	// MIN coverage: every leaf fully placed.
	for _, c := range rep.Min {
		if c.Placed < c.Need {
			t.Errorf("min coverage %s: placed %d < need %d", c.Path, c.Placed, c.Need)
		}
	}
}

func TestSplit_TrinityWeavesTankHealerEarly(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{
		{Role: "tank", Sub: "defensive", Min: 3, Rec: 3},
		{Role: "healer", Sub: "ch_cleric", Min: 6, Rec: 6},
		{Role: "damage", Min: 9, Rec: 9},
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitTrinity, GroupSize: 6}, splitFixture())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// Group 1 must have a tank and a healer in its first seats (weave order).
	g := findGroup(rep, 1)
	if g == nil {
		t.Fatal("group 1 missing")
	}
	hasTank, hasHealer := false, false
	for i, s := range g.Slots {
		if i >= 3 {
			break
		}
		if s.Role == "tank" && s.Sub == "defensive" {
			hasTank = true
		}
		if s.Role == "healer" && s.Sub == "ch_cleric" {
			hasHealer = true
		}
	}
	if !hasTank || !hasHealer {
		t.Errorf("group 1 lacks trinity seats: tank=%v healer=%v (slots %+v)", hasTank, hasHealer, g.Slots)
	}
}

func TestSplit_FocusedClustersSameClass(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{
		{Role: "healer", Sub: "ch_cleric", Min: 4, Rec: 4},
		{Role: "damage", Min: 8, Rec: 8},
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitFocused, GroupSize: 6}, splitFixture())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// All four clerics (Holyheal, Cureall, Bigheal, Tickleheal) should share
	// one group under the focused preference.
	clerics := map[string]bool{"Holyheal": true, "Cureall": true, "Bigheal": true, "Tickleheal": true}
	groups := map[int]int{}
	for _, g := range rep.Groups {
		for _, s := range g.Slots {
			if clerics[s.Member] {
				groups[g.Number]++
			}
		}
	}
	if len(groups) != 1 {
		t.Errorf("clerics spread across %d groups, want 1: %v", len(groups), groups)
	}
	// And the healer slots must all be filled by clerics.
	for _, c := range rep.Min {
		if c.Path == "healer.ch_cleric" && c.Placed < c.Need {
			t.Errorf("focused: cleric CH slots placed %d < need %d", c.Placed, c.Need)
		}
	}
}

func TestSplit_CuratedPinsAndCaps(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{
		{Role: "tank", Sub: "defensive", Min: 1, Rec: 1},
		{Role: "healer", Sub: "ch_cleric", Min: 2, Rec: 3},
		{Role: "damage", Min: 4, Rec: 6},
	}
	req := SplitRequest{
		Preference: SplitCurated,
		GroupSize:  6,
		Wildcards: []Wildcard{
			{Kind: "member", Member: "Bonce", Group: 2, Locked: true},
			{Kind: "class", Value: "clr", Max: 2},
			{Kind: "class", Value: "clr", Min: 2},
		},
	}
	rep, err := Split(splitLeaves(t), enc, req, splitFixture())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// Bonce pinned to group 2.
	g2 := findGroup(rep, 2)
	if g2 == nil {
		t.Fatal("group 2 missing")
	}
	found := false
	for _, s := range g2.Slots {
		if s.Member == "Bonce" {
			found = true
		}
	}
	if !found {
		t.Errorf("Bonce not seated in group 2: %+v", g2.Slots)
	}
	// Cleric cap: at most 2 clerics placed anywhere.
	clericN := 0
	for _, g := range rep.Groups {
		for _, s := range g.Slots {
			if s.Class == CodeCleric {
				clericN++
			}
		}
	}
	if clericN > 2 {
		t.Errorf("cleric cap violated: %d clerics placed (cap 2)", clericN)
	}
	// Cleric min met → no warning mentioning it.
	for _, w := range rep.Warnings {
		if strings.Contains(w, "class=\"clr\"") && strings.Contains(w, "wanted at least") {
			t.Errorf("unexpected min warning despite meeting it: %s", w)
		}
	}
}

func TestSplit_CuratedPinnedUnknownMemberWarns(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{{Role: "damage", Min: 2, Rec: 2}}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitCurated,
		Wildcards:  []Wildcard{{Kind: "member", Member: "Nobody", Group: 1}},
	}, splitFixture())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	found := false
	for _, w := range rep.Warnings {
		if strings.Contains(w, "Nobody") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning about Nobody, got %v", rep.Warnings)
	}
}

func TestSplit_GroupsFullMembersUnassigned(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{{Role: "damage", Min: 3, Rec: 3}}
	// 8 rogues into 3 damage slots with group size 6 → 5 seated... but only
	// 3 damage slots exist and everything else is fill: all 8 seat (fill).
	// Use a tiny group size instead to force overflow: size 3 → 3 groups of
	// 3 = 9 seats; 8 members seat, none unassigned. Make it 12 members.
	var members []RosterMember
	for i := 1; i <= 14; i++ {
		members = append(members, RosterMember{Name: fmt.Sprintf("Rogue%02d", i), Class: CodeRogue})
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitTrinity, GroupSize: 3}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	// 12 groups × 3 = 36 seats ≥ 14 members: all seat. Flip: size 1 forces
	// one member per group → 14 members need 14 groups > 12 → 2 unassigned.
	rep2, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitTrinity, GroupSize: 1}, members)
	if err != nil {
		t.Fatalf("Split size 1: %v", err)
	}
	if len(rep2.Unassigned) != 2 {
		t.Errorf("want 2 unassigned at group size 1, got %d: %+v", len(rep2.Unassigned), rep2.Unassigned)
	}
	for _, u := range rep2.Unassigned {
		if u.Reason == "" {
			t.Errorf("unassigned member %q missing reason", u.Name)
		}
	}
	_ = rep
}

func TestSplit_UnclassedMembersSeatAsFillOnly(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{
		{Role: "healer", Sub: "ch_cleric", Min: 1, Rec: 1},
		{Role: "damage", Min: 2, Rec: 2},
	}
	members := []RosterMember{
		{Name: "Mystery", Class: ""}, // classless: never holds a comp slot
		{Name: "Healfast", Class: CodeCleric},
		{Name: "Stabber", Class: CodeRogue},
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{Preference: SplitTrinity}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	ms := slotByMember(rep, "Mystery")
	if ms == nil {
		t.Fatal("Mystery should still be seated as fill")
	}
	if ms.Role != "" {
		t.Errorf("classless member got a comp role %q — must be fill only", ms.Role)
	}
	for _, c := range rep.Min {
		if c.Need > 0 && c.Placed == 0 {
			t.Errorf("coverage %s: classless member stole the slot or nobody filled it", c.Path)
		}
	}
}

func TestSplit_InvalidPreferenceAndSize(t *testing.T) {
	if _, err := Split(splitLeaves(t), splitEncounter("e"), SplitRequest{Preference: "nope"}, nil); err == nil {
		t.Fatal("want error for invalid preference")
	}
	if _, err := Split(splitLeaves(t), splitEncounter("e"), SplitRequest{Preference: SplitTrinity, GroupSize: 13}, nil); err == nil {
		t.Fatal("want error for group_size > 12")
	}
	if _, err := Split(splitLeaves(t), nil, SplitRequest{Preference: SplitTrinity}, nil); err == nil {
		t.Fatal("want error for nil encounter")
	}
}

func TestSplit_RespectExistingGroupsKeepsLiveGroupsTogether(t *testing.T) {
	enc := splitEncounter("enc")
	enc.Comps = []CompRow{{Role: "damage", Min: 6, Rec: 6}, {Role: "healer", Sub: "ch_cleric", Min: 1, Rec: 1}}
	// Live: 5 rogues in group 3, cleric in group 4.
	members := []RosterMember{
		{Name: "Cureall", Class: CodeCleric, Group: "4"},
		{Name: "R1", Class: CodeRogue, Group: "3"},
		{Name: "R2", Class: CodeRogue, Group: "3"},
		{Name: "R3", Class: CodeRogue, Group: "3"},
		{Name: "R4", Class: CodeRogue, Group: "3"},
		{Name: "R5", Class: CodeRogue, Group: "3"},
		{Name: "R6", Class: CodeRogue}, // no live group
	}
	rep, err := Split(splitLeaves(t), enc, SplitRequest{
		Preference: SplitTrinity, RespectExistingGroups: true, GroupSize: 6,
	}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	inG3 := 0
	for _, g := range rep.Groups {
		for _, s := range g.Slots {
			if g.Number == 3 {
				inG3++
			}
			_ = s
		}
	}
	if inG3 < 5 {
		t.Errorf("live group 3 held together: want >=5 members in group 3, got %d (groups %+v)", inG3, rep.Groups)
	}
}
