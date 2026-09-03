package audit

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestEnsureTopicRequiresBrokerAndTopic(t *testing.T) {
	if EnsureTopic(context.Background(), nil, "user-api-calls") == nil || EnsureTopic(context.Background(), []string{"localhost:19092"}, "") == nil {
		t.Fatal("expected missing Kafka configuration to fail")
	}
}

func TestDetachedContextKeepsTrace(t *testing.T) {
	want := trace.NewSpanContext(trace.SpanContextConfig{TraceID: [16]byte{1}, SpanID: [8]byte{1}, TraceFlags: trace.FlagsSampled})
	got := trace.SpanContextFromContext(DetachedContext(trace.ContextWithSpanContext(context.Background(), want)))
	if got.TraceID() != want.TraceID() || got.SpanID() != want.SpanID() {
		t.Fatal("detached context lost trace")
	}
}
