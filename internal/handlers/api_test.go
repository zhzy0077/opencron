package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
)

func newTestAPI(t *testing.T) *API {
	t.Helper()

	dataDir := t.TempDir()
	s, err := store.New(filepath.Join(dataDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	e := engine.New(s, dataDir, 48*time.Hour)
	t.Cleanup(func() {
		_ = s.Close()
	})
	return &API{
		Store:   s,
		Engine:  e,
		DataDir: dataDir,
	}
}

func seedTask(t *testing.T, api *API) models.Task {
	t.Helper()

	task := models.Task{
		Name:     "example",
		Schedule: "* * * * *",
		Command:  "echo before",
		Enabled:  true,
	}
	if err := api.Store.CreateTask(&task); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}
	return task
}

func runnableCommand() string {
	if runtime.GOOS == "windows" {
		return "cmd /c echo opencron"
	}
	return "echo opencron"
}

func TestUpdateTaskCommandViaAPI(t *testing.T) {
	api := newTestAPI(t)
	task := seedTask(t, api)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/tasks/%d", task.ID), bytes.NewBufferString(`{"command":"echo after"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := api.Store.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("failed to read updated task: %v", err)
	}
	if updated.Command != "echo after" {
		t.Fatalf("expected command to be updated, got %q", updated.Command)
	}
	if updated.Name != task.Name {
		t.Fatalf("expected name to stay unchanged, got %q", updated.Name)
	}
	if updated.Schedule != task.Schedule {
		t.Fatalf("expected schedule to stay unchanged, got %q", updated.Schedule)
	}
}

func TestUpdateTaskCommandViaMCP(t *testing.T) {
	api := newTestAPI(t)
	task := seedTask(t, api)

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "update_task",
			"arguments": map[string]interface{}{
				"id":      task.ID,
				"command": "echo mcp",
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := api.Store.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("failed to read updated task: %v", err)
	}
	if updated.Command != "echo mcp" {
		t.Fatalf("expected command to be updated by MCP, got %q", updated.Command)
	}
}

func TestRunTaskNowViaAPI(t *testing.T) {
	api := newTestAPI(t)
	task := seedTask(t, api)
	task.Command = runnableCommand()
	if err := api.Store.UpdateTask(&task); err != nil {
		t.Fatalf("failed to update task command: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/tasks/%d/run", task.ID), nil)
	rec := httptest.NewRecorder()

	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := api.Store.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("failed to read updated task: %v", err)
	}
	if updated.LastRun.IsZero() {
		t.Fatalf("expected last_run to be updated")
	}
}

func TestRunTaskViaMCP(t *testing.T) {
	api := newTestAPI(t)
	task := seedTask(t, api)
	task.Command = runnableCommand()
	if err := api.Store.UpdateTask(&task); err != nil {
		t.Fatalf("failed to update task command: %v", err)
	}

	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "run_task",
			"arguments": map[string]interface{}{
				"id": task.ID,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	updated, err := api.Store.GetTaskByID(task.ID)
	if err != nil {
		t.Fatalf("failed to read updated task: %v", err)
	}
	if updated.LastRun.IsZero() {
		t.Fatalf("expected last_run to be updated by MCP run_task")
	}
}

func TestGetLogsAPI(t *testing.T) {
	api := newTestAPI(t)
	task := seedTask(t, api)

	logsDir := filepath.Join(api.DataDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}

	// Create a legacy log and a daily log
	legacyLog := filepath.Join(logsDir, fmt.Sprintf("task_%d.log", task.ID))
	dailyLog := filepath.Join(logsDir, fmt.Sprintf("task_%d_20260212.log", task.ID))

	if err := os.WriteFile(legacyLog, []byte("legacy content\n"), 0644); err != nil {
		t.Fatalf("failed to write legacy log: %v", err)
	}
	if err := os.WriteFile(dailyLog, []byte("daily content\n"), 0644); err != nil {
		t.Fatalf("failed to write daily log: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/tasks/%d/logs", task.ID), nil)
	rec := httptest.NewRecorder()

	api.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	expected := "legacy content\ndaily content\n"
	if rec.Body.String() != expected {
		t.Fatalf("expected concatenated logs %q, got %q", expected, rec.Body.String())
	}
}
