package blockbuff

import (
	"database/sql"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

// Entry is one buff block: spell SpellID is refused from other players, either
// always (IfSpellID == 0) or only while IfSpellID is active on the character
// (#blockbuffif). It mirrors a row of the server's character_blocked_buffs.
type Entry struct {
	SpellID   int `json:"spell_id"`
	IfSpellID int `json:"if_spell_id"`
}

// Less orders entries by spell, then condition, for stable output.
func (e Entry) Less(o Entry) bool {
	if e.SpellID != o.SpellID {
		return e.SpellID < o.SpellID
	}
	return e.IfSpellID < o.IfSpellID
}

const (
	kindDesired  = "desired"
	kindObserved = "observed"
)

// Store persists, per character, the block list the player wants (desired) and
// the list PQC last saw the server confirm in the log (observed). Blocks live
// in the game server's database and can only be changed with in-game commands,
// so the two are reconciled by showing the commands that close the gap.
type Store struct {
	db *sql.DB
}

// OpenStore opens user.db at path and runs the blockbuff migrations.
func OpenStore(path string) (*Store, error) {
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
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate user.db: %w", err)
	}
	return s, nil
}

// Close releases the underlying connection.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS blockbuff_entries (
			character   TEXT    NOT NULL,
			kind        TEXT    NOT NULL,
			spell_id    INTEGER NOT NULL,
			if_spell_id INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (character, kind, spell_id, if_spell_id)
		)
	`); err != nil {
		return err
	}
	// updated_at is the log time of the newest server reply applied (gates
	// out-of-order backfill); synced_at is the log time of the last full list
	// (0 = the server's list has never been seen, so extras can't be known).
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS blockbuff_sync (
			character  TEXT PRIMARY KEY,
			updated_at INTEGER NOT NULL DEFAULT 0,
			synced_at  INTEGER NOT NULL DEFAULT 0
		)
	`)
	return err
}

func (s *Store) list(character, kind string) ([]Entry, error) {
	rows, err := s.db.Query(
		`SELECT spell_id, if_spell_id FROM blockbuff_entries WHERE character = ? AND kind = ?`,
		character, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.SpellID, &e.IfSpellID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out, rows.Err()
}

// Desired returns the character's wanted block list.
func (s *Store) Desired(character string) ([]Entry, error) { return s.list(character, kindDesired) }

// Observed returns the block list last confirmed by the server.
func (s *Store) Observed(character string) ([]Entry, error) { return s.list(character, kindObserved) }

// SyncedAt returns the unix time of the last full server list (0 = never).
func (s *Store) SyncedAt(character string) (int64, error) {
	var v int64
	err := s.db.QueryRow(`SELECT synced_at FROM blockbuff_sync WHERE character = ?`, character).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return v, err
}

// SetDesired replaces the character's wanted list, de-duplicating entries.
func (s *Store) SetDesired(character string, entries []Entry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM blockbuff_entries WHERE character = ? AND kind = ?`, character, kindDesired); err != nil {
		return err
	}
	for _, e := range entries {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO blockbuff_entries (character, kind, spell_id, if_spell_id) VALUES (?, ?, ?, ?)`,
			character, kindDesired, e.SpellID, e.IfSpellID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AdoptObserved copies the server's list over the wanted list.
func (s *Store) AdoptObserved(character string) error {
	obs, err := s.Observed(character)
	if err != nil {
		return err
	}
	return s.SetDesired(character, obs)
}

// Characters lists every character with a desired or observed row.
func (s *Store) Characters() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT character FROM blockbuff_entries ORDER BY character`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// fresh reports whether an event at unix time ts is no older than the newest
// one already applied for the character, and bumps updated_at when it is.
// Gating on log time keeps a backfill of an old log from regressing data a
// live session recorded later. Equal times pass: replaying a line is idempotent.
func fresh(tx *sql.Tx, character string, ts int64) (bool, error) {
	var updated int64
	err := tx.QueryRow(`SELECT updated_at FROM blockbuff_sync WHERE character = ?`, character).Scan(&updated)
	if err != nil && err != sql.ErrNoRows {
		return false, err
	}
	if err == nil && ts < updated {
		return false, nil
	}
	if _, err := tx.Exec(
		`INSERT INTO blockbuff_sync (character, updated_at) VALUES (?, ?)
		 ON CONFLICT(character) DO UPDATE SET updated_at = excluded.updated_at`,
		character, ts); err != nil {
		return false, err
	}
	return true, nil
}

// ObserveAdded records that the server confirmed a block. Returns false when
// the event was older than data already applied.
func (s *Store) ObserveAdded(character string, e Entry, ts int64) (bool, error) {
	return s.observeOne(character, e, ts, true)
}

// ObserveRemoved records that the server confirmed a block is gone (either an
// #allowbuff or a "You are not blocking" reply).
func (s *Store) ObserveRemoved(character string, e Entry, ts int64) (bool, error) {
	return s.observeOne(character, e, ts, false)
}

func (s *Store) observeOne(character string, e Entry, ts int64, present bool) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ok, err := fresh(tx, character, ts)
	if err != nil || !ok {
		return false, err
	}
	if present {
		_, err = tx.Exec(
			`INSERT OR IGNORE INTO blockbuff_entries (character, kind, spell_id, if_spell_id) VALUES (?, ?, ?, ?)`,
			character, kindObserved, e.SpellID, e.IfSpellID)
	} else {
		_, err = tx.Exec(
			`DELETE FROM blockbuff_entries WHERE character = ? AND kind = ? AND spell_id = ? AND if_spell_id = ?`,
			character, kindObserved, e.SpellID, e.IfSpellID)
	}
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}

// ObserveSnapshot replaces the observed list with a complete server listing
// (the output of a bare #blockbuff) and marks the character as synced.
func (s *Store) ObserveSnapshot(character string, entries []Entry, ts int64) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	ok, err := fresh(tx, character, ts)
	if err != nil || !ok {
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM blockbuff_entries WHERE character = ? AND kind = ?`, character, kindObserved); err != nil {
		return false, err
	}
	for _, e := range entries {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO blockbuff_entries (character, kind, spell_id, if_spell_id) VALUES (?, ?, ?, ?)`,
			character, kindObserved, e.SpellID, e.IfSpellID); err != nil {
			return false, err
		}
	}
	if _, err := tx.Exec(`UPDATE blockbuff_sync SET synced_at = ? WHERE character = ?`, ts, character); err != nil {
		return false, err
	}
	return true, tx.Commit()
}
