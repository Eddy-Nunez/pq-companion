package character

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MaxSubtaskDepth is how many levels of subtasks may hang under a task. With
// the task itself that is four levels of nesting. A top-level subtask (no
// parent) is depth 1.
const MaxSubtaskDepth = 3

// Errors callers map to client (4xx) responses.
var (
	ErrSubtaskTooDeep    = errors.New("subtasks cannot be nested that deeply")
	ErrSubtaskNotFound   = errors.New("parent subtask not found in this task")
	ErrInvalidTaskImport = errors.New("invalid task file")
)

// Task is a manual to-do entry attached to a character.
type Task struct {
	ID          int       `json:"id"`
	CharacterID int       `json:"character_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Position    int       `json:"position"`
	Completed   bool      `json:"completed"`
	CreatedAt   int64     `json:"created_at"`
	Subtasks    []Subtask `json:"subtasks"`
}

// Subtask is a checkbox item nested under a Task, or under another Subtask
// (up to MaxSubtaskDepth levels). ParentID is nil for a direct child of the
// task. Task.Subtasks holds only the direct children; deeper levels hang off
// Children.
type Subtask struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	ParentID  *int      `json:"parent_id"`
	Name      string    `json:"name"`
	Completed bool      `json:"completed"`
	Position  int       `json:"position"`
	Children  []Subtask `json:"children"`
}

func (s *Store) migrateTasks() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS character_tasks (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			character_id INTEGER NOT NULL,
			name         TEXT    NOT NULL,
			description  TEXT    NOT NULL DEFAULT '',
			position     INTEGER NOT NULL DEFAULT 0,
			completed    INTEGER NOT NULL DEFAULT 0,
			created_at   INTEGER NOT NULL,
			FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_character_tasks_char_pos ON character_tasks(character_id, position)`,
	); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS character_task_subtasks (
			id        INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id   INTEGER NOT NULL,
			name      TEXT    NOT NULL,
			completed INTEGER NOT NULL DEFAULT 0,
			position  INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY (task_id) REFERENCES character_tasks(id) ON DELETE CASCADE
		)
	`); err != nil {
		return err
	}
	if _, err := s.db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_character_task_subtasks_task_pos ON character_task_subtasks(task_id, position)`,
	); err != nil {
		return err
	}
	// parent_id nests a subtask under another subtask. Added after the table
	// shipped, so existing databases get it via ALTER; NULL = direct child of
	// the task, which is exactly what every pre-existing row is.
	hasParent, err := s.columnExists("character_task_subtasks", "parent_id")
	if err != nil {
		return err
	}
	if !hasParent {
		if _, err := s.db.Exec(`ALTER TABLE character_task_subtasks ADD COLUMN parent_id INTEGER`); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(
		`CREATE INDEX IF NOT EXISTS idx_character_task_subtasks_parent ON character_task_subtasks(task_id, parent_id, position)`,
	); err != nil {
		return err
	}
	return nil
}

// columnExists reports whether table has a column named col.
func (s *Store) columnExists(table, col string) (bool, error) {
	// Table name is a compile-time constant at every call site.
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, col) {
			return true, nil
		}
	}
	return false, rows.Err()
}

// ListTasks returns all tasks for the character ordered by position, with subtasks attached.
func (s *Store) ListTasks(characterID int) ([]Task, error) {
	rows, err := s.db.Query(
		`SELECT id, character_id, name, description, position, completed, created_at
		 FROM character_tasks
		 WHERE character_id = ?
		 ORDER BY position, id`,
		characterID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	idx := make(map[int]int)
	for rows.Next() {
		var t Task
		var completed int
		if err := rows.Scan(&t.ID, &t.CharacterID, &t.Name, &t.Description, &t.Position, &completed, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Completed = completed != 0
		t.Subtasks = []Subtask{}
		idx[t.ID] = len(tasks)
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return tasks, nil
	}

	subRows, err := s.db.Query(
		`SELECT s.id, s.task_id, s.parent_id, s.name, s.completed, s.position
		 FROM character_task_subtasks s
		 JOIN character_tasks t ON t.id = s.task_id
		 WHERE t.character_id = ?
		 ORDER BY s.position, s.id`,
		characterID,
	)
	if err != nil {
		return nil, err
	}
	defer subRows.Close()
	var flat []Subtask
	for subRows.Next() {
		var sub Subtask
		var completed int
		var parent sql.NullInt64
		if err := subRows.Scan(&sub.ID, &sub.TaskID, &parent, &sub.Name, &completed, &sub.Position); err != nil {
			return nil, err
		}
		sub.Completed = completed != 0
		if parent.Valid {
			p := int(parent.Int64)
			sub.ParentID = &p
		}
		flat = append(flat, sub)
	}
	if err := subRows.Err(); err != nil {
		return nil, err
	}
	for taskID, subs := range buildSubtaskTrees(flat) {
		if i, ok := idx[taskID]; ok {
			tasks[i].Subtasks = subs
		}
	}
	return tasks, nil
}

// buildSubtaskTrees turns a position-ordered flat subtask list into the
// direct children of each task, with Children filled in recursively. A row
// whose parent is missing (a dangling parent_id) is treated as top-level
// rather than dropped.
func buildSubtaskTrees(flat []Subtask) map[int][]Subtask {
	present := make(map[int]bool, len(flat))
	for _, s := range flat {
		present[s.ID] = true
	}
	children := make(map[int][]Subtask)
	top := make(map[int][]Subtask)
	for _, s := range flat {
		if s.ParentID != nil && present[*s.ParentID] {
			children[*s.ParentID] = append(children[*s.ParentID], s)
		} else {
			top[s.TaskID] = append(top[s.TaskID], s)
		}
	}
	var attach func(s Subtask, depth int) Subtask
	attach = func(s Subtask, depth int) Subtask {
		s.Children = []Subtask{}
		if depth > MaxSubtaskDepth+1 { // cycle / corrupt data guard
			return s
		}
		for _, c := range children[s.ID] {
			s.Children = append(s.Children, attach(c, depth+1))
		}
		return s
	}
	out := make(map[int][]Subtask, len(top))
	for taskID, subs := range top {
		built := make([]Subtask, 0, len(subs))
		for _, s := range subs {
			built = append(built, attach(s, 1))
		}
		out[taskID] = built
	}
	return out
}

// CreateTask inserts a new task at the end of the character's list.
func (s *Store) CreateTask(characterID int, name, description string) (Task, error) {
	var maxPos sql.NullInt64
	if err := s.db.QueryRow(
		`SELECT MAX(position) FROM character_tasks WHERE character_id = ?`,
		characterID,
	).Scan(&maxPos); err != nil {
		return Task{}, err
	}
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`INSERT INTO character_tasks (character_id, name, description, position, completed, created_at)
		 VALUES (?, ?, ?, ?, 0, ?)`,
		characterID, name, description, pos, now,
	)
	if err != nil {
		return Task{}, fmt.Errorf("create task: %w", err)
	}
	id, _ := res.LastInsertId()
	return Task{
		ID:          int(id),
		CharacterID: characterID,
		Name:        name,
		Description: description,
		Position:    pos,
		Completed:   false,
		CreatedAt:   now,
		Subtasks:    []Subtask{},
	}, nil
}

// UpdateTask replaces name/description/completed for a task.
func (s *Store) UpdateTask(id int, name, description string, completed bool) error {
	completedInt := 0
	if completed {
		completedInt = 1
	}
	_, err := s.db.Exec(
		`UPDATE character_tasks SET name=?, description=?, completed=? WHERE id=?`,
		name, description, completedInt, id,
	)
	return err
}

// DeleteTask removes a task and all its subtasks.
func (s *Store) DeleteTask(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`DELETE FROM character_task_subtasks WHERE task_id=?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM character_tasks WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// ReorderTasks rewrites the position of every task for a character to match the
// supplied ID order. Tasks not present in the list are left at the end.
func (s *Store) ReorderTasks(characterID int, orderedIDs []int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for i, id := range orderedIDs {
		if _, err := tx.Exec(
			`UPDATE character_tasks SET position=? WHERE id=? AND character_id=?`,
			i, id, characterID,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// subtaskDepth returns how deep the subtask id sits (a direct child of the
// task is 1) and the task it belongs to. It walks parent links, bounded so a
// corrupt cycle can't loop forever.
func subtaskDepth(q interface {
	QueryRow(string, ...any) *sql.Row
}, id int) (depth, taskID int, err error) {
	cur := id
	for depth = 1; depth <= MaxSubtaskDepth+2; depth++ {
		var parent sql.NullInt64
		var tid int
		if err = q.QueryRow(
			`SELECT task_id, parent_id FROM character_task_subtasks WHERE id = ?`, cur,
		).Scan(&tid, &parent); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, 0, ErrSubtaskNotFound
			}
			return 0, 0, err
		}
		if cur == id {
			taskID = tid
		}
		if !parent.Valid {
			return depth, taskID, nil
		}
		cur = int(parent.Int64)
	}
	return depth, taskID, ErrSubtaskTooDeep
}

// CreateSubtask inserts a new subtask at the end of its sibling list. parentID
// nests it under another subtask of the same task; nil makes it a direct child
// of the task. Returns ErrSubtaskNotFound / ErrSubtaskTooDeep for a bad parent.
func (s *Store) CreateSubtask(taskID int, parentID *int, name string) (Subtask, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Subtask{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var maxPos sql.NullInt64
	if parentID != nil {
		depth, ptask, err := subtaskDepth(tx, *parentID)
		if err != nil {
			return Subtask{}, err
		}
		if ptask != taskID {
			return Subtask{}, ErrSubtaskNotFound
		}
		if depth+1 > MaxSubtaskDepth {
			return Subtask{}, ErrSubtaskTooDeep
		}
		err = tx.QueryRow(
			`SELECT MAX(position) FROM character_task_subtasks WHERE task_id = ? AND parent_id = ?`,
			taskID, *parentID,
		).Scan(&maxPos)
		if err != nil {
			return Subtask{}, err
		}
	} else if err := tx.QueryRow(
		`SELECT MAX(position) FROM character_task_subtasks WHERE task_id = ? AND parent_id IS NULL`,
		taskID,
	).Scan(&maxPos); err != nil {
		return Subtask{}, err
	}
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	res, err := tx.Exec(
		`INSERT INTO character_task_subtasks (task_id, parent_id, name, completed, position) VALUES (?, ?, ?, 0, ?)`,
		taskID, parentID, name, pos,
	)
	if err != nil {
		return Subtask{}, fmt.Errorf("create subtask: %w", err)
	}
	id, _ := res.LastInsertId()
	// A new (incomplete) child means its ancestors can no longer be complete.
	if err := syncAncestors(tx, parentID); err != nil {
		return Subtask{}, err
	}
	if err := tx.Commit(); err != nil {
		return Subtask{}, err
	}
	return Subtask{ID: int(id), TaskID: taskID, ParentID: parentID, Name: name, Position: pos, Children: []Subtask{}}, nil
}

// syncAncestors re-derives the completed flag of every ancestor starting at
// parentID: a parent with children is complete exactly when all of them are.
func syncAncestors(tx *sql.Tx, parentID *int) error {
	cur := parentID
	for hops := 0; cur != nil && hops <= MaxSubtaskDepth+1; hops++ {
		var total, open int
		if err := tx.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(CASE WHEN completed = 0 THEN 1 ELSE 0 END), 0)
			   FROM character_task_subtasks WHERE parent_id = ?`, *cur,
		).Scan(&total, &open); err != nil {
			return err
		}
		if total > 0 {
			done := 0
			if open == 0 {
				done = 1
			}
			if _, err := tx.Exec(`UPDATE character_task_subtasks SET completed = ? WHERE id = ?`, done, *cur); err != nil {
				return err
			}
		}
		var next sql.NullInt64
		if err := tx.QueryRow(`SELECT parent_id FROM character_task_subtasks WHERE id = ?`, *cur).Scan(&next); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			return err
		}
		if !next.Valid {
			return nil
		}
		n := int(next.Int64)
		cur = &n
	}
	return nil
}

// UpdateSubtask replaces name/completed for a subtask. Changing completed
// cascades to every descendant, and the ancestors' flags are re-derived from
// their children. A rename (completed unchanged) touches nothing else.
func (s *Store) UpdateSubtask(id int, name string, completed bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	completedInt := 0
	if completed {
		completedInt = 1
	}
	var oldCompleted int
	var parent sql.NullInt64
	if err := tx.QueryRow(
		`SELECT completed, parent_id FROM character_task_subtasks WHERE id = ?`, id,
	).Scan(&oldCompleted, &parent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // already gone; matches the old UPDATE-no-rows behaviour
		}
		return err
	}
	if _, err := tx.Exec(
		`UPDATE character_task_subtasks SET name=?, completed=? WHERE id=?`,
		name, completedInt, id,
	); err != nil {
		return err
	}
	if oldCompleted != completedInt {
		if _, err := tx.Exec(
			`WITH RECURSIVE d(id) AS (
			     SELECT id FROM character_task_subtasks WHERE parent_id = ?
			     UNION ALL
			     SELECT s.id FROM character_task_subtasks s JOIN d ON s.parent_id = d.id
			 )
			 UPDATE character_task_subtasks SET completed = ? WHERE id IN (SELECT id FROM d)`,
			id, completedInt,
		); err != nil {
			return err
		}
		if parent.Valid {
			p := int(parent.Int64)
			if err := syncAncestors(tx, &p); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// DeleteSubtask removes a subtask and everything nested under it.
func (s *Store) DeleteSubtask(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	var parent sql.NullInt64
	err = tx.QueryRow(`SELECT parent_id FROM character_task_subtasks WHERE id = ?`, id).Scan(&parent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.Exec(
		`WITH RECURSIVE d(id) AS (
		     SELECT ?
		     UNION ALL
		     SELECT s.id FROM character_task_subtasks s JOIN d ON s.parent_id = d.id
		 )
		 DELETE FROM character_task_subtasks WHERE id IN (SELECT id FROM d)`,
		id,
	); err != nil {
		return err
	}
	if parent.Valid {
		p := int(parent.Int64)
		if err := syncAncestors(tx, &p); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── import ────────────────────────────────────────────────────────────────

// TaskExportFormat identifies a task file produced by the Export button.
const TaskExportFormat = "pq-companion-task"

const (
	maxTaskNameLen   = 200
	maxTaskDescLen   = 4000
	maxImportedSteps = 500
)

// TaskImportStep is one node of an imported/exported step tree.
type TaskImportStep struct {
	Name      string           `json:"name"`
	Completed bool             `json:"completed"`
	Steps     []TaskImportStep `json:"steps"`
}

// TaskImport is the on-disk task file: a versioned wrapper around one task and
// its nested steps. Leaves carry their own completed flag; a step with
// children is complete exactly when all of them are, so a template exported
// without progress imports clean.
type TaskImport struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
	Task    struct {
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Completed   bool             `json:"completed"`
		Steps       []TaskImportStep `json:"steps"`
	} `json:"task"`
}

// ImportTask validates in and creates the task and its whole step tree for the
// character in one transaction, appended after the existing tasks.
func (s *Store) ImportTask(characterID int, in TaskImport) (Task, error) {
	if in.Format != TaskExportFormat {
		return Task{}, fmt.Errorf("%w: not a PQ Companion task file", ErrInvalidTaskImport)
	}
	if in.Version != 1 {
		return Task{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidTaskImport, in.Version)
	}
	name := strings.TrimSpace(in.Task.Name)
	if name == "" || len(name) > maxTaskNameLen {
		return Task{}, fmt.Errorf("%w: task name is required (max %d characters)", ErrInvalidTaskImport, maxTaskNameLen)
	}
	if len(in.Task.Description) > maxTaskDescLen {
		return Task{}, fmt.Errorf("%w: description too long", ErrInvalidTaskImport)
	}
	count := 0
	var validate func(steps []TaskImportStep, depth int) error
	validate = func(steps []TaskImportStep, depth int) error {
		if len(steps) > 0 && depth > MaxSubtaskDepth {
			return fmt.Errorf("%w: steps nest more than %d levels deep", ErrInvalidTaskImport, MaxSubtaskDepth)
		}
		for i := range steps {
			steps[i].Name = strings.TrimSpace(steps[i].Name)
			if steps[i].Name == "" || len(steps[i].Name) > maxTaskNameLen {
				return fmt.Errorf("%w: every step needs a name (max %d characters)", ErrInvalidTaskImport, maxTaskNameLen)
			}
			if count++; count > maxImportedSteps {
				return fmt.Errorf("%w: more than %d steps", ErrInvalidTaskImport, maxImportedSteps)
			}
			if err := validate(steps[i].Steps, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(in.Task.Steps, 1); err != nil {
		return Task{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var maxPos sql.NullInt64
	if err := tx.QueryRow(
		`SELECT MAX(position) FROM character_tasks WHERE character_id = ?`, characterID,
	).Scan(&maxPos); err != nil {
		return Task{}, err
	}
	pos := 0
	if maxPos.Valid {
		pos = int(maxPos.Int64) + 1
	}
	now := time.Now().Unix()
	res, err := tx.Exec(
		`INSERT INTO character_tasks (character_id, name, description, position, completed, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		characterID, name, in.Task.Description, pos, boolToInt(in.Task.Completed), now,
	)
	if err != nil {
		return Task{}, fmt.Errorf("import task: %w", err)
	}
	taskID64, _ := res.LastInsertId()
	taskID := int(taskID64)

	var insert func(steps []TaskImportStep, parent *int) error
	insert = func(steps []TaskImportStep, parent *int) error {
		for i, st := range steps {
			// A step with children is complete iff all its children are; the
			// import is built bottom-up below, so compute it first.
			done := st.Completed
			if len(st.Steps) > 0 {
				done = allStepsDone(st.Steps)
			}
			r, err := tx.Exec(
				`INSERT INTO character_task_subtasks (task_id, parent_id, name, completed, position)
				 VALUES (?, ?, ?, ?, ?)`,
				taskID, parent, st.Name, boolToInt(done), i,
			)
			if err != nil {
				return fmt.Errorf("import step: %w", err)
			}
			if len(st.Steps) > 0 {
				id64, _ := r.LastInsertId()
				id := int(id64)
				if err := insert(st.Steps, &id); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := insert(in.Task.Steps, nil); err != nil {
		return Task{}, err
	}
	if err := tx.Commit(); err != nil {
		return Task{}, err
	}

	tasks, err := s.ListTasks(characterID)
	if err != nil {
		return Task{}, err
	}
	for _, t := range tasks {
		if t.ID == taskID {
			return t, nil
		}
	}
	return Task{}, errors.New("imported task not found")
}

func allStepsDone(steps []TaskImportStep) bool {
	for _, st := range steps {
		if len(st.Steps) > 0 {
			if !allStepsDone(st.Steps) {
				return false
			}
		} else if !st.Completed {
			return false
		}
	}
	return true
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
