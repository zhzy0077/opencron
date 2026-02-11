package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

	e := engine.New(s, dataDir)
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
