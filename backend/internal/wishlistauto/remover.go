// Package wishlistauto clears a wishlist entry the moment the character
// actually loots the item, instead of leaving the player to notice and
// delete it by hand. It watches the same raw log line stream every other
// line-consumer sees, but — unlike wishlistwatch, which alerts on any
// mention of a wishlisted item's name anywhere in the log (raid officer
// calling a drop, someone selling one in chat) — it only acts on an
// unambiguous "--You have looted a X.--" line for the active character, via
// the loot package's already-anchored self-loot regex. That precision is
// the point: a chat mention isn't proof the player now owns the item, so it
// must never trigger a removal.
//
// Recurring farm targets (tradeskill materials, quest turn-in components —
// anything the player wants more than one of) opt out per-entry via
// character.WishlistEntry.KeepAfterLoot; see AddWishlistEntry's default and
// SetWishlistKeepAfterLoot.
package wishlistauto

import (
	"log/slog"
	"strings"

	"github.com/jasonsoprovich/pq-companion/backend/internal/loot"
)

// CharacterInfo is the minimal character identity the remover needs to turn
// an active-character name into an id.
type CharacterInfo struct {
	ID   int
	Name string
}

// WishlistEntry is one character's wishlisted item, as seen by the remover —
// enough to match a loot line's item name and know whether removal applies.
type WishlistEntry struct {
	EntryID       int
	ItemID        int
	ItemName      string
	KeepAfterLoot bool
}

// Removed is one entry the remover cleared, for the caller's notification
// (WebSocket broadcast, toast, etc.).
type Removed struct {
	CharacterID   int
	CharacterName string
	Entry         WishlistEntry
}

// Remover watches raw log lines for the active character's own loot line and
// deletes matching, eligible wishlist entries.
type Remover struct {
	activeChar   func() string
	listChars    func() ([]CharacterInfo, error)
	listWishlist func(characterID int) ([]WishlistEntry, error)
	removeEntry  func(characterID, entryID int) error
	onRemoved    func(Removed)
}

// NewRemover constructs a Remover. activeChar returns the current in-game
// character ("" if unknown); listChars/listWishlist/removeEntry are thin
// wrappers over character.Store, kept as closures (matching wishlistwatch)
// so this package doesn't need to import that store's concrete type.
func NewRemover(
	activeChar func() string,
	listChars func() ([]CharacterInfo, error),
	listWishlist func(characterID int) ([]WishlistEntry, error),
	removeEntry func(characterID, entryID int) error,
) *Remover {
	return &Remover{
		activeChar:   activeChar,
		listChars:    listChars,
		listWishlist: listWishlist,
		removeEntry:  removeEntry,
	}
}

// SetOnRemoved registers a callback fired once per entry actually removed.
func (r *Remover) SetOnRemoved(fn func(Removed)) {
	r.onRemoved = fn
}

// HandleLine checks a raw log line for the active character's own self-loot
// event and removes any matching, eligible wishlist entry. Lines that merely
// mention an item's name — chat, a raid drop call — never match
// loot.ParseLoot's anchored self-loot pattern, so they can't cause a
// removal; that's wishlistwatch's job, not this one.
func (r *Remover) HandleLine(msg string) {
	p, ok := loot.ParseLoot(strings.TrimRight(msg, "\r\n"))
	if !ok || !p.Self {
		return
	}
	active := ""
	if r.activeChar != nil {
		active = r.activeChar()
	}
	if active == "" {
		return
	}
	chars, err := r.listChars()
	if err != nil {
		slog.Error("wishlistauto: list characters failed", "err", err)
		return
	}
	var charID int
	var charName string
	found := false
	for _, c := range chars {
		if strings.EqualFold(c.Name, active) {
			charID, charName = c.ID, c.Name
			found = true
			break
		}
	}
	if !found {
		return
	}

	entries, err := r.listWishlist(charID)
	if err != nil {
		slog.Error("wishlistauto: list wishlist failed", "character", charName, "err", err)
		return
	}
	lootedName := strings.ToLower(p.Item)
	for _, e := range entries {
		if e.KeepAfterLoot {
			continue
		}
		if strings.ToLower(e.ItemName) != lootedName {
			continue
		}
		if err := r.removeEntry(charID, e.EntryID); err != nil {
			slog.Error("wishlistauto: remove entry failed", "character", charName, "item", e.ItemName, "err", err)
			continue
		}
		if r.onRemoved != nil {
			r.onRemoved(Removed{CharacterID: charID, CharacterName: charName, Entry: e})
		}
	}
}
