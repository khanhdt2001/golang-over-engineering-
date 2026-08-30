package service

import (
	"context"
	"testing"
)

const userID = "11111111-1111-1111-1111-111111111111"

type fakeRepository struct{ created Task }

func (f *fakeRepository) Create(_ context.Context, task Task) (Task, error) {
	f.created = task
	task.ID = "22222222-2222-2222-2222-222222222222"
	return task, nil
}
func (*fakeRepository) FindByID(context.Context, string) (Task, error)       { return Task{}, nil }
func (*fakeRepository) FindByUserID(context.Context, string) ([]Task, error) { return nil, nil }
func (*fakeRepository) Update(context.Context, string, UpdateInput) (Task, error) {
	return Task{}, nil
}

func TestCreateValidatesAndUsesRepository(t *testing.T) {
	repo := &fakeRepository{}
	_, err := New(repo).Create(context.Background(), CreateInput{Name: "Ship task module", Description: "", UserID: userID})
	if err != nil || repo.created.Name != "Ship task module" || repo.created.UserID != userID {
		t.Fatal("service did not validate and pass the task to the repository")
	}
	if _, err := New(repo).Create(context.Background(), CreateInput{UserID: userID}); err != ErrInvalidInput {
		t.Fatalf("expected invalid input, got %v", err)
	}
}
