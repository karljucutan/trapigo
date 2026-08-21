package bootstrap

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
