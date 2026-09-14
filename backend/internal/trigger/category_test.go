package trigger

import (
	"errors"
	"testing"
	"time"
)

// makeTrigger builds a minimal log-source trigger in the given category.
func makeTrigger(name, packName string) *Trigger {
	return &Trigger{
		ID:        name + ":" + packName,
		Name:      name,
		Enabled:   true,
		Pattern:   `^` + name + `$`,
		PackName:  packName,
		CreatedAt: time.Unix(0, 0).UTC(),
		Actions:   []Action{{Type: ActionOverlayText, Text: name}},
	}
}

// catByName indexes a category slice for assertions. Panics-free helper —
// callers check ok before indexing.
func catByName(cats []Category) map[string]Category {
	m := make(map[string]Category, len(cats))
	for _, c := range cats {
		m[c.Name] = c
	}
	return m
}

func mustList(t *testing.T, s *Store) []Category {
	t.Helper()
	cats, err := s.ListCategories()
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	return cats
}

func TestCreateCategory_PersistsEmpty(t *testing.T) {
	s := openTestStore(t)

	cat, err := s.CreateCategory("My Raids", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if cat.ID == "" {
		t.Fatal("expected a generated id")
	}
	if !cat.Custom || cat.IsBuiltin || cat.Count != 0 || cat.Name != "My Raids" || cat.ParentID != "" {
		t.Fatalf("unexpected category: %+v", cat)
	}

	got := catByName(mustList(t, s))
	c, ok := got["My Raids"]
	if !ok {
		t.Fatal("empty custom category not listed")
	}
	if c.Count != 0 || !c.Custom || c.IsBuiltin || c.ID != cat.ID {
		t.Fatalf("unexpected listed category: %+v", c)
	}
}

func TestCreateCategory_TrimsName(t *testing.T) {
	s := openTestStore(t)
	cat, err := s.CreateCategory("  Spaced  ", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if cat.Name != "Spaced" {
		t.Fatalf("name not trimmed: %q", cat.Name)
	}
}

func TestCreateCategory_Rejects(t *testing.T) {
	s := openTestStore(t)

	if _, err := s.CreateCategory("   ", ""); !errors.Is(err, ErrCategoryNameEmpty) {
		t.Fatalf("empty name: want ErrCategoryNameEmpty, got %v", err)
	}
	if _, err := s.CreateCategory(reservedUncategorized, ""); !errors.Is(err, ErrCategoryReserved) {
		t.Fatalf("reserved name: want ErrCategoryReserved, got %v", err)
	}

	if _, err := s.CreateCategory("Dupe", ""); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := s.CreateCategory("Dupe", ""); !errors.Is(err, ErrCategoryExists) {
		t.Fatalf("duplicate: want ErrCategoryExists, got %v", err)
	}

	// A name already in use by a trigger's pack (materialized into a root
	// category row on insert — see resolveCategoryLink) collides too.
	if err := s.Insert(makeTrigger("t1", "Imported Pack")); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := s.CreateCategory("Imported Pack", ""); !errors.Is(err, ErrCategoryExists) {
		t.Fatalf("in-use name: want ErrCategoryExists, got %v", err)
	}

	// A built-in pack name is reserved at the top level even when not
	// installed.
	packs := AllPacks()
	if len(packs) == 0 {
		t.Skip("no built-in packs to test reservation")
	}
	if _, err := s.CreateCategory(packs[0].PackName, ""); !errors.Is(err, ErrCategoryExists) {
		t.Fatalf("built-in name: want ErrCategoryExists, got %v", err)
	}
}

func TestCreateCategory_Nesting(t *testing.T) {
	s := openTestStore(t)

	parent, err := s.CreateCategory("Raid Triggers", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := s.CreateCategory("Ring War", parent.ID)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	if child.ParentID != parent.ID {
		t.Fatalf("child.ParentID = %q, want %q", child.ParentID, parent.ID)
	}

	// A child cannot itself have children (depth cap).
	if _, err := s.CreateCategory("Too Deep", child.ID); !errors.Is(err, ErrCategoryDepth) {
		t.Fatalf("grandchild: want ErrCategoryDepth, got %v", err)
	}

	// A nonexistent parent is rejected.
	if _, err := s.CreateCategory("Orphan", "does-not-exist"); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("bad parent: want ErrCategoryNotFound, got %v", err)
	}
}

func TestCreateCategory_SiblingsUnderDifferentParentsCanShareAName(t *testing.T) {
	s := openTestStore(t)

	raidA, err := s.CreateCategory("Raid A", "")
	if err != nil {
		t.Fatalf("create Raid A: %v", err)
	}
	raidB, err := s.CreateCategory("Raid B", "")
	if err != nil {
		t.Fatalf("create Raid B: %v", err)
	}
	if _, err := s.CreateCategory("Trash", raidA.ID); err != nil {
		t.Fatalf("create Raid A/Trash: %v", err)
	}
	// Same name, different parent — must succeed.
	if _, err := s.CreateCategory("Trash", raidB.ID); err != nil {
		t.Fatalf("create Raid B/Trash should not collide: %v", err)
	}
	// But a duplicate under the SAME parent is still rejected.
	if _, err := s.CreateCategory("Trash", raidA.ID); !errors.Is(err, ErrCategoryExists) {
		t.Fatalf("duplicate sibling: want ErrCategoryExists, got %v", err)
	}
}

func TestListCategories_CountsAndFlags(t *testing.T) {
	s := openTestStore(t)

	if err := s.Insert(makeTrigger("a", "Custom A")); err != nil {
		t.Fatalf("Insert a: %v", err)
	}
	if err := s.Insert(makeTrigger("b", "Custom A")); err != nil {
		t.Fatalf("Insert b: %v", err)
	}
	if err := s.Insert(makeTrigger("u", "")); err != nil { // Uncategorized
		t.Fatalf("Insert u: %v", err)
	}
	if _, err := s.CreateCategory("Empty One", ""); err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}

	got := catByName(mustList(t, s))

	// Uncategorized (empty pack_name/category_id) is never listed as a category.
	if _, ok := got[""]; ok {
		t.Fatal("empty pack_name should not be a category")
	}
	if _, ok := got[reservedUncategorized]; ok {
		t.Fatal("reserved sentinel should not be a category")
	}
	if c := got["Custom A"]; c.Count != 2 || c.IsBuiltin || c.Custom {
		t.Fatalf("Custom A: %+v (want count=2, not builtin, not custom-row)", c)
	}
	if c := got["Empty One"]; c.Count != 0 || !c.Custom {
		t.Fatalf("Empty One: %+v (want count=0, custom-row)", c)
	}
}

func TestListCategories_RollsUpChildCounts(t *testing.T) {
	s := openTestStore(t)

	parent, err := s.CreateCategory("Raid Triggers", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := s.CreateCategory("Ring War", parent.ID)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}

	direct := makeTrigger("direct", "")
	direct.CategoryID = parent.ID
	if err := s.Insert(direct); err != nil {
		t.Fatalf("insert direct: %v", err)
	}
	nested := makeTrigger("nested", "")
	nested.CategoryID = child.ID
	if err := s.Insert(nested); err != nil {
		t.Fatalf("insert nested: %v", err)
	}

	got := catByName(mustList(t, s))
	if c := got["Raid Triggers"]; c.Count != 2 {
		t.Fatalf("parent count = %d, want 2 (1 direct + 1 rolled up from child)", c.Count)
	}
	if c := got["Ring War"]; c.Count != 1 {
		t.Fatalf("child count = %d, want 1", c.Count)
	}
}

func TestListByCategory_FiltersAndOrders(t *testing.T) {
	s := openTestStore(t)

	if err := s.Insert(makeTrigger("a", "Raid Triggers")); err != nil {
		t.Fatalf("Insert a: %v", err)
	}
	if err := s.Insert(makeTrigger("b", "Raid Triggers")); err != nil {
		t.Fatalf("Insert b: %v", err)
	}
	if err := s.Insert(makeTrigger("other", "Other Category")); err != nil {
		t.Fatalf("Insert other: %v", err)
	}
	if err := s.Insert(makeTrigger("u", "")); err != nil { // Uncategorized
		t.Fatalf("Insert u: %v", err)
	}

	// Manually curated order should be preserved in the export.
	if err := s.ReorderTriggers([]string{"b:Raid Triggers", "a:Raid Triggers"}); err != nil {
		t.Fatalf("ReorderTriggers: %v", err)
	}

	got, err := s.ListByCategory("Raid Triggers")
	if err != nil {
		t.Fatalf("ListByCategory: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 triggers, got %d", len(got))
	}
	if got[0].Name != "b" || got[1].Name != "a" {
		t.Fatalf("expected sort_order to be respected, got %q then %q", got[0].Name, got[1].Name)
	}

	empty, err := s.ListByCategory("Nonexistent")
	if err != nil {
		t.Fatalf("ListByCategory (missing): %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected no triggers for a nonexistent category, got %d", len(empty))
	}
}

func TestListCategories_BuiltinFlag(t *testing.T) {
	s := openTestStore(t)
	packs := AllPacks()
	if len(packs) == 0 {
		t.Skip("no built-in packs")
	}
	builtin := packs[0].PackName
	if err := s.Insert(makeTrigger("x", builtin)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	c, ok := catByName(mustList(t, s))[builtin]
	if !ok {
		t.Fatalf("built-in pack %q not listed", builtin)
	}
	if !c.IsBuiltin {
		t.Fatalf("built-in pack %q not flagged IsBuiltin", builtin)
	}
}

func TestRenameCategory_NoLongerCascadesByName(t *testing.T) {
	s := openTestStore(t)
	cat, err := s.CreateCategory("Old Name", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	a := makeTrigger("a", "")
	a.CategoryID = cat.ID
	if err := s.Insert(a); err != nil {
		t.Fatalf("Insert a: %v", err)
	}
	b := makeTrigger("b", "")
	b.CategoryID = cat.ID
	if err := s.Insert(b); err != nil {
		t.Fatalf("Insert b: %v", err)
	}
	// An unrelated category sharing no relationship with "Old Name" — proves
	// rename touches only rows carrying this category's id, not a name match.
	other, err := s.CreateCategory("Untouched", "")
	if err != nil {
		t.Fatalf("CreateCategory Untouched: %v", err)
	}
	c := makeTrigger("c", "")
	c.CategoryID = other.ID
	if err := s.Insert(c); err != nil {
		t.Fatalf("Insert c: %v", err)
	}

	if err := s.RenameCategory(cat.ID, "New Name"); err != nil {
		t.Fatalf("RenameCategory: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, tr := range list {
		switch tr.Name {
		case "a", "b":
			if tr.CategoryID != cat.ID || tr.PackName != "New Name" {
				t.Fatalf("trigger %q not renamed: category_id=%q pack_name=%q", tr.Name, tr.CategoryID, tr.PackName)
			}
		case "c":
			if tr.CategoryID != other.ID || tr.PackName != "Untouched" {
				t.Fatalf("unrelated trigger %q was touched by rename: category_id=%q pack_name=%q", tr.Name, tr.CategoryID, tr.PackName)
			}
		}
	}
}

func TestRenameCategory_EmptyCustomRow(t *testing.T) {
	s := openTestStore(t)
	cat, err := s.CreateCategory("Before", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if err := s.RenameCategory(cat.ID, "After"); err != nil {
		t.Fatalf("RenameCategory: %v", err)
	}
	got := catByName(mustList(t, s))
	if _, ok := got["Before"]; ok {
		t.Fatal("old name still present after rename")
	}
	if c, ok := got["After"]; !ok || !c.Custom || c.ID != cat.ID {
		t.Fatalf("renamed empty category not persisted: %+v", c)
	}
}

func TestRenameCategory_Rejects(t *testing.T) {
	s := openTestStore(t)
	src, err := s.CreateCategory("Source", "")
	if err != nil {
		t.Fatalf("CreateCategory Source: %v", err)
	}
	if _, err := s.CreateCategory("Target", ""); err != nil {
		t.Fatalf("CreateCategory Target: %v", err)
	}

	if err := s.RenameCategory("does-not-exist", "Whatever"); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("missing source: want ErrCategoryNotFound, got %v", err)
	}
	if err := s.RenameCategory(src.ID, "Target"); !errors.Is(err, ErrCategoryExists) {
		t.Fatalf("collision: want ErrCategoryExists, got %v", err)
	}
	if err := s.RenameCategory(src.ID, "  "); !errors.Is(err, ErrCategoryNameEmpty) {
		t.Fatalf("empty new name: want ErrCategoryNameEmpty, got %v", err)
	}

	// Built-in packs can't be renamed here.
	packs := AllPacks()
	if len(packs) > 0 {
		if err := s.Insert(makeTrigger("x", packs[0].PackName)); err != nil {
			t.Fatalf("Insert builtin trigger: %v", err)
		}
		builtin, ok := catByName(mustList(t, s))[packs[0].PackName]
		if !ok {
			t.Fatalf("built-in category %q not materialized", packs[0].PackName)
		}
		if err := s.RenameCategory(builtin.ID, "Hijacked"); !errors.Is(err, ErrCategoryBuiltin) {
			t.Fatalf("rename builtin: want ErrCategoryBuiltin, got %v", err)
		}
	}
}

func TestDeleteCategory_OrphansTriggers(t *testing.T) {
	s := openTestStore(t)
	cat, err := s.CreateCategory("Doomed", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	a := makeTrigger("a", "")
	a.CategoryID = cat.ID
	if err := s.Insert(a); err != nil {
		t.Fatalf("Insert a: %v", err)
	}
	b := makeTrigger("b", "")
	b.CategoryID = cat.ID
	if err := s.Insert(b); err != nil {
		t.Fatalf("Insert b: %v", err)
	}

	if err := s.DeleteCategory(cat.ID, false); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	// Triggers survive, now Uncategorized.
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected triggers to survive, got %d", len(list))
	}
	for _, tr := range list {
		if tr.PackName != "" || tr.CategoryID != "" {
			t.Fatalf("trigger %q not orphaned: pack=%q category_id=%q", tr.Name, tr.PackName, tr.CategoryID)
		}
	}
	// Category row gone.
	if _, ok := catByName(mustList(t, s))["Doomed"]; ok {
		t.Fatal("category row survived delete")
	}
}

func TestDeleteCategory_PromotesChildrenAndLeavesTheirTriggers(t *testing.T) {
	s := openTestStore(t)
	parent, err := s.CreateCategory("Raid Triggers", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := s.CreateCategory("Ring War", parent.ID)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	direct := makeTrigger("direct", "")
	direct.CategoryID = parent.ID
	if err := s.Insert(direct); err != nil {
		t.Fatalf("insert direct: %v", err)
	}
	nested := makeTrigger("nested", "")
	nested.CategoryID = child.ID
	if err := s.Insert(nested); err != nil {
		t.Fatalf("insert nested: %v", err)
	}

	if err := s.DeleteCategory(parent.ID, false); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	// Parent's own trigger orphans to Uncategorized; the child's trigger is
	// untouched, and the child itself is promoted to top-level.
	got, err := s.Get(direct.ID)
	if err != nil {
		t.Fatalf("Get direct: %v", err)
	}
	if got.CategoryID != "" {
		t.Fatalf("parent's own trigger should be orphaned, category_id=%q", got.CategoryID)
	}
	nestedAfter, err := s.Get(nested.ID)
	if err != nil {
		t.Fatalf("Get nested: %v", err)
	}
	if nestedAfter.CategoryID != child.ID {
		t.Fatalf("child's trigger should be untouched, category_id=%q", nestedAfter.CategoryID)
	}
	promoted, ok := catByName(mustList(t, s))["Ring War"]
	if !ok {
		t.Fatal("child category should survive parent deletion")
	}
	if promoted.ParentID != "" {
		t.Fatalf("child should be promoted to top-level, ParentID=%q", promoted.ParentID)
	}
}

func TestDeleteCategory_DeleteTriggersCascadesToChildren(t *testing.T) {
	s := openTestStore(t)
	parent, err := s.CreateCategory("Raid Triggers", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := s.CreateCategory("Ring War", parent.ID)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	direct := makeTrigger("direct", "")
	direct.CategoryID = parent.ID
	if err := s.Insert(direct); err != nil {
		t.Fatalf("insert direct: %v", err)
	}
	nested := makeTrigger("nested", "")
	nested.CategoryID = child.ID
	if err := s.Insert(nested); err != nil {
		t.Fatalf("insert nested: %v", err)
	}
	bystander := makeTrigger("u", "")
	if err := s.Insert(bystander); err != nil {
		t.Fatalf("insert bystander: %v", err)
	}

	if err := s.DeleteCategory(parent.ID, true); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "u" {
		t.Fatalf("expected only the bystander to survive, got %d: %+v", len(list), list)
	}
}

func TestDeleteCategory_Rejects(t *testing.T) {
	s := openTestStore(t)
	if err := s.DeleteCategory("does-not-exist", false); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("missing: want ErrCategoryNotFound, got %v", err)
	}
	packs := AllPacks()
	if len(packs) > 0 {
		if err := s.Insert(makeTrigger("x", packs[0].PackName)); err != nil {
			t.Fatalf("Insert builtin trigger: %v", err)
		}
		builtin, ok := catByName(mustList(t, s))[packs[0].PackName]
		if !ok {
			t.Fatalf("built-in category %q not materialized", packs[0].PackName)
		}
		if err := s.DeleteCategory(builtin.ID, false); !errors.Is(err, ErrCategoryBuiltin) {
			t.Fatalf("delete builtin: want ErrCategoryBuiltin, got %v", err)
		}
	}
}

func TestCreateCategory_AppendsSortOrder(t *testing.T) {
	s := openTestStore(t)
	a, err := s.CreateCategory("Alpha", "")
	if err != nil {
		t.Fatalf("CreateCategory Alpha: %v", err)
	}
	b, err := s.CreateCategory("Bravo", "")
	if err != nil {
		t.Fatalf("CreateCategory Bravo: %v", err)
	}
	if !(a.SortOrder < b.SortOrder) {
		t.Fatalf("expected Alpha(%d) to sort before Bravo(%d)", a.SortOrder, b.SortOrder)
	}
}

func TestReorderCategories(t *testing.T) {
	s := openTestStore(t)
	ids := map[string]string{}
	for _, n := range []string{"Alpha", "Bravo", "Charlie"} {
		cat, err := s.CreateCategory(n, "")
		if err != nil {
			t.Fatalf("CreateCategory %s: %v", n, err)
		}
		ids[n] = cat.ID
	}
	// Reverse the order.
	items := []CategoryPlacement{
		{ID: ids["Charlie"], SortOrder: 0},
		{ID: ids["Bravo"], SortOrder: 1},
		{ID: ids["Alpha"], SortOrder: 2},
	}
	if err := s.ReorderCategories(items); err != nil {
		t.Fatalf("ReorderCategories: %v", err)
	}
	got := []string{}
	for _, c := range mustList(t, s) {
		got = append(got, c.Name)
	}
	want := []string{"Charlie", "Bravo", "Alpha"}
	if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestReorderCategories_Reparents(t *testing.T) {
	s := openTestStore(t)
	raidA, err := s.CreateCategory("Raid A", "")
	if err != nil {
		t.Fatalf("create Raid A: %v", err)
	}
	raidB, err := s.CreateCategory("Raid B", "")
	if err != nil {
		t.Fatalf("create Raid B: %v", err)
	}
	boss, err := s.CreateCategory("Boss", raidA.ID)
	if err != nil {
		t.Fatalf("create Raid A/Boss: %v", err)
	}

	// Drag "Boss" from under Raid A to under Raid B.
	if err := s.ReorderCategories([]CategoryPlacement{{ID: boss.ID, ParentID: raidB.ID, SortOrder: 0}}); err != nil {
		t.Fatalf("ReorderCategories: %v", err)
	}
	got, ok := catByName(mustList(t, s))["Boss"]
	if !ok {
		t.Fatal("Boss category missing after reparent")
	}
	if got.ParentID != raidB.ID {
		t.Fatalf("Boss.ParentID = %q, want %q", got.ParentID, raidB.ID)
	}
}

func TestReorderCategories_RejectsCyclesAndDepth(t *testing.T) {
	s := openTestStore(t)
	parent, err := s.CreateCategory("Parent", "")
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}
	child, err := s.CreateCategory("Child", parent.ID)
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	other, err := s.CreateCategory("Other", "")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	// A category can't become its own parent.
	if err := s.ReorderCategories([]CategoryPlacement{{ID: parent.ID, ParentID: parent.ID}}); !errors.Is(err, ErrCategoryCycle) {
		t.Fatalf("self-parent: want ErrCategoryCycle, got %v", err)
	}
	// Moving "Parent" under its own child would make it its own descendant.
	if err := s.ReorderCategories([]CategoryPlacement{
		{ID: parent.ID, ParentID: child.ID},
	}); !errors.Is(err, ErrCategoryDepth) {
		t.Fatalf("parent-under-child: want ErrCategoryDepth (child can't have children), got %v", err)
	}
	// A child ("Child") can't gain its own child ("Other" moved under it).
	if err := s.ReorderCategories([]CategoryPlacement{{ID: other.ID, ParentID: child.ID}}); !errors.Is(err, ErrCategoryDepth) {
		t.Fatalf("grandchild via reorder: want ErrCategoryDepth, got %v", err)
	}
	// "Parent" (which has "Child") can't itself become a child of "Other" —
	// that would leave Child two levels deep.
	if err := s.ReorderCategories([]CategoryPlacement{{ID: parent.ID, ParentID: other.ID}}); !errors.Is(err, ErrCategoryDepth) {
		t.Fatalf("parent-with-children-becomes-child: want ErrCategoryDepth, got %v", err)
	}
	// ...unless Child is ALSO relocated out in the same batch, so nothing ends
	// up two levels deep.
	if err := s.ReorderCategories([]CategoryPlacement{
		{ID: parent.ID, ParentID: other.ID},
		{ID: child.ID, ParentID: ""},
	}); err != nil {
		t.Fatalf("parent-becomes-child with child promoted in same batch should succeed: %v", err)
	}
}

func TestReorderCategories_MaterializesPackRow(t *testing.T) {
	s := openTestStore(t)
	packs := AllPacks()
	if len(packs) == 0 {
		t.Skip("no built-in packs")
	}
	pack := packs[0].PackName
	if err := s.Insert(makeTrigger("x", pack)); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	custom, err := s.CreateCategory("Custom", "")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	builtin, ok := catByName(mustList(t, s))[pack]
	if !ok {
		t.Fatalf("built-in category %q not materialized", pack)
	}
	// Put the pack ahead of the custom category.
	items := []CategoryPlacement{
		{ID: builtin.ID, SortOrder: 0},
		{ID: custom.ID, SortOrder: 1},
	}
	if err := s.ReorderCategories(items); err != nil {
		t.Fatalf("ReorderCategories: %v", err)
	}
	cats := mustList(t, s)
	if len(cats) < 2 || cats[0].Name != pack {
		t.Fatalf("expected pack %q first, got %+v", pack, cats)
	}
}

func TestReorderTriggers(t *testing.T) {
	s := openTestStore(t)
	for _, n := range []string{"a", "b", "c"} {
		if err := s.Insert(makeTrigger(n, "Cat")); err != nil {
			t.Fatalf("Insert %s: %v", n, err)
		}
	}
	idOf := func(name string) string { return name + ":Cat" }
	// New order: c, a, b
	if err := s.ReorderTriggers([]string{idOf("c"), idOf("a"), idOf("b")}); err != nil {
		t.Fatalf("ReorderTriggers: %v", err)
	}
	bySort := map[string]int{}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, tr := range list {
		bySort[tr.Name] = tr.SortOrder
	}
	if !(bySort["c"] < bySort["a"] && bySort["a"] < bySort["b"]) {
		t.Fatalf("sort orders not applied: %+v", bySort)
	}
}

func TestNextTriggerSortOrder_AppendsPerCategory(t *testing.T) {
	s := openTestStore(t)
	// makeTrigger uses SortOrder 0; emulate the handler's append by setting it.
	first := makeTrigger("a", "Cat")
	n, err := s.NextTriggerSortOrder("Cat")
	if err != nil {
		t.Fatalf("NextTriggerSortOrder: %v", err)
	}
	if n != 0 {
		t.Fatalf("empty category next = %d, want 0", n)
	}
	first.SortOrder = n
	if err := s.Insert(first); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	n2, err := s.NextTriggerSortOrder("Cat")
	if err != nil {
		t.Fatalf("NextTriggerSortOrder: %v", err)
	}
	if n2 != 1 {
		t.Fatalf("next after one = %d, want 1", n2)
	}
}

func TestResolveOrCreateCategory_ReusesExisting(t *testing.T) {
	s := openTestStore(t)
	first, err := s.ResolveOrCreateCategory("", "Raid Triggers")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	second, err := s.ResolveOrCreateCategory("", "Raid Triggers")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected the same category reused, got %q then %q", first.ID, second.ID)
	}
}
