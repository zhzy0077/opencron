# AGENTS.md - Opencron Development Guide

## Project Overview

Opencron is a cron job scheduler with an HTTP API and MCP (Model Context Protocol) server. It persists tasks in SQLite and executes commands on a schedule.

## Build Commands

```bash
# Build the binary
go build -o opencron .

# Run the application
go run .

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a specific test
go test -run TestUpdateTaskCommandViaAPI ./internal/handlers

# Run tests in a specific package
go test -v ./internal/engine

# Run tests with race detector
go test -race ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Tidy go.mod
go mod tidy
```

## Docker

```bash
# Build Docker image
docker build -t opencron .

# Run container
docker run -p 8080:8080 -v /data:/data opencron
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP server port |
| `DATA_DIR` | . | Directory for SQLite DB and logs |
| `API_KEY` | (none) | API key for protected endpoints |
| `LOG_RETENTION_HOURS` | 48 | How long to keep task logs |

## Code Style Guidelines

### General

- Use Go 1.25+ features where appropriate
- Run `go fmt` and `go vet` before committing
- Keep functions small and focused
- Use meaningful variable names

### Package Structure

```
main.go                 # Entry point
internal/
  engine/               # Cron scheduling and task execution
  handlers/             # HTTP handlers and MCP server
  models/               # Data structures
  store/                # SQLite persistence
```

### Imports

Organize imports in three groups with blank lines between:

1. Standard library
2. Third-party packages
3. Internal packages

```go
import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"

	"github.com/opencron/opencron/internal/engine"
	"github.com/opencron/opencron/internal/models"
	"github.com/opencron/opencron/internal/store"
)
```

### Naming Conventions

- **Packages**: lowercase, short names (e.g., `engine`, `store`)
- **Types/Interfaces**: PascalCase (e.g., `Engine`, `Store`, `API`)
- **Variables/Functions**: camelCase (e.g., `runTask`, `dataDir`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **DB columns**: snake_case (use JSON tags for API)

### Error Handling

- Use `fmt.Errorf` with `%w` for wrapped errors
- Use `errors.Is` for error comparison
- Return errors early, avoid nested conditionals
- Log meaningful error messages at call sites

```go
// Good
if err != nil {
    return fmt.Errorf("failed to create store: %w", err)
}

// Good
if errors.Is(err, sql.ErrNoRows) {
    return nil, ErrTaskNotFound
}
```

### Concurrency

- Use `sync.Mutex` for protecting shared state
- Always `defer mu.Unlock()` after `mu.Lock()`
- Use goroutines for background tasks (e.g., log janitor)

```go
func (e *Engine) Reload() {
    e.mu.Lock()
    defer e.mu.Unlock()
    // ... operation
}
```

### Testing

- Place tests in `*_test.go` files in the same package
- Use `t.Helper()` for test utilities
- Use `t.Cleanup()` for resource teardown
- Use `t.TempDir()` for temporary test data
- Test file names match: `cron.go` → `cron_test.go`

```go
func newTestAPI(t *testing.T) *API {
    t.Helper()
    dataDir := t.TempDir()
    s, err := store.New(filepath.Join(dataDir, "test.db"))
    if err != nil {
        t.Fatalf("failed to create store: %v", err)
    }
    e := engine.New(s, dataDir, 48*time.Hour)
    t.Cleanup(func() { _ = s.Close() })
    return &API{Store: s, Engine: e, DataDir: dataDir}
}
```

### HTTP Handlers

- Set appropriate `Content-Type` headers
- Use proper HTTP status codes
- Return errors as JSON where applicable

```go
w.Header().Set("Content-Type", "application/json")
if err != nil {
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}
```

### JSON Serialization

- Use struct tags for JSON field names
- Use pointers for nullable fields (`*string`, `*bool`)
- Snake_case for DB columns, camelCase for JSON

```go
type Task struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Schedule  string    `json:"schedule"`
    Command   string    `json:"command"`
    Enabled   bool      `json:"enabled"`
    OneShot   bool      `json:"one_shot"`
}
```

### Database

- Use `database/sql` with prepared statements
- Always `defer rows.Close()`
- Use transactions for multi-statement operations
- Handle `sql.ErrNoRows` explicitly

### File Permissions

- Use 0644 for files, 0755 for directories
- Use 0600 for sensitive files (keys, credentials)

## Common Patterns

### Config from Environment

```go
dataDir := os.Getenv("DATA_DIR")
if dataDir == "" {
    dataDir = "."
}
```

### Graceful Shutdown (future)

Currently uses `log.Fatalf` for fatal errors. For production, consider signal handling.

### MCP Server

The `/mcp` endpoint implements JSON-RPC 2.0 for tool-based task management. Tools are defined in `handleMCP` in `internal/handlers/api.go`.
