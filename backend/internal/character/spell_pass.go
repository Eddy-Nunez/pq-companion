package character

import "fmt"

// Spell "pass": a per-character list of spells the player has decided not to
// pursue (a full spellbook, class-unusable spells like cleric Imbue scrolls),
// so the spell checklist stops counting them as missing. Reversible — the
// checklist keeps a "Passed" view to review and un-pass them.

func (s *Store) migrateSpellPass() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS character_spell_pass (
			character_id INTEGER NOT NULL,
			spell_id     INTEGER NOT NULL,
			created_at   INTEGER NOT NULL DEFAULT (strftime('%s','now')),
			PRIMARY KEY (character_id, spell_id),
			FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
		)
	`); err != nil {
		return fmt.Errorf("create character_spell_pass: %w", err)
	}
	return nil
}

// ListSpellPasses returns the spell ids the character has passed on, ordered
// by spell id.
func (s *Store) ListSpellPasses(characterID int) ([]int, error) {
	rows, err := s.db.Query(
		`SELECT spell_id FROM character_spell_pass WHERE character_id = ? ORDER BY spell_id`,
		characterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list spell passes: %w", err)
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan spell pass: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// AddSpellPass marks a spell as passed for the character. Idempotent.
func (s *Store) AddSpellPass(characterID, spellID int) error {
	if _, err := s.db.Exec(
		`INSERT OR IGNORE INTO character_spell_pass (character_id, spell_id) VALUES (?, ?)`,
		characterID, spellID,
	); err != nil {
		return fmt.Errorf("add spell pass: %w", err)
	}
	return nil
}

// RemoveSpellPass un-passes a spell for the character. Idempotent.
func (s *Store) RemoveSpellPass(characterID, spellID int) error {
	if _, err := s.db.Exec(
		`DELETE FROM character_spell_pass WHERE character_id = ? AND spell_id = ?`,
		characterID, spellID,
	); err != nil {
		return fmt.Errorf("remove spell pass: %w", err)
	}
	return nil
}
