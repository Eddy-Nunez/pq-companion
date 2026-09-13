package character

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T) (*Store, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "user.db")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	c, err := s.Create("Testchar", 0, 1, 60)
	if err != nil {
		t.Fatalf("create char: %v", err)
	}
	return s, c.ID
}

func TestAddWishlistAppendsToGlobalOrder(t *testing.T) {
	s, charID := openTestStore(t)
	// Two different buckets — the second add should land at sort_order=1 even
	// though it's the first entry in its own bucket.
	first, err := s.AddWishlistEntry(charID, 1001, "Head", false)
	if err != nil {
		t.Fatalf("add #1: %v", err)
	}
	if first.SortOrder != 0 {
		t.Errorf("first entry sort_order = %d, want 0", first.SortOrder)
	}
	second, err := s.AddWishlistEntry(charID, 1002, "Feet", false)
	if err != nil {
		t.Fatalf("add #2: %v", err)
	}
	if second.SortOrder != 1 {
		t.Errorf("second entry sort_order = %d, want 1 (global, not per-bucket)", second.SortOrder)
	}
	third, err := s.AddWishlistEntry(charID, 1003, "Head", false)
	if err != nil {
		t.Fatalf("add #3: %v", err)
	}
	if third.SortOrder != 2 {
		t.Errorf("third entry sort_order = %d, want 2", third.SortOrder)
	}
}

func TestReorderWishlistRequiresFullList(t *testing.T) {
	s, charID := openTestStore(t)
	a, _ := s.AddWishlistEntry(charID, 1, "Head", false)
	b, _ := s.AddWishlistEntry(charID, 2, "Feet", false)
	c, _ := s.AddWishlistEntry(charID, 3, "Chest", false)

	// Reverse order.
	if err := s.ReorderWishlist(charID, []int{c.ID, b.ID, a.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	got, _ := s.ListWishlist(charID)
	wantOrder := []int{c.ID, b.ID, a.ID}
	for i, e := range got {
		if e.ID != wantOrder[i] {
			t.Errorf("position %d: got id %d want %d", i, e.ID, wantOrder[i])
		}
		if e.SortOrder != i {
			t.Errorf("position %d: sort_order %d want %d", i, e.SortOrder, i)
		}
	}

	// Missing one entry — must fail.
	if err := s.ReorderWishlist(charID, []int{a.ID, b.ID}); err == nil {
		t.Errorf("reorder with short list: expected error, got nil")
	}
	// Duplicate id — must fail.
	if err := s.ReorderWishlist(charID, []int{a.ID, a.ID, b.ID}); err == nil {
		t.Errorf("reorder with dup id: expected error, got nil")
	}
	// Unknown id — must fail.
	if err := s.ReorderWishlist(charID, []int{a.ID, b.ID, 99999}); err == nil {
		t.Errorf("reorder with foreign id: expected error, got nil")
	}
}

func TestSlotLayoutRoundTrip(t *testing.T) {
	s, charID := openTestStore(t)
	layout := []WishlistSlotLayout{
		{SlotBucket: "Chest", Position: 0, Collapsed: false},
		{SlotBucket: "Head", Position: 1, Collapsed: true},
		{SlotBucket: "Feet", Position: 2, Collapsed: false},
	}
	if err := s.ReplaceWishlistSlotLayout(charID, layout); err != nil {
		t.Fatalf("replace layout: %v", err)
	}
	got, err := s.ListWishlistSlotLayout(charID)
	if err != nil {
		t.Fatalf("list layout: %v", err)
	}
	if len(got) != len(layout) {
		t.Fatalf("layout length = %d, want %d", len(got), len(layout))
	}
	for i, l := range got {
		if l != layout[i] {
			t.Errorf("layout[%d] = %+v, want %+v", i, l, layout[i])
		}
	}

	// Replacing with a different set should drop missing buckets.
	if err := s.ReplaceWishlistSlotLayout(charID, []WishlistSlotLayout{
		{SlotBucket: "Head", Position: 0, Collapsed: true},
	}); err != nil {
		t.Fatalf("replace 2: %v", err)
	}
	got, _ = s.ListWishlistSlotLayout(charID)
	if len(got) != 1 || got[0].SlotBucket != "Head" {
		t.Errorf("after replace expected only Head, got %+v", got)
	}
}

// TestRemoveLootedWishlistEntries checks the auto-remove-on-loot query: a
// plain entry is deleted, a KeepAfterLoot entry (a recurring farm target) is
// left in place, and entries for other items/characters are untouched.
func TestRemoveLootedWishlistEntries(t *testing.T) {
	s, charID := openTestStore(t)
	plain, err := s.AddWishlistEntry(charID, 100, "Head", false)
	if err != nil {
		t.Fatalf("add plain: %v", err)
	}
	kept, err := s.AddWishlistEntry(charID, 200, "General", true)
	if err != nil {
		t.Fatalf("add kept: %v", err)
	}
	other, err := s.AddWishlistEntry(charID, 300, "Feet", false)
	if err != nil {
		t.Fatalf("add other: %v", err)
	}

	removed, err := s.RemoveLootedWishlistEntries(charID, plain.ItemID)
	if err != nil {
		t.Fatalf("remove looted (plain): %v", err)
	}
	if len(removed) != 1 || removed[0].ID != plain.ID {
		t.Fatalf("removed = %+v, want just the plain entry", removed)
	}

	removed, err = s.RemoveLootedWishlistEntries(charID, kept.ItemID)
	if err != nil {
		t.Fatalf("remove looted (kept): %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %+v, want none — KeepAfterLoot entry must survive", removed)
	}

	got, err := s.ListWishlist(charID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("remaining entries = %d, want 2 (kept + other)", len(got))
	}
	ids := map[int]bool{kept.ID: false, other.ID: false}
	for _, e := range got {
		if _, ok := ids[e.ID]; !ok {
			t.Errorf("unexpected surviving entry %+v", e)
		}
		ids[e.ID] = true
	}
	for id, seen := range ids {
		if !seen {
			t.Errorf("expected entry %d to survive, it did not", id)
		}
	}
}

// TestSetWishlistKeepAfterLoot checks the toggle used to flag/unflag an
// entry as a recurring target after it's already been added.
func TestSetWishlistKeepAfterLoot(t *testing.T) {
	s, charID := openTestStore(t)
	e, err := s.AddWishlistEntry(charID, 100, "Head", false)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if e.KeepAfterLoot {
		t.Fatalf("new entry KeepAfterLoot = true, want false")
	}
	if err := s.SetWishlistKeepAfterLoot(charID, e.ID, true); err != nil {
		t.Fatalf("set keep: %v", err)
	}
	got, err := s.ListWishlist(charID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || !got[0].KeepAfterLoot {
		t.Fatalf("after toggle: %+v, want KeepAfterLoot=true", got)
	}
	// Scoped to characterID — another character's id must not touch this row.
	if err := s.SetWishlistKeepAfterLoot(charID+999, e.ID, false); err != nil {
		t.Fatalf("set keep (wrong char): %v", err)
	}
	got, _ = s.ListWishlist(charID)
	if len(got) != 1 || !got[0].KeepAfterLoot {
		t.Fatalf("cross-character set mutated the entry: %+v", got)
	}
}

// TestBackfillGlobalOrder simulates an upgrade from the old per-bucket
// sort_order semantics: insert wishlist rows directly using per-bucket
// positions before the slot-layout table exists, then run migrate again
// (simulated by reopening the store) and confirm sort_order is now a
// single per-character global sequence in canonical bucket order.
func TestBackfillGlobalOrder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.db")
	// First open: schema is created. We then drop the slot_layout table to
	// reproduce the "old database" shape, and manually insert per-bucket
	// sort_order rows.
	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open #1: %v", err)
	}
	c, err := s.Create("Backfillchar", 0, 1, 60)
	if err != nil {
		t.Fatalf("create char: %v", err)
	}
	if _, err := s.db.Exec(`DROP TABLE character_wishlist_slot_layout`); err != nil {
		t.Fatalf("drop layout: %v", err)
	}
	// Insert with overlapping per-bucket sort_order values — the old semantics.
	mustInsert := func(itemID int, bucket string, sortOrder int) {
		if _, err := s.db.Exec(
			`INSERT INTO character_wishlist (character_id, item_id, slot_bucket, sort_order, created_at)
			 VALUES (?, ?, ?, ?, 0)`,
			c.ID, itemID, bucket, sortOrder,
		); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	mustInsert(100, "Feet", 0)
	mustInsert(101, "Head", 0)
	mustInsert(102, "Head", 1)
	mustInsert(103, "Chest", 0)
	s.Close()

	// Reopen — migrate should detect the missing layout table and renumber.
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("open #2: %v", err)
	}
	defer s2.Close()

	entries, err := s2.ListWishlist(c.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}
	// Canonical order is Head, Chest, Feet (Head index 2, Chest 14, Feet 16).
	// Within Head, prior sort_order ordering: 101 then 102.
	wantItems := []int{101, 102, 103, 100}
	for i, e := range entries {
		if e.ItemID != wantItems[i] {
			t.Errorf("position %d: item %d want %d (canonical-bucket-order backfill)",
				i, e.ItemID, wantItems[i])
		}
		if e.SortOrder != i {
			t.Errorf("position %d: sort_order %d want %d", i, e.SortOrder, i)
		}
	}
}
