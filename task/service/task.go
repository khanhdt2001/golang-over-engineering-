package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
)

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("task not found")
	ErrUserNotFound = errors.New("user not found")
)

type Task struct {
	ID          string
	Name        string
	Description string
	UserID      string
}

type CreateInput struct {
	Name        string
	Description string
	UserID      string
}

type UpdateInput struct {
	Name        *string
	Description *string
	UserID      *string
}

type Repository interface {
	Create(context.Context, Task) (Task, error)
	FindByID(context.Context, string) (Task, error)
	FindByUserID(context.Context, string) ([]Task, error)
	Update(context.Context, string, UpdateInput) (Task, error)
}

type UserExists func(context.Context, string) error

type Service struct {
	repository Repository
	userExists UserExists
}

func New(repository Repository, userExists UserExists) *Service {
	return &Service{repository: repository, userExists: userExists}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Task, error) {
	if !validTask(input.Name, input.Description, input.UserID) {
		return Task{}, ErrInvalidInput
	}
	if err := s.userExists(ctx, input.UserID); err != nil {
		return Task{}, err
	}
	task, err := s.repository.Create(ctx, Task{
		Name: input.Name, Description: input.Description, UserID: input.UserID,
	})
	if err != nil {
		slog.ErrorContext(ctx, "create task failed", "operation", "create_task", "error", err)
	}
	return task, err
}

func (s *Service) GetByID(ctx context.Context, id string) (Task, error) {
	if !validID(id) {
		return Task{}, ErrInvalidInput
	}
	task, err := s.repository.FindByID(ctx, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.ErrorContext(ctx, "find task failed", "operation", "get_task", "error", err)
	}
	return task, err
}

func (s *Service) GetByUserID(ctx context.Context, userID string) ([]Task, error) {
	if !validID(userID) {
		return nil, ErrInvalidInput
	}
	tasks, err := s.repository.FindByUserID(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "find user tasks failed", "operation", "get_user_tasks", "error", err)
	}
	return tasks, err
}

func (s *Service) Update(ctx context.Context, id string, input UpdateInput) (Task, error) {
	if !validID(id) || !validUpdate(input) {
		return Task{}, ErrInvalidInput
	}
	if input.UserID != nil {
		if err := s.userExists(ctx, *input.UserID); err != nil {
			return Task{}, err
		}
	}
	task, err := s.repository.Update(ctx, id, input)
	if err != nil && !errors.Is(err, ErrNotFound) {
		slog.ErrorContext(ctx, "update task failed", "operation", "update_task", "error", err)
	}
	return task, err
}

func validTask(name, description, userID string) bool {
	return validName(name) && validDescription(description) && validID(userID)
}

func validUpdate(input UpdateInput) bool {
	if input.Name == nil && input.Description == nil && input.UserID == nil {
		return false
	}
	return (input.Name == nil || validName(*input.Name)) &&
		(input.Description == nil || validDescription(*input.Description)) &&
		(input.UserID == nil || validID(*input.UserID))
}

func validName(name string) bool               { return strings.TrimSpace(name) != "" && len(name) <= 100 }
func validDescription(description string) bool { return len(description) <= 1000 }

func validID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, char := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}
