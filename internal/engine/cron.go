package engine

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
	"github.com/robfig/cron/v3"
)

type Engine struct {
	cron         *cron.Cron
	store        *store.Store
	entries      map[int]cron.EntryID
	mu           sync.Mutex
	dataDir      string
	LogRetention time.Duration
}

func New(s *store.Store, dataDir string, retention time.Duration) *Engine {
	return &Engine{
		cron:         cron.New(),
		store:        s,
		entries:      make(map[int]cron.EntryID),
		dataDir:      dataDir,
		LogRetention: retention,
	}
}

func (e *Engine) Start() {
	e.cron.Start()
	e.Reload()
	e.StartLogJanitor()
}

func (e *Engine) StartLogJanitor() {
	// Run log cleanup every hour
	_, _ = e.cron.AddFunc("@hourly", func() {
		e.PurgeOldLogs()
	})
	// Run once at start
	go e.PurgeOldLogs()
}

func (e *Engine) PurgeOldLogs() {
	logsDir := filepath.Join(e.dataDir, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to read logs directory: %v", err)
		}
		return
	}

	cutoff := time.Now().Add(-e.LogRetention)
	purgedCount := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(logsDir, entry.Name())); err == nil {
				purgedCount++
			}
		}
	}

	if purgedCount > 0 {
		log.Printf("Purged %d old log files.", purgedCount)
	}
}

func (e *Engine) Reload() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Clear existing jobs
	for _, entryID := range e.entries {
		e.cron.Remove(entryID)
	}
	e.entries = make(map[int]cron.EntryID)

	tasks, err := e.store.GetTasks()
	if err != nil {
		log.Printf("Failed to load tasks: %v", err)
		return
	}

	for _, t := range tasks {
		if t.Enabled {
			e.addTask(t)
		}
	}
}

func (e *Engine) addTask(t models.Task) {
	entryID, err := e.cron.AddFunc(t.Schedule, func() {
		if _, err := e.runTask(t); err != nil {
			log.Printf("Task %s failed: %v", t.Name, err)
		}
	})

	if err != nil {
		log.Printf("Failed to schedule task %s: %v", t.Name, err)
	} else {
		e.entries[t.ID] = entryID
	}
}

func (e *Engine) RefreshTask(taskID int) {
	e.Reload() // Simplistic approach: reload all on change for now
}

func (e *Engine) RunTaskNow(taskID int) error {
	t, err := e.store.GetTaskByID(taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("task %d not found: %w", taskID, sql.ErrNoRows)
		}
		return err
	}

	_, err = e.runTask(*t)
	return err
}

func (e *Engine) runTask(t models.Task) (deleted bool, err error) {
	log.Printf("Running task %s: %s", t.Name, t.Command)
	now := time.Now()
	if err := e.store.UpdateLastRun(t.ID, now); err != nil {
		log.Printf("Failed to update last_run for task %s (%d): %v", t.Name, t.ID, err)
	}

	logsDir := filepath.Join(e.dataDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create logs directory: %w", err)
	}

	logPath := filepath.Join(logsDir, fmt.Sprintf("task_%d_%s.log", t.ID, now.Format("20060102")))
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open log file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "\n--- Task %s started at %s ---\n", t.Name, now.Format(time.RFC3339))

	if t.Command == "" {
		fmt.Fprintf(f, "--- Task %s failed: empty command ---\n", t.Name)
		return false, fmt.Errorf("empty command")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", t.Command)
	} else {
		cmd = exec.Command("sh", "-c", t.Command)
	}
	cmd.Stdout = f
	cmd.Stderr = f
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(f, "--- Task %s failed: %v ---\n", t.Name, err)
		return false, err
	}

	log.Printf("Task %s finished.", t.Name)
	fmt.Fprintf(f, "--- Task %s finished successfully ---\n", t.Name)

	if t.OneShot {
		if err := e.store.DeleteTask(t.ID); err != nil {
			fmt.Fprintf(f, "--- Failed to delete one-shot task: %v ---\n", err)
			return false, fmt.Errorf("failed to delete one-shot task: %w", err)
		}
		log.Printf("One-shot task %s (%d) deleted after first run.", t.Name, t.ID)
		fmt.Fprintf(f, "--- One-shot task deleted after first run ---\n")
		e.Reload()
		return true, nil
	}

	return false, nil
}
