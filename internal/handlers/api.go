package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
)

type API struct {
	Store   *store.Store
	Engine  *engine.Engine
	DataDir string
}

type taskUpdateRequest struct {
	Name     *string `json:"name"`
	Schedule *string `json:"schedule"`
	Command  *string `json:"command"`
	Enabled  *bool   `json:"enabled"`
	OneShot  *bool   `json:"one_shot"`
}

func (u taskUpdateRequest) isEmpty() bool {
	return u.Name == nil && u.Schedule == nil && u.Command == nil && u.Enabled == nil && u.OneShot == nil
}

func applyTaskUpdate(t *models.Task, u taskUpdateRequest) {
	if u.Name != nil {
		t.Name = *u.Name
	}
	if u.Schedule != nil {
		t.Schedule = *u.Schedule
	}
	if u.Command != nil {
		t.Command = *u.Command
	}
	if u.Enabled != nil {
		t.Enabled = *u.Enabled
	}
	if u.OneShot != nil {
		t.OneShot = *u.OneShot
	}
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/mcp" {
		apiKey := os.Getenv("API_KEY")
		if apiKey != "" {
			if r.Header.Get("X-API-Key") != apiKey {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
	}

	if strings.HasPrefix(r.URL.Path, "/api/tasks") {
		api.handleTasks(w, r)
		return
	}
	if r.URL.Path == "/mcp" {
		api.handleMCP(w, r)
		return
	}
	// Serve static files for everything else
	fs := http.FileServer(http.Dir("./static"))
	fs.ServeHTTP(w, r)
}

func (api *API) handleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	var req struct {
		JSONRPC string                 `json:"jsonrpc"`
		ID      interface{}            `json:"id"`
		Method  string                 `json:"method"`
		Params  map[string]interface{} `json:"params"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	sendResponse := func(result interface{}) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		})
	}

	switch req.Method {
	case "initialize":
		sendResponse(map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{"name": "opencron", "version": "1.0.0"},
		})

	case "notifications/initialized":
		w.WriteHeader(http.StatusNoContent)

	case "tools/list":
		tools := []map[string]interface{}{
			{
				"name":        "list_tasks",
				"description": "List all scheduled cron tasks",
				"inputSchema": map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			},
			{
				"name":        "create_task",
				"description": "Create a new cron task",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":     map[string]interface{}{"type": "string"},
						"schedule": map[string]interface{}{"type": "string", "description": "Standard cron expression (e.g. * * * * *)"},
						"command":  map[string]interface{}{"type": "string"},
						"enabled":  map[string]interface{}{"type": "boolean"},
						"one_shot": map[string]interface{}{"type": "boolean"},
					},
					"required": []string{"name", "schedule", "command"},
				},
			},
			{
				"name":        "update_task",
				"description": "Update a cron task by ID. Supports partial updates, including command changes.",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id":       map[string]interface{}{"type": "integer"},
						"name":     map[string]interface{}{"type": "string"},
						"schedule": map[string]interface{}{"type": "string", "description": "Standard cron expression (e.g. * * * * *)"},
						"command":  map[string]interface{}{"type": "string"},
						"enabled":  map[string]interface{}{"type": "boolean"},
						"one_shot": map[string]interface{}{"type": "boolean"},
					},
					"required": []string{"id"},
				},
			},
			{
				"name":        "delete_task",
				"description": "Delete a cron task by ID",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "integer"},
					},
					"required": []string{"id"},
				},
			},
			{
				"name":        "run_task",
				"description": "Run a task immediately by ID",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{"type": "integer"},
					},
					"required": []string{"id"},
				},
			},
		}
		sendResponse(map[string]interface{}{"tools": tools})

	case "tools/call":
		toolName := req.Params["name"].(string)
		args := req.Params["arguments"].(map[string]interface{})

		var content []map[string]interface{}
		var err error

		switch toolName {
		case "list_tasks":
			tasks, e := api.Store.GetTasks()
			if e == nil {
				data, _ := json.Marshal(tasks)
				content = append(content, map[string]interface{}{"type": "text", "text": string(data)})
			}
			err = e
		case "create_task":
			t := &models.Task{
				Name:     args["name"].(string),
				Schedule: args["schedule"].(string),
				Command:  args["command"].(string),
				Enabled:  true,
			}
			if val, ok := args["enabled"].(bool); ok {
				t.Enabled = val
			}
			if val, ok := args["one_shot"].(bool); ok {
				t.OneShot = val
			}
			err = api.Store.CreateTask(t)
			api.Engine.Reload()
			data, _ := json.Marshal(t)
			content = append(content, map[string]interface{}{"type": "text", "text": "Task created: " + string(data)})
		case "delete_task":
			id := int(args["id"].(float64))
			err = api.Store.DeleteTask(id)
			api.Engine.Reload()
			content = append(content, map[string]interface{}{"type": "text", "text": "Task deleted successfully"})
		case "run_task":
			idValue, ok := args["id"]
			if !ok {
				err = fmt.Errorf("missing required field: id")
				break
			}
			id, convErr := toInt(idValue)
			if convErr != nil {
				err = convErr
				break
			}
			err = api.Engine.RunTaskNow(id)
			if err != nil {
				break
			}
			content = append(content, map[string]interface{}{"type": "text", "text": fmt.Sprintf("Task %d executed", id)})
		case "update_task":
			idValue, ok := args["id"]
			if !ok {
				err = fmt.Errorf("missing required field: id")
				break
			}

			id, convErr := toInt(idValue)
			if convErr != nil {
				err = convErr
				break
			}

			existing, getErr := api.Store.GetTaskByID(id)
			if getErr != nil {
				if getErr == sql.ErrNoRows {
					err = fmt.Errorf("task %d not found", id)
				} else {
					err = getErr
				}
				break
			}

			updated := false
			if val, ok := args["name"].(string); ok {
				existing.Name = val
				updated = true
			}
			if val, ok := args["schedule"].(string); ok {
				existing.Schedule = val
				updated = true
			}
			if val, ok := args["command"].(string); ok {
				existing.Command = val
				updated = true
			}
			if val, ok := args["enabled"].(bool); ok {
				existing.Enabled = val
				updated = true
			}
			if val, ok := args["one_shot"].(bool); ok {
				existing.OneShot = val
				updated = true
			}
			if !updated {
				err = fmt.Errorf("at least one field to update is required")
				break
			}

			err = api.Store.UpdateTask(existing)
			if err != nil {
				break
			}
			api.Engine.Reload()
			data, _ := json.Marshal(existing)
			content = append(content, map[string]interface{}{"type": "text", "text": "Task updated: " + string(data)})
		default:
			http.Error(w, "Unknown tool", http.StatusNotFound)
			return
		}

		if err != nil {
			sendResponse(map[string]interface{}{
				"isError": true,
				"content": []map[string]interface{}{{"type": "text", "text": err.Error()}},
			})
		} else {
			sendResponse(map[string]interface{}{"content": content})
		}

	default:
		sendResponse(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    -32601,
				"message": "Method not found",
			},
		})
	}
}

func (api *API) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	// parts will be ["api", "tasks"], ["api", "tasks", "ID"], ["api", "tasks", "ID", "logs"], or ["api", "tasks", "ID", "run"]

	switch r.Method {
	case "GET":
		if len(parts) == 2 {
			tasks, err := api.Store.GetTasks()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(tasks)
			return
		}

		if len(parts) == 4 && parts[3] == "logs" {
			id, _ := strconv.Atoi(parts[2])
			logsDir := filepath.Join(api.DataDir, "logs")

			// Pattern to match legacy task_ID.log and daily task_ID_YYYYMMDD.log
			// We use two patterns to be precise and avoid matching task_10 when id is 1
			legacyPath := filepath.Join(logsDir, fmt.Sprintf("task_%d.log", id))
			dailyPattern := filepath.Join(logsDir, fmt.Sprintf("task_%d_*.log", id))
			
			matches, _ := filepath.Glob(dailyPattern)
			if _, err := os.Stat(legacyPath); err == nil {
				matches = append([]string{legacyPath}, matches...)
			}

			if len(matches) == 0 {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("No logs found for this task."))
				return
			}

			// Sort matches to ensure order (lexicographical should work for task_ID_YYYYMMDD.log)
			// task_ID.log (if it exists) will come before task_ID_YYYYMMDD.log because . comes before _
			// Actually _ comes after . in ASCII? Let's check: '.' is 46, '_' is 95.
			// So task_1.log will be before task_1_20260212.log.

			var sb strings.Builder
			for _, match := range matches {
				content, err := os.ReadFile(match)
				if err != nil {
					continue
				}
				sb.Write(content)
			}

			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(sb.String()))
			return
		}
	case "POST":
		if len(parts) == 4 && parts[3] == "run" {
			id, err := strconv.Atoi(parts[2])
			if err != nil {
				http.Error(w, "Invalid ID", http.StatusBadRequest)
				return
			}
			if err := api.Engine.RunTaskNow(id); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					http.Error(w, "Task not found", http.StatusNotFound)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		var t models.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := api.Store.CreateTask(&t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.Engine.Reload()
		json.NewEncoder(w).Encode(t)
	case "PUT":
		fallthrough
	case "PATCH":
		// Parse ID from URL /api/tasks/ID
		if len(parts) < 3 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(parts[2])
		if err != nil {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}

		existing, err := api.Store.GetTaskByID(id)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var update taskUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if update.isEmpty() {
			http.Error(w, "No fields to update", http.StatusBadRequest)
			return
		}

		applyTaskUpdate(existing, update)
		if err := api.Store.UpdateTask(existing); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.Engine.Reload()
		json.NewEncoder(w).Encode(existing)
	case "DELETE":
		if len(parts) < 3 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		id, _ := strconv.Atoi(parts[2])
		if err := api.Store.DeleteTask(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.Engine.Reload()
		w.WriteHeader(http.StatusNoContent)
	}
}

func toInt(v interface{}) (int, error) {
	switch n := v.(type) {
	case int:
		return n, nil
	case int32:
		return int(n), nil
	case int64:
		return int(n), nil
	case float64:
		return int(n), nil
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid numeric id")
		}
		return int(i), nil
	default:
		return 0, fmt.Errorf("invalid id type")
	}
}
