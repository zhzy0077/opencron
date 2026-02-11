package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
)

type API struct {
	Store  *store.Store
	Engine *engine.Engine
}

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
					},
					"required": []string{"name", "schedule", "command"},
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
			err = api.Store.CreateTask(t)
			api.Engine.Reload()
			data, _ := json.Marshal(t)
			content = append(content, map[string]interface{}{"type": "text", "text": "Task created: " + string(data)})
		case "delete_task":
			id := int(args["id"].(float64))
			err = api.Store.DeleteTask(id)
			api.Engine.Reload()
			content = append(content, map[string]interface{}{"type": "text", "text": "Task deleted successfully"})
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
	switch r.Method {
	case "GET":
		tasks, err := api.Store.GetTasks()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(tasks)
	case "POST":
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
		// Quick parse ID from URL /api/tasks/ID
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		id, _ := strconv.Atoi(parts[3])
		var t models.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		t.ID = id
		if err := api.Store.UpdateTask(&t); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.Engine.Reload()
		json.NewEncoder(w).Encode(t)
	case "DELETE":
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid ID", http.StatusBadRequest)
			return
		}
		id, _ := strconv.Atoi(parts[3])
		if err := api.Store.DeleteTask(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.Engine.Reload()
		w.WriteHeader(http.StatusNoContent)
	}
}
