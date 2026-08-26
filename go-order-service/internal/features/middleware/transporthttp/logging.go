package transporthttp

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type internalErrorContextKey struct{}

func WithInternalError(ctx context.Context, err error) context.Context {
	if err == nil {
		return ctx
	}
	return context.WithValue(ctx, internalErrorContextKey{}, err)
}

func internalErrorFromContext(ctx context.Context) error {
	if val := ctx.Value(internalErrorContextKey{}); val != nil {
		if err, ok := val.(error); ok {
			return err
		}
	}
	return nil
}

// This can be put in a shared module for all services to use.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now().UTC()

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
			"time", start.Format(time.RFC3339),
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrappedWriter.statusCode,
			"duration_ms", float64(elapsed.Nanoseconds()) / nanosecondsPerMillisecond,
		}
		if err := internalErrorFromContext(req.Context()); err != nil {
			attrs = append(attrs, "error", err.Error())
		}

		slog.Log(req.Context(), logLevel,
			"request completed",
			attrs...,
		)
	})
}

type responseStatusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *responseStatusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
}

func (sr *responseStatusRecorder) Write(b []byte) (int, error) {
	if sr.statusCode == 0 {
		sr.statusCode = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}
