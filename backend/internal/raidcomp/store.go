package raidcomp

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Status is the lifecycle state of a raid encounter. Only 'active' encounters
// should participate in composition checks.
type Status string

const (
	StatusActive      Status = "active"      // comp numbers are kill-verified / authoritative
	StatusPlaceholder Status = "placeholder" // transcribed, comp numbers not yet verified
)

// CompLevel distinguishes the two staffing levels carried on an encounter.
type CompLevel string

const (
	CompMin CompLevel = "min" // hard floor to attempt the encounter
	CompRec CompLevel = "rec" // recommended / comfortable (first kills, learning passes)
)

// CompRow is one editable comp row: a taxonomy leaf carrying both staffing
// levels (MIN floor, REC comfortable). Mirrors eqmon's ingress CompRow.
type CompRow struct {
	Role string `json:"role"`
	Sub  string `json:"sub_role,omitempty"`
	Min  int    `json:"min"`
	Rec  int    `json:"rec"`
}

// Path returns the dotted leaf key: "tank.defensive" or "rgc".
func (r CompRow) Path() string {
	if r.Sub == "" {
		return r.Role
	}
	return r.Role + "." + r.Sub
}

// Encounter is one raid encounter in the knowledge base. Comps are stored
// normalized (one raid_encounter_comps row per leaf per level); Reqs and
// Strategy are child tables. ZoneID is the EQ zoneidnumber (what the Zeal
// pipe reports as Zone), resolved from the zone catalog; 0 = not set.
type Encounter struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Zone   string `json:"zone"`
	ZoneID int    `json:"zone_id,omitempty"`
	// NPCID links this encounter to its boss's npc_types row (quarm.db), so
	// the checker page can pull resists / HP / special abilities / signature
	// spells straight from the game database instead of duplicating them.
	// 0 = not linked.
	NPCID     int               `json:"npc_id,omitempty"`
	Status    Status            `json:"status"`
	Trigger   string            `json:"trigger,omitempty"`
	Reqs      []string          `json:"reqs,omitempty"`
	Strategy  map[string]string `json:"strategy,omitempty"` // section -> text (StrategyOrder keys)
	Source    string            `json:"source,omitempty"`
	Notes     string            `json:"notes,omitempty"`
	Comps     []CompRow         `json:"comps"`
	CreatedAt int64             `json:"created_at"`
	UpdatedAt int64             `json:"updated_at"`
}

// StrategyOrder lists the fixed strategy sections an encounter can carry,
// mirroring eqmon's ingress RaidData.StrategyOrder.
var StrategyOrder = []string{"pulling", "tanking", "healing", "dps", "notes"}

// ErrNotFound is returned when an encounter id does not exist.
var ErrNotFound = errors.New("raid encounter not found")

// Validate checks an encounter against enum + format constraints before any
// write. Taxonomy-role existence is checked against the store's live leaves
// (SaveEncounter does that, since it owns the current taxonomy).
func (e *Encounter) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return errors.New("raidcomp: encounter id required")
	}
	if strings.TrimSpace(e.Name) == "" {
		return errors.New("raidcomp: encounter name required")
	}
	if strings.TrimSpace(e.Zone) == "" {
		return errors.New("raidcomp: encounter zone required")
	}
	if e.ZoneID < 0 {
		return errors.New("raidcomp: zone_id must be >= 0")
	}
	if e.NPCID < 0 {
		return errors.New("raidcomp: npc_id must be >= 0")
	}
	if e.Status != StatusActive && e.Status != StatusPlaceholder {
		return fmt.Errorf("raidcomp: invalid status %q (want active|placeholder)", e.Status)
	}
	validStrategy := map[string]bool{}
	for _, s := range StrategyOrder {
		validStrategy[s] = true
	}
	for section := range e.Strategy {
		if !validStrategy[section] {
			return fmt.Errorf("raidcomp: unknown strategy section %q", section)
		}
	}
	for _, c := range e.Comps {
		if c.Min < 0 || c.Rec < 0 {
			return fmt.Errorf("raidcomp: comp %q: counts must be >= 0", c.Path())
		}
		if c.Rec < c.Min {
			return fmt.Errorf("raidcomp: comp %q: rec (%d) below min (%d)", c.Path(), c.Rec, c.Min)
		}
	}
	return nil
}

// ZoneIDResolver maps a zone long name to its EQ zoneidnumber. Provided by
// the caller (quarm.db lives outside user.db); may be nil.
type ZoneIDResolver func(longName string) int

// Store persists raid encounters (and the user-editable role taxonomy) in
// user.db. Each store opens its own WAL connection to the shared database
// file, same pattern as the lockout / keys / players stores.
type Store struct {
	db     *sql.DB
	zoneID ZoneIDResolver // long_name -> zoneidnumber; used only at seed time
}

// OpenStore opens user.db at path and runs the schema migrations. A fresh
// knowledge base (role taxonomy + starter encounters) is seeded on first
// open: the taxonomy always, encounters only when the table is empty.
func OpenStore(path string, resolveZoneID ZoneIDResolver) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open user.db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping user.db: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, zoneID: resolveZoneID}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate user.db: %w", err)
	}
	if err := s.EnsureSeed(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed raid knowledge base: %w", err)
	}
	return s, nil
}

// Close releases the underlying connection.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS raid_encounters (
			id         TEXT    NOT NULL PRIMARY KEY,
			name       TEXT    NOT NULL,
			zone       TEXT    NOT NULL,
			status     TEXT    NOT NULL DEFAULT 'active',
			trigger    TEXT    NOT NULL DEFAULT '',
			source     TEXT    NOT NULL DEFAULT '',
			notes      TEXT    NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS raid_encounter_comps (
			encounter_id TEXT    NOT NULL,
			comp_level   TEXT    NOT NULL,
			role         TEXT    NOT NULL,
			sub_role     TEXT,
			count        INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (encounter_id, comp_level, role, sub_role)
		)`,
		`CREATE INDEX IF NOT EXISTS raid_encounter_comps_encounter ON raid_encounter_comps(encounter_id)`,
		`CREATE TABLE IF NOT EXISTS raid_encounter_reqs (
			encounter_id TEXT    NOT NULL,
			position     INTEGER NOT NULL,
			text         TEXT    NOT NULL,
			PRIMARY KEY (encounter_id, position)
		)`,
		`CREATE TABLE IF NOT EXISTS raid_encounter_strategy (
			encounter_id TEXT NOT NULL,
			section      TEXT NOT NULL,
			text         TEXT NOT NULL,
			PRIMARY KEY (encounter_id, section)
		)`,
		`CREATE TABLE IF NOT EXISTS raid_roles (
			role        TEXT NOT NULL,
			sub_role    TEXT NOT NULL DEFAULT '',
			label       TEXT NOT NULL,
			class_codes TEXT NOT NULL,
			position    INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (role, sub_role)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("raidcomp migrate: %w", err)
		}
	}
	// Versioned column adds — CREATE IF NOT EXISTS doesn't evolve existing
	// tables, so ADD COLUMN guards run separately. DDL can't be parameterized,
	// so each column carries its own ALTER statement.
	addColumns := map[string]string{
		"zone_id": `ALTER TABLE raid_encounters ADD COLUMN zone_id INTEGER NOT NULL DEFAULT 0`,
		"npc_id":  `ALTER TABLE raid_encounters ADD COLUMN npc_id INTEGER NOT NULL DEFAULT 0`,
	}
	for _, col := range []string{"zone_id", "npc_id"} {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('raid_encounters') WHERE name = ?`, col).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			if _, err := s.db.Exec(addColumns[col]); err != nil {
				return fmt.Errorf("raidcomp migrate add column %s: %w", col, err)
			}
		}
	}
	return nil
}

// EnsureSeed seeds the taxonomy (always). Encounters are no longer
// auto-seeded — the knowledge base starts empty and guilds build their own
// encounters in the Raid Editor — but existing stores still get their zone
// ids backfilled and, once, their legacy sample encounter removed.
func (s *Store) EnsureSeed() error {
	if err := s.EnsureSeedRoles(); err != nil {
		return err
	}
	if err := s.removeLegacySeedEncounter(); err != nil {
		return err
	}
	if s.zoneID == nil {
		return nil
	}
	// Backfill zone ids for encounters seeded before the zone_id column
	// existed (idempotent — only touches rows still at 0). Collect first,
	// then update: the store pins a single SQLite connection, so an UPDATE
	// inside the SELECT cursor's loop would deadlock.
	type zoneFix struct{ id, zone string }
	var fixes []zoneFix
	rows, err := s.db.Query(`SELECT id, zone FROM raid_encounters WHERE zone_id = 0`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var f zoneFix
		if err := rows.Scan(&f.id, &f.zone); err != nil {
			rows.Close()
			return err
		}
		fixes = append(fixes, f)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, f := range fixes {
		if zid := s.zoneID(f.zone); zid > 0 {
			if _, err := s.db.Exec(`UPDATE raid_encounters SET zone_id = ? WHERE id = ?`, zid, f.id); err != nil {
				return err
			}
		}
	}
	return nil
}

// legacySeedNotes is the Notes text carried by the "aow" encounter earlier
// versions auto-seeded on first open (see git history for SeedEncounters).
// Matched verbatim below so removeLegacySeedEncounter only ever deletes the
// untouched sample row, never a user's own "aow"-id encounter.
const legacySeedNotes = "Counts are starting suggestions, not canonical. AoW himself is unslowable but " +
	"the surrounding mobs are not — slower stays for adds. No rgc/lockpicker/tracker/coth " +
	"needed on the Kael path; RGC staffing is for Ssra (Luclin). Adjust debuffer/resist " +
	"coverage to the guild class mix."

// removeLegacySeedEncounter is a one-time cleanup: earlier versions
// auto-seeded a starter "Avatar of War" encounter (id "aow") into every new
// store. The knowledge base no longer ships sample encounters, so any store
// still carrying that exact seeded row — identified by its distinctive Notes
// text, so a user's own edited or re-created "aow" encounter is left alone —
// has it removed.
func (s *Store) removeLegacySeedEncounter() error {
	var notes string
	err := s.db.QueryRow(`SELECT notes FROM raid_encounters WHERE id = ?`, "aow").Scan(&notes)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if notes != legacySeedNotes {
		return nil
	}
	return s.DeleteEncounter("aow")
}

// EnsureSeedRoles inserts the starter taxonomy when the roles table is empty
// (idempotent).
func (s *Store) EnsureSeedRoles() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM raid_roles`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, r := range SeedRoles() {
		if err := s.SaveRole(r); err != nil {
			return err
		}
	}
	return nil
}

// ── Role taxonomy ──────────────────────────────────────────────────────────

// ListRoles returns all taxonomy rows ordered by position (then role/sub).
// This is the canonical ordering for the checker and editor grids.
func (s *Store) ListRoles() ([]Role, error) {
	rows, err := s.db.Query(`SELECT role, sub_role, label, class_codes, position FROM raid_roles ORDER BY position, role, sub_role`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Role
	for rows.Next() {
		var r Role
		var codes string
		if err := rows.Scan(&r.Role, &r.Sub, &r.Label, &codes, &r.Position); err != nil {
			return nil, err
		}
		if codes != "" {
			for _, c := range strings.Split(codes, ",") {
				if code := ClassCode(strings.TrimSpace(c)); code != "" {
					r.Classes = append(r.Classes, code)
				}
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Leaves returns the live taxonomy flattened to RoleLeaf rows (position
// ordered). The composition checker and comp validation read from here, so
// taxonomy edits take effect immediately.
func (s *Store) Leaves() ([]RoleLeaf, error) {
	roles, err := s.ListRoles()
	if err != nil {
		return nil, err
	}
	return LeavesFromRoles(roles), nil
}

// SaveRole inserts or updates one taxonomy row (keyed by role+sub). New rows
// append at the end (position = max + 1). Classes must be known codes; the
// label must be non-empty.
func (s *Store) SaveRole(r Role) error {
	if strings.TrimSpace(r.Role) == "" {
		return errors.New("raidcomp: role id required")
	}
	if strings.TrimSpace(r.Label) == "" {
		return errors.New("raidcomp: role label required")
	}
	if len(r.Classes) == 0 {
		return fmt.Errorf("raidcomp: role %q needs at least one class", r.Role)
	}
	seen := map[ClassCode]bool{}
	var codes []string
	for _, c := range r.Classes {
		if _, ok := ClassNames[c]; !ok {
			return fmt.Errorf("raidcomp: unknown class code %q", c)
		}
		if !seen[c] {
			seen[c] = true
			codes = append(codes, string(c))
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var existingPos int
	err = tx.QueryRow(`SELECT position FROM raid_roles WHERE role = ? AND sub_role = ?`, r.Role, r.Sub).Scan(&existingPos)
	if err == sql.ErrNoRows {
		// new row: append at the end
		var max int
		_ = tx.QueryRow(`SELECT COALESCE(MAX(position), -1) FROM raid_roles`).Scan(&max)
		existingPos = max + 1
	} else if err != nil {
		return err
	}
	position := existingPos
	if _, err := tx.Exec(`
		INSERT INTO raid_roles (role, sub_role, label, class_codes, position) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(role, sub_role) DO UPDATE SET
			label = excluded.label, class_codes = excluded.class_codes, position = excluded.position
	`, r.Role, r.Sub, r.Label, strings.Join(codes, ","), position); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteRole removes a taxonomy row (keyed by role+sub, "" for flat). Roles
// referenced by any encounter comp are protected — the user must clear those
// rows first, so a deleted role can never orphan stale comp data.
func (s *Store) DeleteRole(role, sub string) error {
	if strings.TrimSpace(role) == "" {
		return errors.New("raidcomp: role id required")
	}
	var used int
	if err := s.db.QueryRow(`
		SELECT COUNT(*) FROM raid_encounter_comps WHERE role = ? AND IFNULL(sub_role, '') = ?
	`, role, sub).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("raidcomp: role %q is used by %d encounter comp row(s); remove those comps first", role, used)
	}
	if _, err := s.db.Exec(`DELETE FROM raid_roles WHERE role = ? AND sub_role = ?`, role, sub); err != nil {
		return err
	}
	return nil
}

// ── Encounters ─────────────────────────────────────────────────────────────

// SaveEncounter inserts or replaces an encounter and all its child rows
// (comps, reqs, strategy) in one transaction. Children are deleted then
// re-inserted so edits can remove rows. Comp roles must exist in the live
// taxonomy.
func (s *Store) SaveEncounter(e *Encounter) error {
	if err := e.Validate(); err != nil {
		return err
	}
	leaves, err := s.Leaves()
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, l := range leaves {
		allowed[l.Path()] = true
	}
	for _, c := range e.Comps {
		if !allowed[c.Path()] {
			return fmt.Errorf("raidcomp: comp role %q not in taxonomy", c.Path())
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	now := time.Now().Unix()
	var created int64
	err = tx.QueryRow(`SELECT created_at FROM raid_encounters WHERE id = ?`, e.ID).Scan(&created)
	if err == sql.ErrNoRows {
		created = now
	} else if err != nil {
		return err
	}
	if _, err := tx.Exec(`
		INSERT INTO raid_encounters (id, name, zone, zone_id, npc_id, status, trigger, source, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, zone = excluded.zone, zone_id = excluded.zone_id, npc_id = excluded.npc_id,
			status = excluded.status, trigger = excluded.trigger, source = excluded.source,
			notes = excluded.notes, updated_at = excluded.updated_at
	`, e.ID, e.Name, e.Zone, e.ZoneID, e.NPCID, string(e.Status), e.Trigger, e.Source, e.Notes, created, now); err != nil {
		return fmt.Errorf("insert encounter %q: %w", e.ID, err)
	}

	for _, t := range []string{"raid_encounter_comps", "raid_encounter_reqs", "raid_encounter_strategy"} {
		if _, err := tx.Exec(`DELETE FROM `+t+` WHERE encounter_id = ?`, e.ID); err != nil {
			return err
		}
	}

	for _, c := range e.Comps {
		for _, level := range []CompLevel{CompMin, CompRec} {
			n := c.Min
			if level == CompRec {
				n = c.Rec
			}
			sub := sql.NullString{}
			if c.Sub != "" {
				sub = sql.NullString{String: c.Sub, Valid: true}
			}
			if _, err := tx.Exec(`
				INSERT INTO raid_encounter_comps (encounter_id, comp_level, role, sub_role, count)
				VALUES (?, ?, ?, ?, ?)
			`, e.ID, string(level), c.Role, sub, n); err != nil {
				return err
			}
		}
	}
	for i, r := range e.Reqs {
		if strings.TrimSpace(r) == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO raid_encounter_reqs (encounter_id, position, text) VALUES (?, ?, ?)
		`, e.ID, i, r); err != nil {
			return err
		}
	}
	for section, text := range e.Strategy {
		if strings.TrimSpace(text) == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO raid_encounter_strategy (encounter_id, section, text) VALUES (?, ?, ?)
		`, e.ID, section, text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type encounterRow struct {
	e        Encounter
	comps    []CompRow
	reqs     []string
	strategy map[string]string
}

// ListEncounters returns all encounters with full children, ordered by id.
func (s *Store) ListEncounters() ([]Encounter, error) {
	rows, err := s.db.Query(`
		SELECT id, name, zone, zone_id, npc_id, status, trigger, source, notes, created_at, updated_at
		FROM raid_encounters ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []encounterRow
	for rows.Next() {
		var er encounterRow
		if err := rows.Scan(&er.e.ID, &er.e.Name, &er.e.Zone, &er.e.ZoneID, &er.e.NPCID, &er.e.Status, &er.e.Trigger,
			&er.e.Source, &er.e.Notes, &er.e.CreatedAt, &er.e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, er)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.loadChildren(out); err != nil {
		return nil, err
	}
	encs := make([]Encounter, len(out))
	for i, er := range out {
		er.e.Comps = er.comps
		er.e.Reqs = er.reqs
		er.e.Strategy = er.strategy
		encs[i] = er.e
	}
	return encs, nil
}

// loadChildren bulk-loads comps / reqs / strategy for a set of encounter
// parent rows and merges the min/rec halves of each comp leaf.
func (s *Store) loadChildren(rows []encounterRow) error {
	idxByID := map[string]int{}
	for i, er := range rows {
		idxByID[er.e.ID] = i
	}

	compRows, err := s.db.Query(`
		SELECT encounter_id, comp_level, role, sub_role, count
		FROM raid_encounter_comps
		ORDER BY encounter_id, role, sub_role, comp_level`)
	if err != nil {
		return err
	}
	defer compRows.Close()
	for compRows.Next() {
		var encID, level, role string
		var sub sql.NullString
		var n int
		if err := compRows.Scan(&encID, &level, &role, &sub, &n); err != nil {
			return err
		}
		i, ok := idxByID[encID]
		if !ok {
			continue // orphan rows (defensive; deletions clear children explicitly)
		}
		subStr := ""
		if sub.Valid {
			subStr = sub.String
		}
		cr := CompRow{Role: role, Sub: subStr}
		if level == string(CompMin) {
			cr.Min = n
		} else {
			cr.Rec = n
		}
		rows[i].comps = append(rows[i].comps, cr)
	}
	if err := compRows.Err(); err != nil {
		return err
	}
	// merge the per-level rows into one CompRow per leaf (ORDER BY ... comp_level
	// guarantees the min row precedes the rec row per leaf)
	for i := range rows {
		merged := map[string]*CompRow{}
		for j := range rows[i].comps {
			c := rows[i].comps[j]
			m, ok := merged[c.Path()]
			if !ok {
				cc := c
				merged[c.Path()] = &cc
				continue
			}
			if c.Min != 0 {
				m.Min = c.Min
			}
			if c.Rec != 0 {
				m.Rec = c.Rec
			}
		}
		keys := make([]string, 0, len(merged))
		for k := range merged {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		mergedRows := make([]CompRow, 0, len(keys))
		for _, k := range keys {
			mergedRows = append(mergedRows, *merged[k])
		}
		rows[i].comps = mergedRows
	}

	reqRows, err := s.db.Query(`
		SELECT encounter_id, position, text FROM raid_encounter_reqs ORDER BY encounter_id, position`)
	if err != nil {
		return err
	}
	defer reqRows.Close()
	for reqRows.Next() {
		var encID, text string
		var pos int
		if err := reqRows.Scan(&encID, &pos, &text); err != nil {
			return err
		}
		if i, ok := idxByID[encID]; ok {
			rows[i].reqs = append(rows[i].reqs, text)
		}
	}
	if err := reqRows.Err(); err != nil {
		return err
	}

	stratRows, err := s.db.Query(`
		SELECT encounter_id, section, text FROM raid_encounter_strategy ORDER BY encounter_id, section`)
	if err != nil {
		return err
	}
	defer stratRows.Close()
	for stratRows.Next() {
		var encID, section, text string
		if err := stratRows.Scan(&encID, &section, &text); err != nil {
			return err
		}
		if i, ok := idxByID[encID]; ok {
			if rows[i].strategy == nil {
				rows[i].strategy = map[string]string{}
			}
			rows[i].strategy[section] = text
		}
	}
	return stratRows.Err()
}

// GetEncounter returns one encounter with full children, or ErrNotFound.
func (s *Store) GetEncounter(id string) (*Encounter, error) {
	encs, err := s.ListEncounters()
	if err != nil {
		return nil, err
	}
	// Fine at knowledge-base scale (tens of encounters); keeps one code path
	// so children are always assembled identically.
	for i := range encs {
		if encs[i].ID == id {
			return &encs[i], nil
		}
	}
	return nil, ErrNotFound
}

// DeleteEncounter removes an encounter and all its child rows.
func (s *Store) DeleteEncounter(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, table := range []string{"raid_encounter_comps", "raid_encounter_reqs", "raid_encounter_strategy"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE encounter_id = ?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM raid_encounters WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
