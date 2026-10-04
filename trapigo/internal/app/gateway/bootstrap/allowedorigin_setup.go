package bootstrap

import (
	"net/http"

	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	originmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/origin/middleware"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
)

type originMiddlewareComponents struct {
	middleware func(http.Handler) http.Handler
}

func buildOriginMiddlewareComponents(allowedOriginsCfg *config.AllowedOriginsConfig, frontendRedirectURL string) *originMiddlewareComponents {
	allowedOrigins := origininfra.ParseAllowedOrigins(allowedOriginsCfg.List(), frontendRedirectURL)

	middleware := originmiddleware.NewOriginValidationMiddleware(allowedOrigins)

	return &originMiddlewareComponents{
		middleware: middleware,
	}
}
