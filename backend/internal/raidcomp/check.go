package raidcomp

import "sort"

// RosterMember is one raid participant as seen by the checker, from either the
// live Zeal raid envelope or a manual roster override.
type RosterMember struct {
	Name  string    `json:"name"`
	Class ClassCode `json:"class"` // taxonomy code; "" when unmappable
	Level int       `json:"level,omitempty"`
	Group string    `json:"group,omitempty"`
	Rank  string    `json:"rank,omitempty"`
}

// RowReport is the per-role-leaf result of one comp assessment.
type RowReport struct {
	Path           string   `json:"path"` // "tank.defensive"
	Role           string   `json:"role"`
	Sub            string   `json:"sub_role,omitempty"`
	Need           int      `json:"need"`
	Have           int      `json:"have"`
	Candidates     []string `json:"candidates,omitempty"` // class-eligible member names (max 8)
	MoreCandidates int      `json:"more_candidates,omitempty"`
}

// Gap reports how many more class-eligible members are needed to meet Need
// (0 when Have >= Need). Mirrors EQMon's "GAP n" figure.
func (r RowReport) Gap() int {
	if r.Have >= r.Need {
		return 0
	}
	return r.Need - r.Have
}

// SummaryAnswer aggregates OK/GAP across one comp level.
type SummaryAnswer struct {
	OK       bool `json:"ok"`        // every row met (and at least one row exists)
	Rows     int  `json:"rows"`      // rows assessed
	GapRows  int  `json:"gap_rows"`  // rows below need
	GapCount int  `json:"gap_count"` // total headcount shortfall across gap rows
}

// Summary is the compact OK/GAP rollup for both levels, for a banner read.
type Summary struct {
	Min SummaryAnswer `json:"min"`
	Rec SummaryAnswer `json:"rec"`
}

// CheckReport is the full structured MIN + REC composition report. Unlike
// eqmon (which renders this as clipboard text), the GUI renders it directly.
type CheckReport struct {
	EncounterID   string         `json:"encounter_id"`
	EncounterName string         `json:"encounter_name"`
	Zone          string         `json:"zone,omitempty"`
	ZoneID        int            `json:"zone_id,omitempty"`
	RosterTotal   int            `json:"roster_total"`  // members seen (all classes)
	RosterMapped  int            `json:"roster_mapped"` // members resolved to a taxonomy code
	Members       []RosterMember `json:"members,omitempty"`
	Min           []RowReport    `json:"min"`
	Rec           []RowReport    `json:"rec"`
	Summary       Summary        `json:"summary"`
}

// Check comps a roster against one encounter's min/rec levels, using the live
// (user-editable) taxonomy leaves.
//
// Only leaves actually present in the encounter's comps AND carrying a
// non-zero need at the assessed level are included (a role the encounter
// doesn't require — e.g. "rgc: 0" — is omitted rather than printed as a
// 0/0 OK row; mirrored for REC via its own need). Members without a taxonomy
// code are excluded from counts — that also means they never appear in
// candidates. Candidates are class-eligible members, not assignments (one
// member can be eligible for several roles); the UI surfaces that caveat.
func Check(leaves []RoleLeaf, enc *Encounter, members []RosterMember) CheckReport {
	rep := CheckReport{
		EncounterID:   enc.ID,
		EncounterName: enc.Name,
		Zone:          enc.Zone,
		ZoneID:        enc.ZoneID,
		RosterTotal:   len(members),
	}
	byCode := map[ClassCode][]RosterMember{}
	for _, m := range members {
		if m.Class == "" {
			continue
		}
		rep.RosterMapped++
		byCode[m.Class] = append(byCode[m.Class], m)
	}
	if len(members) > 0 {
		rep.Members = members
	}

	rowsByKey := make(map[string]CompRow, len(enc.Comps))
	for _, c := range enc.Comps {
		rowsByKey[leafKey(c.Role, c.Sub)] = c
	}

	assess := func(rec bool) []RowReport {
		var out []RowReport
		for _, leaf := range leaves {
			row, ok := rowsByKey[leafKey(leaf.Role, leaf.Sub)]
			if !ok {
				continue
			}
			need := row.Min
			if rec {
				need = row.Rec
			}
			if need == 0 {
				// Not required at this level — keep the report to roles with
				// an actual staffing need (min_count 0 => absent).
				continue
			}
			var have int
			var cands []RosterMember
			for _, code := range leaf.Classes {
				for _, m := range byCode[code] {
					have++
					cands = append(cands, m)
				}
			}
			rr := RowReport{
				Path: leaf.Path(),
				Role: leaf.Role,
				Sub:  leaf.Sub,
				Need: need,
				Have: have,
			}
			if have < need && len(cands) > 0 {
				sort.Slice(cands, func(i, j int) bool { return cands[i].Name < cands[j].Name })
				cut := 8
				if len(cands) < cut {
					cut = len(cands)
				}
				for _, c := range cands[:cut] {
					rr.Candidates = append(rr.Candidates, c.Name)
				}
				if len(cands) > cut {
					rr.MoreCandidates = len(cands) - cut
				}
			}
			out = append(out, rr)
		}
		return out
	}

	rep.Min = assess(false)
	rep.Rec = assess(true)
	rep.Summary.Min = summarize(rep.Min)
	rep.Summary.Rec = summarize(rep.Rec)
	return rep
}

func summarize(rows []RowReport) SummaryAnswer {
	var s SummaryAnswer
	s.Rows = len(rows)
	if s.Rows == 0 {
		return s // not OK: nothing assessed yet
	}
	s.OK = true
	for _, r := range rows {
		if g := r.Gap(); g > 0 {
			s.OK = false
			s.GapRows++
			s.GapCount += g
		}
	}
	return s
}

func leafKey(role, sub string) string {
	if sub == "" {
		return role
	}
	return role + "\x00" + sub
}
