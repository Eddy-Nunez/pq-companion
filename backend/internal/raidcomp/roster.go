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
// before Get treats it as gone. Zeal emits MsgRaid every main-loop tick while
// the client is in a raid (verified live 2026-09-14: ~10/sec) — and emits
// NOTHING when the raid disbands (is_in_raid() simply goes false; there is no
// "raid over" message). So a gap this long with a connected pipe means the
// raid ENDED, or the client froze — either way the roster is no longer live.
// The window only needs to clear zoning hitches (a slow zone load pauses the
// ticks too); if we age out mid-zone, the next MsgRaid tick re-populates the
// roster within one tick and the fingerprint change re-broadcasts. 30s is
// comfortably above any observed zone load and a huge improvement over
// reporting a disbanded raid for 5 minutes (the pre-per-tick-era window).
const rosterStaleAfter = 30 * time.Second

// Roster keeps the latest live raid roster in memory. It is written from the
// Zeal pipe dispatch in cmd/server/main.go and read by the checker API. The
// roster is intentionally not persisted — it is live state, like the current
// target snapshot.
type Roster struct {
	mu   sync.RWMutex
	snap Snapshot
	seen bool
	// lastStale mirrors "the roster is currently aged out" for StaleChange.
	// Starts true (nothing populated), Set clears it, Clear and detected
	// staleness set it.
	lastStale bool
}

// NewRoster returns an empty roster.
func NewRoster() *Roster {
	return &Roster{lastStale: true}
}

// now is swappable so staleness tests don't sleep.
var now = time.Now

// Set replaces the latest snapshot and marks the roster as seen.
func (r *Roster) Set(zoneID int, zone string, members []Member) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = Snapshot{UpdatedAt: now().Unix(), ZoneID: zoneID, Zone: zone, Members: members}
	r.seen = true
	r.lastStale = false
}

// Clear drops the current snapshot. Called from the pipe's OnDisconnect
// handler so a stale roster doesn't keep reporting "in a raid" after Zeal
// goes away — same cleanup every other pipe-only consumer does there. Marks
// the roster stale so the StaleChange watcher doesn't double-report the
// same emptying (the disconnect path already broadcasts).
func (r *Roster) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snap = Snapshot{}
	r.seen = false
	r.lastStale = true
}

// Get returns the latest snapshot. seen is false until the first Set, or once
// the snapshot has gone stale (see rosterStaleAfter) — either way, the
// frontend shows a fallback source rather than a roster that's actually gone.
func (r *Roster) Get() (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.seen && now().Sub(time.Unix(r.snap.UpdatedAt, 0)) > rosterStaleAfter {
		return Snapshot{}, false
	}
	return r.snap, r.seen
}

// StaleChange reports — once per transition — that a previously-fresh roster
// has aged out (raid disbanded with no wire signal, or the client froze).
// The caller (a low-frequency watcher in main.go) pushes one raid.roster WS
// event on true so connected clients re-fetch and drop their "in raid" state
// immediately instead of waiting for a manual refresh. Roster start-up (never
// populated) and Clear (disconnect already broadcasts) are not changes.
func (r *Roster) StaleChange() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	stale := !r.seen || now().Sub(time.Unix(r.snap.UpdatedAt, 0)) > rosterStaleAfter
	if stale && !r.lastStale {
		r.lastStale = true
		return true
	}
	return false
}
