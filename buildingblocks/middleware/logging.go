package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/karljucutan/buildingblocks/correlation"
	"github.com/karljucutan/buildingblocks/errors"
)

// LoggingMiddleware logs HTTP requests with structured logging using slog.
// Captures status code, duration, and any internal errors.
// Request IDs are extracted from X-Request-ID header or generated as UUIDs.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now().UTC()
		requestID := correlation.ExtractOrGenerateRequestID(req)

		correlation.SetRequestIDHeader(rw, requestID)

		wrappedWriter := &responseStatusRecorder{ResponseWriter: rw, statusCode: http.StatusOK}
		next.ServeHTTP(wrappedWriter, req)

		const nanosecondsPerMillisecond = 1000000.0
		elapsed := time.Since(start)

		logLevel := slog.LevelInfo
		if wrappedWriter.statusCode >= http.StatusBadRequest {
			logLevel = slog.LevelWarn
		}
		if wrappedWriter.statusCode >= http.StatusInternalServerError {
			logLevel = slog.LevelError
		}

		attrs := []any{
			"request_id", requestID,
			"time", start.Format(time.RFC3339),
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrappedWriter.statusCode,
			"duration_ms", float64(elapsed.Nanoseconds()) / nanosecondsPerMillisecond,
		}

		if err := errors.InternalErrorFromContext(req.Context()); err != nil {
			attrs = append(attrs, "error", err.Error())
		}

		slog.Log(req.Context(), logLevel, "request completed", attrs...)
	})
}

// responseStatusRecorder wraps http.ResponseWriter to capture the status code.
type responseStatusRecorder struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader records the status code before writing the header.
func (sr *responseStatusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
}

// Write ensures status is set to 200 if not explicitly written, then writes the response.
func (sr *responseStatusRecorder) Write(b []byte) (int, error) {
	if sr.statusCode == 0 {
		sr.statusCode = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}
