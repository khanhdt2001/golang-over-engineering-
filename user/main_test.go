package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"google.golang.org/grpc"

	"over-engineering/user/audit"
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

func TestLogGRPCRequests(t *testing.T) {
	old := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	publisher := &testPublisher{done: make(chan struct{})}
	_, err := logGRPCRequests(publisher)(context.Background(), &userpb.SignUpRequest{Password: "secret-password"}, &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/SignUp"}, func(context.Context, any) (any, error) {
		return nil, errors.New("invalid input")
	})
	if err == nil {
		t.Fatal("expected handler error")
	}
	select {
	case <-publisher.done:
	case <-time.After(time.Second):
		t.Fatal("audit event was not published")
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["level"] != "ERROR" || entry["msg"] != "gRPC request complete" || entry["method"] != "/user.v1.UserService/SignUp" || entry["status"] != "Unknown" || entry["error"] != "invalid input" {
		t.Fatalf("unexpected gRPC request log: %s", logs.String())
	}
	if strings.Contains(string(publisher.event.Input), "secret-password") || !strings.Contains(string(publisher.event.Input), "[REDACTED]") {
		t.Fatalf("unexpected audit event: %#v", publisher.event)
	}
}
