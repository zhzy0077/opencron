package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
)

func TestLogPurging(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.New(filepath.Join(dataDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	e := New(s, dataDir, 48*time.Hour)

	logsDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create some dummy log files
	now := time.Now()
	oldTime := now.Add(-50 * time.Hour)
	recentTime := now.Add(-10 * time.Hour)

	oldFile := filepath.Join(logsDir, "task_1_20260210.log")
	recentFile := filepath.Join(logsDir, "task_1_20260212.log")

	if err := os.WriteFile(oldFile, []byte("old logs"), 0644); err != nil {
		t.Fatalf("failed to write old file: %v", err)
	}
	if err := os.Chtimes(oldFile, oldTime, oldTime); err != nil {
		t.Fatalf("failed to set old file time: %v", err)
	}

	if err := os.WriteFile(recentFile, []byte("recent logs"), 0644); err != nil {
		t.Fatalf("failed to write recent file: %v", err)
	}
	if err := os.Chtimes(recentFile, recentTime, recentTime); err != nil {
		t.Fatalf("failed to set recent file time: %v", err)
	}

	e.PurgeOldLogs()

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Errorf("expected old file to be purged, but it still exists")
	}

	if _, err := os.Stat(recentFile); err != nil {
		t.Errorf("expected recent file to still exist, but got error: %v", err)
	}
}

func TestRunTaskDailyLogs(t *testing.T) {
	dataDir := t.TempDir()
	s, err := store.New(filepath.Join(dataDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	e := New(s, dataDir, 48*time.Hour)
	task := models.Task{
		ID:      1,
		Name:    "test",
		Command: "echo test",
	}

	_, err = e.runTask(task)
	if err != nil {
		t.Fatalf("runTask failed: %v", err)
	}

	now := time.Now()
	expectedFile := filepath.Join(dataDir, "logs", fmt.Sprintf("task_1_%s.log", now.Format("20060102")))
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected daily log file to exist at %s, but got: %v", expectedFile, err)
	}
}
