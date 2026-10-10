package logparser

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupAndPurge(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "eqlog_Test_pq.proj.txt")

	oldLine := "[" + time.Now().AddDate(0, 0, -60).Format("Mon Jan 02 15:04:05 2006") + "] old line"
	newLine := "[" + time.Now().Format("Mon Jan 02 15:04:05 2006") + "] new line"
	original := oldLine + "\n" + newLine + "\n"
	if err := os.WriteFile(logPath, []byte(original), 0644); err != nil {
		t.Fatalf("write log: %v", err)
	}

	backupPath, err := BackupAndPurge(logPath, TrimOptions{KeepDays: 30})
	if err != nil {
		t.Fatalf("BackupAndPurge: %v", err)
	}
	if !strings.HasSuffix(backupPath, ".bak.zip") {
		t.Fatalf("backup path %q does not end in .bak.zip", backupPath)
	}

	// The zip archive should contain the full, unfiltered original content.
	r, err := zip.OpenReader(backupPath)
	if err != nil {
		t.Fatalf("open backup zip: %v", err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("expected 1 entry in backup zip, got %d", len(r.File))
	}
	entry := r.File[0]
	if entry.Name != filepath.Base(logPath) {
		t.Errorf("entry name = %q, want %q", entry.Name, filepath.Base(logPath))
	}

	rc, err := entry.Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}
	if string(got) != original {
		t.Errorf("backup content = %q, want %q", got, original)
	}

	// The live log should be trimmed to only the recent entry.
	trimmed, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read trimmed log: %v", err)
	}
	if strings.Contains(string(trimmed), "old line") {
		t.Errorf("trimmed log still contains old line: %q", trimmed)
	}
	if !strings.Contains(string(trimmed), "new line") {
		t.Errorf("trimmed log missing new line: %q", trimmed)
	}
}

func TestBackupAndPurgeKeepBytes(t *testing.T) {
	ts := "[" + time.Now().Format("Mon Jan 02 15:04:05 2006") + "] "
	lines := []string{ts + "line one", ts + "line two", ts + "line three"}
	original := strings.Join(lines, "\n") + "\n"

	tests := []struct {
		name      string
		keepBytes int64
		want      string
	}{
		{"larger than file keeps everything", 1 << 20, original},
		{"cut on line boundary keeps that line", int64(len(lines[2]) + 1), lines[2] + "\n"},
		{"cut mid-line drops the partial line", int64(len(lines[2]) + 5), lines[2] + "\n"},
		{"too small for any whole line", 3, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "eqlog_Test_pq.proj.txt")
			if err := os.WriteFile(logPath, []byte(original), 0644); err != nil {
				t.Fatalf("write log: %v", err)
			}
			if _, err := BackupAndPurge(logPath, TrimOptions{KeepBytes: tt.keepBytes}); err != nil {
				t.Fatalf("BackupAndPurge: %v", err)
			}
			got, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read trimmed log: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("trimmed = %q, want %q", got, tt.want)
			}
		})
	}
}
