package popflag

import "testing"

// This file is a full-coverage sweep of every distinct line popflags.cpp can
// print (as of the 2026-09-28 re-check against the live source, commit
// cec9e73a — see the header comment on popflags_cmd.go), transcribed by hand
// from the actual c->Message(...) call sites rather than sampled. Its only
// job is to catch a MatchPopFlagsLine recognition gap: a line the server can
// really print that the live Consumer's buffering gate doesn't recognize,
// which would flush mid-block and split one '#popflags' reading into two
// stored snapshots instead of one. It is deliberately NOT about whether
// ParsePopFlagsReport extracts the "right" state from each line — the other
// tests in this package already cover that — a notice/lore/zone-header line
// is expected to contribute nothing and still must be recognized.
//
// This exact sweep (run against the pre-2026-09-28 popflags_cmd.go) is what
// found the two real gaps fixed in that commit: the four "Tier N: In
// progress" overview lines, and PopFlagsPrintTime's "Complete the elemental
// progression..." hint (printed whenever Time access is Locked — i.e. for
// nearly every player who runs the command before finishing Tier 4).

func TestMatchPopFlagsLineFullCoverage(t *testing.T) {
	var lines []string
	add := func(ls ...string) { lines = append(lines, ls...) }

	// Section headers (popFlagsHeaders, popflags.cpp lines 312/383/491/607/799/848).
	add(
		"=== Planes of Power Progression ===",
		"=== Tier 1 Progression ===",
		"=== Tier 2 Progression ===",
		"=== Tier 3 Progression ===",
		"=== Tier 4 Progression ===",
		"=== Plane of Time ===",
	)

	// Zone sub-headers ("--- X ---", one per PopFlagsPrintTierN zone section).
	add(
		"--- Plane of Justice ---",
		"--- Plane of Disease ---",
		"--- Plane of Nightmare ---",
		"--- Plane of Innovation ---",
		"--- Plane of Valor ---",
		"--- Plane of Storms ---",
		"--- Crypt of Decay ---",
		"--- Plane of Torment ---",
		"--- Halls of Honor ---",
		"--- Bastion of Thunder ---",
		"--- Plane of Tactics ---",
		"--- Grand Librarian Maelin ---",
		"--- Tower of Solusek Ro ---",
		"--- Elemental Planes ---",
	)

	// Overview per-tier status (PopFlagsProgressStatus: Complete / In progress
	// / Not started — Tier 5 excluded from "In progress" since its two
	// PopFlagsProgressStatus args are always identical, so that state is
	// unreachable for it).
	for _, tier := range []string{"Tier 1", "Tier 2", "Tier 3", "Tier 4"} {
		add(tier+": Complete", tier+": In progress", tier+": Not started")
	}
	add("Tier 5 - Plane of Time: Complete", "Tier 5 - Plane of Time: Not started")

	// PopFlagsPrintStage lines: every stage string plus "Not started" for each
	// of the six generic stage arrays.
	stageLabels := []struct {
		label  string
		stages []string
	}{
		{"Mavuin's case", popFlagsMavuinStages},
		{"Fuirstel progression", popFlagsFuirstelStages},
		{"Thelin progression", popFlagsThelinStages},
		{"Aerin`Dar progression", popFlagsAerindarStages},
		{"Tylis progression", popFlagsTylisStages},
		{"Giwin and Zek progression", popFlagsZekStages},
	}
	for _, sl := range stageLabels {
		add(sl.label + ": Not started")
		for _, s := range sl.stages {
			add(sl.label + ": " + s)
		}
	}

	// The two Karana print sites: their own stage arrays, "Not started", and
	// the "combined into Zebuxoruk lore" short-circuit variant.
	karanaLabels := []struct {
		label  string
		stages []string
	}{
		{"Askr and Karana progression", popFlagsKaranaTier2Stages},
		{"Agnarr and Karana progression", popFlagsKaranaTier3Stages},
	}
	for _, kl := range karanaLabels {
		add(kl.label + ": Not started")
		for _, s := range kl.stages {
			add(kl.label + ": " + s)
		}
		add(kl.label + ": Complete; combined into Zebuxoruk lore")
	}

	// popFlagsBitLines: Complete/Incomplete for every hohtrials/sol_room bit,
	// plus the "None completed" line each section prints when the qglobal is
	// entirely empty.
	add("Halls of Honor trials: None completed", "Tower wing flags: None completed")
	for _, b := range popFlagsBitLines {
		add(b.label+": Complete", b.label+": Incomplete")
	}

	// Every remaining literal "Label: Value" line — transcribed from each
	// call site in popflags.cpp, not just popFlagsLiteralLines' keys (so a
	// state this test doesn't know is missing from that map still gets
	// exercised against MatchPopFlagsLine here).
	add(
		"Seventh Hammer access: Unlocked", "Seventh Hammer access: Locked",
		"Crypt of Decay access: Unlocked", "Crypt of Decay access: Locked",
		"Giwin and Manaetic Behemoth progression: Progress recorded",
		"Giwin and Manaetic Behemoth progression: Not started",
		"Factory door access: Unlocked", "Factory door access: Locked",
		"Lower Crypt access: Unlocked", "Lower Crypt access: Locked",
		"Saryrn cipher half: Combined into Cipher",
		"Saryrn cipher half: Complete",
		"Saryrn cipher half: Incomplete",
		"Mithaniel Marr cipher half: Combined into Cipher",
		"Mithaniel Marr cipher half: Complete",
		"Mithaniel Marr cipher half: Incomplete",
		"Cipher information: Received", "Cipher information: Missing",
		"Zebuxoruk lore: Received", "Zebuxoruk lore: Missing",
		"Combined Zek information: Received", "Combined Zek information: Missing",
		"Final elemental information: Received", "Final elemental information: Missing",
		"Plane of Fire progression: Unlocked",
		"Plane of Fire progression: In progress",
		"Plane of Fire progression: Not started",
		"Plane of Earth B access: Unlocked", "Plane of Earth B access: Locked",
		"Plane of Time access: Unlocked", "Plane of Time access: Locked",
		"Air, Earth, and Water access: Unlocked", "Air, Earth, and Water access: Locked",
		"Plane of Fire access: Unlocked", "Plane of Fire access: Locked",
	)

	// Pending memory lines — one per PopFlagsPrintPending call site (13 total).
	for desc := range popFlagsPendingDesc {
		add("Pending memory: " + desc)
	}

	// PopFlagsPrintSeerNotice's two message pairs.
	add(
		"A checklist memory is ready to be unlocked.",
		"Sit near Seer Mal Nae`Shi and say 'unlock memories', then check #popflags again.",
		"Pending checklist memories exist, but their prerequisite steps are incomplete.",
		"Complete the unfinished progression shown above, then return to Seer Mal Nae`Shi.",
	)

	// Maelin section hint (Tier 3 only) and Time section hint (printed
	// whenever Time access is Locked).
	add(
		"If one of these is missing, hail Maelin and ask about new lore and new information.",
		"Complete the elemental progression, combine the four elemental essences, and return to Grand Librarian Maelin.",
	)

	// PopFlagsPrintTierCompletion — one arbitrary "Tier complete: <lore>" per
	// tier (the prefix match must accept any lore text, so these five distinct
	// lore strings all matter, not just one representative sample).
	add(
		"Tier complete: Justice has been served, Disease's grip weakened, Nightmare ended, and Innovation's hidden plot uncovered.",
		"Tier complete: Valor has been proven, Storms weathered, the Crypt of Decay purged, and Torment overcome.",
		"Tier complete: Honor has been earned, the Bastion of Thunder conquered, the Warlord defeated, and the Burning Prince's plot exposed. The Elemental Planes now stand open.",
		"Tier complete: The powers of Air, Earth, Fire, and Water have been joined. The way into the Plane of Time now stands open.",
		"Tier complete: The Plane of Time recognizes your soul. Beyond its shifting portals, the gods await.",
	)

	// Overview's trailer.
	add("Details: #popflags 1, 2, 3, 4, or 5 (tier1-tier5 also work).")

	seen := map[string]bool{}
	for _, line := range lines {
		if seen[line] {
			continue // some lines are shared across print sites; only check once
		}
		seen[line] = true
		if !MatchPopFlagsLine(line) {
			t.Errorf("MatchPopFlagsLine(%q) = false, want true (line popflags.cpp can really print)", line)
		}
	}
	if len(seen) < 100 {
		t.Fatalf("only %d distinct lines exercised — this sweep should cover well over 100; a helper likely returned fewer entries than expected", len(seen))
	}
}

// TestMatchPopFlagsLineRejectsUnknownSection guards the two error-path lines
// printed by an invalid '#popflags <arg>' (e.g. a typo) — these are NOT part
// of any real progression block (no header precedes them) and are correctly
// left unrecognized so the consumer doesn't buffer them as if they were.
func TestMatchPopFlagsLineRejectsUnknownSection(t *testing.T) {
	for _, line := range []string{
		"Unknown #popflags section: asdf",
		"Valid sections: overview, 1-5, tier1-tier5, and time.",
		"The all option is restricted to server staff.",
	} {
		if MatchPopFlagsLine(line) {
			t.Errorf("MatchPopFlagsLine(%q) = true, want false (error-path line, not a real report line)", line)
		}
	}
}
