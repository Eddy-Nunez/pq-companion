package raidcomp

import (
	"fmt"
	"sort"
)

// SplitPreference selects the grouping strategy for a composition proposal.
type SplitPreference string

const (
	// SplitTrinity assembles balanced groups around the classic trinity:
	// each round seats one tank, one healer, CC/debuff coverage, utility,
	// then damage — "Tank-Healer-CC" per group.
	SplitTrinity SplitPreference = "trinity"
	// SplitFocused builds class-focused groups: same-class members cluster
	// into shared groups (e.g. all clerics together for a CH chain), then
	// comp slots are assigned to seated members, class-eligibility first.
)

// Valid reports whether p is a known split preference.
func (p SplitPreference) Valid() bool {
	return p == SplitTrinity
}

// SplitRequest is one composition-proposal request: a roster, the target
// composition (encounter), and a grouping preference.
type SplitRequest struct {
	// Preference is trinity (baseline; empty also means trinity).
	Preference SplitPreference `json:"preference"`
	// GroupSize caps group size (EQ standard 6). 0 = 6.
	GroupSize int `json:"group_size,omitempty"`
	// RespectExistingGroups seats members into their live Zeal group where
	// possible (live groups cohere) instead of pure balancing.
	RespectExistingGroups bool `json:"respect_existing_groups,omitempty"`
	// Cohorts splits the roster into this many smaller raids, each targeting
	// the FULL encounter template (best effort — per-cohort coverage and
	// warnings report shortfalls; see docs/raid-split-cohorts-plan.md).
	// 0/1 = single raid (legacy shape).
	Cohorts int `json:"cohorts,omitempty"`
	// Shapes lists the enabled group-shape template ids (encounter shapes,
	// docs/raid-group-compositions-plan.md): named role multisets the weave
	// seats into whole groups BEFORE the trinity pass. Requires cohort mode
	// (cohorts >= 2); unknown ids are a request error.
	Shapes []string `json:"shapes,omitempty"`
	// ShapeDistribution controls per-raid shape placement in cohort mode:
	// "replicate" (default) — every raid fields every enabled shape;
	// "distribute" — shape i applies to raid i, shapes beyond the raid
	// count are ignored with a warning (small templates slot into one raid
	// without unbalancing the others).
	ShapeDistribution string `json:"shape_distribution,omitempty"`
}

// Slot is one proposed assignment: a member filling a comp-role slot (or a
// plain group seat).
type Slot struct {
	Member string    `json:"member"`
	Class  ClassCode `json:"class,omitempty"`
	Role   string    `json:"role"`
	Sub    string    `json:"sub_role,omitempty"`
	Path   string    `json:"path"`
	Label  string    `json:"label"`
	// Level is the comp level this slot satisfies (min|rec). Empty for
	// fill seats (members seated without a comp role).
	Level CompLevel `json:"level,omitempty"`
	// Group is the 1-based group number the proposal places the member in.
	Group int    `json:"group"`
	Rank  string `json:"rank,omitempty"`
}

// ProposedGroup is one proposed group with its filled slots in seat order.
type ProposedGroup struct {
	Number int    `json:"number"`
	Size   int    `json:"size"`
	Slots  []Slot `json:"slots"`
	// Shape is the group-shape identifier this group was formed from
	// (docs/raid-group-compositions-plan.md); empty for trinity-formed
	// groups. Under-filled shapes leave open seats — visible shortfall,
	// never filled by the trinity pass.
	Shape string `json:"shape_id,omitempty"`
}

// Unassigned is a roster member the proposal could not seat, with the reason.
type Unassigned struct {
	Name   string    `json:"name"`
	Class  ClassCode `json:"class,omitempty"`
	Reason string    `json:"reason"`
}

// Coverage is the per-leaf staffing outcome of the proposal at one level.
type Coverage struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Need   int    `json:"need"`   // target staffing at the assessed level
	Placed int    `json:"placed"` // slots the proposal filled
}

// SplitReport is the full proposal: groups, unseated members, coverage
// rollups at both comp levels, and non-fatal warnings.
type SplitReport struct {
	EncounterID   string          `json:"encounter_id"`
	EncounterName string          `json:"encounter_name"`
	Preference    SplitPreference `json:"preference"`
	GroupSize     int             `json:"group_size"`
	Groups        []ProposedGroup `json:"groups"`
	Unassigned    []Unassigned    `json:"unassigned"`
	Min           []Coverage      `json:"min"`
	Rec           []Coverage      `json:"rec"`
	Warnings      []string        `json:"warnings,omitempty"`
	// Cohorts carries the per-cohort reports (docs/raid-split-cohorts-plan.md).
	// Single-raid requests populate exactly one entry mirroring the legacy
	// fields above; cohort mode (>1) is authoritative there and leaves the
	// legacy fields empty — never null (the no-null rule).
	Cohorts []CohortReport `json:"cohorts"`
}

// CohortReport is one smaller raid produced by cohort mode: its own groups
// and its own MIN/REC coverage verdict against the full template.
type CohortReport struct {
	Number      int             `json:"number"`
	Groups      []ProposedGroup `json:"groups"`
	Min         []Coverage      `json:"min"`
	Rec         []Coverage      `json:"rec"`
	RosterCount int             `json:"roster_count"`
	Warnings    []string        `json:"warnings,omitempty"`
}

// Shape distribution modes (docs/raid-group-compositions-plan.md §5).
const (
	// ShapeDistributionReplicate: every cohort fields every enabled shape.
	ShapeDistributionReplicate = "replicate"
	// ShapeDistributionDistribute: shape i applies to cohort i (no wrap —
	// shapes beyond the cohort count are ignored with a warning, §7).
	ShapeDistributionDistribute = "distribute"
)

// maxSplitGroups mirrors EQ's raid cap: 12 groups of 6.
const maxSplitGroups = 12

// maxSplitCohorts caps cohort mode (docs/raid-split-cohorts-plan.md): beyond
// six smaller raids the per-cohort comps stop being meaningful.
// MaxSplitCohorts caps cohort mode (docs/raid-split-cohorts-plan.md): beyond
// six smaller raids the per-cohort comps stop being meaningful.
const MaxSplitCohorts = 6

// splitMember is the internal roster member shape plus seat/role state.
type splitMember struct {
	name    string
	code    ClassCode
	liveGrp string // live Zeal group number, "" when unknown
	rank    string
	group   int // proposed group (0 = unseated)
	slot    *splitSlot
	// cohort mode: cohort is the member's raid (0 = unset in the legacy
	// single-raid path); localGrp is their live group renumbered within its
	// cohort block (0 = no live group).
	cohort   int
	localGrp int
}

// splitSlot is one open comp slot to fill.
type splitSlot struct {
	leaf   RoleLeaf
	level  CompLevel
	bucket int
	cohort int // cohort mode: which raid this slot belongs to (0 = legacy)
	filled bool
	member *splitMember
}

// splitLeaf pairs a taxonomy leaf with its comp needs at both levels.
type splitLeaf struct {
	leaf     RoleLeaf
	min, rec int
}

// roleBucket orders leaves for trinity weaving: tank < healer < slow/debuff
// < utility < damage. Unknown roles land in utility.
func roleBucket(l RoleLeaf) int {
	switch {
	case l.Role == "tank":
		return 0
	case l.Role == "healer":
		return 1
	case l.Role == "slower" || l.Role == "debuffer" || l.Role == "cc":
		return 2
	case l.Role == "damage":
		return 4
	default:
		return 3
	}
}

// orderedSplitLeaves resolves the encounter's comps against the taxonomy
// into leaves with needs, in taxonomy position order. Zero-need leaves are
// skipped at both levels (mirrors Check's "no need, no row" rule).
func orderedSplitLeaves(leaves []RoleLeaf, enc *Encounter) []splitLeaf {
	compByLeaf := make(map[string]CompRow, len(enc.Comps))
	for _, c := range enc.Comps {
		compByLeaf[leafKey(c.Role, c.Sub)] = c
	}
	var out []splitLeaf
	for _, leaf := range leaves {
		row, ok := compByLeaf[leafKey(leaf.Role, leaf.Sub)]
		if !ok || (row.Min == 0 && row.Rec == 0) {
			continue
		}
		out = append(out, splitLeaf{leaf: leaf, min: row.Min, rec: row.Rec})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].leaf.Position < out[j].leaf.Position })
	return out
}

// buildSplitSlots expands the ordered leaves into fillable slots: every MIN
// slot first, then the REC slots beyond MIN as best-effort extras.
func buildSplitSlots(ordered []splitLeaf) []splitSlot {
	var slots []splitSlot
	for _, sl := range ordered {
		b := roleBucket(sl.leaf)
		for i := 0; i < sl.min; i++ {
			slots = append(slots, splitSlot{leaf: sl.leaf, level: CompMin, bucket: b})
		}
		for i := 0; i < sl.rec-sl.min; i++ {
			slots = append(slots, splitSlot{leaf: sl.leaf, level: CompRec, bucket: b})
		}
	}
	return slots
}

// weave interleaves bucket-ordered slot queues round-robin so each group
// round draws one tank, one healer, CC/debuff, utility, then damage — the
// trinity shape.
func weave(slots []splitSlot) []splitSlot {
	var queues [5][]splitSlot
	for _, s := range slots {
		queues[s.bucket] = append(queues[s.bucket], s)
	}
	out := make([]splitSlot, 0, len(slots))
	for round := 0; ; round++ {
		empty := true
		for b := 0; b < 5; b++ {
			if round < len(queues[b]) {
				out = append(out, queues[b][round])
				empty = false
			}
		}
		if empty {
			return out
		}
	}
}

// minBeforeRec stably partitions slots so every MIN seat precedes every REC
// seat, preserving the woven order within each level. Weave priority is:
// achieve MIN, then role balance (the interleave), then REC extras. Without
// it a REC seat drained from a bucket queue before a sibling's MIN seat can
// claim the only eligible member and strand the MIN seat (live-caught
// 2026-10-04: slower.REC hogged an enchanter that debuffer.slows.MIN
// needed, leaving debuffer 0/1).
func minBeforeRec(slots []splitSlot) []splitSlot {
	out := make([]splitSlot, 0, len(slots))
	for _, s := range slots {
		if s.level == CompMin {
			out = append(out, s)
		}
	}
	for _, s := range slots {
		if s.level != CompMin {
			out = append(out, s)
		}
	}
	return out
}

// eligible reports whether m may fill s's slot: unroled, classed, and
// class-eligible for the leaf.
func (m *splitMember) eligible(s *splitSlot) bool {
	return m.slot == nil && m.code != "" && classInSet(m.code, s.leaf.Classes)
}

func classInSet(c ClassCode, set ClassSet) bool {
	for _, code := range set {
		if code == c {
			return true
		}
	}
	return false
}

// Split generates a group-composition proposal from a roster and an
// encounter's target composition. Pure input → report (never mutates the
// store), like Check: slots are proposals, not authoritative assignments.
//
// MIN slots are the hard floor and fill first; REC extras are best-effort.
// A member fills a comp slot only when their class is in the leaf's
// ClassSet; members without a resolvable class are seated as fill but never
// hold comp slots.
func Split(leaves []RoleLeaf, enc *Encounter, req SplitRequest, members []RosterMember) (*SplitReport, error) {
	if enc == nil {
		return nil, fmt.Errorf("raidcomp: encounter required")
	}
	if req.Preference == "" {
		req.Preference = SplitTrinity // baseline default
	}
	if !req.Preference.Valid() {
		return nil, fmt.Errorf("raidcomp: invalid preference %q (want trinity)", req.Preference)
	}
	if req.ShapeDistribution == "" {
		req.ShapeDistribution = ShapeDistributionReplicate
	}
	if req.ShapeDistribution != ShapeDistributionReplicate && req.ShapeDistribution != ShapeDistributionDistribute {
		return nil, fmt.Errorf("raidcomp: invalid shape_distribution %q (want replicate|distribute)", req.ShapeDistribution)
	}
	// Shapes are a multi-raid feature (user-confirmed decision, plan §8.3):
	// single-raid proposals weave trinity only.
	if len(req.Shapes) > 0 && req.Cohorts <= 1 {
		return nil, fmt.Errorf("raidcomp: shapes require cohort mode (cohorts >= 2)")
	}
	if _, err := resolveShapes(enc, req.Shapes); err != nil {
		return nil, err
	}
	size := req.GroupSize
	if size <= 0 {
		size = 6
	}
	if size < 1 || size > 12 {
		return nil, fmt.Errorf("raidcomp: group_size must be 1..12, got %d", size)
	}
	if req.Cohorts < 0 || req.Cohorts > MaxSplitCohorts {
		return nil, fmt.Errorf("raidcomp: cohorts must be 0..%d, got %d", MaxSplitCohorts, req.Cohorts)
	}
	if req.Cohorts > 1 {
		return splitCohorts(leaves, enc, req, members)
	}

	rep := &SplitReport{
		EncounterID:   enc.ID,
		EncounterName: enc.Name,
		Preference:    req.Preference,
		GroupSize:     size,
		// Nil slices marshal as JSON null; the frontend renders these as
		// arrays unconditionally, so they must always be [] (same
		// no-null rule as the taxonomy DTOs).
		Unassigned: []Unassigned{},
		Warnings:   []string{},
		Cohorts:    []CohortReport{},
	}
	if len(enc.Comps) == 0 {
		rep.Warnings = append(rep.Warnings, "encounter has no composition recorded — proposal seats members without comp roles")
	}

	ordered := orderedSplitLeaves(leaves, enc)
	slots := buildSplitSlots(ordered)
	if req.Preference == SplitTrinity {
		slots = weave(slots)
		slots = minBeforeRec(slots)
	}

	ms := make([]splitMember, 0, len(members))
	for _, m := range members {
		if m.Name == "" {
			continue
		}
		ms = append(ms, splitMember{name: m.Name, code: m.Class, liveGrp: m.Group, rank: m.Rank})
	}

	groups := newGroupTracker(size)

	fillSlotsInGroups(slots, ms, groups, req, rep)
	seatRemaining(ms, groups, req.RespectExistingGroups, rep)

	rep.Groups = assembleGroups(ms, 0, size)
	// buildCoverage returns empty (non-nil) slices so min/rec are [] not
	// null even when the encounter has no comps.
	rep.Min, rep.Rec = buildCoverage(ordered, slots)
	if rep.Min == nil {
		rep.Min = []Coverage{}
	}
	if rep.Rec == nil {
		rep.Rec = []Coverage{}
	}
	sort.Slice(rep.Unassigned, func(i, j int) bool { return rep.Unassigned[i].Name < rep.Unassigned[j].Name })
	// Mirror the legacy shape as a single-entry Cohorts list so clients can
	// consume one uniform shape (the UI reads cohorts first).
	seated := 0
	for i := range ms {
		if ms[i].group != 0 {
			seated++
		}
	}
	rep.Cohorts = append(rep.Cohorts, CohortReport{
		Number: 1, Groups: rep.Groups, Min: rep.Min, Rec: rep.Rec,
		RosterCount: seated, Warnings: rep.Warnings,
	})
	return rep, nil
}

// groupTracker tracks per-group occupancy against the size cap. Reserved
// groups are claimed by shape groups (docs/raid-group-compositions-plan.md):
// the trinity pass, live-group seeding, and free-agent cursors all skip
// them, so an under-filled shape keeps its open seats visible.
type groupTracker struct {
	size     int
	count    map[int]int
	reserved map[int]bool
}

func newGroupTracker(size int) *groupTracker {
	return &groupTracker{size: size, count: map[int]int{}, reserved: map[int]bool{}}
}

func (g *groupTracker) space(grp int) bool {
	return !g.reserved[grp] && g.count[grp] < g.size
}

func (g *groupTracker) seat(grp int) {
	g.count[grp]++
}

// reserve claims grp for a shape group: every later cursor (trinity slots,
// live-group seeding, leftovers) skips it.
func (g *groupTracker) reserve(grp int) {
	g.reserved[grp] = true
}

func (g *groupTracker) firstOpen(from int) int {
	for grp := from; grp <= maxSplitGroups; grp++ {
		if g.space(grp) {
			return grp
		}
	}
	return 0
}

// liveGroupNum parses a live Zeal group string ("1".."12"); "0"/""/invalid
// → 0 (no live group).
func liveGroupNum(g string) int {
	if g == "" || g == "0" {
		return 0
	}
	n := 0
	for _, r := range g {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	if n < 1 || n > maxSplitGroups {
		return 0
	}
	return n
}

// fillSlotsInGroups walks the (optionally woven) slot pass with a group
// cursor: each slot fills in the current group, advancing when full. With
// RespectExistingGroups, a chosen member seats into their LIVE group (when
// it has space) so live groups cohere instead of spreading across the
// cursor order.
func fillSlotsInGroups(slots []splitSlot, ms []splitMember, groups *groupTracker, req SplitRequest, rep *SplitReport) {
	gi := 1
	capWarned := false
	for si := range slots {
		s := &slots[si]
		for gi <= maxSplitGroups && !groups.space(gi) {
			gi++
		}
		if gi > maxSplitGroups {
			if !capWarned {
				rep.Warnings = append(rep.Warnings, fmt.Sprintf("composition exceeds %d groups — remaining slots unfilled", maxSplitGroups))
				capWarned = true
			}
			break
		}
		m := pickForSlot(s, ms, gi, req)
		if m == nil {
			continue // unfilled; coverage reports the shortfall
		}
		if m.group == 0 {
			seat := gi
			if req.RespectExistingGroups {
				if lg := liveGroupNum(m.liveGrp); lg > 0 && groups.space(lg) {
					seat = lg
				}
			}
			groups.seat(seat)
			m.group = seat
		}
		m.slot = s
		s.filled = true
		s.member = m
	}
}

// pickForSlot selects the best eligible member for s when filling group gi.
// Priority: a member already seated in gi > unseated members whose live
// group is gi (RespectExistingGroups) > any other unseated member; ties
// break by name for determinism.
func pickForSlot(s *splitSlot, ms []splitMember, gi int, req SplitRequest) *splitMember {
	var best *splitMember
	for i := range ms {
		m := &ms[i]
		if !m.eligible(s) {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		as, bs := m.scoreFor(gi, req), best.scoreFor(gi, req)
		if as != bs {
			if as > bs {
				best = m
			}
			continue
		}
		if m.name < best.name {
			best = m
		}
	}
	return best
}

// scoreFor ranks candidates for a slot in group gi: seated-in-gi first,
// then live-group affinity when RespectExistingGroups is set, then everyone
// else.
func (m *splitMember) scoreFor(gi int, req SplitRequest) int {
	switch {
	case m.group == gi:
		return 3
	case m.group != 0:
		return 0 // seated elsewhere; only when nobody better exists
	case req.RespectExistingGroups && liveGroupNum(m.liveGrp) == gi:
		return 2
	default:
		return 1
	}
}

// seatRemaining seats members the allocation left unseated: prefer their
// live group (when RespectExistingGroups), else the first group with space.
// Members that fit nowhere come back unassigned with the reason.
func seatRemaining(ms []splitMember, groups *groupTracker, respect bool, rep *SplitReport) {
	var order []*splitMember
	for i := range ms {
		if ms[i].group == 0 {
			order = append(order, &ms[i])
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].name < order[j].name })
	for _, m := range order {
		grp := 0
		if respect {
			if lg := liveGroupNum(m.liveGrp); lg > 0 && groups.space(lg) {
				grp = lg
			}
		}
		if grp == 0 {
			grp = groups.firstOpen(1)
		}
		if grp == 0 {
			rep.Unassigned = append(rep.Unassigned, Unassigned{
				Name: m.name, Class: m.code,
				Reason: fmt.Sprintf("all %d groups are full (size %d)", maxSplitGroups, groups.size),
			})
			continue
		}
		groups.seat(grp)
		m.group = grp
	}
}

// assembleGroups turns seat state into the report's group list, carrying
// each member's slot metadata (role/level) onto their seat row. cohort
// filters the member set: 0 = all (legacy single-raid path), otherwise only
// members of that cohort.
func assembleGroups(ms []splitMember, cohort, size int) []ProposedGroup {
	byGroup := map[int][]Slot{}
	maxGrp := 0
	for i := range ms {
		m := &ms[i]
		if m.group == 0 || (cohort != 0 && m.cohort != cohort) {
			continue
		}
		if m.group > maxGrp {
			maxGrp = m.group
		}
		slot := Slot{
			Member: m.name, Class: m.code, Group: m.group, Rank: m.rank,
		}
		if m.slot != nil {
			slot.Role = m.slot.leaf.Role
			slot.Sub = m.slot.leaf.Sub
			slot.Path = m.slot.leaf.Path()
			slot.Label = m.slot.leaf.Label
			slot.Level = m.slot.level
		}
		byGroup[m.group] = append(byGroup[m.group], slot)
	}
	out := make([]ProposedGroup, 0, maxGrp)
	for g := 1; g <= maxGrp; g++ {
		slots := byGroup[g]
		if len(slots) == 0 {
			continue
		}
		out = append(out, ProposedGroup{Number: g, Size: size, Slots: slots})
	}
	return out
}

// buildCoverage computes placed-vs-need per leaf per level from slot fills.
func buildCoverage(ordered []splitLeaf, slots []splitSlot) (minC, recC []Coverage) {
	placedBy := make(map[string]int, len(ordered)*2)
	for i := range slots {
		s := &slots[i]
		if s.filled {
			placedBy[s.leaf.Path()+"\x00"+string(s.level)]++
		}
	}
	for _, sl := range ordered {
		minC = append(minC, Coverage{
			Path: sl.leaf.Path(), Label: sl.leaf.Label,
			Need: sl.min, Placed: placedBy[sl.leaf.Path()+"\x00"+string(CompMin)],
		})
		recC = append(recC, Coverage{
			Path: sl.leaf.Path(), Label: sl.leaf.Label,
			Need: sl.rec, Placed: placedBy[sl.leaf.Path()+"\x00"+string(CompRec)],
		})
	}
	return minC, recC
}

// ── cohort mode (docs/raid-split-cohorts-plan.md) ──────────────────────────

// ── shape groups (docs/raid-group-compositions-plan.md §5) ─────────────

// resolveShapes filters the encounter's shapes by the request's enabled ids,
// in encounter declaration order. Unknown ids are a loud request error (the
// wildcards-400 precedent); duplicate request ids are deduped.
func resolveShapes(enc *Encounter, ids []string) ([]Shape, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	var out []Shape
	seen := map[string]bool{}
	for _, sh := range enc.Shapes {
		if !want[sh.ID] || seen[sh.ID] {
			continue
		}
		seen[sh.ID] = true
		out = append(out, sh)
	}
	for _, id := range ids {
		if !seen[id] {
			return nil, fmt.Errorf("raidcomp: unknown shape %q (encounter defines no such group composition)", id)
		}
	}
	return out, nil
}

// shapesForCohorts maps each cohort (1..K) to the ordered shapes it must
// field. replicate (default): every cohort gets every shape. distribute:
// shape i applies to cohort i — no wrap; shapes beyond the cohort count are
// ignored with a warning (plan §7), so small templates slot into one raid
// without unbalancing the others.
func shapesForCohorts(enabled []Shape, K int, dist string) (map[int][]Shape, []string) {
	per := make(map[int][]Shape, K)
	var warnings []string
	switch dist {
	case ShapeDistributionDistribute:
		for i, sh := range enabled {
			if i < K {
				per[i+1] = append(per[i+1], sh)
			} else {
				warnings = append(warnings, fmt.Sprintf(
					"shape %q ignored — distribute applies at most one shape per raid and there are only %d raids", sh.ID, K))
			}
		}
	default: // replicate
		for c := 1; c <= K; c++ {
			per[c] = append(per[c], enabled...)
		}
	}
	return per, warnings
}

// seatShape picks the members for one shape's rows: for each row (in row
// order) up to Count eligible members — same-cohort members preferred over
// free agents, name tie-break (the existing picker's rule). Picked members
// are consumed for the whole proposal (slot set) and adopt the shape's
// cohort when unaffiliated. Rows short of their count return warnings —
// best-effort, never silent (plan §5.5). A row whose role left the taxonomy
// fills nobody (storage validates rows at save time; this is the
// taxonomy-changed-under-us safety net).
func seatShape(sh Shape, leafByPath map[string]RoleLeaf, ms []splitMember, cohort int) ([]*splitMember, map[string]int, []string) {
	var members []*splitMember
	credits := map[string]int{}
	var warnings []string
	for _, r := range sh.Rows {
		leaf, ok := leafByPath[r.Path()]
		if !ok {
			warnings = append(warnings, fmt.Sprintf(
				"raid %d: shape %q: %s 0/%d (role not in taxonomy)", cohort, sh.ID, r.Path(), r.Count))
			continue
		}
		placed := 0
		for placed < r.Count {
			m := pickShapeMember(leaf.Classes, ms, cohort)
			if m == nil {
				break
			}
			// Synthetic slot: carries the row's leaf onto the member's seat
			// (report metadata + coverage attribution) and consumes the member
			// for the whole proposal — the slot==nil eligibility rule does the
			// rest, untouched.
			m.slot = &splitSlot{leaf: leaf, cohort: cohort}
			if m.cohort == 0 {
				m.cohort = cohort
			}
			members = append(members, m)
			credits[r.Path()]++
			placed++
		}
		if placed < r.Count {
			warnings = append(warnings, fmt.Sprintf(
				"raid %d: shape %q: %s %d/%d", cohort, sh.ID, r.Path(), placed, r.Count))
		}
	}
	return members, credits, warnings
}

// pickShapeMember selects the best eligible member for a shape seat:
// members of the shape's cohort first, then unaffiliated free agents;
// members of OTHER cohorts are never eligible (a raid's shape must not
// starve a sibling raid's pick); ties break by name.
func pickShapeMember(classes ClassSet, ms []splitMember, cohort int) *splitMember {
	var best *splitMember
	for i := range ms {
		m := &ms[i]
		if m.slot != nil || m.code == "" || !classInSet(m.code, classes) {
			continue
		}
		if m.cohort != 0 && m.cohort != cohort {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		as, bs := m.cohort == cohort, best.cohort == cohort
		if as != bs {
			if as {
				best = m
			}
			continue
		}
		if m.name < best.name {
			best = m
		}
	}
	return best
}

// claimShapeGroups claims group numbers for one shape in a cohort's layout:
// a pinned shape takes its group number when within the cohort's group
// budget and unclaimed — otherwise it falls back to the first free number
// with a warning (plan §5); unpinned shapes take the first free numbers.
// A shape needing more than `size` seats spills into the next free numbers,
// tagged with the same shape id. Claims are based on the PICKED count, so
// an under-filled shape never wastes group room on seats it could not fill.
func claimShapeGroups(sh Shape, picked, budget, size, cohort int, taken map[int]string, warnings *[]string) []int {
	need := (picked + size - 1) / size
	take := func(from int) int {
		for g := from; g <= maxSplitGroups; g++ {
			if _, ok := taken[g]; !ok {
				return g
			}
		}
		return 0
	}
	base := 0
	if sh.GroupNumber != 0 {
		_, collide := taken[sh.GroupNumber]
		if sh.GroupNumber <= budget && !collide {
			base = sh.GroupNumber
		} else {
			base = take(1)
			*warnings = append(*warnings, fmt.Sprintf(
				"raid %d: shape %q pinned to group %d is unavailable (collision or beyond this raid's %d groups) — using group %d",
				cohort, sh.ID, sh.GroupNumber, budget, base))
		}
	} else {
		base = take(1)
	}
	if base == 0 {
		*warnings = append(*warnings, fmt.Sprintf(
			"raid %d: shape %q could not claim a group (all %d taken) — its members weave into free groups",
			cohort, sh.ID, maxSplitGroups))
		return nil
	}
	grps := []int{base}
	for len(grps) < need {
		next := take(grps[len(grps)-1] + 1)
		if next == 0 {
			break
		}
		grps = append(grps, next)
	}
	return grps
}

// applyShapeCoverage credits shape-seated members toward the encounter
// template's coverage (plan §5.3): a shape's seats fill the matching leaf's
// MIN need first, then its REC need, so a HealStack's 5 clerics satisfy the
// ch_cleric need. Trinity-placed slots are counted separately by
// buildCoverage; the two never overlap (shape members are consumed before
// the trinity pass).
func applyShapeCoverage(ordered []splitLeaf, credits map[string]int, minC, recC []Coverage) {
	for i, sl := range ordered {
		credit := credits[sl.leaf.Path()]
		if credit == 0 {
			continue
		}
		mc := credit
		if mc > sl.min {
			mc = sl.min
		}
		minC[i].Placed += mc
		if rem := credit - mc; rem > 0 {
			rc := rem
			if rc > sl.rec-sl.min {
				rc = sl.rec - sl.min
			}
			recC[i].Placed += rc
		}
	}
}

// splitCohorts partitions the roster into K smaller raids, each targeting the
// FULL encounter template (best effort). Trinity baseline, plus the
// encounter's enabled group shapes when the request opts in: shape groups
// seat first (replicate or distribute per raid, pins honored with
// first-free fallback), consume their members, and their seats count toward
// the template's MIN/REC coverage.
//
// Pipeline: shape member picking (per cohort, before any trinity slot) →
// per-cohort woven full-template slots → live-group cohort seeding
// (distinct live groups chunked into K contiguous blocks — pre-formed tower
// groups stay intact) → interleaved allocation (cohorts round-robin per slot
// index, so scarce classes spread evenly; seeded members prefer their own
// cohort) → free-agent fill distribution (smallest cohort first) → per-cohort
// layout (seeded members at their live group renumbered within its block,
// everything else in woven order via the group cursor) → per-cohort coverage
// and MIN-shortfall warnings.
func splitCohorts(leaves []RoleLeaf, enc *Encounter, req SplitRequest, members []RosterMember) (*SplitReport, error) {
	K := req.Cohorts
	size := req.GroupSize
	if size <= 0 {
		size = 6
	}
	rep := &SplitReport{
		EncounterID:   enc.ID,
		EncounterName: enc.Name,
		Preference:    req.Preference,
		GroupSize:     size,
		// Legacy fields stay empty (never null) in cohort mode; the UI reads
		// the Cohorts list.
		Groups:     []ProposedGroup{},
		Unassigned: []Unassigned{},
		Min:        []Coverage{},
		Rec:        []Coverage{},
		Warnings:   []string{},
		Cohorts:    make([]CohortReport, 0, K),
	}
	if len(enc.Comps) == 0 {
		rep.Warnings = append(rep.Warnings, "encounter has no composition recorded — proposal seats members without comp roles")
	}

	ordered := orderedSplitLeaves(leaves, enc)

	// Per-cohort woven full-template slots, tagged with their cohort.
	cohortSlots := make([][]splitSlot, K+1)
	for c := 1; c <= K; c++ {
		s := buildSplitSlots(ordered)
		weave(s)
		for i := range s {
			s[i].cohort = c
		}
		cohortSlots[c] = s
	}

	// Resolve members.
	ms := make([]splitMember, 0, len(members))
	for _, m := range members {
		if m.Name == "" {
			continue
		}
		ms = append(ms, splitMember{name: m.Name, code: m.Class, liveGrp: m.Group, rank: m.Rank})
	}

	// Cohort seeding: chunk the DISTINCT live group numbers present into K
	// contiguous blocks (sorted, near-equal). Members without a live group
	// are free agents.
	var liveNumbers []int
	seen := map[int]bool{}
	for i := range ms {
		if n := liveGroupNum(ms[i].liveGrp); n > 0 && !seen[n] {
			seen[n] = true
			liveNumbers = append(liveNumbers, n)
		}
	}
	sort.Ints(liveNumbers)
	cohortOfLive := map[int]int{}
	localOfLive := map[int]int{}
	idx := 0
	for c := 1; c <= K && idx < len(liveNumbers); c++ {
		blockLen := len(liveNumbers)/K + boolToInt(c <= len(liveNumbers)%K)
		for j := 0; j < blockLen && idx < len(liveNumbers); j++ {
			cohortOfLive[liveNumbers[idx]] = c
			localOfLive[liveNumbers[idx]] = j + 1
			idx++
		}
	}
	for i := range ms {
		if n := liveGroupNum(ms[i].liveGrp); n > 0 {
			if c, ok := cohortOfLive[n]; ok {
				ms[i].cohort = c
				ms[i].localGrp = localOfLive[n]
			}
		}
	}

	// Shape member picking (docs/raid-group-compositions-plan.md §5): each
	// cohort's shapes pick their members BEFORE any trinity slot — shape
	// groups seat first, ignore live-group affinity, and consume their
	// members for the whole proposal.
	enabled, err := resolveShapes(enc, req.Shapes)
	if err != nil {
		return nil, err
	}
	perCohortShapes, distWarnings := shapesForCohorts(enabled, K, req.ShapeDistribution)
	rep.Warnings = append(rep.Warnings, distWarnings...)
	leafByPath := make(map[string]RoleLeaf, len(leaves))
	for _, l := range leaves {
		leafByPath[l.Path()] = l
	}
	type shapeClaim struct {
		shape   Shape
		members []*splitMember
	}
	claims := make([][]shapeClaim, K+1)
	shapeCredits := make([]map[string]int, K+1)
	shapeWarnings := make([][]string, K+1)
	for c := 1; c <= K; c++ {
		shapeCredits[c] = map[string]int{}
		for _, sh := range perCohortShapes[c] {
			members, credits, warns := seatShape(sh, leafByPath, ms, c)
			claims[c] = append(claims[c], shapeClaim{shape: sh, members: members})
			for path, n := range credits {
				shapeCredits[c][path] += n
			}
			shapeWarnings[c] = append(shapeWarnings[c], warns...)
		}
	}

	// Interleaved allocation: walk the cohorts round-robin per slot index so
	// scarce classes alternate between cohorts instead of cohort 1 hoarding
	// them. Seeded members prefer their own cohort's slots; ties by name.
	// Priority is MIN first, then role balance, then REC — every cohort's
	// MIN seats are decided before any REC seat may claim a candidate, so a
	// REC seat can never strand a sibling MIN seat (live-caught 2026-10-04:
	// slower.REC hogged an enchanter that debuffer.slows.MIN needed).
	maxLen := 0
	for c := 1; c <= K; c++ {
		if len(cohortSlots[c]) > maxLen {
			maxLen = len(cohortSlots[c])
		}
	}
	for _, level := range []CompLevel{CompMin, CompRec} {
		for i := 0; i < maxLen; i++ {
			for c := 1; c <= K; c++ {
				if i >= len(cohortSlots[c]) || cohortSlots[c][i].level != level {
					continue
				}
				s := &cohortSlots[c][i]
				m := pickCohortMember(s, ms)
				if m == nil {
					continue
				}
				m.slot = s
				s.filled = true
				s.member = m
			}
		}
	}

	// Free agents slotted into a cohort belong to it from here on.
	for i := range ms {
		if ms[i].cohort == 0 && ms[i].slot != nil {
			ms[i].cohort = ms[i].slot.cohort
		}
	}

	// Headcount per cohort, then free-agent fill distribution round-robin to
	// the smallest cohort (everyone lands in some raid — the roster IS the
	// pool being split).
	counts := make([]int, K+1)
	for i := range ms {
		if ms[i].cohort != 0 {
			counts[ms[i].cohort]++
		}
	}
	var free []*splitMember
	for i := range ms {
		if ms[i].cohort == 0 {
			free = append(free, &ms[i])
		}
	}
	sort.Slice(free, func(i, j int) bool { return free[i].name < free[j].name })
	for _, m := range free {
		smallest := 1
		for c := 2; c <= K; c++ {
			if counts[c] < counts[smallest] {
				smallest = c
			}
		}
		m.cohort = smallest
		counts[smallest]++
	}

	// Per-cohort layout and reports.
	for c := 1; c <= K; c++ {
		cr := CohortReport{
			Number: c, Groups: []ProposedGroup{}, Min: []Coverage{}, Rec: []Coverage{}, Warnings: []string{},
		}
		cr.RosterCount = counts[c]
		if counts[c] == 0 {
			cr.Warnings = append(cr.Warnings, fmt.Sprintf("raid %d has no members — roster too small for %d cohorts", c, K))
			rep.Cohorts = append(rep.Cohorts, cr)
			continue
		}
		budget := (counts[c] + size - 1) / size // groups this cohort fields
		tracker := newGroupTracker(size)

		cr.Warnings = append(cr.Warnings, shapeWarnings[c]...)

		// Shape groups claim their group numbers first: pins honored (within
		// the cohort's group budget, no collision), first-free otherwise,
		// overflow spills into the next free numbers. Reserved groups are
		// skipped by every later cursor, so an under-filled shape keeps its
		// open seats visible instead of being quietly topped up.
		claimed := map[int]string{}
		for ci := range claims[c] {
			cl := &claims[c][ci]
			if len(cl.members) == 0 {
				continue
			}
			grps := claimShapeGroups(cl.shape, len(cl.members), budget, size, c, claimed, &cr.Warnings)
			for _, g := range grps {
				claimed[g] = cl.shape.ID
				tracker.reserve(g)
			}
			mi := 0
			for _, g := range grps {
				for tracker.count[g] < size && mi < len(cl.members) {
					m := cl.members[mi]
					mi++
					tracker.seat(g)
					m.group = g
				}
			}
		}

		// Seeded members seat at their live group renumbered within its
		// block (when it fits the cohort's group budget).
		for i := range ms {
			m := &ms[i]
			if m.cohort != c || m.localGrp == 0 || m.group != 0 {
				continue
			}
			if m.localGrp <= budget && tracker.space(m.localGrp) {
				tracker.seat(m.localGrp)
				m.group = m.localGrp
			}
		}

		// Slot members seat in woven order via the cursor (the trinity shape).
		gi := 1
		for si := range cohortSlots[c] {
			s := &cohortSlots[c][si]
			if !s.filled {
				continue
			}
			m := s.member
			if m.group != 0 {
				continue
			}
			for gi <= maxSplitGroups && !tracker.space(gi) {
				gi++
			}
			if gi > maxSplitGroups {
				break
			}
			tracker.seat(gi)
			m.group = gi
		}

		// Any cohort member still unseated (fills, seeded overflow): cursor.
		for i := range ms {
			m := &ms[i]
			if m.cohort != c || m.group != 0 {
				continue
			}
			for gi <= maxSplitGroups && !tracker.space(gi) {
				gi++
			}
			if gi > maxSplitGroups {
				break
			}
			tracker.seat(gi)
			m.group = gi
		}

		cr.Groups = assembleGroups(ms, c, size)
		for gi := range cr.Groups {
			cr.Groups[gi].Shape = claimed[cr.Groups[gi].Number]
		}
		cr.Min, cr.Rec = buildCoverage(ordered, cohortSlots[c])
		applyShapeCoverage(ordered, shapeCredits[c], cr.Min, cr.Rec)
		if cr.Min == nil {
			cr.Min = []Coverage{}
		}
		if cr.Rec == nil {
			cr.Rec = []Coverage{}
		}
		for _, row := range cr.Min {
			if gap := row.Need - row.Placed; gap > 0 {
				cr.Warnings = append(cr.Warnings, fmt.Sprintf(
					"raid %d: %s %d/%d of MIN (short %d)", c, row.Label, row.Placed, row.Need, gap))
			}
		}
		rep.Cohorts = append(rep.Cohorts, cr)
	}
	return rep, nil
}

// pickCohortMember selects the best eligible member for a cohort slot:
// members seeded into the slot's cohort first, then free agents; ties break
// by name. Members seeded into a DIFFERENT cohort are never eligible.
func pickCohortMember(s *splitSlot, ms []splitMember) *splitMember {
	var best *splitMember
	for i := range ms {
		m := &ms[i]
		if !m.eligible(s) {
			continue
		}
		if m.cohort != 0 && m.cohort != s.cohort {
			continue
		}
		if best == nil {
			best = m
			continue
		}
		as, bs := cohortScore(m, s), cohortScore(best, s)
		if as != bs {
			if as > bs {
				best = m
			}
			continue
		}
		if m.name < best.name {
			best = m
		}
	}
	return best
}

func cohortScore(m *splitMember, s *splitSlot) int {
	if m.cohort == s.cohort {
		return 2
	}
	return 1
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ── cohort auto-suggest (GET /api/raids/split/plan) ────────────────────────

// SplitPlanLeaf is one comp row's staffing picture against the live roster:
// how many members are class-eligible vs how many the comp asks per raid.
type SplitPlanLeaf struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Eligible int    `json:"eligible"`
	Min      int    `json:"min"`
	Rec      int    `json:"rec"`
}

// SplitPlanReport is the cohort auto-suggest: how many COMPLETE MIN comps
// the roster can staff (MaxCohorts, capped at MaxSplitCohorts), which leaf
// is the binding constraint (BindingPath), and the per-leaf detail behind
// that number.
type SplitPlanReport struct {
	EncounterID   string          `json:"encounter_id"`
	EncounterName string          `json:"encounter_name"`
	RosterTotal   int             `json:"roster_total"`
	RosterMapped  int             `json:"roster_mapped"`
	MaxCohorts    int             `json:"max_cohorts"`
	BindingPath   string          `json:"binding_path,omitempty"`
	Leaves        []SplitPlanLeaf `json:"leaves"`
}

// SplitPlan computes the cohort auto-suggest for one encounter against a
// roster. Pure input → report, like Check/Split.
func SplitPlan(leaves []RoleLeaf, enc *Encounter, members []RosterMember) SplitPlanReport {
	rep := SplitPlanReport{
		EncounterID:   enc.ID,
		EncounterName: enc.Name,
		RosterTotal:   len(members),
		Leaves:        []SplitPlanLeaf{},
	}
	byCode := map[ClassCode]int{}
	for _, m := range members {
		if m.Class == "" {
			continue
		}
		rep.RosterMapped++
		byCode[m.Class]++
	}
	compByLeaf := make(map[string]CompRow, len(enc.Comps))
	for _, c := range enc.Comps {
		compByLeaf[leafKey(c.Role, c.Sub)] = c
	}
	best := 0
	seenMin := false // best==0 is a legitimate ratio (unstaffable leaf) — track
	// initialization separately or every leaf re-binds
	for _, leaf := range leaves {
		row, ok := compByLeaf[leafKey(leaf.Role, leaf.Sub)]
		if !ok || (row.Min == 0 && row.Rec == 0) {
			continue
		}
		eligible := 0
		for _, code := range leaf.Classes {
			eligible += byCode[code]
		}
		rep.Leaves = append(rep.Leaves, SplitPlanLeaf{
			Path: leaf.Path(), Label: leaf.Label,
			Eligible: eligible, Min: row.Min, Rec: row.Rec,
		})
		if row.Min <= 0 {
			continue
		}
		ratio := eligible / row.Min // 0 when the roster cannot even staff one comp
		if !seenMin || ratio < best {
			best = ratio
			rep.BindingPath = leaf.Path()
			seenMin = true
		}
	}
	rep.MaxCohorts = best
	if rep.MaxCohorts < 1 {
		rep.MaxCohorts = 1 // advisory floor: a single raid is always attemptable
	}
	if rep.MaxCohorts > MaxSplitCohorts {
		rep.MaxCohorts = MaxSplitCohorts
	}
	return rep
}
