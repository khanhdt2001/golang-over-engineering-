package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

type Event struct {
	Service   string          `json:"service"`
	Protocol  string          `json:"protocol"`
	Operation string          `json:"operation"`
	Input     json.RawMessage `json:"input"`
	Output    json.RawMessage `json:"output"`
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
}

type Publisher interface {
	Publish(context.Context, Event) error
	Close() error
}

type KafkaPublisher struct{ writer *kafka.Writer }

func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
	return &KafkaPublisher{writer: &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
	}}
}

func (p *KafkaPublisher) Publish(ctx context.Context, event Event) (err error) {
	ctx, span := otel.Tracer("over-engineering/user/audit").Start(ctx, "kafka.publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(semconv.MessagingSystemKafka, semconv.MessagingOperationName("publish"), semconv.MessagingDestinationName(p.writer.Topic)),
	)
	defer finishSpan(span, &err)
	message, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{Value: message, Time: event.Timestamp})
}

func (p *KafkaPublisher) Close() error { return p.writer.Close() }

// DetachedContext keeps the request trace after its HTTP or gRPC context is canceled.
func DetachedContext(ctx context.Context) context.Context {
	return trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))
}

func finishSpan(span trace.Span, err *error) {
	if *err != nil {
		span.RecordError(*err)
		span.SetStatus(codes.Error, (*err).Error())
	}
	span.End()
}

func EnsureTopic(ctx context.Context, brokers []string, topic string) error {
	if len(brokers) == 0 || brokers[0] == "" || topic == "" {
		return fmt.Errorf("Kafka broker and topic are required")
	}
	bootstrap, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return err
	}
	defer bootstrap.Close()
	controller, err := bootstrap.Controller()
	if err != nil {
		return err
	}
	conn, err := kafka.DialContext(ctx, "tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return err
	}
	defer conn.Close()
	partitions, err := conn.ReadPartitions(topic)
	if err == nil && len(partitions) > 0 {
		return nil
	}
	if err != nil && !errors.Is(err, kafka.UnknownTopicOrPartition) {
		return err
	}
	return conn.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1})
}

func RedactJSON(data []byte) json.RawMessage {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return json.RawMessage(`null`)
	}
	return mustJSON(redact(value))
}

func redact(value any) any {
	switch value := value.(type) {
	case map[string]any:
		for key, field := range value {
			if key == "password" || key == "token" {
				value[key] = "[REDACTED]"
			} else {
				value[key] = redact(field)
			}
		}
	case []any:
		for index, field := range value {
			value[index] = redact(field)
		}
	}
	return value
}

func mustJSON(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}
