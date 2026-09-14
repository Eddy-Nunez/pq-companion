package raidcomp

import (
	"sync"
	"time"
)

// Member is one live raid participant captured from the Zeal raid envelope
// (MsgRaid type 5). Code is resolved at capture time from the Zeal 1-indexed
// class id so the checker does not need the live pipe again.
type Member struct {
	Name  string    `json:"name"`
	Level int       `json:"level"`
	Class int       `json:"class"` // Zeal 1-indexed class id (0 = unknown)
	Code  ClassCode `json:"code,omitempty"`
	Group string    `json:"group,omitempty"`
	Rank  string    `json:"rank,omitempty"`
}

// Snapshot is the latest raid roster seen by the Zeal pipe. ZoneID is the
// EQ zoneidnumber reported by the MsgPlayer envelope; Zone is the resolved
// short name.
type Snapshot struct {
	UpdatedAt int64    `json:"updated_at,omitempty"`
	ZoneID    int      `json:"zone_id,omitempty"`
	Zone      string   `json:"zone,omitempty"`
	Members   []Member `json:"members"`
}

// rosterStaleAfter is how long a MsgRaid snapshot is trusted with no refresh
// before Get treats it as gone. Zeal re-sends MsgRaid on every roster change
// and periodically besides, so a gap this long means the pipe died without
// a clean disconnect (or Zeal itself hung) — same failure shape documented
// for the NPC overlay pipe. Clear on OnDisconnect handles the clean case;
// this is the belt-and-suspenders fallback for the unclean one.
const rosterStaleAfter = 5 * time.Minute

// Roster keeps the latest live raid roster in memory. It is written from the
// Zeal pipe dispatch in cmd/server/main.go and read by the checker API. The
// roster is intentionally not persisted — it is live state, like the current
// target snapshot.
type Roster struct {
	mu   sync.RWMutex
	snap Snapshot
	seen bool
}

// NewRoster returns an empty roster.
func NewRoster() *Roster {
	return &Roster{}
}

// Set replaces the latest snapshot and marks the roster as seen.
func (r *Roster) Set(zoneID int, zone string, members []Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = Snapshot{UpdatedAt: time.Now().Unix(), ZoneID: zoneID, Zone: zone, Members: members}
	r.seen = true
}

// Clear drops the current snapshot. Called from the pipe's OnDisconnect
// handler so a stale roster doesn't keep reporting "in a raid" after Zeal
// goes away — same cleanup every other pipe-only consumer does there.
func (r *Roster) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = Snapshot{}
	r.seen = false
}

// Get returns the latest snapshot. seen is false until the first Set, or once
// the snapshot has gone stale (see rosterStaleAfter) — either way, the
// frontend shows a fallback source rather than a roster that's actually gone.
func (r *Roster) Get() (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.seen && time.Since(time.Unix(r.snap.UpdatedAt, 0)) > rosterStaleAfter {
		return Snapshot{}, false
	}
	return r.snap, r.seen
}
