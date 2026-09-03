package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"

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

func TestLogRequests(t *testing.T) {
	old := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })

	for _, test := range []struct {
		status int
		level  string
	}{
		{http.StatusCreated, "INFO"},
		{http.StatusConflict, "ERROR"},
		{http.StatusInternalServerError, "ERROR"},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			logs.Reset()
			request := httptest.NewRequest(http.MethodPost, "/v1/users/sign-up", nil)
			logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
			})).ServeHTTP(httptest.NewRecorder(), request)

			var entry map[string]any
			if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["level"] != test.level || entry["msg"] != "request complete" || entry["method"] != http.MethodPost || entry["path"] != request.URL.Path || entry["status"] != float64(test.status) {
				t.Fatalf("unexpected request log: %s", logs.String())
			}
		})
	}

	t.Run("handler error", func(t *testing.T) {
		logs.Reset()
		logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			handleError(w, errors.New("database unavailable"))
		})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

		var entry map[string]any
		if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["error"] != "database unavailable" {
			t.Fatalf("unexpected request log: %s", logs.String())
		}
	})
}

func TestLogRequestsIncludesTraceIDs(t *testing.T) {
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(trace.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(old) })

	var logs bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(oldLogger) })

	handler := otelhttp.NewHandler(logRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})), "test")
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var entry map[string]any
	if err := json.Unmarshal(logs.Bytes(), &entry); err != nil || entry["trace_id"] == "" || entry["span_id"] == "" {
		t.Fatalf("expected trace fields in request log: %s", logs.String())
	}
}

func TestAuditRequestsRedactsSecrets(t *testing.T) {
	publisher := &testPublisher{done: make(chan struct{})}
	handler := auditRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"token":"secret-token","user":{"email":"ada@example.com"}}`))
	}), publisher)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/users/sign-in", bytes.NewBufferString(`{"email":"ada@example.com","password":"secret-password"}`)))
	select {
	case <-publisher.done:
	case <-time.After(time.Second):
		t.Fatal("audit event was not published")
	}
	event := string(publisher.event.Input) + string(publisher.event.Output)
	if strings.Contains(event, "secret-password") || strings.Contains(event, "secret-token") || !strings.Contains(event, "[REDACTED]") {
		t.Fatalf("secrets were not redacted: %s", event)
	}
}
