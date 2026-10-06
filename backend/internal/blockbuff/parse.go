package blockbuff

import (
	"regexp"
	"strconv"
)

// The reply lines come from zone/gm_commands/blockbuff.cpp in EQMacEmu
// (PR #479). <cond> is the optional " while <Name> (<id>) is on you" suffix.
const condRe = `(?: while (.+?) \((\d+)\) is on you)?`

var (
	reAdded   = regexp.MustCompile(`^Blocking (.+?) \((\d+)\) from other players` + condRe + `\.$`)
	reRemoved = regexp.MustCompile(`^No longer blocking (.+?) \((\d+)\)` + condRe + `\.$`)
	reNotNow  = regexp.MustCompile(`^You are not blocking (.+?) \((\d+)\)` + condRe + `\.$`)
	reNoneAll = regexp.MustCompile(`^You are not blocking any buffs\.$`)
	// A bare "#blockbuff" prints one "<Name> (<id>)[<cond>]" line per block —
	// no prefix and no period — so a row is only trusted after the caller
	// checks the names against the spell table (see NameCheck).
	reListRow = regexp.MustCompile(`^(.+?) \((\d+)\)` + condRe + `$`)
)

// Kind classifies a parsed log line.
type Kind int

const (
	KindNone    Kind = iota
	KindAdded        // #blockbuff / #blockbuffif confirmed
	KindRemoved      // #allowbuff confirmed
	KindAbsent       // "You are not blocking X": nothing to remove
	KindEmpty        // "You are not blocking any buffs."
	KindListRow      // one row of a #blockbuff listing
)

// Line is the result of parsing one log line.
type Line struct {
	Kind      Kind
	Entry     Entry
	Name      string // spell name as printed by the server
	IfName    string // condition spell name, "" when unconditional
	hasIfName bool
}

// NameCheck returns the spell name for an id, or "" when unknown. It lets the
// parser reject look-alike lines ("Foo (12)") that aren't about a real spell.
type NameCheck func(spellID int) string

func entryFrom(m []string) (Line, bool) {
	id, err := strconv.Atoi(m[2])
	if err != nil {
		return Line{}, false
	}
	l := Line{Name: m[1], Entry: Entry{SpellID: id}}
	if len(m) > 3 && m[3] != "" {
		ifID, err := strconv.Atoi(m[4])
		if err != nil {
			return Line{}, false
		}
		l.Entry.IfSpellID, l.IfName, l.hasIfName = ifID, m[3], true
	}
	return l, true
}

// valid reports whether the printed names agree with the spell table.
func (l Line) valid(names NameCheck) bool {
	if names == nil {
		return true
	}
	if names(l.Entry.SpellID) != l.Name {
		return false
	}
	return !l.hasIfName || names(l.Entry.IfSpellID) == l.IfName
}

// Parse classifies msg (the log text after the timestamp). Reply lines are
// accepted as-is; bare list rows only when names confirm them.
func Parse(msg string, names NameCheck) Line {
	if reNoneAll.MatchString(msg) {
		return Line{Kind: KindEmpty}
	}
	for _, p := range []struct {
		re   *regexp.Regexp
		kind Kind
	}{
		{reAdded, KindAdded}, {reRemoved, KindRemoved}, {reNotNow, KindAbsent},
	} {
		if m := p.re.FindStringSubmatch(msg); m != nil {
			if l, ok := entryFrom(m); ok {
				l.Kind = p.kind
				return l
			}
		}
	}
	if m := reListRow.FindStringSubmatch(msg); m != nil {
		if l, ok := entryFrom(m); ok && names != nil && l.valid(names) {
			l.Kind = KindListRow
			return l
		}
	}
	return Line{}
}
