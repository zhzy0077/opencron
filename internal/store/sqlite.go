package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/opencron/opencron/internal/models"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func (s *Store) Close() error {
	return s.db.Close()
}

func hasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
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
		one_shot BOOLEAN DEFAULT FALSE,
		created_at DATETIME,
		last_run DATETIME
	);`

	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}

	// Migrate older databases that don't yet have one_shot.
	hasOneShot, err := hasColumn(db, "tasks", "one_shot")
	if err != nil {
		return nil, err
	}
	if !hasOneShot {
		if _, err = db.Exec(`ALTER TABLE tasks ADD COLUMN one_shot BOOLEAN DEFAULT FALSE`); err != nil {
			return nil, err
		}
	}

	return &Store{db: db}, nil
}

func (s *Store) CreateTask(task *models.Task) error {
	task.CreatedAt = time.Now()
	query := `INSERT INTO tasks (name, schedule, command, enabled, one_shot, created_at, last_run) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, task.Name, task.Schedule, task.Command, task.Enabled, task.OneShot, task.CreatedAt, time.Time{})
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
	rows, err := s.db.Query(`SELECT id, name, schedule, command, enabled, one_shot, created_at, last_run FROM tasks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		var lastRun sql.NullTime
		if err := rows.Scan(&t.ID, &t.Name, &t.Schedule, &t.Command, &t.Enabled, &t.OneShot, &t.CreatedAt, &lastRun); err != nil {
			return nil, err
		}
		if lastRun.Valid {
			t.LastRun = lastRun.Time
		}
		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (s *Store) GetTaskByID(id int) (*models.Task, error) {
	row := s.db.QueryRow(`SELECT id, name, schedule, command, enabled, one_shot, created_at, last_run FROM tasks WHERE id=?`, id)

	var t models.Task
	var lastRun sql.NullTime
	if err := row.Scan(&t.ID, &t.Name, &t.Schedule, &t.Command, &t.Enabled, &t.OneShot, &t.CreatedAt, &lastRun); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	if lastRun.Valid {
		t.LastRun = lastRun.Time
	}
	return &t, nil
}

func (s *Store) UpdateTask(task *models.Task) error {
	query := `UPDATE tasks SET name=?, schedule=?, command=?, enabled=?, one_shot=? WHERE id=?`
	_, err := s.db.Exec(query, task.Name, task.Schedule, task.Command, task.Enabled, task.OneShot, task.ID)
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
