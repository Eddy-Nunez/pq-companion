// Package raidcomp implements the raid knowledge base and raid composition
// checker, ported from eqmon:
//
//   - raids/base.yaml     -> the role taxonomy (seeded into user.db, seed.go)
//   - lua/comp-check.lua  -> the MIN/REC composition check (check.go)
//   - ingress/RaidData.cs -> the encounter editor data model (store.go)
//
// Class codes and per-role class lists (best-to-worst preference, damage
// excepted) mirror eqmon so the seed taxonomy can be transcribed 1:1. The
// taxonomy itself is user-editable once seeded — this file only defines the
// class-code vocabulary everything else is validated against.
package raidcomp

import (
	"sort"
	"strings"
	"unicode"
)

// ClassCode is the short taxonomy code used across the raid knowledge base
// and the composition checker.
type ClassCode string

const (
	CodeBard         ClassCode = "brd"
	CodeBeastlord    ClassCode = "bst"
	CodeCleric       ClassCode = "clr"
	CodeDruid        ClassCode = "dru"
	CodeEnchanter    ClassCode = "enc"
	CodeMagician     ClassCode = "mag"
	CodeMonk         ClassCode = "mnk"
	CodeNecromancer  ClassCode = "nec"
	CodePaladin      ClassCode = "pal"
	CodeRanger       ClassCode = "rng"
	CodeRogue        ClassCode = "rog"
	CodeShadowKnight ClassCode = "sk"
	CodeShaman       ClassCode = "shm"
	CodeWarrior      ClassCode = "war"
	CodeWizard       ClassCode = "wiz"
)

// ClassNames maps each taxonomy code to its canonical EQ class name, used for
// display and for resolving free-text class input ("Shadow Knight").
var ClassNames = map[ClassCode]string{
	CodeBard:         "Bard",
	CodeBeastlord:    "Beastlord",
	CodeCleric:       "Cleric",
	CodeDruid:        "Druid",
	CodeEnchanter:    "Enchanter",
	CodeMagician:     "Magician",
	CodeMonk:         "Monk",
	CodeNecromancer:  "Necromancer",
	CodePaladin:      "Paladin",
	CodeRanger:       "Ranger",
	CodeRogue:        "Rogue",
	CodeShadowKnight: "Shadow Knight",
	CodeShaman:       "Shaman",
	CodeWarrior:      "Warrior",
	CodeWizard:       "Wizard",
}

// zealClassIDs maps Zeal's 1-indexed class id (1=Warrior .. 15=Beastlord,
// same 0-indexed EQ class ordering PQ Companion uses elsewhere) to the
// taxonomy code.
var zealClassIDs = map[int]ClassCode{
	1:  CodeWarrior,
	2:  CodeCleric,
	3:  CodePaladin,
	4:  CodeRanger,
	5:  CodeShadowKnight,
	6:  CodeDruid,
	7:  CodeMonk,
	8:  CodeBard,
	9:  CodeRogue,
	10: CodeShaman,
	11: CodeNecromancer,
	12: CodeWizard,
	13: CodeMagician,
	14: CodeEnchanter,
	15: CodeBeastlord,
}

// CodeForZealID resolves a Zeal pipe class id (1..15) to a taxonomy code.
// Returns ok=false for unknown/zero ids.
func CodeForZealID(id int) (ClassCode, bool) {
	c, ok := zealClassIDs[id]
	return c, ok
}

// CodeForInput resolves free-text class input to a taxonomy code: a bare code
// ("sk", "CLR") or a class name ("Shadow Knight", "shadowknight",
// "SHADOW KNIGHT") both work. Same normalization eqmon's class_code applies
// (lowercase, letters only) plus the raw-code fast path.
func CodeForInput(s string) (ClassCode, bool) {
	c := ClassCode(strings.ToLower(strings.TrimSpace(s)))
	if _, ok := ClassNames[c]; ok {
		return c, true
	}
	key := foldClass(s)
	for code, name := range ClassNames {
		if foldClass(name) == key {
			return code, true
		}
	}
	return "", false
}

// foldClass normalizes a class name the way eqmon does: lowercase, keep only
// letters, so "Shadow Knight" == "shadowknight" == "SHADOW KNIGHT".
func foldClass(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ClassSet is an ordered list of class codes. Order is preference
// (best-to-worst) everywhere EXCEPT damage, which is intentionally not
// preference-ordered (mirrors raids/base.yaml).
type ClassSet []ClassCode

// Role is one editable taxonomy row: a flat role (Sub == "") or one sub-type
// of a grouped role (e.g. tank.defensive). Label is the display name; the
// role/sub ids stay stable. Position orders the leaves (checker + editor).
type Role struct {
	Role     string   `json:"role"`
	Sub      string   `json:"sub_role,omitempty"`
	Label    string   `json:"label"`
	Classes  ClassSet `json:"classes"`
	Position int      `json:"position"`
}

// RoleLeaf is one assessable/editable comp row derived from Role rows. The
// checker and the editor both enumerate the taxonomy as RoleLeaf rows.
type RoleLeaf struct {
	Role     string
	Sub      string // "" for flat roles
	Label    string
	Classes  ClassSet
	Position int
}

// Path returns the dotted leaf key: "tank.defensive" or "rgc".
func (l RoleLeaf) Path() string {
	if l.Sub == "" {
		return l.Role
	}
	return l.Role + "." + l.Sub
}

// LeavesFromRoles flattens Role rows into RoleLeaf rows ordered by Position
// (stable for equal positions). The checker and comp validation use this.
func LeavesFromRoles(roles []Role) []RoleLeaf {
	leaves := make([]RoleLeaf, 0, len(roles))
	for _, r := range roles {
		leaves = append(leaves, RoleLeaf{
			Role: r.Role, Sub: r.Sub, Label: r.Label, Classes: r.Classes, Position: r.Position,
		})
	}
	sort.SliceStable(leaves, func(i, j int) bool { return leaves[i].Position < leaves[j].Position })
	return leaves
}
