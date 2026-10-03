package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"

	authinfra "github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
	webcommand "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/command"
	webquery "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/query"
	webtransport "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/transporthttp"
	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
)

type authComponents struct {
	handler    *webtransport.AuthHandler
	middleware func(http.Handler) http.Handler
}

func buildAuthComponents(cfg *config.Config) (*authComponents, error) {
	keycloakClient, err := authinfra.NewKeycloakClient(authinfra.Config{
		IssuerURL:    cfg.Keycloak.IssuerURL,
		ClientID:     cfg.Keycloak.ClientID,
		ClientSecret: cfg.Keycloak.ClientSecret,
		RedirectURI:  cfg.Keycloak.RedirectURI,
	}, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("initialize keycloak client: %w", err)
	}

	jwtValidator := authinfra.NewJWTValidatorWithJWKSCacheTTL(
		cfg.Keycloak.IssuerURL,
		cfg.Keycloak.ClientID,
		time.Duration(cfg.Auth.JWKSCacheTTLSec)*time.Second,
		http.DefaultClient,
	)
	cookieManager := authinfra.NewCookieManager(cfg.Auth.CookieSecure, parseSameSite(cfg.Auth.CookieSameSite), "/")
	stateTTL := time.Duration(cfg.Auth.StateExpirationSec) * time.Second
	stateStore := authinfra.NewInMemoryOAuthStateStore(stateTTL)
	allowedOrigins := parseAllowedOrigins(cfg.Auth.FrontendAllowedOrigins, cfg.Auth.FrontendRedirectURL)

	loginCommand := &webcommand.LoginCommand{
		KeycloakClient: keycloakClient,
		StateStore:     stateStore,
		StateTTL:       stateTTL,
	}
	callbackCommand := &webcommand.CallbackCommand{
		KeycloakClient: keycloakClient,
		StateStore:     stateStore,
		Validator:      jwtValidator,
	}
	logoutCommand := &webcommand.LogoutCommand{CookieManager: cookieManager}
	meQuery := &webquery.GetCurrentUserQuery{}
	handler := &webtransport.AuthHandler{
		LoginCommand:    loginCommand,
		CallbackCommand: callbackCommand,
		LogoutCommand:   logoutCommand,
		MeQuery:         meQuery,
		CookieManager:   cookieManager,
		FrontendURL:     cfg.Auth.FrontendRedirectURL,
		AllowedOrigins:  allowedOrigins,
	}

	middleware := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:      jwtValidator,
		CookieManager:  cookieManager,
		AllowedOrigins: allowedOrigins,
		RequiredScopes: cfg.Auth.RequiredScopes,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			return keycloakClient.RefreshToken(ctx, refreshToken)
		},
	})

	return &authComponents{
		handler:    handler,
		middleware: middleware,
	}, nil
}

func parseSameSite(raw string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax":
		fallthrough
	default:
		return http.SameSiteLaxMode
	}
}

func parseAllowedOrigins(raw []string, fallbackFrontendURL string) []string {
	values := []string{}
	for _, origin := range raw {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}

	if len(values) == 0 {
		fallback := strings.TrimSpace(fallbackFrontendURL)
		if fallback != "" {
			values = append(values, fallback)
		}
	}

	return values
}
