package wishlistauto

import "testing"

// testWorld is a small in-memory character/wishlist fixture the remover's
// closures read from, so tests don't need a real character.Store.
type testWorld struct {
	chars     []CharacterInfo
	wishlists map[int][]WishlistEntry
	removed   []struct {
		characterID int
		entryID     int
	}
}

func newTestRemover(active string, w *testWorld) (*Remover, *[]Removed) {
	var got []Removed
	r := NewRemover(
		func() string { return active },
		func() ([]CharacterInfo, error) { return w.chars, nil },
		func(characterID int) ([]WishlistEntry, error) { return w.wishlists[characterID], nil },
		func(characterID, entryID int) error {
			w.wishlists[characterID] = removeEntry(w.wishlists[characterID], entryID)
			return nil
		},
	)
	r.SetOnRemoved(func(rm Removed) { got = append(got, rm) })
	return r, &got
}

func removeEntry(entries []WishlistEntry, entryID int) []WishlistEntry {
	out := entries[:0]
	for _, e := range entries {
		if e.EntryID != entryID {
			out = append(out, e)
		}
	}
	return out
}

func TestHandleLine_SelfLootRemovesEntry(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "Robe of the Lost Circle"}},
		},
	}
	r, got := newTestRemover("Khura", world)

	r.HandleLine("--You have looted a Robe of the Lost Circle.--")

	if len(*got) != 1 {
		t.Fatalf("removed = %d entries, want 1", len(*got))
	}
	if (*got)[0].Entry.EntryID != 10 || (*got)[0].CharacterID != 1 {
		t.Errorf("removed = %+v, want entry 10 for character 1", (*got)[0])
	}
	if len(world.wishlists[1]) != 0 {
		t.Errorf("wishlist still has %d entries, want 0", len(world.wishlists[1]))
	}
}

func TestHandleLine_KeepAfterLootSurvives(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "Bone Chips", KeepAfterLoot: true}},
		},
	}
	r, got := newTestRemover("Khura", world)

	r.HandleLine("--You have looted a Bone Chips.--")

	if len(*got) != 0 {
		t.Fatalf("removed = %d entries, want 0 (KeepAfterLoot must survive)", len(*got))
	}
	if len(world.wishlists[1]) != 1 {
		t.Errorf("wishlist has %d entries, want 1 (untouched)", len(world.wishlists[1]))
	}
}

func TestHandleLine_OtherCharacterLootIgnored(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "Robe of the Lost Circle"}},
		},
	}
	r, got := newTestRemover("Khura", world)

	// Someone else's loot line must never remove Khura's entry — only an
	// unambiguous self-loot line for the active character does.
	r.HandleLine("--Grokii has looted a Robe of the Lost Circle.--")

	if len(*got) != 0 {
		t.Fatalf("removed = %d entries, want 0 (not a self-loot line)", len(*got))
	}
}

func TestHandleLine_MereMentionIgnored(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "Robe of the Lost Circle"}},
		},
	}
	r, got := newTestRemover("Khura", world)

	// A raid officer calling the drop, or a chat mention, is not proof of
	// possession — must not match loot.ParseLoot's anchored self-loot regex.
	r.HandleLine("Robe of the Lost Circle is up for bid, whisper Khura.")

	if len(*got) != 0 {
		t.Fatalf("removed = %d entries, want 0 (mention only, not a loot line)", len(*got))
	}
}

func TestHandleLine_NoActiveCharacterNoop(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "Robe of the Lost Circle"}},
		},
	}
	r, got := newTestRemover("", world)

	r.HandleLine("--You have looted a Robe of the Lost Circle.--")

	if len(*got) != 0 {
		t.Fatalf("removed = %d entries, want 0 (no active character known)", len(*got))
	}
}

func TestHandleLine_MatchIsCaseInsensitive(t *testing.T) {
	world := &testWorld{
		chars: []CharacterInfo{{ID: 1, Name: "Khura"}},
		wishlists: map[int][]WishlistEntry{
			1: {{EntryID: 10, ItemID: 100, ItemName: "robe of the lost circle"}},
		},
	}
	r, got := newTestRemover("Khura", world)

	r.HandleLine("--You have looted a Robe Of The Lost Circle.--")

	if len(*got) != 1 {
		t.Fatalf("removed = %d entries, want 1 (case-insensitive match)", len(*got))
	}
}
