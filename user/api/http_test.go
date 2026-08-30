package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
