package middleware

import (
	"net/http"

	"github.com/karljucutan/trapigo/trapigo/internal/features/origin/domain"
)

type OriginValidationMiddleware struct {
	allowedOrigins *domain.AllowedOrigins
}

func NewOriginValidationMiddleware(allowedOrigins *domain.AllowedOrigins) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// NOTE: This is a server-side origin allowlist only. It prevents requests from
			// untrusted origins from reaching the app, but it does not enable browser-enforced
			// CORS for cross-origin JavaScript reads. For that, the server must also return
			// Access-Control-Allow-* response headers and handle preflight OPTIONS requests.
			//
			// Example for browser-enforced CORS (kept commented out intentionally for now):
			// if origin := r.Header.Get("Origin"); origin != "" {
			//     w.Header().Set("Access-Control-Allow-Origin", origin)
			//     w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			//     w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			// }
			// if r.Method == http.MethodOptions {
			//     w.WriteHeader(http.StatusNoContent)
			//     return
			// }
			origin := r.Header.Get("Origin")
			if origin != "" && !allowedOrigins.Validate(origin) {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
