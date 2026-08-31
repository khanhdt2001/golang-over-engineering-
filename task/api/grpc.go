package api

import (
	"context"
	"errors"

	taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"over-engineering/task/service"
)

type grpcHandler struct {
	taskpb.UnimplementedTaskServiceServer
	service *service.Service
}

func NewGRPC(service *service.Service) taskpb.TaskServiceServer { return grpcHandler{service: service} }

func (h grpcHandler) CreateTask(ctx context.Context, request *taskpb.CreateTaskRequest) (*taskpb.CreateTaskResponse, error) {
	task, err := h.service.Create(ctx, service.CreateInput{Name: request.GetName(), Description: request.GetDescription(), UserID: request.GetUserId()})
	if err != nil {
		return nil, grpcError(err)
	}
	return &taskpb.CreateTaskResponse{Task: protobufTask(task)}, nil
}

func (h grpcHandler) GetTask(ctx context.Context, request *taskpb.GetTaskRequest) (*taskpb.GetTaskResponse, error) {
	task, err := h.service.GetByID(ctx, request.GetId())
	if err != nil {
		return nil, grpcError(err)
	}
	return &taskpb.GetTaskResponse{Task: protobufTask(task)}, nil
}

func (h grpcHandler) ListUserTasks(ctx context.Context, request *taskpb.ListUserTasksRequest) (*taskpb.ListUserTasksResponse, error) {
	tasks, err := h.service.GetByUserID(ctx, request.GetUserId())
	if err != nil {
		return nil, grpcError(err)
	}
	response := make([]*taskpb.Task, len(tasks))
	for i, task := range tasks {
		response[i] = protobufTask(task)
	}
	return &taskpb.ListUserTasksResponse{Tasks: response}, nil
}

func (h grpcHandler) UpdateTask(ctx context.Context, request *taskpb.UpdateTaskRequest) (*taskpb.UpdateTaskResponse, error) {
	task, err := h.service.Update(ctx, request.GetId(), service.UpdateInput{Name: request.Name, Description: request.Description, UserID: request.UserId})
	if err != nil {
		return nil, grpcError(err)
	}
	return &taskpb.UpdateTaskResponse{Task: protobufTask(task)}, nil
}

func protobufTask(task service.Task) *taskpb.Task {
	return &taskpb.Task{Id: task.ID, Name: task.Name, Description: task.Description, UserId: task.UserID}
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
