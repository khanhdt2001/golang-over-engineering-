package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailExists        = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidInput       = errors.New("invalid input")
	ErrUnauthorized       = errors.New("unauthorized")
)

type User struct {
	ID       string
	Email    string
	Username string
}

type SignUpInput struct {
	Email    string
	Password string
	Username string
}

type UpdateInput struct {
	Email    *string
	Password *string
	Username *string
}

// Repository is the database boundary. Tests can provide a fake implementation.
type Repository interface {
	CreateUser(context.Context, string, string, string) (User, error)
	FindByEmail(context.Context, string) (User, string, error)
	UpdateUser(context.Context, string, *string, *string, *string) (User, error)
	CreateSession(context.Context, string, string, time.Time) error
	FindSessionUser(context.Context, string) (string, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) SignUp(ctx context.Context, input SignUpInput) (User, error) {
	if !validEmail(input.Email) || !validPassword(input.Password) || !validUsername(input.Username) {
		return User{}, ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(ctx, "hash password failed", "operation", "sign_up", "error", err)
		return User{}, err
	}
	user, err := s.repository.CreateUser(ctx, strings.ToLower(input.Email), string(hash), input.Username)
	if err != nil && !errors.Is(err, ErrEmailExists) {
		slog.ErrorContext(ctx, "create user failed", "operation", "sign_up", "error", err)
	}
	return user, err
}

func (s *Service) SignIn(ctx context.Context, email, password string) (User, string, error) {
	if !validEmail(email) || !validPassword(password) {
		return User{}, "", ErrInvalidInput
	}
	user, hash, err := s.repository.FindByEmail(ctx, strings.ToLower(email))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "find user failed", "operation", "sign_in", "error", err)
		}
		return User{}, "", ErrInvalidCredentials
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := newToken()
	if err != nil {
		slog.ErrorContext(ctx, "create session token failed", "operation", "sign_in", "error", err)
		return User{}, "", err
	}
	if err := s.repository.CreateSession(ctx, tokenHash(token), user.ID, time.Now().Add(24*time.Hour)); err != nil {
		slog.ErrorContext(ctx, "create session failed", "operation", "sign_in", "error", err)
		return User{}, "", err
	}
	return user, token, nil
}

func (s *Service) UpdateProfile(ctx context.Context, token string, input UpdateInput) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	userID, err := s.repository.FindSessionUser(ctx, tokenHash(token))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.ErrorContext(ctx, "find session failed", "operation", "update_profile", "error", err)
		}
		return User{}, ErrUnauthorized
	}
	if !validUpdate(input) {
		return User{}, ErrInvalidInput
	}
	var passwordHash *string
	if input.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*input.Password), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(ctx, "hash password failed", "operation", "update_profile", "error", err)
			return User{}, err
		}
		value := string(hash)
		passwordHash = &value
	}
	if input.Email != nil {
		value := strings.ToLower(*input.Email)
		input.Email = &value
	}
	user, err := s.repository.UpdateUser(ctx, userID, input.Email, input.Username, passwordHash)
	if err != nil && !errors.Is(err, ErrEmailExists) {
		slog.ErrorContext(ctx, "update user failed", "operation", "update_profile", "error", err)
	}
	return user, err
}

func validUpdate(input UpdateInput) bool {
	if input.Email == nil && input.Password == nil && input.Username == nil {
		return false
	}
	return (input.Email == nil || validEmail(*input.Email)) &&
		(input.Password == nil || validPassword(*input.Password)) &&
		(input.Username == nil || validUsername(*input.Username))
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254
}

func validPassword(password string) bool { return len(password) >= 8 && len(password) <= 72 }
func validUsername(username string) bool {
	return len(strings.TrimSpace(username)) >= 1 && len(username) <= 100
}

func newToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
