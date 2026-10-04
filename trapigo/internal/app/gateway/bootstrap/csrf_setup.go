package bootstrap

import (
	"log/slog"
	"net/http"

	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
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

	return &csrfMiddlewareComponents{
		middleware: protection.Handler,
	}
}
