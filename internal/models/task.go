package models

import "time"

type Task struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Command   string    `json:"command"`
	Enabled   bool      `json:"enabled"`
	OneShot   bool      `json:"one_shot"`
	CreatedAt time.Time `json:"created_at"`
	LastRun   time.Time `json:"last_run"`
}
