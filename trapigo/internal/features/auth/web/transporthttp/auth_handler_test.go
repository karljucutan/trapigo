package transporthttp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/command"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/query"
	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	originmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/origin/middleware"
	"golang.org/x/oauth2"
)

func TestHandleLogin_RedirectsToKeycloak(t *testing.T) {
	stateStore := infrastructure.NewInMemoryOAuthStateStore(10 * time.Minute)
	loginCommand := &command.LoginCommand{
		KeycloakClient: loginClientStub{authURL: "https://id.example.com/login"},
		StateStore:     stateStore,
	}
	handler := &WebAuthHandler{
		LoginCommand:  loginCommand,
		CookieManager: infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	req := httptest.NewRequest(http.MethodGet, "/web/auth/login", nil)
	res := httptest.NewRecorder()
	handler.HandleLogin(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, res.Code)
	}
	if res.Header().Get("Location") != "https://id.example.com/login" {
		t.Fatalf("unexpected redirect URL: %s", res.Header().Get("Location"))
	}
}

func TestHandleLogin_RejectsExternalReturnURL(t *testing.T) {
	handler := &WebAuthHandler{FrontendURL: "http://localhost:3000"}
	req := httptest.NewRequest(http.MethodGet, "/web/auth/login?return_to=https%3A%2F%2Fevil.example", nil)
	res := httptest.NewRecorder()
	handler.HandleLogin(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", res.Code)
	}
}

func TestHandleCallback_WithValidCodeAndState_SetsCookiesAndRedirects(t *testing.T) {
	validator, issuer, key := newValidatorForHandlerTest(t)
	stateStore := infrastructure.NewInMemoryOAuthStateStore(10 * time.Minute)
	if err := stateStore.Save(context.Background(), &domain.OAuthState{
		State:        "state-123",
		Nonce:        "nonce-123",
		CodeVerifier: "verifier-123",
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	callbackCommand := &command.CallbackCommand{
		KeycloakClient: callbackClientStub{token: &oauth2.Token{
			AccessToken:  buildSignedToken(t, issuer, "trapigo", key, time.Now().Add(10*time.Minute), time.Now().Add(-time.Minute)),
			RefreshToken: "refresh-123",
			Expiry:       time.Now().Add(10 * time.Minute),
		}},
		StateStore: stateStore,
		Validator:  validator,
	}
	cookieManager := infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/")
	handler := &WebAuthHandler{
		CallbackCommand: callbackCommand,
		CookieManager:   cookieManager,
		FrontendURL:     "http://localhost:3000",
	}

	req := httptest.NewRequest(http.MethodGet, "/web/auth/callback?code=ok&state=state-123", nil)
	res := httptest.NewRecorder()
	handler.HandleCallback(res, req)

	if res.Code != http.StatusFound {
		t.Fatalf("expected %d, got %d", http.StatusFound, res.Code)
	}
	if res.Header().Get("Location") != "http://localhost:3000" {
		t.Fatalf("unexpected frontend redirect: %s", res.Header().Get("Location"))
	}
	cookies := res.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected auth cookies to be set, got %d", len(cookies))
	}
}

func TestHandleLoginAndCallback_ReturnsToDashboard(t *testing.T) {
	validator, issuer, key := newValidatorForHandlerTest(t)
	stateStore := infrastructure.NewInMemoryOAuthStateStore(10 * time.Minute)
	handler := &WebAuthHandler{
		LoginCommand: &command.LoginCommand{
			KeycloakClient: stateReturningLoginClient{},
			StateStore:     stateStore,
		},
		CallbackCommand: &command.CallbackCommand{
			KeycloakClient: callbackClientStub{token: &oauth2.Token{
				AccessToken: buildSignedToken(t, issuer, "trapigo", key, time.Now().Add(10*time.Minute), time.Now().Add(-time.Minute)),
				Expiry:      time.Now().Add(10 * time.Minute),
			}},
			StateStore: stateStore,
			Validator:  validator,
		},
		CookieManager: infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
		FrontendURL:   "http://localhost:3000",
	}
	const returnURL = "http://localhost:3000/dashboard?tab=logs#latest"
	loginRes := httptest.NewRecorder()
	handler.HandleLogin(loginRes, httptest.NewRequest(http.MethodGet, "/web/auth/login?return_to="+url.QueryEscape(returnURL), nil))
	if loginRes.Code != http.StatusFound {
		t.Fatalf("login returned %d: %s", loginRes.Code, loginRes.Body.String())
	}
	authURL, err := url.Parse(loginRes.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := authURL.Query().Get("state")
	callbackPath := "/web/auth/callback?code=ok&state=" + url.QueryEscape(state)
	callbackRes := httptest.NewRecorder()
	handler.HandleCallback(callbackRes, httptest.NewRequest(http.MethodGet, callbackPath, nil))
	if callbackRes.Code != http.StatusFound || callbackRes.Header().Get("Location") != returnURL {
		t.Fatalf("callback returned %d, redirect %q; want %q", callbackRes.Code, callbackRes.Header().Get("Location"), returnURL)
	}
	if len(callbackRes.Result().Cookies()) == 0 {
		t.Fatal("expected an authentication cookie")
	}
	replayRes := httptest.NewRecorder()
	handler.HandleCallback(replayRes, httptest.NewRequest(http.MethodGet, callbackPath, nil))
	if replayRes.Code != http.StatusUnauthorized {
		t.Fatalf("callback replay returned %d, want unauthorized", replayRes.Code)
	}
}

type stateReturningLoginClient struct{}

func (stateReturningLoginClient) BuildAuthorizationURL(_ context.Context, params infrastructure.AuthorizationURLParams) (string, error) {
	return "https://id.example.com/login?state=" + url.QueryEscape(params.State), nil
}

func TestHandleCallback_InvalidOrExpiredState_ReturnsUnauthorized(t *testing.T) {
	validator, _, _ := newValidatorForHandlerTest(t)
	stateStore := infrastructure.NewInMemoryOAuthStateStore(50 * time.Millisecond)
	if err := stateStore.Save(context.Background(), &domain.OAuthState{
		State:        "state-exp",
		Nonce:        "nonce-exp",
		CodeVerifier: "verifier-exp",
		CreatedAt:    time.Now().Add(-time.Minute).UTC(),
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	callbackCommand := &command.CallbackCommand{
		KeycloakClient: callbackClientStub{token: &oauth2.Token{AccessToken: "token"}},
		StateStore:     stateStore,
		Validator:      validator,
	}
	handler := &WebAuthHandler{
		CallbackCommand: callbackCommand,
		CookieManager:   infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	res1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/web/auth/callback?code=ok&state=missing", nil)
	handler.HandleCallback(res1, req1)
	if res1.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for missing state, got %d", res1.Code)
	}

	time.Sleep(60 * time.Millisecond)
	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/web/auth/callback?code=ok&state=state-exp", nil)
	handler.HandleCallback(res2, req2)
	if res2.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for expired state, got %d", res2.Code)
	}
}

func TestHandleLogout_ClearsCookies(t *testing.T) {
	handler := &WebAuthHandler{
		LogoutCommand: &command.LogoutCommand{},
		CookieManager: infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	req := httptest.NewRequest(http.MethodPost, "/web/auth/logout", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.AddCookie(&http.Cookie{Name: infrastructure.RefreshTokenCookieName, Value: "refresh-123"})
	res := httptest.NewRecorder()
	allowedOrigins := origininfra.ParseAllowedOrigins([]string{"http://localhost:3000"}, "")
	originMiddleware := originmiddleware.NewOriginValidationMiddleware(allowedOrigins)
	originMiddleware(http.HandlerFunc(handler.HandleLogout)).ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
	body := res.Body.String()
	if !strings.Contains(body, "logged_out") {
		t.Fatalf("unexpected body: %s", body)
	}
	cookies := res.Result().Cookies()
	if len(cookies) < 2 {
		t.Fatalf("expected clear cookies, got %d", len(cookies))
	}
}

func TestHandleLogout_RejectsInvalidOrigin(t *testing.T) {
	handler := &WebAuthHandler{
		LogoutCommand: &command.LogoutCommand{},
		CookieManager: infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	req := httptest.NewRequest(http.MethodPost, "/web/auth/logout", nil)
	req.Header.Set("Origin", "http://evil.local")
	res := httptest.NewRecorder()

	allowedOrigins := origininfra.ParseAllowedOrigins([]string{"http://localhost:3000"}, "")
	originMiddleware := originmiddleware.NewOriginValidationMiddleware(allowedOrigins)
	originMiddleware(http.HandlerFunc(handler.HandleLogout)).ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, res.Code)
	}
}

func TestHandleLogout_RejectsNonPostMethod(t *testing.T) {
	handler := &WebAuthHandler{
		LogoutCommand: &command.LogoutCommand{},
		CookieManager: infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	req := httptest.NewRequest(http.MethodGet, "/web/auth/logout", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res := httptest.NewRecorder()

	methodGuard := func(method string, next http.Handler) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if req.Method != method {
				http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			next.ServeHTTP(rw, req)
		})
	}
	methodGuard(http.MethodPost, http.HandlerFunc(handler.HandleLogout)).ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d, got %d", http.StatusMethodNotAllowed, res.Code)
	}
}

func TestHandleCallback_DoesNotLeakSensitiveErrors(t *testing.T) {
	handler := &WebAuthHandler{
		CallbackCommand: &command.CallbackCommand{},
		CookieManager:   infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/"),
	}

	req := httptest.NewRequest(http.MethodGet, "/web/auth/callback?code=bad&state=bad", nil)
	res := httptest.NewRecorder()

	handler.HandleCallback(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, res.Code)
	}
	body := res.Body.String()
	if strings.Contains(strings.ToLower(body), "secret") || strings.Contains(strings.ToLower(body), "token") {
		t.Fatalf("unexpected sensitive text in response body: %s", body)
	}
}

func TestHandleMe_ReturnsAuthenticatedUserAnd401WhenMissing(t *testing.T) {
	handler := &WebAuthHandler{MeQuery: &query.GetCurrentUserQuery{}}

	validatorClaims := &domain.Claims{Subject: "u1", Username: "alice", Issuer: "issuer"}
	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator: tokenValidatorStub{claims: validatorClaims},
	})

	successReq := httptest.NewRequest(http.MethodGet, "/web/auth/me", nil)
	successReq.Header.Set("Authorization", "Bearer token")
	successRes := httptest.NewRecorder()
	authn(http.HandlerFunc(handler.HandleMe)).ServeHTTP(successRes, successReq)
	if successRes.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, successRes.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(successRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["subject"] != "u1" {
		t.Fatalf("unexpected subject: %#v", payload)
	}

	unauthReq := httptest.NewRequest(http.MethodGet, "/web/auth/me", nil)
	unauthRes := httptest.NewRecorder()
	handler.HandleMe(unauthRes, unauthReq)
	if unauthRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, unauthRes.Code)
	}
}

type loginClientStub struct {
	authURL string
}

func (s loginClientStub) BuildAuthorizationURL(ctx context.Context, params infrastructure.AuthorizationURLParams) (string, error) {
	return s.authURL, nil
}

type callbackClientStub struct {
	token *oauth2.Token
}

func (s callbackClientStub) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	return s.token, nil
}

type tokenValidatorStub struct {
	claims *domain.Claims
}

func (s tokenValidatorStub) Validate(ctx context.Context, token string) (*domain.Claims, error) {
	return s.claims, nil
}

func newValidatorForHandlerTest(t *testing.T) (*infrastructure.JWTValidator, string, *rsa.PrivateKey) {
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

func buildSignedToken(t *testing.T, issuer, clientID string, key *rsa.PrivateKey, expiresAt, notBefore time.Time) string {
	t.Helper()

	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   "user-1",
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
