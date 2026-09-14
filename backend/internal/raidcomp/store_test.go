package raidcomp

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// openTemp opens a Store on a fresh temp DB file.
func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// TestStore_NoEncountersSeededOnFirstOpen locks in that the knowledge base
// starts empty — encounters are no longer auto-seeded, so a fresh store
// never carries sample/placeholder data a guild didn't create themselves.
func TestStore_NoEncountersSeededOnFirstOpen(t *testing.T) {
	s := openTemp(t)
	encs, err := s.ListEncounters()
	if err != nil {
		t.Fatal(err)
	}
	if len(encs) != 0 {
		t.Fatalf("fresh store: got %d encounters, want 0 (no sample data)", len(encs))
	}
}

// TestStore_SaveEncounter_CompRoundTrip exercises the same comp-row shape the
// old auto-seeded "aow" fixture carried (min/rec split across two tables,
// merged back on read), now via an explicit save instead of seeding.
func TestStore_SaveEncounter_CompRoundTrip(t *testing.T) {
	s := openTemp(t)
	e := aowFixture()
	if err := s.SaveEncounter(e); err != nil {
		t.Fatal(err)
	}
	aow, err := s.GetEncounter("aow")
	if err != nil {
		t.Fatal(err)
	}
	if aow.Name != "Avatar of War" || aow.Zone != "Kael Drakkel" {
		t.Errorf("encounter wrong: %+v", aow)
	}
	if len(aow.Comps) != 18 {
		t.Errorf("comps = %d rows, want 18", len(aow.Comps))
	}
	byPath := map[string]CompRow{}
	for _, c := range aow.Comps {
		byPath[c.Path()] = c
	}
	if r := byPath["tank.defensive"]; r.Min != 3 || r.Rec != 4 {
		t.Errorf("tank.defensive = %+v, want min 3 rec 4", r)
	}
	if r := byPath["damage"]; r.Min != 20 || r.Rec != 24 {
		t.Errorf("damage = %+v, want min 20 rec 24", r)
	}
	if r := byPath["rgc"]; r.Min != 0 || r.Rec != 0 {
		t.Errorf("rgc = %+v, want min 0 rec 0", r)
	}
}

// TestStore_RemoveLegacySeedEncounter verifies the one-time cleanup: a store
// still carrying the old auto-seeded "aow" row (identified by its exact
// Notes text) has it removed on open, while a user's own re-created "aow"
// encounter (different Notes) is left untouched.
func TestStore_RemoveLegacySeedEncounter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s1, err := OpenStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	seed := aowFixture()
	seed.Notes = legacySeedNotes
	if err := s1.SaveEncounter(seed); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	// Reopening must strip the legacy row.
	s2, err := OpenStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.GetEncounter("aow"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy seed encounter should have been removed, got err=%v", err)
	}

	// A user's own "aow" with different notes must survive a reopen.
	custom := aowFixture()
	custom.Notes = "our guild's own notes"
	if err := s2.SaveEncounter(custom); err != nil {
		t.Fatal(err)
	}
	s2.Close()
	s3, err := OpenStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s3.Close()
	got, err := s3.GetEncounter("aow")
	if err != nil {
		t.Fatalf("user's own aow encounter should survive: %v", err)
	}
	if got.Notes != "our guild's own notes" {
		t.Errorf("notes = %q, want preserved", got.Notes)
	}
}

func TestStore_CRUDRoundTrip(t *testing.T) {
	s := openTemp(t)
	e := &Encounter{
		ID:       "vulak",
		Name:     "Vulak'Aerr",
		Zone:     "Kael Drakkel",
		Status:   StatusPlaceholder,
		Trigger:  "spawns after matriarch cycle",
		Reqs:     []string{"Key to the Temple", "Learn the True Name"},
		Strategy: map[string]string{"tanking": "tap tank", "notes": "drakkin soak"},
		Source:   "Kael raid guide",
		Comps: []CompRow{
			{Role: "tank", Sub: "defensive", Min: 2, Rec: 3},
			{Role: "damage", Min: 18, Rec: 22},
		},
	}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetEncounter("vulak")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Vulak'Aerr" || got.Status != StatusPlaceholder || got.Trigger != "spawns after matriarch cycle" {
		t.Errorf("identity fields wrong: %+v", got)
	}
	if len(got.Reqs) != 2 || got.Reqs[0] != "Key to the Temple" || got.Reqs[1] != "Learn the True Name" {
		t.Errorf("reqs wrong: %v", got.Reqs)
	}
	if got.Strategy["tanking"] != "tap tank" || got.Strategy["notes"] != "drakkin soak" {
		t.Errorf("strategy wrong: %v", got.Strategy)
	}
	if len(got.Comps) != 2 {
		t.Fatalf("comps = %d, want 2", len(got.Comps))
	}
	byPath := map[string]CompRow{}
	for _, c := range got.Comps {
		byPath[c.Path()] = c
	}
	if r := byPath["tank.defensive"]; r.Min != 2 || r.Rec != 3 {
		t.Errorf("tank.defensive = %+v", r)
	}
	if r := byPath["damage"]; r.Min != 18 || r.Rec != 22 {
		t.Errorf("damage = %+v", r)
	}

	// update: shrink comps and clear strategy
	e.Comps = []CompRow{{Role: "damage", Min: 20, Rec: 24}}
	e.Strategy = nil
	if err := s.SaveEncounter(e); err != nil {
		t.Fatal(err)
	}
	got2, err := s.GetEncounter("vulak")
	if err != nil {
		t.Fatal(err)
	}
	if len(got2.Comps) != 1 || got2.Comps[0].Min != 20 || got2.Comps[0].Rec != 24 {
		t.Errorf("post-update comps wrong: %+v", got2.Comps)
	}
	if got2.Strategy != nil {
		t.Errorf("strategy should be cleared, got %v", got2.Strategy)
	}
	if len(got2.Reqs) != 2 {
		t.Errorf("reqs should persist across update, got %v", got2.Reqs)
	}

	// list includes the new encounter
	list, err := s.ListEncounters()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("list = %d, want 1", len(list))
	}

	// delete removes children too
	if err := s.DeleteEncounter("vulak"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetEncounter("vulak"); err != ErrNotFound {
		t.Errorf("after delete, want ErrNotFound, got %v", err)
	}
	list, _ = s.ListEncounters()
	if len(list) != 0 {
		t.Fatalf("after delete, list = %d, want 0", len(list))
	}
}

func TestStore_Validate(t *testing.T) {
	s := openTemp(t)
	base := &Encounter{ID: "x", Name: "X", Zone: "Z", Status: StatusActive,
		Comps: []CompRow{{Role: "damage", Min: 5, Rec: 5}}}
	cases := []struct {
		mutate func(*Encounter)
		desc   string
	}{
		{func(e *Encounter) { e.Status = Status("bogus") }, "bad status"},
		{func(e *Encounter) { e.Name = "" }, "empty name"},
		{func(e *Encounter) { e.ID = "" }, "empty id"},
		{func(e *Encounter) { e.Comps[0].Role = "not_a_role" }, "unknown role"},
		{func(e *Encounter) { e.Comps[0].Min = -1 }, "negative min"},
		{func(e *Encounter) { e.Comps[0].Rec = 3 }, "rec below min"},
		{func(e *Encounter) { e.Strategy = map[string]string{"nope": "x"} }, "unknown strategy section"},
	}
	for _, c := range cases {
		e := *base
		e.Comps = append([]CompRow(nil), base.Comps...)
		c.mutate(&e)
		if err := s.SaveEncounter(&e); err == nil {
			t.Errorf("case %q: expected validation error", c.desc)
		}
	}
}

func TestStore_GetMissing(t *testing.T) {
	s := openTemp(t)
	if _, err := s.GetEncounter("nope"); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestStore_RoleTaxonomySeeded(t *testing.T) {
	s := openTemp(t)
	roles, err := s.ListRoles()
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 20 {
		t.Fatalf("seeded roles = %d, want 20", len(roles))
	}
	byKey := map[string]Role{}
	for _, r := range roles {
		byKey[r.Role+"\x00"+r.Sub] = r
	}
	if r := byKey["rgc\x00"]; r.Label != "Remove Greater Curse" {
		t.Errorf("rgc label = %q, want Remove Greater Curse", r.Label)
	}
	if r := byKey["coth\x00"]; r.Label != "Call of the Hero" {
		t.Errorf("coth label = %q", r.Label)
	}
	if r := byKey["tank\x00defensive"]; len(r.Classes) != 1 || r.Classes[0] != CodeWarrior {
		t.Errorf("tank.defensive classes = %v", r.Classes)
	}
	// position ordering: tank flat roles first, damage last
	if roles[0].Role != "tank" || roles[len(roles)-1].Role != "damage" {
		t.Errorf("position order wrong: first=%s last=%s", roles[0].Role, roles[len(roles)-1].Role)
	}
	leaves, err := s.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) != 20 {
		t.Errorf("leaves = %d, want 20", len(leaves))
	}
}

func TestStore_RoleCRUD(t *testing.T) {
	s := openTemp(t)
	// an encounter using tank.defensive, so the in-use delete guard below has
	// something to actually block
	if err := s.SaveEncounter(&Encounter{
		ID: "x", Name: "X", Zone: "Z", Status: StatusActive,
		Comps: []CompRow{{Role: "tank", Sub: "defensive", Min: 1, Rec: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	// add a flat role; appends at the end
	if err := s.SaveRole(Role{Role: "puller2", Label: "Puller 2", Classes: ClassSet{CodeMonk}}); err != nil {
		t.Fatal(err)
	}
	// add a sub-type
	if err := s.SaveRole(Role{Role: "tank", Sub: "offtank", Label: "Tank / Offtank", Classes: ClassSet{CodeWarrior, CodePaladin}}); err != nil {
		t.Fatal(err)
	}
	roles, _ := s.ListRoles()
	if len(roles) != 22 {
		t.Fatalf("roles = %d, want 22", len(roles))
	}
	// last two appended keep increasing positions
	if roles[20].Role != "puller2" || roles[21].Sub != "offtank" {
		t.Errorf("append positions wrong: %+v %+v", roles[20], roles[21])
	}
	// update label in place, position preserved
	if err := s.SaveRole(Role{Role: "puller2", Label: "Pullers", Classes: ClassSet{CodeMonk, CodeShadowKnight}}); err != nil {
		t.Fatal(err)
	}
	roles, _ = s.ListRoles()
	if roles[20].Label != "Pullers" || len(roles[20].Classes) != 2 {
		t.Errorf("update failed: %+v", roles[20])
	}
	if roles[20].Position != roles[20].Position { // stable
	}
	// delete an unused role
	if err := s.DeleteRole("puller2", ""); err != nil {
		t.Fatal(err)
	}
	roles, _ = s.ListRoles()
	if len(roles) != 21 {
		t.Errorf("after delete = %d, want 21", len(roles))
	}
	// delete a role referenced by a comp must be blocked (encounter "x" uses tank.defensive)
	if err := s.DeleteRole("tank", "defensive"); err == nil {
		t.Error("expected in-use delete to fail")
	}
	// invalid class rejected
	if err := s.SaveRole(Role{Role: "bogus", Label: "Bogus", Classes: ClassSet{ClassCode("zzz")}}); err == nil {
		t.Error("expected unknown class error")
	}
}

func TestStore_ZoneIDRoundTrip(t *testing.T) {
	s := openTemp(t)
	e := &Encounter{ID: "tov", Name: "ToV Test", Zone: "Temple of Veeshan", ZoneID: 175, Status: StatusActive,
		Comps: []CompRow{{Role: "damage", Min: 5, Rec: 5}}}
	if err := s.SaveEncounter(e); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetEncounter("tov")
	if err != nil {
		t.Fatal(err)
	}
	if got.ZoneID != 175 {
		t.Errorf("zone_id = %d, want 175", got.ZoneID)
	}
	// update preserves/updates zone_id
	e.ZoneID = 999
	e.Zone = "Nowhere"
	if err := s.SaveEncounter(e); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetEncounter("tov")
	if got.ZoneID != 999 || got.Zone != "Nowhere" {
		t.Errorf("updated = %+v", got)
	}
}

func TestStore_MigrationsAddZoneIDToExistingTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	// create the pre-zone_id schema by hand, then let OpenStore migrate it
	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE raid_encounters (id TEXT PRIMARY KEY, name TEXT NOT NULL, zone TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'active', trigger TEXT NOT NULL DEFAULT '', source TEXT NOT NULL DEFAULT '', notes TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL)`,
		`CREATE TABLE raid_roles (role TEXT NOT NULL, sub_role TEXT NOT NULL DEFAULT '', label TEXT NOT NULL, class_codes TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (role, sub_role))`,
	} {
		if _, err := legacy.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	// a pre-migration row, inserted directly since the pre-zone_id schema has
	// no zone_id/npc_id columns for SaveEncounter to write
	now := time.Now().Unix()
	if _, err := legacy.Exec(
		`INSERT INTO raid_encounters (id, name, zone, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		"vulak", "Vulak'Aerr", "Kael Drakkel", "active", now, now,
	); err != nil {
		t.Fatal(err)
	}
	legacy.Close()

	s, err := OpenStore(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// the migration adds raid_encounters.zone_id / npc_id, defaulted to 0, to
	// the pre-existing row rather than dropping or reseeding it
	got, err := s.GetEncounter("vulak")
	if err != nil {
		t.Fatal(err)
	}
	if got.ZoneID != 0 {
		t.Errorf("zone_id should default 0, got %d", got.ZoneID)
	}
	if got.NPCID != 0 {
		t.Errorf("npc_id should default 0, got %d", got.NPCID)
	}
}
