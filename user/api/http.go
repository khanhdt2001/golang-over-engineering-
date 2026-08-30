package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"over-engineering/user/service"
)

type handler struct{ service *service.Service }

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

type profileUpdate struct {
	Email    *string `json:"email"`
	Password *string `json:"password"`
	Username *string `json:"username"`
}

type userResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

func New(service *service.Service) http.Handler {
	h := handler{service: service}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/users/sign-up", h.signUp)
	mux.HandleFunc("POST /v1/users/sign-in", h.signIn)
	mux.HandleFunc("PATCH /v1/users/me", h.updateProfile)
	return logRequests(mux)
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
		slog.LogAttrs(r.Context(), level, "request complete", attrs...)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	err         error
}

func (w *responseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (h handler) signUp(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	user, err := h.service.SignUp(r.Context(), service.SignUpInput(input))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response(user))
}

func (h handler) signIn(w http.ResponseWriter, r *http.Request) {
	var input credentials
	if !decodeJSON(w, r, &input) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	user, token, err := h.service.SignIn(r.Context(), input.Email, input.Password)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": response(user)})
}

func (h handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	var input profileUpdate
	if !decodeJSON(w, r, &input) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	user, err := h.service.UpdateProfile(r.Context(), bearerToken(r.Header.Get("Authorization")), service.UpdateInput(input))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response(user))
}

func handleError(w http.ResponseWriter, err error) {
	if writer, ok := w.(*responseWriter); ok {
		writer.err = err
	}
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrEmailExists):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func bearerToken(header string) string {
	return strings.TrimPrefix(strings.TrimSpace(header), "Bearer ")
}
func response(user service.User) userResponse {
	return userResponse{ID: user.ID, Email: user.Email, Username: user.Username}
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
