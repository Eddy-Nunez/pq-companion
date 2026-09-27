// aa-descriptions reads an EverQuest/TAKP eqstr_en.txt string table and emits
// a JSON map of altadv_vars.eqmacid → description for every AA whose name is
// present in the strings file.
//
// The Quarm SQLite dump (quarm.db) carries no AA description text — that data
// lives client-side in eqstr_en.txt (TAKP) / dbstr_us.txt (retail). Each AA
// name in the strings file is followed on the next line by its description,
// regardless of which numeric range the IDs land in. We match by exact name.
//
// Run once after pulling a new strings file from a TAKP client. The output
// JSON is checked into the repo and embedded into the Go binary at build
// time — see internal/db/aa_descriptions.go.
//
// Usage:
//
//	go run ./cmd/aa-descriptions \
//	    -strings /path/to/eqstr_en.txt \
//	    -db backend/data/quarm.db \
//	    -out backend/internal/db/aa_descriptions.json
package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type stringsEntry struct {
	id   int
	text string
}

func main() {
	stringsPath := flag.String("strings", "", "path to eqstr_en.txt")
	dbPath := flag.String("db", "backend/data/quarm.db", "path to quarm.db")
	outPath := flag.String("out", "backend/internal/db/aa_descriptions.json", "output JSON path")
	flag.Parse()

	if *stringsPath == "" {
		log.Fatal("-strings is required")
	}

	entries, err := readStrings(*stringsPath)
	if err != nil {
		log.Fatalf("read strings: %v", err)
	}
	log.Printf("loaded %d strings", len(entries))

	aas, err := readAAs(*dbPath)
	if err != nil {
		log.Fatalf("read aas: %v", err)
	}
	log.Printf("loaded %d AA rows from altadv_vars", len(aas))

	// Index strings by exact text. An AA name can collide with unrelated
	// system strings, so we record every match and pick the best candidate
	// per name — the one whose immediate next entry looks like a real
	// description (longest, with sentence-ish content).
	byText := make(map[string][]int, len(entries))
	byNormText := make(map[string][]int, len(entries))
	for i, e := range entries {
		byText[e.text] = append(byText[e.text], i)
		byNormText[normalizeName(e.text)] = append(byNormText[normalizeName(e.text)], i)
	}

	// stringIDByID overrides the matched description for AAs whose entry in
	// eqstr_en.txt is misaligned (the line after the name is a different
	// AA's name or description) but whose real text is still findable
	// in-file. Detected by inspection: a block of six AA name/description
	// pairs (Advanced Innate Strength/Stamina/Agility/Dexterity/
	// Intelligence/Wisdom) sits interleaved with two unrelated AAs — Planar
	// Power (eqmacid 142) and Planar Durability (eqmacid 143) — whose own
	// name/description pairs are *also* present but out of their natural
	// order. The net effect: Advanced Innate Strength's real description
	// sits 4 lines after its name (Planar Power's full pair is spliced in
	// between), and Planar Durability's real description sits 11 lines
	// after its own name — immediately after Advanced Innate Wisdom's name,
	// which makes it look at a glance like Wisdom's description instead.
	// (This caused a real bug: without help, automatic matching attributes
	// Planar Durability's real text to Advanced Innate Wisdom instead, since
	// "Advanced Innate Wisdom" appears exactly once in the file and its
	// literal next line is Planar Durability's description, not its own.
	// Advanced Innate Wisdom's own real text is a stray duplicate sitting
	// elsewhere, right after Innate Enlightenment's description.)
	stringIDByID := map[int]int{
		129: 5550, // Advanced Innate Strength
		134: 5563, // Advanced Innate Wisdom
		143: 5560, // Planar Durability
	}
	stringByID := make(map[int]string, len(entries))
	for _, e := range entries {
		stringByID[e.id] = e.text
	}

	// textOverrideByEQMacID hard-codes the description for AAs whose correct
	// text does not exist anywhere in the strings file (so stringIDByID has
	// no valid target to point at). Detected by inspection: "Ingenuity"
	// (eqmacid 210, Warrior/Monk/Rogue crit-on-proc AA) has only one "next
	// line" candidate in eqstr_en.txt — an unrelated "Horse Models" client
	// toggle message that happens to sit right after an unrelated UI
	// filter-category string that is coincidentally also the literal text
	// "Ingenuity" (an item-search filter label, not the AA name entry). That
	// false match passed looksLikeDescription and got picked up as the AA's
	// description (reported by a user; the real description isn't present
	// in this strings file at all — cross-verified against pqdi.cc, Project
	// Quarm Database Interface — so it's hard-coded here instead of pointed
	// at a string id).
	//
	// Note on pqdi.cc as a cross-check: it is NOT infallible. While
	// investigating this Ingenuity report, pqdi.cc's own listing for
	// "Planar Durability" showed the same wrong Strength-boost text this
	// generator was mistakenly producing for eqmacid 134 (Advanced Innate
	// Wisdom) — pqdi.cc appears to source from a similarly-misaligned
	// strings dump and inherited a related mistake. Advanced Innate Wisdom
	// and Planar Durability are two genuinely distinct AAs (confirmed via
	// Allakhazam: https://everquest.allakhazam.com/wiki/EQ:Planar_Durability
	// vs https://everquest.allakhazam.com/wiki/EQ:Natural_Durability, and
	// via altadv_vars itself, which has six related-but-distinct rows —
	// eqmacid 1 Innate Strength, 6 Innate Wisdom, 28 Natural Durability,
	// 129 Advanced Innate Strength, 134 Advanced Innate Wisdom, 143 Planar
	// Durability). Advanced Innate Wisdom needs no override at all — see
	// stringIDByID's comment; Planar Durability's real text is recovered via
	// stringIDByID (143 → 5560) instead of a hard-coded override, since it
	// does exist in-file once you know where to look.
	textOverrideByEQMacID := map[int]string{
		210: "Years of experimentation have lead to the discovery of how to gain additional performance (in the form of critical spell hits) from weapons and other items. You may train in this ability at or after levels 61, 63, and 65.",
	}

	descriptions := make(map[int]string, len(aas))
	missing := make([]string, 0)
	ambiguous := make([]string, 0)
	seen := make(map[string]bool)

	for _, aa := range aas {
		if seen[aa.name] {
			// Duplicate names in altadv_vars (legacy + current eqmacid) all
			// share the same description; reuse what we already resolved.
			if d, ok := descriptions[lookupByName(aa.name, aas, descriptions)]; ok {
				descriptions[aa.eqmacid] = d
			}
			continue
		}
		seen[aa.name] = true

		// Hard-coded text override first — these are AAs whose real
		// description isn't recoverable from the strings file at all.
		if t, ok := textOverrideByEQMacID[aa.eqmacid]; ok {
			descriptions[aa.eqmacid] = t
			continue
		}

		// Hard override next — these are known misalignments in the
		// strings file's data layout where the real text IS in the file,
		// just at a non-adjacent id.
		if sid, ok := stringIDByID[aa.eqmacid]; ok {
			if t, ok := stringByID[sid]; ok {
				descriptions[aa.eqmacid] = t
				continue
			}
		}

		idxs := byText[aa.name]
		desc, candidates := pickDescription(idxs, entries)
		// Fall back to a normalized lookup for names that differ only by
		// punctuation/whitespace/extra qualifiers — e.g. DB has "Ayonaes
		// Tutelage" but strings file has "Ayonae's Tutelage", "Lifeburn" vs
		// "Life Burn", "Fletching Mastery" vs "Fletching/Bowyer Mastery".
		if desc == "" {
			desc, candidates = pickDescription(byNormText[normalizeName(aa.name)], entries)
		}
		if desc == "" {
			missing = append(missing, aa.name)
			continue
		}
		// More than one line in the strings file reads as a plausible
		// description for this name — pickDescription silently took the
		// longest, but an unrelated collision (like Ingenuity's item-filter
		// label vs. its AA name) can win that tiebreak. Flag it for manual
		// review rather than trusting it silently, same as a missing match.
		if candidates > 1 {
			ambiguous = append(ambiguous, aa.name)
		}
		descriptions[aa.eqmacid] = desc
	}

	// Backfill duplicate-name AAs whose first encounter happened after the
	// canonical entry was recorded (loop order).
	for _, aa := range aas {
		if _, ok := descriptions[aa.eqmacid]; ok {
			continue
		}
		if d, ok := descriptions[lookupByName(aa.name, aas, descriptions)]; ok {
			descriptions[aa.eqmacid] = d
		}
	}

	// Cross-check: flag any description text shared by two or more
	// *differently-named* AAs. Legitimate sharing only happens between
	// legacy/current eqmacid rows of the SAME name (handled above via
	// `seen`/lookupByName and excluded here); any other collision means one
	// of the two AAs picked up text that actually belongs to the other —
	// exactly the bug that hit "Planar Durability" (which silently matched
	// "Advanced Innate Strength"'s real description). This check requires
	// the strings file to actually contain each AA's own text somewhere, so
	// it can't catch a case like Ingenuity, where the wrong match doesn't
	// collide with any other real AA's text.
	//
	// knownSharedDescriptions allowlists pairs already verified to be a
	// genuine data duplication (both AAs are adjacent to their own name in
	// the strings file — not a misalignment) rather than a match bug, so
	// they don't get re-flagged and re-investigated on every run:
	//   - "Fury of Magic" / "Fury of Magic Mastery": confirmed via
	//     Allakhazam that live EQ reuses the exact same flavor text for
	//     both — Mastery is later ranks of the same ability line, not a
	//     separately-worded AA.
	//   - "Bestial Frenzy" / "Harmonious Attack": both sit correctly
	//     adjacent to their own name in eqstr_en.txt (no shuffle), each
	//     with byte-identical text — the TAKP client genuinely used one
	//     generic "chance of double attack" sentence for both AAs at this
	//     (frozen, PoP-era) client version. Live retail rewords each
	//     differently in later expansions, but that's not what Quarm ships.
	knownSharedDescriptions := map[string]bool{
		"This ability further increases your chance to score a critical hit with your direct damage spells.":                                    true,
		"This ability grants you a chance of performing a double attack in any given combat round.  You may train in this ability once each level after reaching level 61.": true,
	}
	namesByDesc := make(map[string]map[string]bool, len(descriptions))
	for _, aa := range aas {
		d, ok := descriptions[aa.eqmacid]
		if !ok {
			continue
		}
		if namesByDesc[d] == nil {
			namesByDesc[d] = make(map[string]bool)
		}
		namesByDesc[d][aa.name] = true
	}
	sharedDesc := make(map[string][]string)
	for d, names := range namesByDesc {
		if len(names) < 2 || knownSharedDescriptions[d] {
			continue
		}
		list := make([]string, 0, len(names))
		for n := range names {
			list = append(list, n)
		}
		sort.Strings(list)
		sharedDesc[d] = list
	}

	if err := writeJSON(*outPath, descriptions); err != nil {
		log.Fatalf("write json: %v", err)
	}

	log.Printf("wrote %d descriptions to %s", len(descriptions), *outPath)
	if len(missing) > 0 {
		sort.Strings(missing)
		log.Printf("no description match for %d AA name(s):", len(missing))
		for _, n := range missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", n)
		}
	}
	if len(ambiguous) > 0 {
		sort.Strings(ambiguous)
		log.Printf("ambiguous match (multiple candidate lines in the strings file) for %d AA name(s) — verify these by hand:", len(ambiguous))
		for _, n := range ambiguous {
			fmt.Fprintf(os.Stderr, "  - %s\n", n)
		}
	}
	if len(sharedDesc) > 0 {
		descs := make([]string, 0, len(sharedDesc))
		for d := range sharedDesc {
			descs = append(descs, d)
		}
		sort.Strings(descs)
		log.Printf("%d description(s) are shared by differently-named AAs — one of each group almost certainly has the wrong text:", len(sharedDesc))
		for _, d := range descs {
			fmt.Fprintf(os.Stderr, "  - %s\n    %q\n", strings.Join(sharedDesc[d], " / "), d)
		}
	}
}

func readStrings(path string) ([]stringsEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make([]stringsEntry, 0, 8192)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		// First two lines are header (`EQST0002`) + count (`0 5808`).
		if lineNum <= 2 {
			continue
		}
		// Each entry is `<id> <text>`. The text can contain spaces; we only
		// split on the first space.
		sp := strings.IndexByte(line, ' ')
		if sp < 0 {
			continue
		}
		id, err := strconv.Atoi(line[:sp])
		if err != nil {
			continue
		}
		out = append(out, stringsEntry{id: id, text: line[sp+1:]})
	}
	return out, sc.Err()
}

type aaRow struct {
	skillID int
	eqmacid int
	name    string
}

func readAAs(path string) ([]aaRow, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Match the filter ListAvailableAAs uses minus the class bitmask, so we
	// cover every AA that any class could see. Order by eqmacid for a
	// deterministic JSON output.
	rows, err := conn.Query(`
		SELECT skill_id, eqmacid, name
		FROM altadv_vars
		WHERE name != 'NOT USED'
		  AND cost > 0
		  AND eqmacid > 0
		  AND class_type != 0
		ORDER BY eqmacid
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []aaRow
	for rows.Next() {
		var r aaRow
		if err := rows.Scan(&r.skillID, &r.eqmacid, &r.name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// pickDescription chooses the best description from candidate name-line
// indexes. The next line after the name is the description; we prefer the
// longest one because non-AA collisions tend to be short tooltips while AA
// description text is paragraph-length. The second return value is how many
// candidates passed looksLikeDescription — more than one means the pick was
// a tiebreak, not a unique match, so the caller should flag it for review.
func pickDescription(idxs []int, entries []stringsEntry) (string, int) {
	best := ""
	candidates := 0
	for _, i := range idxs {
		if i+1 >= len(entries) {
			continue
		}
		next := entries[i+1].text
		// Reject the obvious case where the "next" line is itself another
		// AA name (one or two short words, no sentence punctuation).
		if !looksLikeDescription(next) {
			continue
		}
		candidates++
		if len(next) > len(best) {
			best = next
		}
	}
	return best, candidates
}

// normalizeName lower-cases and strips apostrophes, slashes, and any
// "/secondary" qualifier so naming variants between altadv_vars and
// eqstr_en.txt collapse to the same key.
func normalizeName(s string) string {
	s = strings.ToLower(s)
	// Drop alternate names after a slash: "Fletching/Bowyer Mastery" → "Fletching Mastery".
	if i := strings.IndexByte(s, '/'); i >= 0 {
		// Replace `/<word>` with empty. The chunk runs to the next space.
		end := strings.IndexByte(s[i:], ' ')
		if end < 0 {
			s = s[:i]
		} else {
			s = s[:i] + s[i+end:]
		}
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\'', '`', '’', ' ', '\t', '-':
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func looksLikeDescription(s string) bool {
	if len(s) < 20 {
		return false
	}
	// Real descriptions are sentences — they contain at least one period or
	// have multiple words. A short proper-noun line like "Dire Charm" fails
	// both checks.
	return strings.ContainsAny(s, ".!?") || strings.Count(s, " ") >= 4
}

// lookupByName returns the eqmacid of an already-resolved AA matching the
// given name, or 0 if none yet recorded.
func lookupByName(name string, aas []aaRow, descs map[int]string) int {
	for _, aa := range aas {
		if aa.name == name {
			if _, ok := descs[aa.eqmacid]; ok {
				return aa.eqmacid
			}
		}
	}
	return 0
}

func writeJSON(path string, descs map[int]string) error {
	// Stringify keys so the JSON is portable and diff-friendly. Sort by
	// numeric eqmacid for stable output across runs.
	keys := make([]int, 0, len(descs))
	for k := range descs {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	out := make(map[string]string, len(descs))
	for _, k := range keys {
		out[strconv.Itoa(k)] = descs[k]
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
