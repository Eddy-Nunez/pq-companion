package raidcomp

// SeedRoles returns the starter role taxonomy, transcribed from eqmon's
// raids/base.yaml with human display labels (internal ids stay stable).
// Positions follow eqmon's ROLE_ORDER flattened (subs before the next role).
func SeedRoles() []Role {
	roles := []Role{
		// tank
		{Role: "tank", Sub: "defensive", Label: "Tank / Defensive", Classes: ClassSet{CodeWarrior}, Position: 0},
		{Role: "tank", Sub: "snap", Label: "Tank / Snap Aggro", Classes: ClassSet{CodeShadowKnight, CodePaladin}, Position: 1},
		// healer
		{Role: "healer", Sub: "ch_cleric", Label: "Healer / CH Cleric", Classes: ClassSet{CodeCleric}, Position: 2},
		{Role: "healer", Sub: "ch_druid", Label: "Healer / CH Druid", Classes: ClassSet{CodeDruid}, Position: 3},
		// flat roles
		{Role: "rgc", Label: "Remove Greater Curse", Classes: ClassSet{CodeCleric, CodeDruid, CodeShaman, CodePaladin}, Position: 4},
		{Role: "lockpicker", Label: "Lockpicker", Classes: ClassSet{CodeRogue, CodeBard}, Position: 5},
		{Role: "traps", Label: "Traps", Classes: ClassSet{CodeRogue, CodeBard}, Position: 6},
		{Role: "tracker", Label: "Tracker", Classes: ClassSet{CodeRanger, CodeDruid, CodeBard}, Position: 7},
		{Role: "coth", Label: "Call of the Hero", Classes: ClassSet{CodeMagician}, Position: 8},
		{Role: "slower", Label: "Slower", Classes: ClassSet{CodeShaman, CodeEnchanter, CodeBeastlord}, Position: 9},
		{Role: "puller", Label: "Puller", Classes: ClassSet{CodeShadowKnight, CodeMonk}, Position: 10},
		// debuffer
		{Role: "debuffer", Sub: "slows", Label: "Debuffer / Slows", Classes: ClassSet{CodeShaman, CodeEnchanter}, Position: 11},
		{Role: "debuffer", Sub: "cripple", Label: "Debuffer / Cripple", Classes: ClassSet{CodeShaman, CodeEnchanter}, Position: 12},
		{Role: "debuffer", Sub: "mr", Label: "Debuffer / MR", Classes: ClassSet{CodeEnchanter, CodeShaman, CodeMagician}, Position: 13},
		{Role: "debuffer", Sub: "dr", Label: "Debuffer / Disease Resist", Classes: ClassSet{CodeShaman, CodeMagician, CodeNecromancer}, Position: 14},
		{Role: "debuffer", Sub: "pr", Label: "Debuffer / Poison Resist", Classes: ClassSet{CodeShaman, CodeMagician, CodeNecromancer}, Position: 15},
		{Role: "debuffer", Sub: "fr", Label: "Debuffer / Fire Resist", Classes: ClassSet{CodeShaman, CodeMagician, CodeNecromancer, CodeDruid}, Position: 16},
		{Role: "debuffer", Sub: "cr", Label: "Debuffer / Cold Resist", Classes: ClassSet{CodeShaman, CodeMagician}, Position: 17},
		// more flat
		{Role: "mind_wrack", Label: "Mind Wrack", Classes: ClassSet{CodeNecromancer}, Position: 18},
		{Role: "damage", Label: "Damage", Classes: ClassSet{CodeRogue, CodeRanger, CodeMonk, CodeMagician, CodeWizard, CodeNecromancer, CodeBeastlord}, Position: 19},
	}
	return roles
}

// SeedEncounters returns the starter raid knowledge base, transcribed from
// eqmon's raids/velious.yaml (the aow encounter). Counts are eqmon's
// suggested starting values, sized for a ~54-person raid. ZoneID is resolved
// at first-open time by the OpenStore zone resolver (the user.db store cannot
// read quarm.db itself).
//
// Zone corrected to "Kael Drakkel" (quarm.db zoneidnumber 113) — eqmon's
// source data listed "Temple of Veeshan", which doesn't match where Avatar of
// War actually spawns and was inconsistent with this same seed's own Notes
// field ("No rgc/lockpicker/tracker/coth needed on the Kael path"). Spelling
// matches quarm.db's zone.long_name exactly so ZoneIDByLongName resolves it.
func SeedEncounters() []Encounter {
	notes := "Counts are starting suggestions, not canonical. AoW himself is unslowable but " +
		"the surrounding mobs are not — slower stays for adds. No rgc/lockpicker/tracker/coth " +
		"needed on the Kael path; RGC staffing is for Ssra (Luclin). Adjust debuffer/resist " +
		"coverage to the guild class mix."
	return []Encounter{{
		ID:     "aow",
		Name:   "Avatar of War",
		Zone:   "Kael Drakkel",
		Status: StatusActive,
		Notes:  notes,
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 3, Rec: 4},
			{Role: "tank", Sub: "snap", Min: 2, Rec: 2},
			{Role: "healer", Sub: "ch_cleric", Min: 5, Rec: 6},
			{Role: "healer", Sub: "ch_druid", Min: 2, Rec: 2},
			{Role: "rgc", Min: 0, Rec: 0},
			{Role: "lockpicker", Min: 0, Rec: 0},
			{Role: "traps", Min: 1, Rec: 1},
			{Role: "tracker", Min: 0, Rec: 0},
			{Role: "coth", Min: 0, Rec: 0},
			{Role: "slower", Min: 2, Rec: 2},
			{Role: "debuffer", Sub: "slows", Min: 1, Rec: 1},
			{Role: "debuffer", Sub: "cripple", Min: 1, Rec: 1},
			{Role: "debuffer", Sub: "mr", Min: 1, Rec: 1},
			{Role: "debuffer", Sub: "dr", Min: 0, Rec: 0},
			{Role: "debuffer", Sub: "pr", Min: 0, Rec: 0},
			{Role: "debuffer", Sub: "fr", Min: 0, Rec: 0},
			{Role: "debuffer", Sub: "cr", Min: 0, Rec: 0},
			{Role: "damage", Min: 20, Rec: 24},
		},
	}}
}
