package raidcomp

import (
	"fmt"
	"sort"
	"strings"
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
	SplitFocused SplitPreference = "focused"
	// SplitCurated is trinity seating plus user pin/cap rules (wildcards):
	// exact members pinned to groups, per-class / per-path / any caps.
	SplitCurated SplitPreference = "curated"
)

// Valid reports whether p is a known split preference.
func (p SplitPreference) Valid() bool {
	switch p {
	case SplitTrinity, SplitFocused, SplitCurated:
		return true
	}
	return false
}

// Wildcard is one curated rule constraining the proposal. Kind selects the
// shape; zero-value fields are ignored per kind.
type Wildcard struct {
	// Kind is one of "member", "class", "path", "any".
	Kind string `json:"kind"`
	// Value is the class code ("clr") or role path ("healer.ch_cleric")
	// for the class / path kinds.
	Value string `json:"value,omitempty"`
	// Member is the exact roster name for the member kind.
	Member string `json:"member,omitempty"`
	// Group pins the rule to one group number (1-based). 0 = any group.
	Group int `json:"group,omitempty"`
	// Max caps how many matching members may be placed anywhere in the
	// proposal (class/path/any kinds; members capped out of seating come
	// back unassigned with a reason). 0 = unlimited.
	Max int `json:"max,omitempty"`
	// Min asks for at least this many matching placements; unmet Mins are
	// reported as warnings, not hard failures.
	Min int `json:"min,omitempty"`
	// Locked pins a member to their pinned (or live) group; the allocator
	// seats them before anything else moves.
	Locked bool `json:"locked,omitempty"`
}

// SplitRequest is one composition-proposal request: a roster, the target
// composition (encounter), and a grouping preference.
type SplitRequest struct {
	// Preference is trinity | focused | curated.
	Preference SplitPreference `json:"preference"`
	// GroupSize caps group size (EQ standard 6). 0 = 6.
	GroupSize int `json:"group_size,omitempty"`
	// RespectExistingGroups seats members into their live Zeal group where
	// possible (live groups cohere) instead of pure balancing.
	RespectExistingGroups bool `json:"respect_existing_groups,omitempty"`
	// Wildcards only apply to SplitCurated.
	Wildcards []Wildcard `json:"wildcards,omitempty"`
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
}

// maxSplitGroups mirrors EQ's raid cap: 12 groups of 6.
const maxSplitGroups = 12

// splitMember is the internal roster member shape plus seat/role state.
type splitMember struct {
	name    string
	code    ClassCode
	liveGrp string // live Zeal group number, "" when unknown
	rank    string
	group   int // proposed group (0 = unseated)
	slot    *splitSlot
	pinned  bool
}

// splitSlot is one open comp slot to fill.
type splitSlot struct {
	leaf   RoleLeaf
	level  CompLevel
	bucket int
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
// hold comp slots. Curated caps (Wildcard.Max) bound how many matching
// members may be placed at all — capped-out members return unassigned.
func Split(leaves []RoleLeaf, enc *Encounter, req SplitRequest, members []RosterMember) (*SplitReport, error) {
	if enc == nil {
		return nil, fmt.Errorf("raidcomp: encounter required")
	}
	if !req.Preference.Valid() {
		return nil, fmt.Errorf("raidcomp: invalid preference %q (want trinity|focused|curated)", req.Preference)
	}
	size := req.GroupSize
	if size <= 0 {
		size = 6
	}
	if size < 1 || size > 12 {
		return nil, fmt.Errorf("raidcomp: group_size must be 1..12, got %d", size)
	}

	rep := &SplitReport{
		EncounterID:   enc.ID,
		EncounterName: enc.Name,
		Preference:    req.Preference,
		GroupSize:     size,
	}
	if len(enc.Comps) == 0 {
		rep.Warnings = append(rep.Warnings, "encounter has no composition recorded — proposal seats members without comp roles")
	}

	ordered := orderedSplitLeaves(leaves, enc)
	slots := buildSplitSlots(ordered)
	if req.Preference == SplitTrinity {
		slots = weave(slots)
	}

	// Resolve members; index by name for curated rules.
	ms := make([]splitMember, 0, len(members))
	byName := make(map[string]*splitMember, len(members))
	for _, m := range members {
		if m.Name == "" {
			continue
		}
		ms = append(ms, splitMember{name: m.Name, code: m.Class, liveGrp: m.Group, rank: m.Rank})
		byName[m.Name] = &ms[len(ms)-1]
	}

	groups := newGroupTracker(size)

	switch req.Preference {
	case SplitFocused:
		// Phase 1: seat by class clusters; Phase 2: assign comp slots to
		// seated members.
		seatFocusedClusters(ms, groups)
		assignSlotsToSeated(slots, ms)
		seatRemaining(ms, groups, req.RespectExistingGroups, rep, nil)
	default: // trinity + curated
		var caps *curatedCaps
		if req.Preference == SplitCurated {
			caps = newCuratedCaps(req.Wildcards)
			applyCuratedRules(req.Wildcards, ms, byName, groups, caps, rep)
		}
		fillSlotsInGroups(slots, ms, groups, req, caps, rep)
		seatRemaining(ms, groups, req.RespectExistingGroups, rep, caps)
		applyCuratedMinChecks(req.Wildcards, ms, rep)
	}

	rep.Groups = assembleGroups(ms, size)
	rep.Min, rep.Rec = buildCoverage(ordered, slots)
	sort.Slice(rep.Unassigned, func(i, j int) bool { return rep.Unassigned[i].Name < rep.Unassigned[j].Name })
	return rep, nil
}

// groupTracker tracks per-group occupancy against the size cap.
type groupTracker struct {
	size  int
	count map[int]int
}

func newGroupTracker(size int) *groupTracker {
	return &groupTracker{size: size, count: map[int]int{}}
}

func (g *groupTracker) space(grp int) bool {
	return g.count[grp] < g.size
}

func (g *groupTracker) seat(grp int) {
	g.count[grp]++
}

func (g *groupTracker) firstOpen(from int) int {
	for grp := from; grp <= maxSplitGroups; grp++ {
		if g.space(grp) {
			return grp
		}
	}
	return 0
}

// applyCuratedRules pre-seats pinned/locked members and pre-charges their
// class against the curated caps. Non-member rules are validated lazily by
// the caps machinery during allocation.
func applyCuratedRules(rules []Wildcard, ms []splitMember, byName map[string]*splitMember, groups *groupTracker, caps *curatedCaps, rep *SplitReport) {
	for _, w := range rules {
		if (w.Kind != "member" && w.Kind != "any") || w.Member == "" {
			continue
		}
		m, ok := byName[w.Member]
		if !ok {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("pinned member %q is not on the roster", w.Member))
			continue
		}
		if m.group != 0 {
			continue // already pinned
		}
		grp := w.Group
		if grp == 0 {
			grp = liveGroupNum(m.liveGrp)
		}
		if grp < 1 || grp > maxSplitGroups {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("pinned member %q: group %d out of range 1..%d", w.Member, w.Group, maxSplitGroups))
			continue
		}
		if !groups.space(grp) {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf("pinned member %q: group %d is full", w.Member, grp))
			continue
		}
		groups.seat(grp)
		m.group = grp
		m.pinned = true
		if caps != nil {
			caps.commitClass(m.code)
		}
	}
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
func fillSlotsInGroups(slots []splitSlot, ms []splitMember, groups *groupTracker, req SplitRequest, caps *curatedCaps, rep *SplitReport) {
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
		var capVeto func(*splitMember, *splitSlot) bool
		if caps != nil {
			capVeto = caps.vetoSlot
		}
		m := pickForSlot(s, ms, gi, req, capVeto)
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
		if caps != nil {
			caps.commitClass(m.code)
			caps.commitPath(s.leaf.Path())
		}
	}
}

// curatedCaps tracks class/path/any placement counts against Wildcard.Max.
type curatedCaps struct {
	byClass map[ClassCode]int
	byPath  map[string]int
	anyCap  int // 0 = unlimited
	anyN    int
}

func newCuratedCaps(rules []Wildcard) *curatedCaps {
	c := &curatedCaps{byClass: map[ClassCode]int{}, byPath: map[string]int{}}
	for _, w := range rules {
		if w.Max <= 0 {
			continue
		}
		switch w.Kind {
		case "class":
			c.byClass[normClass(w.Value)] = w.Max
		case "path":
			c.byPath[strings.TrimSpace(w.Value)] = w.Max
		case "any":
			if c.anyCap == 0 {
				c.anyCap = w.Max
			}
		}
	}
	return c
}

func normClass(v string) ClassCode {
	return ClassCode(strings.ToLower(strings.TrimSpace(v)))
}

// vetoSlot reports whether seating m into s would exceed a curated cap
// (slot pass): class cap spent, path cap spent, or any-cap spent.
func (c *curatedCaps) vetoSlot(m *splitMember, s *splitSlot) bool {
	if c == nil {
		return false
	}
	// allowed reports whether seating m into s stays within every cap.
	allowed := func(m *splitMember, s *splitSlot) bool {
		if n, ok := c.byClass[m.code]; ok && n <= 0 {
			return false
		}
		if n, ok := c.byPath[s.leaf.Path()]; ok && n <= 0 {
			return false
		}
		return !(c.anyCap > 0 && c.anyN >= c.anyCap)
	}
	return allowed(m, s)
}

// exhaustedForSeating reports whether m may not be SEATED at all (used in
// the fill phase, where no slot context exists): their class cap or the any
// cap is spent.
func (c *curatedCaps) exhaustedForSeating(m *splitMember) bool {
	if c == nil {
		return false
	}
	if n, ok := c.byClass[m.code]; ok && n <= 0 {
		return true
	}
	return c.anyCap > 0 && c.anyN >= c.anyCap
}

// commitClass records one placement of m's class against the caps.
func (c *curatedCaps) commitClass(code ClassCode) {
	if c == nil {
		return
	}
	if n, ok := c.byClass[code]; ok {
		c.byClass[code] = n - 1
	}
	if c.anyCap > 0 {
		c.anyN++
	}
}

// commitPath records one placement of a role path against the caps.
func (c *curatedCaps) commitPath(path string) {
	if c == nil {
		return
	}
	if n, ok := c.byPath[path]; ok {
		c.byPath[path] = n - 1
	}
}

// pickForSlot selects the best eligible member for s when filling group gi.
// Priority: a member already seated in gi (curated pin) > unseated members
// whose live group is gi (RespectExistingGroups) > any other unseated
// member; ties break by name for determinism. capVeto may exclude members
// that would exceed a curated cap.
func pickForSlot(s *splitSlot, ms []splitMember, gi int, req SplitRequest, capVeto func(*splitMember, *splitSlot) bool) *splitMember {
	var best *splitMember
	for i := range ms {
		m := &ms[i]
		if !m.eligible(s) {
			continue
		}
		if capVeto != nil && !capVeto(m, s) {
			// vetoSlot returns allowed=false when a cap is exceeded — skip.
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

// scoreFor ranks candidates for a slot in group gi: seated-in-gi first
// (curated pin), then live-group affinity when re-balancing is off, then
// everyone else.
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

// seatFocusedClusters seats every classed member by class clusters (phase 1
// of the focused preference): classes in name order, members in name order.
// A cluster fills its group contiguously; a NEW class opens the next group
// once the current one already carries 3+ members and none of this class —
// small tails top up the previous group instead of wasting a seat.
func seatFocusedClusters(ms []splitMember, groups *groupTracker) {
	var codes []ClassCode
	byCode := map[ClassCode][]*splitMember{}
	for i := range ms {
		m := &ms[i]
		if m.code == "" {
			continue // classless members seat in the fill phase
		}
		if _, ok := byCode[m.code]; !ok {
			codes = append(codes, m.code)
		}
		byCode[m.code] = append(byCode[m.code], m)
	}
	sort.Slice(codes, func(i, j int) bool { return ClassNames[codes[i]] < ClassNames[codes[j]] })

	gi := 1
	for _, code := range codes {
		for _, m := range byCode[code] {
			if m.group != 0 {
				continue
			}
			if !groups.space(gi) {
				next := groups.firstOpen(gi + 1)
				if next == 0 {
					return
				}
				gi = next
			} else if groups.count[gi] >= 3 && classCountInGroup(ms, gi, code) == 0 {
				// Current group is carrying a different class's cluster —
				// open a fresh group for this class.
				next := groups.firstOpen(gi + 1)
				if next == 0 {
					return
				}
				gi = next
			}
			groups.seat(gi)
			m.group = gi
		}
	}
}

// classCountInGroup counts seated members of code in group grp.
func classCountInGroup(ms []splitMember, grp int, code ClassCode) int {
	n := 0
	for i := range ms {
		if ms[i].group == grp && ms[i].code == code {
			n++
		}
	}
	return n
}

// assignSlotsToSeated assigns comp slots to already-seated members (the
// focused path): each slot goes to the alphabetically-first unroled,
// class-eligible member seated in the earliest group that has one.
func assignSlotsToSeated(slots []splitSlot, ms []splitMember) {
	for si := range slots {
		s := &slots[si]
		for gi := 1; gi <= maxSplitGroups; gi++ {
			var best *splitMember
			for i := range ms {
				m := &ms[i]
				if m.group == gi && m.eligible(s) {
					if best == nil || m.name < best.name {
						best = m
					}
				}
			}
			if best != nil {
				best.slot = s
				s.filled = true
				s.member = best
				break
			}
		}
	}
}

// seatRemaining seats members the allocation left unseated: prefer their
// live group (when RespectExistingGroups), else the first group with space.
// Members excluded by a curated cap, or that fit nowhere, come back
// unassigned with the reason.
func seatRemaining(ms []splitMember, groups *groupTracker, respect bool, rep *SplitReport, caps *curatedCaps) {
	var order []*splitMember
	for i := range ms {
		if ms[i].group == 0 {
			order = append(order, &ms[i])
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].name < order[j].name })
	for _, m := range order {
		if caps != nil && caps.exhaustedForSeating(m) {
			rep.Unassigned = append(rep.Unassigned, Unassigned{
				Name: m.name, Class: m.code,
				Reason: fmt.Sprintf("excluded by curated cap (%s)", capDescribe(caps, m.code)),
			})
			continue
		}
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
		if caps != nil {
			// Fill seats consume caps too, so a capped class can't exceed its
			// Max through the back door.
			caps.commitClass(m.code)
		}
	}
}

// capDescribe names the cap that excluded a member, for the unassigned
// reason ("class clr ≤ 2"). The stored counter is what remains, so the
// original Max is recovered as remaining + placed; simpler to report the
// rule shape without arithmetic games.
func capDescribe(c *curatedCaps, code ClassCode) string {
	if _, ok := c.byClass[code]; ok {
		return fmt.Sprintf("class %s capped", code)
	}
	return "wildcard cap"
}

// assembleGroups turns seat state into the report's group list, carrying
// each member's slot metadata (role/level) onto their seat row.
func assembleGroups(ms []splitMember, size int) []ProposedGroup {
	byGroup := map[int][]Slot{}
	maxGrp := 0
	for i := range ms {
		m := &ms[i]
		if m.group == 0 {
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

// applyCuratedMinChecks reports curated Min rules the proposal could not
// meet (Min semantics are advisory: unmet Mins surface as warnings).
func applyCuratedMinChecks(rules []Wildcard, ms []splitMember, rep *SplitReport) {
	for _, w := range rules {
		if w.Min <= 0 || (w.Kind != "class" && w.Kind != "path" && w.Kind != "any") {
			continue
		}
		n := 0
		for i := range ms {
			m := &ms[i]
			if m.group == 0 {
				continue
			}
			switch w.Kind {
			case "class":
				if m.code == normClass(w.Value) {
					n++
				}
			case "path":
				if m.slot != nil && m.slot.leaf.Path() == strings.TrimSpace(w.Value) {
					n++
				}
			case "any":
				n++
			}
		}
		if n < w.Min {
			rep.Warnings = append(rep.Warnings, fmt.Sprintf(
				"wildcard %s=%q: wanted at least %d placement(s), proposal made %d",
				w.Kind, w.Value, w.Min, n))
		}
	}
}
