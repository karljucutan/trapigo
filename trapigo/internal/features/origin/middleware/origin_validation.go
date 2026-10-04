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
			origin := r.Header.Get("Origin")
			if origin != "" && !allowedOrigins.Validate(origin) {
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
