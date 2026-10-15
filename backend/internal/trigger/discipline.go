package trigger

// disciplineReuse mirrors EQMacEmu's Client::CastDiscipline /
// DisciplineUseLevel (zone/effects.cpp) for the discipline spell IDs the
// built-in packs time. The server derives a disc's reuse from the caster's
// level, not from spells_new.recast_time (which is wrong for discs — see
// LIMITATIONS.md), so a flat CooldownSecs drifts once a character outlevels
// the disc. That became visible when PoP raised the cap to 65: Duelist at 65
// is 24:36, not 30:00.
type disciplineReuse struct {
	baseSecs int
	// useLevel is the level the class gains the disc, keyed by class index.
	// Most disc spell IDs are single-class; Resistant/Fearless are shared and
	// differ between pure melee and hybrids.
	useLevel map[int]int
}

var (
	pureMelee = []int{ClassWarrior, ClassMonk, ClassRogue}
	hybrids   = []int{ClassPaladin, ClassRanger, ClassShadowknight, ClassBard, ClassBeastlord}
)

func levels(classes []int, lvl int) map[int]int {
	m := make(map[int]int, len(classes))
	for _, c := range classes {
		m[c] = lvl
	}
	return m
}

var disciplineReuseBySpell = map[int]disciplineReuse{
	4498: {1620, map[int]int{ClassWarrior: 60}},                           // Aggressive
	4501: {1800, map[int]int{ClassWarrior: 57}},                           // Precision
	4499: {900, map[int]int{ClassWarrior: 55}},                            // Defensive
	4503: {900, map[int]int{ClassWarrior: 52}},                            // Evasive
	4508: {4320, map[int]int{ClassMonk: 60}},                              // Ashenhand
	4674: {3600, map[int]int{ClassWarrior: 56}},                           // Furious
	4509: {3600, map[int]int{ClassMonk: 53}},                              // Whirlwind
	4673: {3600, map[int]int{ClassRogue: 53}},                             // Counterattack
	4510: {720, map[int]int{ClassMonk: 51}},                               // Stonestance
	4671: {720, map[int]int{ClassBeastlord: 55}},                          // Protective Spirit
	4511: {540, map[int]int{ClassMonk: 52}},                               // Thunderkick
	4670: {3600, map[int]int{ClassWarrior: 59}},                           // Fortitude
	4502: {3600, map[int]int{ClassMonk: 54}},                              // Voiddance
	4675: {1800, map[int]int{ClassWarrior: 58}},                           // Fellstrike
	4512: {1800, map[int]int{ClassMonk: 56}},                              // Innerflame
	4676: {1800, map[int]int{ClassRogue: 59}},                             // Duelist
	4678: {1800, map[int]int{ClassBeastlord: 60}},                         // Bestial Rage
	4513: {1800, map[int]int{ClassMonk: 57}},                              // Hundred Fists
	4677: {1800, map[int]int{ClassRogue: 58}},                             // Blinding Speed
	4672: {1800, map[int]int{ClassWarrior: 53}},                           // Charge
	4505: {1800, map[int]int{ClassRogue: 54}},                             // Deadeye
	4514: {3600, map[int]int{ClassWarrior: 54}},                           // Mighty Strike
	4515: {1800, map[int]int{ClassRogue: 55}},                             // Nimble
	4507: {594, map[int]int{ClassMonk: 59}},                               // Silentfist
	4517: {1800, map[int]int{ClassRogue: 57}},                             // Kinesthetics
	4500: {4320, map[int]int{ClassPaladin: 55}},                           // Holyforge
	4518: {4320, map[int]int{ClassPaladin: 60}},                           // Sanctification
	4506: {4320, map[int]int{ClassRanger: 55}},                            // Trueshot
	4519: {4320, map[int]int{ClassRanger: 60}},                            // Weapon Shield
	4520: {4320, map[int]int{ClassShadowknight: 55}},                      // Unholy Aura
	4504: {4320, map[int]int{ClassShadowknight: 60}},                      // Leech Curse
	4516: {4320, map[int]int{ClassBard: 55}},                              // Deftdance
	4586: {4320, map[int]int{ClassBard: 60}},                              // Puretone
	4585: {3600, mergeLevels(levels(pureMelee, 30), levels(hybrids, 51))}, // Resistant
	4587: {3600, mergeLevels(levels(pureMelee, 40), levels(hybrids, 54))}, // Fearless
}

func mergeLevels(a, b map[int]int) map[int]int {
	for k, v := range b {
		a[k] = v
	}
	return a
}

// DisciplineReuseSecs returns the server's reuse time for a discipline spell
// cast by a character of the given class index and level, and ok=false when
// spellID isn't a known disc or the class can't use it.
//
// Formula (EQMacEmu CastDiscipline): base - 54s per level above the disc's
// use level, clamped to [234, 4320]. Discs with a base under 1620s stop
// scaling at level 60.
func DisciplineReuseSecs(spellID, class, level int) (int, bool) {
	d, ok := disciplineReuseBySpell[spellID]
	if !ok || level <= 0 {
		return 0, false
	}
	useLevel, ok := d.useLevel[class]
	if !ok || level < useLevel {
		return 0, false
	}
	if d.baseSecs < 1620 && level > 60 {
		level = 60
	}
	secs := d.baseSecs - (level-useLevel)*54
	if secs > 4320 {
		secs = 4320
	}
	if secs < 234 {
		secs = 234
	}
	return secs, true
}
