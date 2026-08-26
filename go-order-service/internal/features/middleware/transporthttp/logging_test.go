package transporthttp

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLoggingMiddlewareLogsResponseStatus(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	handler := LoggingMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusCreated)
		_, _ = rw.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "method=GET") || !strings.Contains(output, "path=/ping") || !strings.Contains(output, "status=201") {
		t.Fatalf("expected log output to include request method, path, and status, got: %q", output)
	}
}

func TestLoggingMiddlewareLogsErrorResponses(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(previous)

	handler := LoggingMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		*req = *req.WithContext(WithInternalError(req.Context(), errors.New("customer validation failed")))
		http.Error(rw, "customer validation failed", http.StatusBadRequest)
	}))

	req := httptest.NewRequest(http.MethodPost, "/orders", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}

	output := buf.String()
	if !strings.Contains(output, "level=WARN") || !strings.Contains(output, "status=400") || !strings.Contains(output, "customer validation failed") {
		t.Fatalf("expected log output to include warning, status=400, and the actual error message, got: %q", output)
	}
}
