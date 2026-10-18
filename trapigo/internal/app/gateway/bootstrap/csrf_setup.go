package bootstrap

import (
	"log/slog"
	"net/http"

	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

type csrfMiddlewareComponents struct {
	middleware func(http.Handler) http.Handler
}

func buildCSRFProtectionMiddlewareComponents(allowedOriginsCfg *config.AllowedOriginsConfig, frontendRedirectURL string) *csrfMiddlewareComponents {
	protection := http.NewCrossOriginProtection()
	allowedOrigins := origininfra.ParseAllowedOrigins(allowedOriginsCfg.List(), frontendRedirectURL)
	for _, origin := range allowedOrigins.List() {
		if err := protection.AddTrustedOrigin(origin); err != nil {
			slog.Warn("invalid trusted origin for CSRF protection", "origin", origin, "error", err)
		}
	}

	// return &csrfMiddlewareComponents{
	// 	middleware: protection.Handler,
	// }

	return &csrfMiddlewareComponents{
		middleware: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, span := otel.Tracer("trapigo-gateway").Start(r.Context(), "gateway.csrf_check")
				err := protection.Check(r)
				if err != nil {
					span.SetAttributes(attribute.String("trapigo.policy.outcome", "rejected"))
				} else {
					span.SetAttributes(attribute.String("trapigo.policy.outcome", "passed"))
				}
				span.End()

				if err != nil {
					http.Error(w, err.Error(), http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		},
	}
}
