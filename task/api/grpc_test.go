package api

import (
	"context"
	"testing"

	taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"over-engineering/task/service"
)

type grpcRepository struct{}

func (grpcRepository) Create(_ context.Context, task service.Task) (service.Task, error) {
	task.ID = "22222222-2222-2222-2222-222222222222"
	return task, nil
}
func (grpcRepository) FindByID(context.Context, string) (service.Task, error) {
	return service.Task{}, nil
}
func (grpcRepository) FindByUserID(context.Context, string) ([]service.Task, error) {
	return nil, nil
}
func (grpcRepository) Update(context.Context, string, service.UpdateInput) (service.Task, error) {
	return service.Task{}, nil
}

func TestGRPCCreateTask(t *testing.T) {
	server := NewGRPC(service.New(grpcRepository{}))
	response, err := server.CreateTask(context.Background(), &taskpb.CreateTaskRequest{Name: "Ship", UserId: "11111111-1111-1111-1111-111111111111"})
	if err != nil || response.GetTask().GetId() != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("unexpected create response: %#v, %v", response, err)
	}

	_, err = server.CreateTask(context.Background(), &taskpb.CreateTaskRequest{UserId: "11111111-1111-1111-1111-111111111111"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid request code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}
