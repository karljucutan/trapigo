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
	webAuthHandler *webtransport.WebAuthHandler
	middleware     func(http.Handler) http.Handler
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

	loginCommand := &webcommand.LoginCommand{
		IdentityProvider: keycloakClient,
		StateStore:       stateStore,
		StateTTL:         stateTTL,
	}
	callbackCommand := &webcommand.CallbackCommand{
		IdentityProvider: keycloakClient,
		StateStore:       stateStore,
		Validator:        jwtValidator,
	}
	logoutCommand := &webcommand.LogoutCommand{CookieManager: cookieManager, IdentityProvider: keycloakClient}
	meQuery := &webquery.GetCurrentUserQuery{}
	handler := &webtransport.WebAuthHandler{
		LoginCommand:    loginCommand,
		CallbackCommand: callbackCommand,
		LogoutCommand:   logoutCommand,
		MeQuery:         meQuery,
		CookieManager:   cookieManager,
		FrontendURL:     cfg.Auth.FrontendRedirectURL,
	}

	middleware := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:      jwtValidator,
		CookieManager:  cookieManager,
		RequiredScopes: cfg.Auth.RequiredScopes,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			return keycloakClient.RefreshToken(ctx, refreshToken)
		},
	})

	return &authComponents{
		webAuthHandler: handler,
		middleware:     middleware,
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
