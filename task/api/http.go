package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	"over-engineering/task/audit"
	"over-engineering/task/service"
)

type handler struct{ service *service.Service }

type taskInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      string `json:"user_id"`
}

type taskUpdate struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	UserID      *string `json:"user_id"`
}

type taskResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	UserID      string `json:"user_id"`
}

func New(service *service.Service) http.Handler {
	return newHandler(service, nil)
}

func NewWithPublisher(service *service.Service, publisher audit.Publisher) http.Handler {
	return newHandler(service, publisher)
}

func newHandler(service *service.Service, publisher audit.Publisher) http.Handler {
	h := handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/tasks", h.create)
	mux.HandleFunc("GET /v1/tasks/{id}", h.getByID)
	mux.HandleFunc("GET /v1/users/{userID}/tasks", h.getByUserID)
	mux.HandleFunc("PATCH /v1/tasks/{id}", h.update)
	return otelhttp.NewHandler(auditRequests(logRequests(mux), publisher), "task-api.http")
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	var input taskInput
	if !decodeJSON(w, r, &input) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	task, err := h.service.Create(r.Context(), service.CreateInput(input))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response(task))
}

func (h handler) getByID(w http.ResponseWriter, r *http.Request) {
	task, err := h.service.GetByID(r.Context(), r.PathValue("id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(task))
}

func (h handler) getByUserID(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.GetByUserID(r.Context(), r.PathValue("userID"))
	if err != nil {
		handleError(w, err)
		return
	}
	responses := make([]taskResponse, len(tasks))
	for i, task := range tasks {
		responses[i] = response(task)
	}
	writeJSON(w, http.StatusOK, responses)
}

func (h handler) update(w http.ResponseWriter, r *http.Request) {
	var input taskUpdate
	if !decodeJSON(w, r, &input) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	task, err := h.service.Update(r.Context(), r.PathValue("id"), service.UpdateInput(input))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(task))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		writer := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		level := slog.LevelInfo
		if writer.status >= http.StatusBadRequest {
			level = slog.LevelError
		}
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", writer.status),
			slog.String("duration", time.Since(start).String()),
		}
		if writer.err != nil {
			attrs = append(attrs, slog.String("error", writer.err.Error()))
		}
		if span := trace.SpanContextFromContext(r.Context()); span.IsValid() {
			attrs = append(attrs, slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
		}
		slog.LogAttrs(r.Context(), level, "request complete", attrs...)
	})
}

func auditRequests(next http.Handler, publisher audit.Publisher) http.Handler {
	if publisher == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
		r.Body = io.NopCloser(bytes.NewReader(body))
		writer := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(writer, r)
		requestBody := any(string(body))
		if json.Valid(body) {
			requestBody = json.RawMessage(body)
		}
		input, _ := json.Marshal(map[string]any{
			"method": r.Method, "path": r.URL.Path, "query": r.URL.RawQuery, "body": requestBody,
		})
		output := json.RawMessage(`null`)
		if json.Valid(writer.body.Bytes()) {
			output = writer.body.Bytes()
		} else if writer.body.Len() > 0 {
			output, _ = json.Marshal(map[string]string{"body": writer.body.String()})
		}
		event := audit.Event{Service: "task", Protocol: "http", Operation: r.Method + " " + r.URL.Path, Input: input, Output: output, Status: strconv.Itoa(writer.status), Timestamp: time.Now().UTC()}

		publishContext := audit.DetachedContext(r.Context())
		go func() {
			if err := publisher.Publish(publishContext, event); err != nil {
				slog.ErrorContext(publishContext, "publish task API audit event failed", "error", err)
			}
		}()

	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	err         error
}

type auditResponseWriter struct {
	http.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *auditResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func handleError(w http.ResponseWriter, err error) {
	if writer, ok := w.(*responseWriter); ok {
		writer.err = err
	}
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrUserNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func response(task service.Task) taskResponse {
	return taskResponse{ID: task.ID, Name: task.Name, Description: task.Description, UserID: task.UserID}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(destination) == nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
