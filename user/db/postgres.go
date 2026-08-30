package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"

	"over-engineering/user/service"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  username TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_lower_key ON users ((lower(email)));
CREATE TABLE IF NOT EXISTS user_sessions (
  token_hash TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TIMESTAMPTZ NOT NULL
);`

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

func (p *Postgres) CreateUser(ctx context.Context, email, passwordHash, username string) (service.User, error) {
	var user service.User
	err := p.db.QueryRowContext(ctx, `INSERT INTO users (email, password_hash, username) VALUES ($1, $2, $3) RETURNING id, email, username`, email, passwordHash, username).Scan(&user.ID, &user.Email, &user.Username)
	if duplicate(err) {
		return service.User{}, service.ErrEmailExists
	}
	return user, err
}

func (p *Postgres) FindByEmail(ctx context.Context, email string) (service.User, string, error) {
	var user service.User
	var passwordHash string
	err := p.db.QueryRowContext(ctx, `SELECT id, email, username, password_hash FROM users WHERE lower(email) = $1`, email).Scan(&user.ID, &user.Email, &user.Username, &passwordHash)
	return user, passwordHash, err
}

func (p *Postgres) UpdateUser(ctx context.Context, id string, email, username, passwordHash *string) (service.User, error) {
	var user service.User
	err := p.db.QueryRowContext(ctx, `UPDATE users SET email = COALESCE($1, email), username = COALESCE($2, username), password_hash = COALESCE($3, password_hash), updated_at = now() WHERE id = $4 RETURNING id, email, username`, email, username, passwordHash, id).Scan(&user.ID, &user.Email, &user.Username)
	if duplicate(err) {
		return service.User{}, service.ErrEmailExists
	}
	return user, err
}

func (p *Postgres) CreateSession(ctx context.Context, tokenHash, userID string, expiresAt time.Time) error {
	_, err := p.db.ExecContext(ctx, `INSERT INTO user_sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, tokenHash, userID, expiresAt)
	return err
}

func (p *Postgres) FindSessionUser(ctx context.Context, tokenHash string) (string, error) {
	var userID string
	err := p.db.QueryRowContext(ctx, `SELECT user_id FROM user_sessions WHERE token_hash = $1 AND expires_at > now()`, tokenHash).Scan(&userID)
	return userID, err
}

func duplicate(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

var _ service.Repository = (*Postgres)(nil)
