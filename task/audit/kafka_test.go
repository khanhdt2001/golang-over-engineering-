package audit

import (
	"context"
	"testing"
)

func TestEnsureTopicRequiresBrokerAndTopic(t *testing.T) {
	if EnsureTopic(context.Background(), nil, "task-api-calls") == nil || EnsureTopic(context.Background(), []string{"localhost:19092"}, "") == nil {
		t.Fatal("expected missing Kafka configuration to fail")
	}
}
