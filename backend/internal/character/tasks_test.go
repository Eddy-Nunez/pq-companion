package character

import (
	"errors"
	"testing"
)

func mustSub(t *testing.T, s *Store, taskID int, parent *int, name string) Subtask {
	t.Helper()
	sub, err := s.CreateSubtask(taskID, parent, name)
	if err != nil {
		t.Fatalf("CreateSubtask(%q): %v", name, err)
	}
	return sub
}

func findTask(t *testing.T, s *Store, charID, taskID int) Task {
	t.Helper()
	tasks, err := s.ListTasks(charID)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range tasks {
		if tk.ID == taskID {
			return tk
		}
	}
	t.Fatalf("task %d not listed", taskID)
	return Task{}
}

func TestSubtasksNestToMaxDepth(t *testing.T) {
	s, charID := openTestStore(t)
	task, err := s.CreateTask(charID, "Epic", "")
	if err != nil {
		t.Fatal(err)
	}

	a := mustSub(t, s, task.ID, nil, "a")
	b := mustSub(t, s, task.ID, &a.ID, "b")
	c := mustSub(t, s, task.ID, &b.ID, "c")
	if _, err := s.CreateSubtask(task.ID, &c.ID, "too deep"); !errors.Is(err, ErrSubtaskTooDeep) {
		t.Fatalf("4th subtask level: err = %v, want ErrSubtaskTooDeep", err)
	}

	got := findTask(t, s, charID, task.ID)
	if len(got.Subtasks) != 1 || len(got.Subtasks[0].Children) != 1 ||
		len(got.Subtasks[0].Children[0].Children) != 1 || got.Subtasks[0].Children[0].Children[0].Name != "c" {
		t.Fatalf("tree not built as nested: %+v", got.Subtasks)
	}
	if got.Subtasks[0].Children[0].Children[0].Children == nil {
		t.Error("leaf Children should marshal as [], not null")
	}
}

func TestCreateSubtaskRejectsParentFromAnotherTask(t *testing.T) {
	s, charID := openTestStore(t)
	t1, _ := s.CreateTask(charID, "one", "")
	t2, _ := s.CreateTask(charID, "two", "")
	p := mustSub(t, s, t1.ID, nil, "p")
	if _, err := s.CreateSubtask(t2.ID, &p.ID, "x"); !errors.Is(err, ErrSubtaskNotFound) {
		t.Fatalf("err = %v, want ErrSubtaskNotFound", err)
	}
	missing := 99999
	if _, err := s.CreateSubtask(t1.ID, &missing, "x"); !errors.Is(err, ErrSubtaskNotFound) {
		t.Fatalf("missing parent: err = %v, want ErrSubtaskNotFound", err)
	}
}

func TestSubtaskCompletionCascadesAndRollsUp(t *testing.T) {
	s, charID := openTestStore(t)
	task, _ := s.CreateTask(charID, "Epic", "")
	parent := mustSub(t, s, task.ID, nil, "parent")
	k1 := mustSub(t, s, task.ID, &parent.ID, "k1")
	k2 := mustSub(t, s, task.ID, &parent.ID, "k2")
	gk := mustSub(t, s, task.ID, &k1.ID, "gk")

	// Completing the only grandchild completes k1 but not parent (k2 open).
	if err := s.UpdateSubtask(gk.ID, "gk", true); err != nil {
		t.Fatal(err)
	}
	p := findTask(t, s, charID, task.ID).Subtasks[0]
	if p.Completed || !p.Children[0].Completed {
		t.Fatalf("after gk done: parent=%v k1=%v, want false/true", p.Completed, p.Children[0].Completed)
	}

	// Completing k2 completes the parent.
	if err := s.UpdateSubtask(k2.ID, "k2", true); err != nil {
		t.Fatal(err)
	}
	if p = findTask(t, s, charID, task.ID).Subtasks[0]; !p.Completed {
		t.Fatal("parent should complete once all children are done")
	}

	// Unchecking the parent cascades down to every descendant.
	if err := s.UpdateSubtask(parent.ID, "parent", false); err != nil {
		t.Fatal(err)
	}
	p = findTask(t, s, charID, task.ID).Subtasks[0]
	if p.Completed || p.Children[0].Completed || p.Children[1].Completed || p.Children[0].Children[0].Completed {
		t.Fatalf("uncheck should cascade down: %+v", p)
	}

	// Checking a child then renaming the parent must not wipe that progress.
	if err := s.UpdateSubtask(k2.ID, "k2", true); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateSubtask(parent.ID, "renamed", false); err != nil {
		t.Fatal(err)
	}
	p = findTask(t, s, charID, task.ID).Subtasks[0]
	if p.Name != "renamed" || !p.Children[1].Completed {
		t.Fatalf("rename lost child progress: %+v", p)
	}
}

func TestDeleteSubtaskRemovesDescendantsAndResyncs(t *testing.T) {
	s, charID := openTestStore(t)
	task, _ := s.CreateTask(charID, "Epic", "")
	parent := mustSub(t, s, task.ID, nil, "parent")
	done := mustSub(t, s, task.ID, &parent.ID, "done")
	open := mustSub(t, s, task.ID, &parent.ID, "open")
	mustSub(t, s, task.ID, &open.ID, "deep")
	if err := s.UpdateSubtask(done.ID, "done", true); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteSubtask(open.ID); err != nil {
		t.Fatal(err)
	}
	p := findTask(t, s, charID, task.ID).Subtasks[0]
	if len(p.Children) != 1 || !p.Completed {
		t.Fatalf("after deleting the open branch: children=%d completed=%v, want 1/true", len(p.Children), p.Completed)
	}
	var left int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM character_task_subtasks WHERE name='deep'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatal("descendant 'deep' was orphaned instead of deleted")
	}
}

func TestMigrationAddsParentColumnToExistingTable(t *testing.T) {
	s, _ := openTestStore(t)
	// Simulate a pre-nesting database: drop the column's index and table, then
	// recreate the old shape and re-run the migration.
	for _, q := range []string{
		`DROP INDEX IF EXISTS idx_character_task_subtasks_parent`,
		`DROP TABLE character_task_subtasks`,
		`CREATE TABLE character_task_subtasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, task_id INTEGER NOT NULL,
			name TEXT NOT NULL, completed INTEGER NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0)`,
		`INSERT INTO character_task_subtasks (task_id, name) VALUES (1, 'legacy')`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := s.migrateTasks(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := s.migrateTasks(); err != nil {
		t.Fatalf("migrate is not idempotent: %v", err)
	}
	var parent *int
	if err := s.db.QueryRow(`SELECT parent_id FROM character_task_subtasks WHERE name='legacy'`).Scan(&parent); err != nil || parent != nil {
		t.Fatalf("legacy row should have NULL parent_id: %v %v", parent, err)
	}
}

func TestImportTask(t *testing.T) {
	s, charID := openTestStore(t)
	valid := func() TaskImport {
		var in TaskImport
		in.Format, in.Version = TaskExportFormat, 1
		in.Task.Name = "Cleric Epic"
		in.Task.Steps = []TaskImportStep{
			{Name: "Part 1", Steps: []TaskImportStep{
				{Name: "Get crown", Completed: true},
				{Name: "Turn in", Completed: true},
			}},
			{Name: "Part 2", Completed: true},
			{Name: "Part 3", Steps: []TaskImportStep{{Name: "x"}}},
		}
		return in
	}

	got, err := s.ImportTask(charID, valid())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Cleric Epic" || len(got.Subtasks) != 3 {
		t.Fatalf("imported shape wrong: %+v", got)
	}
	if !got.Subtasks[0].Completed || len(got.Subtasks[0].Children) != 2 {
		t.Errorf("Part 1 should be complete (all children done): %+v", got.Subtasks[0])
	}
	if !got.Subtasks[1].Completed || got.Subtasks[2].Completed {
		t.Errorf("leaf/branch completion wrong: part2=%v part3=%v", got.Subtasks[1].Completed, got.Subtasks[2].Completed)
	}

	tooDeep := valid()
	tooDeep.Task.Steps = []TaskImportStep{{Name: "1", Steps: []TaskImportStep{{Name: "2", Steps: []TaskImportStep{{Name: "3", Steps: []TaskImportStep{{Name: "4"}}}}}}}}
	bad := map[string]TaskImport{
		"wrong format":  func() TaskImport { in := valid(); in.Format = "other"; return in }(),
		"wrong version": func() TaskImport { in := valid(); in.Version = 2; return in }(),
		"no name":       func() TaskImport { in := valid(); in.Task.Name = "  "; return in }(),
		"empty step":    func() TaskImport { in := valid(); in.Task.Steps[0].Name = ""; return in }(),
		"too deep":      tooDeep,
	}
	before := len(mustList(t, s, charID))
	for name, in := range bad {
		if _, err := s.ImportTask(charID, in); !errors.Is(err, ErrInvalidTaskImport) {
			t.Errorf("%s: err = %v, want ErrInvalidTaskImport", name, err)
		}
	}
	if after := len(mustList(t, s, charID)); after != before {
		t.Errorf("failed imports left partial tasks behind: %d -> %d", before, after)
	}
}

func mustList(t *testing.T, s *Store, charID int) []Task {
	t.Helper()
	tasks, err := s.ListTasks(charID)
	if err != nil {
		t.Fatal(err)
	}
	return tasks
}
