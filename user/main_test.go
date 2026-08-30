package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
)

func TestLogGRPCRequests(t *testing.T) {
	old := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	_, err := logGRPCRequests(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/user.v1.UserService/SignUp"}, func(context.Context, any) (any, error) {
		return nil, errors.New("invalid input")
	})
	if err == nil {
		t.Fatal("expected handler error")
	}

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["level"] != "ERROR" || entry["msg"] != "gRPC request complete" || entry["method"] != "/user.v1.UserService/SignUp" || entry["status"] != "Unknown" || entry["error"] != "invalid input" {
		t.Fatalf("unexpected gRPC request log: %s", logs.String())
	}
}
