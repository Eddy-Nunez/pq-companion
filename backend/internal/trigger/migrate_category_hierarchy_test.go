package trigger

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// buildLegacyCategoryDB writes a user.db at path using the pre-hierarchy
// schema — trigger_categories keyed by name (no id/parent_id), and no
// category_id column on triggers — so migrateCategoryHierarchy has real work
// to do when OpenStore runs the current schema/migration chain against it.
// Mirrors the shape store.go's CREATE TABLE IF NOT EXISTS used before this
// feature (see git history), trimmed to just the columns this test needs.
func buildLegacyCategoryDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer db.Close()

	stmts := []string{
		`CREATE TABLE triggers (
			id         TEXT NOT NULL PRIMARY KEY,
			name       TEXT NOT NULL,
			enabled    INTEGER NOT NULL DEFAULT 1,
			pattern    TEXT NOT NULL,
			actions    TEXT NOT NULL DEFAULT '[]',
			pack_name  TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE trigger_categories (
			name       TEXT    NOT NULL PRIMARY KEY,
			created_at INTEGER NOT NULL,
			explicit   INTEGER NOT NULL DEFAULT 1,
			sort_order INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT INTO trigger_categories (name, created_at, explicit, sort_order) VALUES ('Raid Triggers', 1000, 1, 0)`,
		// A slash-named category from a GINA import predating this feature —
		// migration must leave it exactly as one category, not split it.
		`INSERT INTO trigger_categories (name, created_at, explicit, sort_order) VALUES ('Raid/Boss Mechanics', 1000, 1, 1)`,
		`INSERT INTO triggers (id, name, enabled, pattern, pack_name, created_at) VALUES ('t1', 'Alpha', 1, '^a$', 'Raid Triggers', 1000)`,
		`INSERT INTO triggers (id, name, enabled, pattern, pack_name, created_at) VALUES ('t2', 'Beta', 1, '^b$', 'Raid/Boss Mechanics', 1000)`,
		// In-use pack_name with no persisted trigger_categories row at all —
		// the built-in/imported-pack case materializePackCategoryRows exists for.
		`INSERT INTO triggers (id, name, enabled, pattern, pack_name, created_at) VALUES ('t3', 'Gamma', 1, '^c$', 'Never Persisted', 1000)`,
		`INSERT INTO triggers (id, name, enabled, pattern, pack_name, created_at) VALUES ('t4', 'Delta', 1, '^d$', '', 1000)`, // Uncategorized
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

func TestMigrateCategoryHierarchy_UpgradesLegacySchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "user.db")
	buildLegacyCategoryDB(t, path)

	s, err := OpenStore(path)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer s.Close()

	cats := catByName(mustList(t, s))

	raid, ok := cats["Raid Triggers"]
	if !ok || raid.ID == "" || raid.ParentID != "" {
		t.Fatalf("Raid Triggers not migrated to a root id-keyed row: %+v", raid)
	}
	// The slash name survives verbatim — migration never auto-splits it.
	slashed, ok := cats["Raid/Boss Mechanics"]
	if !ok || slashed.ID == "" || slashed.ParentID != "" {
		t.Fatalf("Raid/Boss Mechanics not preserved verbatim: %+v", slashed)
	}
	neverPersisted, ok := cats["Never Persisted"]
	if !ok {
		t.Fatal("in-use pack_name with no prior row was not materialized")
	}
	if neverPersisted.Explicit {
		t.Fatalf("materialized pack category should be Explicit=false, got %+v", neverPersisted)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byName := make(map[string]*Trigger, len(list))
	for _, tr := range list {
		byName[tr.Name] = tr
	}
	if got := byName["Alpha"].CategoryID; got != raid.ID {
		t.Fatalf("Alpha.CategoryID = %q, want %q", got, raid.ID)
	}
	if got := byName["Beta"].CategoryID; got != slashed.ID {
		t.Fatalf("Beta.CategoryID = %q, want %q", got, slashed.ID)
	}
	if got := byName["Gamma"].CategoryID; got != neverPersisted.ID {
		t.Fatalf("Gamma.CategoryID = %q, want %q", got, neverPersisted.ID)
	}
	if got := byName["Delta"].CategoryID; got != "" {
		t.Fatalf("Delta (Uncategorized) should stay unlinked, got CategoryID=%q", got)
	}

	// Idempotent: re-running OpenStore's migration chain against the now
	// up-to-date file must not duplicate rows or change any id.
	s.Close()
	s2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("second OpenStore: %v", err)
	}
	defer s2.Close()
	cats2 := catByName(mustList(t, s2))
	if cats2["Raid Triggers"].ID != raid.ID {
		t.Fatalf("category id changed across re-migration: %q vs %q", cats2["Raid Triggers"].ID, raid.ID)
	}
	if n := len(mustList(t, s2)); n != len(cats) {
		t.Fatalf("category count changed across re-migration: %d vs %d", n, len(cats))
	}
}
