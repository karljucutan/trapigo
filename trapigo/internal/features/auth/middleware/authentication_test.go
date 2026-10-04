package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	originmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/origin/middleware"
	"golang.org/x/oauth2"
)

func TestAuthenticationMiddleware_AllowsBearerToken(t *testing.T) {
	validator, issuer, key := newValidatorForTest(t)
	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: infrastructure.NewCookieManager(true, http.SameSiteLaxMode, "/"),
	})

	token := buildSignedToken(t, issuer, "trapigo", "user-123", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	var seenClaims *domain.Claims
	handler := middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		claims, ok := ClaimsFromContext(req.Context())
		if !ok {
			t.Fatal("expected claims in request context")
		}
		seenClaims = claims
		rw.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
	if seenClaims == nil || seenClaims.Subject != "user-123" {
		t.Fatalf("expected claims for user-123, got %#v", seenClaims)
	}
}

func TestAuthenticationMiddleware_RefreshesExpiredCookieToken(t *testing.T) {
	validator, issuer, key := newValidatorForTest(t)
	cookieManager := infrastructure.NewCookieManager(true, http.SameSiteLaxMode, "/")
	refreshed := false

	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			refreshed = true
			if refreshToken != "refresh-123" {
				t.Fatalf("unexpected refresh token: %s", refreshToken)
			}
			return &oauth2.Token{
				AccessToken:  buildSignedToken(t, issuer, "trapigo", "user-456", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute)),
				RefreshToken: "refresh-123",
				Expiry:       time.Now().Add(5 * time.Minute),
			}, nil
		},
	})

	expiredAccess := buildSignedToken(t, issuer, "trapigo", "user-456", key, time.Now().Add(-time.Minute), time.Now().Add(-2*time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.AddCookie(cookieManager.AccessTokenCookie(expiredAccess, time.Minute))
	req.AddCookie(cookieManager.RefreshTokenCookie("refresh-123", 30*time.Minute))
	res := httptest.NewRecorder()

	handler := middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		claims, ok := ClaimsFromContext(req.Context())
		if !ok {
			t.Fatal("expected claims in request context")
		}
		if claims.Subject != "user-456" {
			t.Fatalf("expected refreshed subject user-456, got %s", claims.Subject)
		}
		rw.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(res, req)

	if !refreshed {
		t.Fatal("expected refresh callback to run")
	}
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if !strings.Contains(res.Header().Get("Set-Cookie"), infrastructure.AccessTokenCookieName) {
		t.Fatalf("expected refreshed access token cookie to be set, got %q", res.Header().Get("Set-Cookie"))
	}
}

func TestAuthenticationMiddleware_Returns401WithoutCredentials(t *testing.T) {
	validator, _, _ := newValidatorForTest(t)
	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: infrastructure.NewCookieManager(true, http.SameSiteLaxMode, "/"),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	res := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Fatal("next handler should not run")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestAuthenticationMiddleware_BearerWinsOverCookie(t *testing.T) {
	validator, issuer, key := newValidatorForTest(t)
	cookieManager := infrastructure.NewCookieManager(true, http.SameSiteLaxMode, "/")

	validBearer := buildSignedToken(t, issuer, "trapigo", "bearer-user", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))
	invalidCookie := "not-a-jwt"

	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			t.Fatal("refresh should not be called when bearer token is present")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+validBearer)
	req.AddCookie(cookieManager.AccessTokenCookie(invalidCookie, time.Minute))
	res := httptest.NewRecorder()

	var subject string
	handler := middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		claims, ok := ClaimsFromContext(req.Context())
		if !ok {
			t.Fatal("expected claims in context")
		}
		subject = claims.Subject
		rw.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
	if subject != "bearer-user" {
		t.Fatalf("expected bearer subject, got %s", subject)
	}
}

func TestAuthenticationMiddleware_RejectsUnsafeCookieRequestFromInvalidOrigin(t *testing.T) {
	validator, issuer, key := newValidatorForTest(t)
	cookieManager := infrastructure.NewCookieManager(true, http.SameSiteLaxMode, "/")
	token := buildSignedToken(t, issuer, "trapigo", "user-123", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))

	authMiddleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
	})
	allowedOrigins := origininfra.ParseAllowedOrigins([]string{"http://localhost:3000"}, "")
	originMiddleware := originmiddleware.NewOriginValidationMiddleware(allowedOrigins)

	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	req.Header.Set("Origin", "http://evil.local")
	req.AddCookie(cookieManager.AccessTokenCookie(token, time.Minute))
	res := httptest.NewRecorder()

	originMiddleware(authMiddleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Fatal("next handler should not run")
	}))).ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestAuthenticationMiddleware_AllowsUnsafeBearerRequestWithoutOriginCheck(t *testing.T) {
	validator, issuer, key := newValidatorForTest(t)
	token := buildSignedToken(t, issuer, "trapigo", "user-123", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))

	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator: validator,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Origin", "http://evil.local")
	res := httptest.NewRecorder()

	middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
}

func TestAuthenticationMiddleware_RejectsMissingRequiredScopes(t *testing.T) {
	middleware := NewAuthenticationMiddleware(AuthenticationMiddlewareConfig{
		Validator: tokenValidatorFunc(func(ctx context.Context, token string) (*domain.Claims, error) {
			return &domain.Claims{Subject: "user-1", Scopes: []string{"profile"}}, nil
		}),
		RequiredScopes: []string{"orders:read"},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer token")
	res := httptest.NewRecorder()

	middleware(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Fatal("request should not proceed without required scopes")
	})).ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

type tokenValidatorFunc func(ctx context.Context, token string) (*domain.Claims, error)

func (f tokenValidatorFunc) Validate(ctx context.Context, token string) (*domain.Claims, error) {
	return f(ctx, token)
}

func newValidatorForTest(t *testing.T) (*infrastructure.JWTValidator, string, *rsa.PrivateKey) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	publicKey := &privateKey.PublicKey
	n := base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes())
	kid := "test-key"

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/protocol/openid-connect/certs" {
			http.NotFound(rw, req)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"keys": []map[string]string{{
				"kid": kid,
				"kty": "RSA",
				"use": "sig",
				"n":   n,
				"e":   e,
			}},
		})
	}))
	t.Cleanup(server.Close)

	validator := infrastructure.NewJWTValidator(server.URL, "trapigo", server.Client())
	return validator, server.URL, privateKey
}

func buildSignedToken(t *testing.T, issuer, clientID, subject string, key *rsa.PrivateKey, expiresAt, notBefore time.Time) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   subject,
		Audience:  jwt.ClaimStrings{clientID},
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		NotBefore: jwt.NewNumericDate(notBefore),
		IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	str, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return str
}
