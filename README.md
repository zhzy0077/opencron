# Opencron

Opencron is a minimalist cron service written in Go. It provides a simple web UI and API to manage and monitor cron tasks.

## Features

- **Web UI**: Simple interface to view, create, edit, and delete tasks.
- **API**: JSON API for programmatic access.
- **Persistence**: Tasks are stored in a SQLite database (`opencron.db`).
- **Cron Engine**: Reliable task scheduling using `robfig/cron`.

## Getting Started

1.  **Build**
    ```bash
    go build .
    ```

2.  **Run**
    ```bash
    ./opencron
    ```

3.  **Use**
    Open [http://localhost:8080](http://localhost:8080) in your browser.

## API Endpoints

- `GET /api/tasks`: List all tasks.
- `POST /api/tasks`: Create a new task.
- `PUT /api/tasks/{id}`: Update a task.
- `DELETE /api/tasks/{id}`: Delete a task.

## License

MIT
