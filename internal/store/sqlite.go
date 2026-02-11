package store

import (
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
	"github.com/opencron/opencron/internal/models"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		schedule TEXT,
		command TEXT,
		enabled BOOLEAN,
		created_at DATETIME,
		last_run DATETIME
	);`

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) CreateTask(task *models.Task) error {
	task.CreatedAt = time.Now()
	query := `INSERT INTO tasks (name, schedule, command, enabled, created_at, last_run) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, task.Name, task.Schedule, task.Command, task.Enabled, task.CreatedAt, time.Time{})
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	task.ID = int(id)
	return nil
}

func (s *Store) GetTasks() ([]models.Task, error) {
	rows, err := s.db.Query(`SELECT id, name, schedule, command, enabled, created_at, last_run FROM tasks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var lastRun sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Schedule, &t.Command, &t.Enabled, &t.CreatedAt, &lastRun); err != nil {
			return nil, err
		}
		if lastRun.Valid {
			t.LastRun = lastRun.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Store) UpdateTask(task *models.Task) error {
	query := `UPDATE tasks SET name=?, schedule=?, command=?, enabled=? WHERE id=?`
	_, err := s.db.Exec(query, task.Name, task.Schedule, task.Command, task.Enabled, task.ID)
	return err
}

func (s *Store) UpdateLastRun(id int, t time.Time) error {
	_, err := s.db.Exec(`UPDATE tasks SET last_run=? WHERE id=?`, t, id)
	return err
}

func (s *Store) DeleteTask(id int) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE id=?`, id)
	return err
}
