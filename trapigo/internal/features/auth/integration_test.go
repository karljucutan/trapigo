package auth

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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	authgateway "github.com/karljucutan/trapigo/trapigo/internal/features/auth/gateway"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
	origininfra "github.com/karljucutan/trapigo/trapigo/internal/features/origin/infrastructure"
	originmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/origin/middleware"
	webcommand "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/command"
	webquery "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/query"
	webtransport "github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/transporthttp"
	"golang.org/x/oauth2"
)

func TestIntegration_BrowserRequestToAPIAfterLoginCookie(t *testing.T) {
	validator, issuer, key := newValidatorForIntegrationTest(t)
	cookieManager := infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/")
	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
	})

	api := authn(authgateway.AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "" {
			http.Error(rw, "missing authorization", http.StatusUnauthorized)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})))

	access := buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	req.AddCookie(cookieManager.AccessTokenCookie(access, 5*time.Minute))
	res := httptest.NewRecorder()

	api.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
}

func TestIntegration_RefreshOnExpiredAccessToken(t *testing.T) {
	validator, issuer, key := newValidatorForIntegrationTest(t)
	cookieManager := infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/")
	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			return &oauth2.Token{
				AccessToken:  buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute)),
				RefreshToken: refreshToken,
				Expiry:       time.Now().Add(5 * time.Minute),
			}, nil
		},
	})

	api := authn(authgateway.AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "" {
			http.Error(rw, "missing authorization", http.StatusUnauthorized)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})))

	expired := buildToken(t, issuer, "trapigo", key, time.Now().Add(-time.Minute), time.Now().Add(-2*time.Minute))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	req.AddCookie(cookieManager.AccessTokenCookie(expired, time.Minute))
	req.AddCookie(cookieManager.RefreshTokenCookie("refresh-abc", 30*time.Minute))
	res := httptest.NewRecorder()

	api.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
}

func TestIntegration_NativeBearerDirectFlow(t *testing.T) {
	validator, issuer, key := newValidatorForIntegrationTest(t)
	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator: validator,
	})

	accessToken := buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute))
	var observed string
	api := authn(authgateway.AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		observed = req.Header.Get("Authorization")
		rw.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res := httptest.NewRecorder()

	api.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, res.Code)
	}
	if observed != "Bearer "+accessToken {
		t.Fatalf("expected downstream bearer token, got %q", observed)
	}
}

func TestIntegration_InvalidTokensRejected(t *testing.T) {
	validator, _, _ := newValidatorForIntegrationTest(t)
	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator: validator,
	})

	api := authn(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Fatal("request should not proceed with invalid token")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	res := httptest.NewRecorder()

	api.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestIntegration_ConcurrentRefreshRequests(t *testing.T) {
	validator, issuer, key := newValidatorForIntegrationTest(t)
	cookieManager := infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/")
	var refreshCalls int64

	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
		RefreshToken: func(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
			atomic.AddInt64(&refreshCalls, 1)
			return &oauth2.Token{
				AccessToken:  buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute)),
				RefreshToken: refreshToken,
				Expiry:       time.Now().Add(5 * time.Minute),
			}, nil
		},
	})

	api := authn(authgateway.AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "" {
			http.Error(rw, "missing authorization", http.StatusUnauthorized)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})))

	expired := buildToken(t, issuer, "trapigo", key, time.Now().Add(-time.Minute), time.Now().Add(-2*time.Minute))
	const workers = 8
	var wg sync.WaitGroup
	results := make(chan int, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
			req.AddCookie(cookieManager.AccessTokenCookie(expired, time.Minute))
			req.AddCookie(cookieManager.RefreshTokenCookie("refresh-abc", 30*time.Minute))
			res := httptest.NewRecorder()
			api.ServeHTTP(res, req)
			results <- res.Code
		}()
	}

	wg.Wait()
	close(results)

	for status := range results {
		if status != http.StatusOK {
			t.Fatalf("expected %d for all concurrent requests, got %d", http.StatusOK, status)
		}
	}
	if atomic.LoadInt64(&refreshCalls) == 0 {
		t.Fatal("expected refresh callback to be called for concurrent requests")
	}
}

func TestIntegration_BrowserLoginMeLogoutCycle(t *testing.T) {
	validator, issuer, key := newValidatorForIntegrationTest(t)
	stateStore := infrastructure.NewInMemoryOAuthStateStore(5 * time.Minute)
	cookieManager := infrastructure.NewCookieManager(false, http.SameSiteLaxMode, "/")

	loginCommand := &webcommand.LoginCommand{
		KeycloakClient: browserLoginClientStub{authURL: "https://keycloak.example.com/auth"},
		StateStore:     stateStore,
	}
	callbackCommand := &webcommand.CallbackCommand{
		KeycloakClient: browserCallbackClientStub{token: &oauth2.Token{
			AccessToken:  buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute)),
			RefreshToken: "refresh-xyz",
			Expiry:       time.Now().Add(5 * time.Minute),
		}},
		StateStore: stateStore,
		Validator:  validator,
	}
	handler := &webtransport.AuthHandler{
		LoginCommand:    loginCommand,
		CallbackCommand: callbackCommand,
		LogoutCommand:   &webcommand.LogoutCommand{},
		MeQuery:         &webquery.GetCurrentUserQuery{},
		CookieManager:   cookieManager,
		FrontendURL:     "http://localhost:3000",
	}

	loginReq := httptest.NewRequest(http.MethodGet, "/web/auth/login", nil)
	loginRes := httptest.NewRecorder()
	handler.HandleLogin(loginRes, loginReq)
	if loginRes.Code != http.StatusFound {
		t.Fatalf("expected %d, got %d", http.StatusFound, loginRes.Code)
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/web/auth/callback?code=ok&state=s1", nil)
	callbackRes := httptest.NewRecorder()
	handler.CallbackCommand = &webcommand.CallbackCommand{
		KeycloakClient: browserCallbackClientStub{token: &oauth2.Token{
			AccessToken:  buildToken(t, issuer, "trapigo", key, time.Now().Add(5*time.Minute), time.Now().Add(-time.Minute)),
			RefreshToken: "refresh-xyz",
			Expiry:       time.Now().Add(5 * time.Minute),
		}},
		StateStore: staticStateStore{state: &domain.OAuthState{State: "s1", CodeVerifier: "ver", Nonce: "n", CreatedAt: time.Now()}},
		Validator:  validator,
	}
	handler.HandleCallback(callbackRes, callbackReq)
	if callbackRes.Code != http.StatusFound {
		t.Fatalf("expected %d, got %d", http.StatusFound, callbackRes.Code)
	}

	authn := authmiddleware.NewAuthenticationMiddleware(authmiddleware.AuthenticationMiddlewareConfig{
		Validator:     validator,
		CookieManager: cookieManager,
	})
	meReq := httptest.NewRequest(http.MethodGet, "/web/auth/me", nil)
	for _, c := range callbackRes.Result().Cookies() {
		meReq.AddCookie(c)
	}
	meRes := httptest.NewRecorder()
	authn(http.HandlerFunc(handler.HandleMe)).ServeHTTP(meRes, meReq)
	if meRes.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, meRes.Code)
	}
	if strings.Contains(meRes.Body.String(), "access_token") || strings.Contains(meRes.Body.String(), "refresh_token") {
		t.Fatalf("token leaked in /web/auth/me body: %s", meRes.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/web/auth/logout", nil)
	logoutReq.Header.Set("Origin", "http://localhost:3000")
	for _, c := range callbackRes.Result().Cookies() {
		logoutReq.AddCookie(c)
	}
	logoutRes := httptest.NewRecorder()
	
	allowedOrigins := origininfra.ParseAllowedOrigins([]string{"http://localhost:3000"}, "")
	originMiddleware := originmiddleware.NewOriginValidationMiddleware(allowedOrigins)
	originMiddleware(http.HandlerFunc(handler.HandleLogout)).ServeHTTP(logoutRes, logoutReq)
	if logoutRes.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, logoutRes.Code)
	}
}

type browserLoginClientStub struct {
	authURL string
}

func (s browserLoginClientStub) BuildAuthorizationURL(ctx context.Context, params infrastructure.AuthorizationURLParams) (string, error) {
	return s.authURL, nil
}

type browserCallbackClientStub struct {
	token *oauth2.Token
}

func (s browserCallbackClientStub) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	return s.token, nil
}

type staticStateStore struct {
	state *domain.OAuthState
}

func (s staticStateStore) Save(ctx context.Context, state *domain.OAuthState) error {
	return nil
}

func (s staticStateStore) Get(ctx context.Context, stateValue string) (*domain.OAuthState, error) {
	if s.state == nil || s.state.State != stateValue {
		return nil, domain.ErrInvalidState
	}
	return s.state, nil
}

func (s staticStateStore) Delete(ctx context.Context, stateValue string) error {
	return nil
}

func newValidatorForIntegrationTest(t *testing.T) (*infrastructure.JWTValidator, string, *rsa.PrivateKey) {
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

func buildToken(t *testing.T, issuer, clientID string, key *rsa.PrivateKey, expiresAt, notBefore time.Time) string {
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
