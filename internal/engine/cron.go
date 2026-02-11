package engine

import (
	"log"
	"os/exec"
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
}

func New(s *store.Store) *Engine {
	return &Engine{
		cron:    cron.New(),
		store:   s,
		entries: make(map[int]cron.EntryID),
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

		// Basic command execution (shell-like)
		parts := strings.Fields(t.Command)
		if len(parts) == 0 {
			return
		}
		cmd := exec.Command(parts[0], parts[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Task %s failed: %v\nOutput: %s", t.Name, err, string(output))
		} else {
			log.Printf("Task %s finished.\nOutput: %s", t.Name, string(output))
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
