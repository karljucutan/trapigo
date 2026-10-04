package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	"golang.org/x/oauth2"
)

type TokenValidator interface {
	Validate(ctx context.Context, token string) (*domain.Claims, error)
}

type RefreshTokenFunc func(ctx context.Context, refreshToken string) (*oauth2.Token, error)

type AuthenticationMiddlewareConfig struct {
	Validator      TokenValidator
	CookieManager  *infrastructure.CookieManager
	RefreshToken   RefreshTokenFunc
	RequiredScopes []string
}

type authClaimsKey struct{}
type authAccessTokenKey struct{}
type authMethodKey struct{}

type authMethod string

const (
	authMethodBearer authMethod = "bearer"
	authMethodCookie authMethod = "cookie"
)

func NewAuthenticationMiddleware(cfg AuthenticationMiddlewareConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			claims, accessToken, method, err := authenticateRequest(rw, req, cfg)
			if err != nil {
				http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(req.Context(), authClaimsKey{}, claims)
			ctx = context.WithValue(ctx, authAccessTokenKey{}, accessToken)
			ctx = context.WithValue(ctx, authMethodKey{}, method)
			next.ServeHTTP(rw, req.WithContext(ctx))
		})
	}
}

func ClaimsFromContext(ctx context.Context) (*domain.Claims, bool) {
	claims, ok := ctx.Value(authClaimsKey{}).(*domain.Claims)
	return claims, ok
}

func AccessTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(authAccessTokenKey{}).(string)
	if !ok || strings.TrimSpace(token) == "" {
		return "", false
	}
	return token, true
}

func WithAccessToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, authAccessTokenKey{}, token)
}

func authenticateRequest(rw http.ResponseWriter, req *http.Request, cfg AuthenticationMiddlewareConfig) (*domain.Claims, string, authMethod, error) {
	if req == nil {
		return nil, "", "", domain.ErrInvalidJWT
	}

	// Prefer explicit bearer auth when the client sends a token in the Authorization header.
	// This is usually the most direct, user-supplied credential and should take precedence over cookies.
	if tokenString := requestBearerToken(req); tokenString != "" {
		claims, err := validateToken(req.Context(), cfg, tokenString)
		if err != nil {
			return nil, "", "", err
		}
		return claims, tokenString, authMethodBearer, nil
	}

	// No bearer token was supplied, so fall back to cookie-based auth.
	if cfg.CookieManager != nil {
		// Read the access token from the session cookie, if one exists.
		if tokenString, ok := cfg.CookieManager.ExtractToken(req, infrastructure.AccessTokenCookieName); ok {
			claims, err := validateToken(req.Context(), cfg, tokenString)
			if err == nil {
				return claims, tokenString, authMethodCookie, nil
			}

			// Only expired access tokens are eligible for refresh. Other validation failures are treated as auth failures.
			if !errors.Is(err, domain.ErrExpiredToken) {
				return nil, "", "", err
			}

			// If the access token expired but there is no refresh strategy configured, the session is invalid.
			if cfg.RefreshToken == nil {
				clearAuthCookies(rw, cfg.CookieManager)
				return nil, "", "", err
			}

			// We need the refresh token to mint a new access token without forcing the user to log in again.
			refreshToken, ok := cfg.CookieManager.ExtractToken(req, infrastructure.RefreshTokenCookieName)
			if !ok {
				clearAuthCookies(rw, cfg.CookieManager)
				return nil, "", "", err
			}

			newToken, refreshErr := cfg.RefreshToken(req.Context(), refreshToken)
			if refreshErr != nil {
				clearAuthCookies(rw, cfg.CookieManager)
				return nil, "", "", refreshErr
			}
			if newToken == nil || strings.TrimSpace(newToken.AccessToken) == "" {
				clearAuthCookies(rw, cfg.CookieManager)
				return nil, "", "", domain.ErrInvalidJWT
			}

			// Refresh succeeded, so rotate the cookies in the response and continue with the new access token.
			setTokenCookies(rw, cfg.CookieManager, newToken)
			claims, validateErr := validateToken(req.Context(), cfg, newToken.AccessToken)
			if validateErr != nil {
				clearAuthCookies(rw, cfg.CookieManager)
				return nil, "", "", validateErr
			}
			return claims, newToken.AccessToken, authMethodCookie, nil
		}
	}

	return nil, "", "", domain.ErrInvalidJWT
}

func requestBearerToken(req *http.Request) string {
	header := req.Header.Get("Authorization")
	if strings.TrimSpace(header) == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func validateToken(ctx context.Context, cfg AuthenticationMiddlewareConfig, tokenString string) (*domain.Claims, error) {
	if cfg.Validator == nil {
		return nil, domain.ErrInvalidJWT
	}
	if strings.TrimSpace(tokenString) == "" {
		return nil, domain.ErrInvalidJWT
	}
	claims, err := cfg.Validator.Validate(ctx, tokenString)
	if err != nil {
		return nil, err
	}
	if claims == nil {
		return nil, domain.ErrInvalidJWT
	}
	if !hasRequiredScopes(claims.Scopes, cfg.RequiredScopes) {
		return nil, domain.ErrInvalidJWT
	}
	return claims, nil
}

func hasRequiredScopes(granted []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := map[string]struct{}{}
	for _, scope := range granted {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			set[scope] = struct{}{}
		}
	}
	for _, needed := range required {
		needed = strings.TrimSpace(needed)
		if needed == "" {
			continue
		}
		if _, ok := set[needed]; !ok {
			return false
		}
	}
	return true
}

func setTokenCookies(rw http.ResponseWriter, manager *infrastructure.CookieManager, token *oauth2.Token) {
	if manager == nil || token == nil {
		return
	}
	if token.AccessToken != "" {
		// Access-token cookie lifetime should be long enough for the browser to keep the cookie
		// while the server can still refresh it before the user is forced to re-authenticate.
		// We intentionally do not tie it exactly to the JWT's exp claim if we want seamless
		// refresh behavior, because the browser will drop the cookie once its own Max-Age expires.
		expiry := time.Until(token.Expiry)
		if expiry <= 0 {
			expiry = 5 * time.Minute
		}
		http.SetCookie(rw, manager.AccessTokenCookie(token.AccessToken, expiry))
	}
	if token.RefreshToken != "" {
		// Refresh token cookie should align with the identity provider's refresh-token lifetime,
		// not with the access-token JWT expiry. In practice, oauth2.Token does not expose the
		// refresh token's own expiration directly, so the app normally uses a configured session TTL
		// or a server-side policy here. Keeping this longer than the access-token cookie is what
		// allows the user to remain signed in until the refresh session expires or logout occurs.
		expiry := 30 * time.Minute
		http.SetCookie(rw, manager.RefreshTokenCookie(token.RefreshToken, expiry))
	}
}

func clearAuthCookies(rw http.ResponseWriter, manager *infrastructure.CookieManager) {
	if manager == nil {
		return
	}
	http.SetCookie(rw, manager.ClearAccessTokenCookie())
	http.SetCookie(rw, manager.ClearRefreshTokenCookie())
}
