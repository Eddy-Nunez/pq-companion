package killtracker

import (
	"fmt"
	"regexp"
)

// Preset is a built-in, ready-made set of counters for a known event.
type Preset struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Counters    []CounterDef `json:"counters"`
}

// Presets returns every built-in preset.
func Presets() []Preset {
	return []Preset{bertoxPreset()}
}

func presetByKey(key string) (Preset, bool) {
	for _, p := range Presets() {
		if p.Key == key {
			return p, true
		}
	}
	return Preset{}, false
}

// bertoxPreset models the Bertoxxulous event in the Crypt of Decay, taken
// from EQMacEmu/quests codecay/encounters/Bertox.lua:
//
//   - Killing #Spectre_of_Corruption starts the event: the controller's
//     "start" timer fires 350s later (trash spawns enable), the expiry warning
//     at 7080s and the event fails at 7380s.
//   - Each Knight/Priest/Necromancer of Decay death (and their Elite forms)
//     bumps trashKills; BOSS_TABLE spawns an Adan king at 44/48/52/56 and
//     84/88/92/96 (nothing between 57 and 83), each announced by a
//     zone_emote naming the boss — "We call to you corrupted King of
//     Lxanvom, <Name>, ..." — which the trash counter resyncs from.
//   - The 8th Adan death (any of them) spawns the last four kings, the 12th
//     spawns Bertoxxulous. Both are announced by their own emotes.
func bertoxPreset() Preset {
	trashBosses := []struct {
		at   int
		name string
	}{
		{44, "Darwol Adan"}, {48, "Feig Adan"}, {52, "Xhut Adan"}, {56, "Kavilis Adan"},
		{84, "Raddi Adan"}, {88, "Wavadozzik Adan"}, {92, "Zandal Adan"}, {96, "Akkapan Adan"},
	}
	var milestones []Milestone
	var syncs []SyncRule
	for _, b := range trashBosses {
		milestones = append(milestones, Milestone{At: b.at, Label: b.name + " spawns"})
		syncs = append(syncs, SyncRule{
			Pattern: regexp.QuoteMeta(fmt.Sprintf("King of Lxanvom, %s,", b.name)),
			SetTo:   b.at,
		})
	}

	const (
		group = "Bertoxxulous"
		zone  = "The Crypt of Decay"
		// Spectre of Corruption's death emote — the event's real start.
		spectreDeath = `Crazed laughter is heard as you notice a foul creature standing before you`
		// The controller's "start" timer, 350s after the Spectre dies.
		trashStart = `Great soldiers of decay you are summoned forth`
		// The controller's "expire_warning" timer, 7080s in.
		expireWarning = `Your time is close in coming to an end`
		expired       = `Perhaps you would care to try when you are more powerful fools`
		bertoxDead    = `Bertoxxulous has been defeated`
	)
	reset := spectreDeath + "|" + trashStart

	return Preset{
		Key:  "bertox",
		Name: "Bertoxxulous (Crypt of Decay)",
		Description: "Counts event trash toward the eight trash-spawned Adan kings, " +
			"counts Adan kills toward the final four and Bertoxxulous, and runs the " +
			"123-minute event clock. Resyncs from the boss-spawn emotes.",
		Counters: []CounterDef{
			{
				Name:    "Event trash",
				Group:   group,
				Enabled: true,
				Mobs: []string{
					"Knight of Decay", "Priest of Decay", "Necromancer of Decay",
					"Elite Knight of Decay", "Elite Priest of Decay", "Elite Necromancer of Decay",
				},
				Zone:         zone,
				Goal:         96,
				Milestones:   milestones,
				WarnBefore:   2,
				Syncs:        syncs,
				ResetPattern: reset,
				Clock: &Clock{
					Label:        "Event time left",
					DurationSecs: 7380,
					Marks: []ClockMark{
						{Pattern: spectreDeath, AtSecs: 0},
						{Pattern: trashStart, AtSecs: 350},
						{Pattern: expireWarning, AtSecs: 7080},
					},
					StopPatterns: []string{expired, bertoxDead},
				},
				AlertOverlay: true,
			},
			{
				Name:    "Adan kings",
				Group:   group,
				Enabled: true,
				Mobs: []string{
					"Darwol Adan", "Feig Adan", "Xhut Adan", "Kavilis Adan",
					"Raddi Adan", "Wavadozzik Adan", "Zandal Adan", "Akkapan Adan",
					"Meedo Adan", "Qezzin Adan", "Pzo Adan", "Bhaly Adan",
				},
				Zone: zone,
				Goal: 12,
				Milestones: []Milestone{
					{At: 8, Label: "Final four Adan kings spawn"},
					{At: 12, Label: "Bertoxxulous spawns"},
				},
				Syncs: []SyncRule{
					{Pattern: regexp.QuoteMeta("We call to you the last corrupted Kings of Lxanvom"), SetTo: 8},
					{Pattern: regexp.QuoteMeta("A sinister vision enters your mind of a faceless one"), SetTo: 12},
				},
				ResetPattern: reset,
				AlertOverlay: true,
			},
		},
	}
}
