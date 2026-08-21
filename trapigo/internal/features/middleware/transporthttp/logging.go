package transporthttp

import (
	"log/slog"
	"net/http"
	"time"
)

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		start := time.Now().UTC()

		wrappedWriter := &statusRecorder{ResponseWriter: rw, statusCode: http.StatusOK}
		next.ServeHTTP(wrappedWriter, req)

		const nanosecondsPerMillisecond = 1000000.0
		elapsed := time.Since(start)
		slog.Info("request completed",
			"time", start.Format(time.RFC3339),
			"method", req.Method,
			"path", req.URL.Path,
			"status", wrappedWriter.statusCode,
			"duration_ms", float64(elapsed.Nanoseconds())/nanosecondsPerMillisecond,
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(statusCode int) {
	sr.statusCode = statusCode
	sr.ResponseWriter.WriteHeader(statusCode)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.statusCode == 0 {
		sr.statusCode = http.StatusOK
	}
	return sr.ResponseWriter.Write(b)
}

// Go generates this routing implicitly for embedded fields:
// func (sr *statusRecorder) Header() http.Header {
//     return sr.ResponseWriter.Header()
// }
