package main

import (
	"context"
	"testing"
	"time"

	taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
	"google.golang.org/grpc"

	"over-engineering/task/audit"
)

type testPublisher struct {
	event audit.Event
	done  chan struct{}
}

func (p *testPublisher) Publish(_ context.Context, event audit.Event) error {
	p.event = event
	close(p.done)
	return nil
}
func (*testPublisher) Close() error { return nil }

func TestGRPCAuditIncludesInputAndOutput(t *testing.T) {
	publisher := &testPublisher{done: make(chan struct{})}
	interceptor := logGRPCRequests(publisher)
	_, err := interceptor(context.Background(), &taskpb.GetTaskRequest{Id: "task-id"}, &grpc.UnaryServerInfo{FullMethod: "/task.v1.TaskService/GetTask"}, func(context.Context, any) (any, error) {
		return &taskpb.GetTaskResponse{Task: &taskpb.Task{Id: "task-id"}}, nil
	})
	select {
	case <-publisher.done:
	case <-time.After(time.Second):
		t.Fatal("audit event was not published")
	}
	if err != nil || string(publisher.event.Input) != `{"id":"task-id"}` || string(publisher.event.Output) != `{"task":{"id":"task-id"}}` {
		t.Fatalf("unexpected audit event: %#v, %v", publisher.event, err)
	}
}
