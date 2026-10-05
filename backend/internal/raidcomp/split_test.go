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

// ── cohort mode (full template per cohort) ─────────────────────────────────

// cohortComp: each cohort targets the FULL template — 1 defensive tank,
// 2 CH clerics, 3 damage = 6 members per raid.
func cohortComp(id string) *Encounter {
	return &Encounter{
		ID: id, Name: "Cohort Encounter", Zone: "kael", Status: StatusActive,
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 1, Rec: 1},
			{Role: "healer", Sub: "ch_cleric", Min: 2, Rec: 2},
			{Role: "damage", Min: 3, Rec: 3},
		},
	}
}

func cohortRoster() []RosterMember {
	members := []RosterMember{
		{Name: "TankA", Class: CodeWarrior},
		{Name: "TankB", Class: CodeWarrior},
		{Name: "ClericA", Class: CodeCleric},
		{Name: "ClericB", Class: CodeCleric},
		{Name: "ClericC", Class: CodeCleric},
		{Name: "ClericD", Class: CodeCleric},
	}
	for i := 1; i <= 6; i++ {
		members = append(members, RosterMember{Name: fmt.Sprintf("Rogue%02d", i), Class: CodeRogue})
	}
	return members
}

func cohortCoverage(t *testing.T, cr CohortReport, path string) Coverage {
	t.Helper()
	for _, c := range cr.Min {
		if c.Path == path {
			return c
		}
	}
	t.Fatalf("cohort %d has no coverage row for %s", cr.Number, path)
	return Coverage{}
}

func TestSplit_Cohorts_FairScarceClassSplit(t *testing.T) {
	// 4 clerics, 2 cohorts each needing 2: the interleave must deal them
	// 2/2, not 3/1 or 4/0.
	rep, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 2}, cohortRoster())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(rep.Cohorts) != 2 {
		t.Fatalf("want 2 cohorts, got %d", len(rep.Cohorts))
	}
	for _, cr := range rep.Cohorts {
		cov := cohortCoverage(t, cr, "healer.ch_cleric")
		if cov.Placed != 2 {
			t.Errorf("raid %d clerics placed %d, want 2 (fair split)", cr.Number, cov.Placed)
		}
		if cov.Placed < cov.Need && len(cr.Warnings) == 0 {
			t.Errorf("raid %d short of MIN but no warning", cr.Number)
		}
	}
}

func TestSplit_Cohorts_ShortfallWarnsHonestly(t *testing.T) {
	// 3 clerics, 2 cohorts each needing 2 → 2/1 and raid 2 warns.
	members := []RosterMember{
		{Name: "TankA", Class: CodeWarrior},
		{Name: "TankB", Class: CodeWarrior},
		{Name: "ClericA", Class: CodeCleric},
		{Name: "ClericB", Class: CodeCleric},
		{Name: "ClericC", Class: CodeCleric},
		{Name: "Rogue01", Class: CodeRogue},
		{Name: "Rogue02", Class: CodeRogue},
		{Name: "Rogue03", Class: CodeRogue},
	}
	rep, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 2}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	c1 := cohortCoverage(t, rep.Cohorts[0], "healer.ch_cleric")
	c2 := cohortCoverage(t, rep.Cohorts[1], "healer.ch_cleric")
	if c1.Placed != 2 || c2.Placed != 1 {
		t.Errorf("cleric split = %d/%d, want 2/1 (interleave order)", c1.Placed, c2.Placed)
	}
	found := false
	for _, w := range rep.Cohorts[1].Warnings {
		if strings.Contains(w, "ch_cleric") || strings.Contains(w, "CH Cleric") {
			found = true
		}
	}
	if !found {
		t.Errorf("raid 2 should carry a MIN shortfall warning; warnings=%v", rep.Cohorts[1].Warnings)
	}
}

func TestSplit_MinSeatsBeforeSiblingRecSeats(t *testing.T) {
	// Regression for the weave priority (MIN, then role balance, then REC):
	// slower and debuffer.slows share the util bucket, so the old drain
	// order was slower.MIN → slower.REC → debuffer.MIN. With a shaman and
	// an enchanter in the raid, slower.REC claimed the enchanter and
	// debuffer.slows.MIN sat empty (live-caught 2026-10-04: bot-agnarr
	// debuffer 0/1 while every enchanter sat slower.REC). Name tie-break:
	// "Ack" < "Bee", so Ack must take slower.MIN and Bee debuffer.MIN.
	leaves := []RoleLeaf{
		{Position: 1, Role: "slower", Label: "Slower", Classes: []ClassCode{CodeShaman, CodeEnchanter}},
		{Position: 2, Role: "debuffer", Sub: "slows", Label: "Debuff Slows", Classes: []ClassCode{CodeEnchanter}},
		{Position: 3, Role: "damage", Label: "Damage", Classes: []ClassCode{CodeRogue}},
	}
	enc := &Encounter{ID: "sib", Name: "Sibling", Zone: "kael", Status: StatusActive,
		Comps: []CompRow{
			{Role: "slower", Min: 1, Rec: 1},
			{Role: "debuffer", Sub: "slows", Min: 1},
			{Role: "damage", Min: 1},
		},
	}
	members := []RosterMember{
		{Name: "Ack", Class: CodeShaman},
		{Name: "Bee", Class: CodeEnchanter},
		{Name: "Rogue1", Class: CodeRogue},
		{Name: "Rogue2", Class: CodeRogue},
		{Name: "Rogue3", Class: CodeRogue},
	}
	rep, err := Split(leaves, enc, SplitRequest{Preference: SplitTrinity}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	placed := map[string]string{}
	for _, g := range rep.Groups {
		for _, s := range g.Slots {
			if s.Path != "" {
				placed[s.Path] = s.Member
			}
		}
	}
	if got := placed["debuffer.slows"]; got != "Bee" {
		t.Errorf("debuffer.slows.MIN = %q, want Bee seated before slower.REC can claim her", got)
	}
	if got := placed["slower"]; got != "Ack" {
		t.Errorf("slower = %q, want Ack on the MIN seat (Bee must not be hogged by slower.REC)", got)
	}
}

func TestSplit_Cohorts_EveryoneSeated(t *testing.T) {
	rep, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 2}, cohortRoster())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	total := 0
	for _, cr := range rep.Cohorts {
		for _, g := range cr.Groups {
			if len(g.Slots) > g.Size {
				t.Errorf("raid %d group %d overfull: %d/%d", cr.Number, g.Number, len(g.Slots), g.Size)
			}
			total += len(g.Slots)
		}
		if cr.RosterCount != 6 {
			t.Errorf("raid %d roster_count = %d, want 6 (12 members / 2 cohorts)", cr.Number, cr.RosterCount)
		}
	}
	if total != 12 {
		t.Errorf("seated %d of 12 members", total)
	}
	if len(rep.Unassigned) != 0 {
		t.Errorf("cohort mode should seat everyone; unassigned=%v", rep.Unassigned)
	}
}

func TestSplit_Cohorts_LiveGroupSeedingKeepsTowerGroups(t *testing.T) {
	// 12 members across 4 live groups: groups 1,2 → raid 1; 3,4 → raid 2.
	var members []RosterMember
	for g := 1; g <= 4; g++ {
		for i := 0; i < 3; i++ {
			members = append(members, RosterMember{
				Name:  fmt.Sprintf("G%dm%d", g, i),
				Class: CodeRogue,
				Group: fmt.Sprintf("%d", g),
			})
		}
	}
	rep, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 2}, members)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	for _, cr := range rep.Cohorts {
		want := cr.Number // raid 1 = live groups 1,2; raid 2 = 3,4
		for _, g := range cr.Groups {
			for _, s := range g.Slots {
				prefix := fmt.Sprintf("G%d", want*2-1)
				prefix2 := fmt.Sprintf("G%d", want*2)
				if !strings.HasPrefix(s.Member, prefix) && !strings.HasPrefix(s.Member, prefix2) {
					t.Errorf("raid %d group %d holds %q — live groups leaked across cohorts", cr.Number, g.Number, s.Member)
				}
			}
		}
	}
}

func TestSplit_Cohorts_RejectsFocusedAndCurated(t *testing.T) {
	if _, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitFocused, Cohorts: 2}, cohortRoster()); err == nil {
		t.Fatal("focused + cohorts must be rejected")
	}
	if _, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitCurated, Cohorts: 2}, cohortRoster()); err == nil {
		t.Fatal("curated + cohorts must be rejected")
	}
	if _, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 7}, cohortRoster()); err == nil {
		t.Fatal("cohorts 7 must be rejected (max 6)")
	}
}

func TestSplit_Cohorts_LegacyShapeForSingleRaid(t *testing.T) {
	rep, err := Split(splitLeaves(t), cohortComp("c"), SplitRequest{Preference: SplitTrinity, Cohorts: 1}, cohortRoster())
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(rep.Groups) == 0 || len(rep.Min) == 0 {
		t.Fatal("cohorts=1 must populate the legacy fields")
	}
	if len(rep.Cohorts) != 1 || len(rep.Cohorts[0].Groups) == 0 {
		t.Fatal("cohorts=1 must mirror the legacy shape as one cohort entry")
	}
}

func TestSplitPlan_CapsAndBinding(t *testing.T) {
	// cohortComp: tank min 1 (2 wars → ratio 2), ch_cleric min 2 (4 clerics → 2),
	// damage min 3 (6 rogues → 2) → max 2 cohorts.
	plan := SplitPlan(splitLeaves(t), cohortComp("c"), cohortRoster())
	if plan.MaxCohorts != 2 {
		t.Errorf("max_cohorts = %d, want 2", plan.MaxCohorts)
	}
	// Starve clerics to 2 (ratio 1) while tanks keep ratio 2: binding is
	// ch_cleric, the first leaf to hit the minimum ratio.
	starved := []RosterMember{
		{Name: "TankA", Class: CodeWarrior},
		{Name: "TankB", Class: CodeWarrior},
		{Name: "ClericA", Class: CodeCleric},
		{Name: "ClericB", Class: CodeCleric},
		{Name: "Rogue01", Class: CodeRogue},
		{Name: "Rogue02", Class: CodeRogue},
		{Name: "Rogue03", Class: CodeRogue},
	}
	plan = SplitPlan(splitLeaves(t), cohortComp("c"), starved)
	if plan.MaxCohorts != 1 {
		t.Errorf("starved max_cohorts = %d, want 1", plan.MaxCohorts)
	}
	if plan.BindingPath != "healer.ch_cleric" {
		t.Errorf("binding = %q, want healer.ch_cleric", plan.BindingPath)
	}
	// Roster can't staff one comp: ratio 0 floors to the advisory 1.
	lone := []RosterMember{{Name: "Solo", Class: CodeRogue}}
	plan = SplitPlan(splitLeaves(t), cohortComp("c"), lone)
	if plan.MaxCohorts != 1 {
		t.Errorf("unstaffable max_cohorts = %d, want 1", plan.MaxCohorts)
	}
}
