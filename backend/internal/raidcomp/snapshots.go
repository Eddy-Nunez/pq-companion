package raidcomp

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RosterSnapshot is a saved copy of the live raid roster — who was in the
// raid, at what level/class, in which group — kept as attendance and group
// composition history. Members are stored as a JSON blob: a snapshot is an
// immutable historical record, never queried member-by-member.
type RosterSnapshot struct {
	ID          int64    `json:"id"`
	TakenAt     int64    `json:"taken_at"`
	ZoneID      int      `json:"zone_id,omitempty"`
	Zone        string   `json:"zone,omitempty"`
	Label       string   `json:"label"`
	MemberCount int      `json:"member_count"`
	Members     []Member `json:"members,omitempty"` // omitted in list responses
}

// ErrSnapshotNotFound is returned when a roster snapshot id does not exist.
var ErrSnapshotNotFound = errors.New("raid roster snapshot not found")

// maxSnapshotLabel caps the user-entered label length.
const maxSnapshotLabel = 200

func (s *Store) migrateSnapshots() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS raid_roster_snapshots (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			taken_at     INTEGER NOT NULL,
			zone_id      INTEGER NOT NULL DEFAULT 0,
			zone         TEXT    NOT NULL DEFAULT '',
			label        TEXT    NOT NULL DEFAULT '',
			member_count INTEGER NOT NULL DEFAULT 0,
			members_json TEXT    NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS raid_roster_snapshots_taken_at ON raid_roster_snapshots(taken_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("raidcomp migrate snapshots: %w", err)
		}
	}
	return nil
}

func normalizeSnapshotLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if len(label) > maxSnapshotLabel {
		return "", fmt.Errorf("raidcomp: snapshot label longer than %d characters", maxSnapshotLabel)
	}
	return label, nil
}

// SaveSnapshot persists a roster snapshot and returns it with its new id.
// An empty roster is rejected — there is nothing to record.
func (s *Store) SaveSnapshot(snap Snapshot, label string) (*RosterSnapshot, error) {
	if len(snap.Members) == 0 {
		return nil, errors.New("raidcomp: no raid members to snapshot")
	}
	label, err := normalizeSnapshotLabel(label)
	if err != nil {
		return nil, err
	}
	blob, err := json.Marshal(snap.Members)
	if err != nil {
		return nil, fmt.Errorf("encode snapshot members: %w", err)
	}
	takenAt := time.Now().Unix()
	res, err := s.db.Exec(`
		INSERT INTO raid_roster_snapshots (taken_at, zone_id, zone, label, member_count, members_json)
		VALUES (?, ?, ?, ?, ?, ?)
	`, takenAt, snap.ZoneID, snap.Zone, label, len(snap.Members), string(blob))
	if err != nil {
		return nil, fmt.Errorf("insert roster snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &RosterSnapshot{
		ID: id, TakenAt: takenAt, ZoneID: snap.ZoneID, Zone: snap.Zone,
		Label: label, MemberCount: len(snap.Members), Members: snap.Members,
	}, nil
}

// ListSnapshots returns snapshot summaries (no members), newest first.
func (s *Store) ListSnapshots() ([]RosterSnapshot, error) {
	rows, err := s.db.Query(`
		SELECT id, taken_at, zone_id, zone, label, member_count
		FROM raid_roster_snapshots ORDER BY taken_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RosterSnapshot{}
	for rows.Next() {
		var r RosterSnapshot
		if err := rows.Scan(&r.ID, &r.TakenAt, &r.ZoneID, &r.Zone, &r.Label, &r.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetSnapshot returns one snapshot with its members, or ErrSnapshotNotFound.
func (s *Store) GetSnapshot(id int64) (*RosterSnapshot, error) {
	var r RosterSnapshot
	var blob string
	err := s.db.QueryRow(`
		SELECT id, taken_at, zone_id, zone, label, member_count, members_json
		FROM raid_roster_snapshots WHERE id = ?`, id).
		Scan(&r.ID, &r.TakenAt, &r.ZoneID, &r.Zone, &r.Label, &r.MemberCount, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSnapshotNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(blob), &r.Members); err != nil {
		return nil, fmt.Errorf("decode snapshot %d members: %w", id, err)
	}
	if r.Members == nil {
		r.Members = []Member{}
	}
	return &r, nil
}

// RenameSnapshot updates a snapshot's label.
func (s *Store) RenameSnapshot(id int64, label string) error {
	label, err := normalizeSnapshotLabel(label)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE raid_roster_snapshots SET label = ? WHERE id = ?`, label, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrSnapshotNotFound
	}
	return nil
}

// DeleteSnapshot removes a snapshot. Deleting a missing id is not an error.
func (s *Store) DeleteSnapshot(id int64) error {
	_, err := s.db.Exec(`DELETE FROM raid_roster_snapshots WHERE id = ?`, id)
	return err
}
