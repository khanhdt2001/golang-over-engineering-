package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:19092"), ",")
	var wait sync.WaitGroup
	for _, topic := range topics(envOr("KAFKA_TOPICS", "user-api-calls,task-api-calls")) {
		wait.Go(func() { logTopic(ctx, brokers, topic) })
	}
	wait.Wait()
}

func logTopic(ctx context.Context, brokers []string, topic string) {
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, Partition: 0, StartOffset: kafka.FirstOffset})
	defer reader.Close()
	for {
		message, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("read Kafka message", "topic", topic, "error", err)
			time.Sleep(time.Second)
			continue
		}
		slog.Info("Kafka message", "topic", message.Topic, "partition", message.Partition, "offset", message.Offset, "value", string(message.Value))
	}
}

func topics(value string) []string {
	var result []string
	for _, topic := range strings.Split(value, ",") {
		if topic = strings.TrimSpace(topic); topic != "" {
			result = append(result, topic)
		}
	}
	return result
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
