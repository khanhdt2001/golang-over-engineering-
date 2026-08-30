package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"over-engineering/user/api"
	"over-engineering/user/db"
	"over-engineering/user/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:15432/users?sslmode=disable"
	}
	repository, err := db.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	if err := repository.Migrate(); err != nil {
		log.Fatal(err)
	}

	users := service.New(repository)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPCRequests))
	userpb.RegisterUserServiceServer(grpcServer, api.NewGRPC(users))
	reflection.Register(grpcServer)
	grpcListener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}
	go func() { log.Fatal(grpcServer.Serve(grpcListener)) }()

	log.Println("user HTTP API listening on :8080; gRPC API listening on :9090")
	log.Fatal(http.ListenAndServe(":8080", api.New(users)))
}

func logGRPCRequests(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	response, err := handler(ctx, request)
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
	}
	attrs := []slog.Attr{
		slog.String("method", info.FullMethod),
		slog.String("status", status.Code(err).String()),
		slog.String("duration", time.Since(start).String()),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	slog.LogAttrs(ctx, level, "gRPC request complete", attrs...)
	return response, err
}
