package middleware

import (
	"net/http"
	"strings"

	"github.com/karljucutan/trapigo/trapigo/internal/features/origin/domain"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type OriginValidationMiddleware struct {
	allowedOrigins *domain.AllowedOrigins
}

func NewOriginValidationMiddleware(allowedOrigins *domain.AllowedOrigins) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Vary", "Origin")
			origin := r.Header.Get("Origin")
			_, span := otel.Tracer("trapigo-gateway").Start(r.Context(), "gateway.origin_check")
			if origin != "" && !allowedOrigins.Validate(origin) {
				span.SetAttributes(attribute.String("trapigo.policy.outcome", "rejected"))
				span.End()
				http.Error(w, "Origin not allowed", http.StatusForbidden)
				return
			}
			span.SetAttributes(attribute.String("trapigo.policy.outcome", "passed"))
			span.End()

			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Expose-Headers", "Retry-After")

				if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
					w.Header().Add("Vary", "Access-Control-Request-Method")
					w.Header().Add("Vary", "Access-Control-Request-Headers")
					if r.Header.Get("Access-Control-Request-Method") != http.MethodGet {
						http.Error(w, "CORS method not allowed", http.StatusForbidden)
						return
					}
					for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
						header = strings.TrimSpace(header)
						if header != "" && !strings.EqualFold(header, "traceparent") {
							http.Error(w, "CORS header not allowed", http.StatusForbidden)
							return
						}
					}
					w.Header().Set("Access-Control-Allow-Methods", http.MethodGet)
					w.Header().Set("Access-Control-Allow-Headers", "traceparent")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
