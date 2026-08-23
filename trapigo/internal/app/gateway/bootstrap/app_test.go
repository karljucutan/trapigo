package bootstrap

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	middleware "github.com/karljucutan/trapigo/trapigo/internal/features/middleware/transporthttp"
)

func TestLoggingMiddlewareLogsResponseStatus(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)).With("app", "trapigo"))
	defer slog.SetDefault(previous)

	handler := middleware.LoggingMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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
	if !strings.Contains(output, "method=GET") || !strings.Contains(output, "path=/ping") || !strings.Contains(output, "status=201") || !strings.Contains(output, "app=trapigo") {
		t.Fatalf("expected log output to include request method, path, status, and app id, got: %q", output)
	}
}

func TestSetDefaultLoggerUsesConfiguredLogLevel(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = originalStdout
	}()

	setDefaultLogger()
	slog.Debug("debug log should be visible when LOG_LEVEL=DEBUG")

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	if !strings.Contains(string(out), "level=DEBUG") {
		t.Fatalf("expected debug log output, got: %q", string(out))
	}
}
