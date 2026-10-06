package blockbuff

import (
	"fmt"
	"sort"
)

// Status of one row in the reconciled view.
const (
	StatusApplied     = "applied"     // wanted and the server has it
	StatusNeedsBlock  = "needs_block" // wanted, server doesn't have it
	StatusNeedsAllow  = "needs_allow" // on the server but no longer wanted
	StatusUnconfirmed = "unconfirmed" // wanted; server list not seen yet
	StatusServerOnly  = "server_only" // seen on the server, not wanted; list incomplete so no #allowbuff
)

// Row is one block in the reconciled desired/observed view.
type Row struct {
	Entry
	Desired  bool   `json:"desired"`
	Observed bool   `json:"observed"`
	Status   string `json:"status"`
}

// Reconcile merges the wanted and server lists. synced says whether a full
// server listing has been seen: without one, absence from observed proves
// nothing, so wanted rows read "unconfirmed" and no #allowbuff is suggested.
func Reconcile(desired, observed []Entry, synced bool) []Row {
	byKey := map[Entry]*Row{}
	for _, e := range desired {
		byKey[e] = &Row{Entry: e, Desired: true}
	}
	for _, e := range observed {
		if r, ok := byKey[e]; ok {
			r.Observed = true
		} else {
			byKey[e] = &Row{Entry: e, Observed: true}
		}
	}
	rows := make([]Row, 0, len(byKey))
	for _, r := range byKey {
		switch {
		case r.Desired && r.Observed:
			r.Status = StatusApplied
		case r.Desired && synced:
			r.Status = StatusNeedsBlock
		case r.Desired:
			r.Status = StatusUnconfirmed
		case synced:
			r.Status = StatusNeedsAllow
		default:
			r.Status = StatusServerOnly
		}
		rows = append(rows, *r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Entry.Less(rows[j].Entry) })
	return rows
}

// Command renders the in-game command that adds (block) or removes a block.
func Command(e Entry, block bool) string {
	switch {
	case block && e.IfSpellID != 0:
		return fmt.Sprintf("#blockbuffif %d %d", e.SpellID, e.IfSpellID)
	case block:
		return fmt.Sprintf("#blockbuff %d", e.SpellID)
	case e.IfSpellID != 0:
		return fmt.Sprintf("#allowbuff %d %d", e.SpellID, e.IfSpellID)
	default:
		return fmt.Sprintf("#allowbuff %d", e.SpellID)
	}
}

// Commands lists the commands that bring the server in line with the wanted
// list. Re-blocking something already blocked is harmless server-side, so the
// "unconfirmed" rows are included.
func Commands(rows []Row) []string {
	out := []string{}
	for _, r := range rows {
		switch r.Status {
		case StatusNeedsBlock, StatusUnconfirmed:
			out = append(out, Command(r.Entry, true))
		case StatusNeedsAllow:
			out = append(out, Command(r.Entry, false))
		}
	}
	return out
}
