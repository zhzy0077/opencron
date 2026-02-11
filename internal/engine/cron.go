package engine

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
	"github.com/robfig/cron/v3"
)

type Engine struct {
	cron    *cron.Cron
	store   *store.Store
	entries map[int]cron.EntryID
	mu      sync.Mutex
	dataDir string
}

func New(s *store.Store, dataDir string) *Engine {
	return &Engine{
		cron:    cron.New(),
		store:   s,
		entries: make(map[int]cron.EntryID),
		dataDir: dataDir,
	}
}

func (e *Engine) Start() {
	e.cron.Start()
	e.Reload()
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
		log.Printf("Running task %s: %s", t.Name, t.Command)
		e.store.UpdateLastRun(t.ID, time.Now())

		logsDir := filepath.Join(e.dataDir, "logs")
		// Ensure logs directory exists
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			log.Printf("Failed to create logs directory: %v", err)
			return
		}

		logPath := filepath.Join(logsDir, fmt.Sprintf("task_%d.log", t.ID))
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file for task %s: %v", t.Name, err)
			return
		}
		defer f.Close()

		// Write a header for this run
		fmt.Fprintf(f, "\n--- Task %s started at %s ---\n", t.Name, time.Now().Format(time.RFC3339))

		parts := strings.Fields(t.Command)
		if len(parts) == 0 {
			return
		}
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Stdout = f
		cmd.Stderr = f

		if err := cmd.Run(); err != nil {
			log.Printf("Task %s failed: %v", t.Name, err)
			fmt.Fprintf(f, "--- Task %s failed: %v ---\n", t.Name, err)
		} else {
			log.Printf("Task %s finished.", t.Name)
			fmt.Fprintf(f, "--- Task %s finished successfully ---\n", t.Name)
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
