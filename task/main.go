package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"over-engineering/task/api"
	"over-engineering/task/db"
	"over-engineering/task/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://app:app@localhost:25432/tasks?sslmode=disable"
	}
	repository, err := db.Open(dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	if err := repository.Migrate(); err != nil {
		log.Fatal(err)
	}

	userGRPCAddress := os.Getenv("USER_GRPC_ADDR")
	if userGRPCAddress == "" {
		userGRPCAddress = "localhost:9090"
	}
	userConnection, err := grpc.NewClient(userGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer userConnection.Close()
	users := userpb.NewUserServiceClient(userConnection)
	tasks := service.New(repository, func(ctx context.Context, id string) error {
		requestContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := users.CheckUser(requestContext, &userpb.CheckUserRequest{Id: id})
		if status.Code(err) == codes.NotFound {
			return service.ErrUserNotFound
		}
		return err
	})
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(logGRPCRequests))
	taskpb.RegisterTaskServiceServer(grpcServer, api.NewGRPC(tasks))
	reflection.Register(grpcServer)
	grpcListener, err := net.Listen("tcp", ":9091")
	if err != nil {
		log.Fatal(err)
	}
	go func() { log.Fatal(grpcServer.Serve(grpcListener)) }()

	log.Println("task HTTP API listening on :8081; gRPC API listening on :9091")
	log.Fatal(http.ListenAndServe(":8081", api.New(tasks)))
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
