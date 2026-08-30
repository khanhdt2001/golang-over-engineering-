package db

import (
	"context"
	"database/sql"
	"errors"

	_ "github.com/jackc/pgx/v5/stdlib"

	"over-engineering/task/service"
)

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  user_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
DROP INDEX IF EXISTS tasks_user_id_time_idx;
ALTER TABLE tasks DROP COLUMN IF EXISTS time;
CREATE INDEX IF NOT EXISTS tasks_user_id_id_idx ON tasks (user_id, id);`

type Postgres struct{ db *sql.DB }

func Open(dsn string) (*Postgres, error) {
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, err
	}
	return &Postgres{db: database}, nil
}

func (p *Postgres) Close() error   { return p.db.Close() }
func (p *Postgres) Migrate() error { _, err := p.db.Exec(schema); return err }

func (p *Postgres) Create(ctx context.Context, task service.Task) (service.Task, error) {
	err := p.db.QueryRowContext(ctx, `INSERT INTO tasks (name, description, user_id) VALUES ($1, $2, $3) RETURNING id, name, description, user_id`, task.Name, task.Description, task.UserID).Scan(&task.ID, &task.Name, &task.Description, &task.UserID)
	return task, err
}

func (p *Postgres) FindByID(ctx context.Context, id string) (service.Task, error) {
	var task service.Task
	err := p.db.QueryRowContext(ctx, `SELECT id, name, description, user_id FROM tasks WHERE id = $1`, id).Scan(&task.ID, &task.Name, &task.Description, &task.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Task{}, service.ErrNotFound
	}
	return task, err
}

func (p *Postgres) FindByUserID(ctx context.Context, userID string) ([]service.Task, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT id, name, description, user_id FROM tasks WHERE user_id = $1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]service.Task, 0)
	for rows.Next() {
		var task service.Task
		if err := rows.Scan(&task.ID, &task.Name, &task.Description, &task.UserID); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (p *Postgres) Update(ctx context.Context, id string, input service.UpdateInput) (service.Task, error) {
	var task service.Task
	err := p.db.QueryRowContext(ctx, `UPDATE tasks SET name = COALESCE($1, name), description = COALESCE($2, description), user_id = COALESCE($3, user_id), updated_at = now() WHERE id = $4 RETURNING id, name, description, user_id`, input.Name, input.Description, input.UserID, id).Scan(&task.ID, &task.Name, &task.Description, &task.UserID)
	if errors.Is(err, sql.ErrNoRows) {
		return service.Task{}, service.ErrNotFound
	}
	return task, err
}

var _ service.Repository = (*Postgres)(nil)
