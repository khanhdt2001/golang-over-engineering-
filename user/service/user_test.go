package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type fakeRepository struct {
	createdEmail string
	createdHash  string
	createErr    error
	existsErr    error
}

func (f *fakeRepository) CreateUser(_ context.Context, email, passwordHash, _ string) (User, error) {
	f.createdEmail, f.createdHash = email, passwordHash
	if f.createErr != nil {
		return User{}, f.createErr
	}
	return User{ID: "user-1", Email: email, Username: "Ada"}, nil
}
func (*fakeRepository) FindByEmail(context.Context, string) (User, string, error) {
	return User{}, "", nil
}
func (f *fakeRepository) UserExists(context.Context, string) error { return f.existsErr }
func (*fakeRepository) UpdateUser(context.Context, string, *string, *string, *string) (User, error) {
	return User{}, nil
}
func (*fakeRepository) CreateSession(context.Context, string, string, time.Time) error { return nil }
func (*fakeRepository) FindSessionUser(context.Context, string) (string, error)        { return "", nil }

func TestSignUpUsesRepository(t *testing.T) {
	repo := &fakeRepository{}
	_, err := New(repo).SignUp(context.Background(), SignUpInput{Email: "ADA@EXAMPLE.COM", Password: "password", Username: "Ada"})
	if err != nil || repo.createdEmail != "ada@example.com" || bcrypt.CompareHashAndPassword([]byte(repo.createdHash), []byte("password")) != nil {
		t.Fatal("service did not validate, normalize, and hash before calling the repository")
	}
}

func TestSignUpLogsRepositoryError(t *testing.T) {
	old := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	databaseErr := errors.New("database unavailable")
	_, err := New(&fakeRepository{createErr: databaseErr}).SignUp(context.Background(), SignUpInput{Email: "ada@example.com", Password: "password", Username: "Ada"})
	if !errors.Is(err, databaseErr) {
		t.Fatalf("expected repository error, got %v", err)
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["msg"] != "create user failed" || entry["operation"] != "sign_up" || entry["error"] != databaseErr.Error() {
		t.Fatalf("unexpected service log: %s", logs.String())
	}
}

func TestUserExists(t *testing.T) {
	err := New(&fakeRepository{}).UserExists(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := New(&fakeRepository{existsErr: sql.ErrNoRows}).UserExists(context.Background(), "11111111-1111-1111-1111-111111111111"); err != ErrNotFound {
		t.Fatalf("missing user error = %v, want %v", err, ErrNotFound)
	}
}
