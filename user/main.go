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

	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"over-engineering/user/api"
	"over-engineering/user/audit"
	"over-engineering/user/db"
	"over-engineering/user/service"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if shutdown, err := setupTelemetry(context.Background(), "user-api"); err != nil {
		slog.Error("configure tracing", "error", err)
	} else {
		defer shutdown(context.Background())
	}

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
	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:19092"), ",")
	topic := envOr("KAFKA_TOPIC", "user-api-calls")
	kafkaContext, cancelKafka := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelKafka()
	if err := audit.EnsureTopic(kafkaContext, brokers, topic); err != nil {
		log.Fatal(err)
	}
	publisher := audit.NewKafkaPublisher(brokers, topic)
	defer publisher.Close()
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()), grpc.UnaryInterceptor(logGRPCRequests(publisher)))
	userpb.RegisterUserServiceServer(grpcServer, api.NewGRPC(users))
	reflection.Register(grpcServer)
	grpcListener, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatal(err)
	}
	go func() { log.Fatal(grpcServer.Serve(grpcListener)) }()

	log.Println("user HTTP API listening on :8080; gRPC API listening on :9090")
	log.Fatal(http.ListenAndServe(":8080", api.NewWithPublisher(users, publisher)))
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
	event := audit.Event{Service: "user", Protocol: "grpc", Operation: operation, Input: audit.RedactJSON(protobufJSON(request)), Output: audit.RedactJSON(output), Status: status.Code(err).String(), Timestamp: time.Now().UTC()}
	if publishErr := publisher.Publish(ctx, event); publishErr != nil {
		slog.ErrorContext(ctx, "publish user API audit event failed", "error", publishErr)
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
