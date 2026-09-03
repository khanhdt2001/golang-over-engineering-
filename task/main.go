package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	taskpb "github.com/khanhdt2001/golang-over-engineering-/proto/task/v1"
	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"over-engineering/task/api"
	"over-engineering/task/audit"
	"over-engineering/task/db"
	"over-engineering/task/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if shutdown, err := setupTelemetry(context.Background(), "task-api"); err != nil {
		slog.Error("configure tracing", "error", err)
	} else {
		defer shutdown(context.Background())
	}

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
	userConnection, err := grpc.NewClient(userGRPCAddress, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
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
	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:19092"), ",")
	topic := envOr("KAFKA_TOPIC", "task-api-calls")
	kafkaContext, cancelKafka := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelKafka()
	if err := audit.EnsureTopic(kafkaContext, brokers, topic); err != nil {
		log.Fatal(err)
	}
	publisher := audit.NewKafkaPublisher(brokers, topic)
	defer publisher.Close()
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()), grpc.UnaryInterceptor(logGRPCRequests(publisher)))
	taskpb.RegisterTaskServiceServer(grpcServer, api.NewGRPC(tasks))
	reflection.Register(grpcServer)
	grpcListener, err := net.Listen("tcp", ":9091")
	if err != nil {
		log.Fatal(err)
	}
	go func() { log.Fatal(grpcServer.Serve(grpcListener)) }()

	log.Println("task HTTP API listening on :8081; gRPC API listening on :9091")
	log.Fatal(http.ListenAndServe(":8081", api.NewWithPublisher(tasks, publisher)))
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func logGRPCRequests(publisher audit.Publisher) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
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
		if span := trace.SpanContextFromContext(ctx); span.IsValid() {
			attrs = append(attrs, slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
		}
		slog.LogAttrs(ctx, level, "gRPC request complete", attrs...)
		go publishGRPCAudit(audit.DetachedContext(ctx), publisher, info.FullMethod, request, response, err)
		return response, err
	}
}

func publishGRPCAudit(ctx context.Context, publisher audit.Publisher, operation string, request, response any, err error) {
	output := protobufJSON(response)
	if err != nil {
		output, _ = json.Marshal(map[string]string{"error": err.Error()})
	}
	event := audit.Event{
		Service: "task", Protocol: "grpc", Operation: operation,
		Input: protobufJSON(request), Output: output, Status: status.Code(err).String(), Timestamp: time.Now().UTC(),
	}
	if publishErr := publisher.Publish(ctx, event); publishErr != nil {
		slog.ErrorContext(ctx, "publish task API audit event failed", "error", publishErr)
	}
}

func protobufJSON(value any) json.RawMessage {
	message, ok := value.(proto.Message)
	if !ok {
		return json.RawMessage(`null`)
	}
	data, err := protojson.Marshal(message)
	if err != nil {
		return json.RawMessage(`null`)
	}
	return data
}
