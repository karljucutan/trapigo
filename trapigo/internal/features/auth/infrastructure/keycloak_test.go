package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestS256ChallengeFromVerifier_KnownRFCVector(t *testing.T) {
	// RFC 7636 Appendix B test vector - using official oauth2 package
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	if challenge != "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM" {
		t.Fatalf("unexpected challenge: %s", challenge)
	}
}

func TestBuildAuthorizationURL_IncludesExpectedParams(t *testing.T) {
	server := newMockOIDCServer(t, tokenAssertion{})
	defer server.Close()

	client, err := NewKeycloakClient(Config{
		IssuerURL:   server.URL,
		ClientID:    "trapigo-gateway",
		RedirectURI: "https://api.example.com/web/auth/callback",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	authURL, err := client.BuildAuthorizationURL(context.Background(), AuthorizationURLParams{
		State:         "state123",
		Nonce:         "nonce456",
		CodeChallenge: "challenge789",
		Scopes:        []string{"openid", "profile", "email"},
	})
	if err != nil {
		t.Fatalf("BuildAuthorizationURL returned error: %v", err)
	}

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}
	q := parsed.Query()

	if q.Get("client_id") != "trapigo-gateway" {
		t.Fatalf("unexpected client_id: %s", q.Get("client_id"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("unexpected response_type: %s", q.Get("response_type"))
	}
	if q.Get("scope") != "openid profile email" {
		t.Fatalf("unexpected scope: %s", q.Get("scope"))
	}
	if q.Get("state") != "state123" {
		t.Fatalf("unexpected state: %s", q.Get("state"))
	}
	if q.Get("nonce") != "nonce456" {
		t.Fatalf("unexpected nonce: %s", q.Get("nonce"))
	}
	if q.Get("code_challenge") != "challenge789" {
		t.Fatalf("unexpected code_challenge: %s", q.Get("code_challenge"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("unexpected code_challenge_method: %s", q.Get("code_challenge_method"))
	}
}

func TestDiscoverUsesInternalRealmURLAndPreservesPublicAuthorizationURL(t *testing.T) {
	const publicRealmURL = "http://localhost:8081/auth/realms/CompanyRealmA"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/auth/realms/CompanyRealmA/.well-known/openid-configuration" {
			http.NotFound(rw, req)
			return
		}
		_ = json.NewEncoder(rw).Encode(map[string]string{
			"issuer":                 publicRealmURL,
			"authorization_endpoint": publicRealmURL + "/protocol/openid-connect/auth",
			"token_endpoint":         publicRealmURL + "/protocol/openid-connect/token",
			"jwks_uri":               publicRealmURL + "/protocol/openid-connect/certs",
			"end_session_endpoint":   publicRealmURL + "/protocol/openid-connect/logout",
		})
	}))
	t.Cleanup(server.Close)

	client, err := NewKeycloakClient(Config{
		IssuerURL:        publicRealmURL,
		InternalRealmURL: server.URL + "/auth/realms/CompanyRealmA",
		ClientID:         "trapigo-gateway",
		RedirectURI:      "http://localhost:8081/web/auth/callback",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewKeycloakClient returned error: %v", err)
	}

	doc, err := client.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	if doc.AuthorizationEndpoint != publicRealmURL+"/protocol/openid-connect/auth" {
		t.Fatalf("expected public authorization endpoint, got %s", doc.AuthorizationEndpoint)
	}
	internalEndpoint := server.URL + "/auth/realms/CompanyRealmA/protocol/openid-connect/"
	if doc.TokenEndpoint != internalEndpoint+"token" {
		t.Fatalf("expected internal token endpoint, got %s", doc.TokenEndpoint)
	}
	if doc.JWKSURI != internalEndpoint+"certs" {
		t.Fatalf("expected internal JWKS endpoint, got %s", doc.JWKSURI)
	}
	if doc.EndSessionEndpoint != publicRealmURL+"/protocol/openid-connect/logout" {
		t.Fatalf("expected public end-session endpoint, got %s", doc.EndSessionEndpoint)
	}
	logoutURL, err := client.BuildLogoutURL(context.Background(), "http://localhost:3000/", "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(logoutURL)
	if err != nil || parsed.Host != "localhost:8081" {
		t.Fatalf("expected browser-accessible logout URL, got %s, %v", logoutURL, err)
	}
}

func TestExchangeAuthorizationCode_SendsExpectedPayload(t *testing.T) {
	assertion := tokenAssertion{}
	server := newMockOIDCServer(t, assertion)
	defer server.Close()

	client, err := NewKeycloakClient(Config{
		IssuerURL:    server.URL,
		ClientID:     "trapigo-gateway",
		ClientSecret: "super-secret",
		RedirectURI:  "https://api.example.com/web/auth/callback",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	token, err := client.ExchangeAuthorizationCode(context.Background(), "auth-code", "verifier-1234567890123456789012345678901234567890123")
	if err != nil {
		t.Fatalf("ExchangeAuthorizationCode returned error: %v", err)
	}

	if token.AccessToken != "access-token-value" {
		t.Fatalf("unexpected access token: %s", token.AccessToken)
	}
}

func TestRefreshToken_SendsExpectedPayload(t *testing.T) {
	assertion := tokenAssertion{}
	server := newMockOIDCServer(t, assertion)
	defer server.Close()

	client, err := NewKeycloakClient(Config{
		IssuerURL:    server.URL,
		ClientID:     "trapigo-gateway",
		ClientSecret: "super-secret",
		RedirectURI:  "https://api.example.com/web/auth/callback",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	token, err := client.RefreshToken(context.Background(), "refresh-123")
	if err != nil {
		t.Fatalf("RefreshToken returned error: %v", err)
	}

	if token.RefreshToken != "refresh-token-value" {
		t.Fatalf("unexpected refresh token: %s", token.RefreshToken)
	}
}

type tokenAssertion struct{}

func TestBuildLogoutURL(t *testing.T) {
	server := newMockOIDCServer(t, tokenAssertion{})
	defer server.Close()

	client, err := NewKeycloakClient(Config{
		IssuerURL:    server.URL,
		ClientID:     "trapigo-gateway",
		ClientSecret: "super-secret",
		RedirectURI:  "https://api.example.com/web/auth/callback",
	}, server.Client())
	if err != nil {
		t.Fatalf("NewClient returned error: %v", err)
	}

	const returnURL = "https://app.example.com/?logged_out=true"
	const idToken = "signed.id.token"
	logoutURL, err := client.BuildLogoutURL(context.Background(), returnURL, idToken)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(logoutURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != server.URL+"/protocol/openid-connect/logout" {
		t.Fatalf("unexpected logout endpoint: %s", logoutURL)
	}
	if parsed.Query().Get("client_id") != "trapigo-gateway" ||
		parsed.Query().Get("post_logout_redirect_uri") != returnURL ||
		parsed.Query().Get("id_token_hint") != idToken || len(parsed.Query()) != 3 {
		t.Fatalf("unexpected logout parameters: %s", logoutURL)
	}
	logoutURL, err = client.BuildLogoutURL(context.Background(), returnURL, "")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err = url.Parse(logoutURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("id_token_hint") != "" || len(parsed.Query()) != 2 {
		t.Fatalf("expected logout URL without an ID token hint, got %s", logoutURL)
	}
	for _, invalid := range []string{"", "/dashboard", "//evil.example", "javascript:alert(1)", "https://user:pass@app.example.com", "https://app.example.com/#fragment"} {
		if _, err := client.BuildLogoutURL(context.Background(), invalid, ""); err == nil {
			t.Fatalf("expected invalid return URL %q to fail", invalid)
		}
	}
}

func TestBuildLogoutURL_DiscoveryFailures(t *testing.T) {
	for _, scenario := range []string{"missing endpoint", "server error", "canceled context"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if scenario == "server error" {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]string{
					"authorization_endpoint": "https://id.example.com/auth",
					"token_endpoint":         "https://id.example.com/token",
				})
			}))
			defer server.Close()
			client, err := NewKeycloakClient(Config{
				IssuerURL: server.URL, ClientID: "trapigo", RedirectURI: "https://app.example.com/callback",
			}, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled context" {
				cancel()
			}
			if _, err := client.BuildLogoutURL(ctx, "https://app.example.com/", ""); err == nil {
				t.Fatal("expected logout preparation to fail")
			}
		})
	}
}

type logoutDiscoveryTransport struct {
	t *testing.T
}

func (s logoutDiscoveryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	deadline, ok := req.Context().Deadline()
	if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > logoutDiscoveryHTTPTimeout {
		s.t.Errorf("expected discovery timeout bounded by %s", logoutDiscoveryHTTPTimeout)
	}
	return nil, context.DeadlineExceeded
}

func TestBuildLogoutURL_BoundsDiscoveryHTTPTime(t *testing.T) {
	client, err := NewKeycloakClient(Config{
		IssuerURL: "https://id.example.com", ClientID: "trapigo", RedirectURI: "https://app.example.com/callback",
	}, &http.Client{Transport: logoutDiscoveryTransport{t: t}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.BuildLogoutURL(context.Background(), "https://app.example.com/", ""); err == nil {
		t.Fatal("expected discovery timeout error")
	}
}

func newMockOIDCServer(t *testing.T, _ tokenAssertion) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(map[string]string{
				"issuer":                 server.URL,
				"authorization_endpoint": server.URL + "/protocol/openid-connect/auth",
				"token_endpoint":         server.URL + "/protocol/openid-connect/token",
				"jwks_uri":               server.URL + "/protocol/openid-connect/certs",
				"end_session_endpoint":   server.URL + "/protocol/openid-connect/logout",
			})
		case "/protocol/openid-connect/logout":
			t.Error("BFF must not call the browser logout endpoint")
			rw.WriteHeader(http.StatusInternalServerError)
		case "/protocol/openid-connect/token":
			if req.Method != http.MethodPost {
				t.Fatalf("expected POST method, got %s", req.Method)
			}
			if req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Fatalf("unexpected content-type: %s", req.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			if err := req.Body.Close(); err != nil {
				t.Fatalf("failed to close request body: %v", err)
			}

			form, err := url.ParseQuery(string(body))
			if err != nil {
				t.Fatalf("failed to parse request body as form: %v", err)
			}
			if form.Get("client_id") != "trapigo-gateway" {
				t.Fatalf("unexpected client_id: %s", form.Get("client_id"))
			}
			if form.Get("client_secret") != "super-secret" {
				t.Fatalf("unexpected client_secret: %s", form.Get("client_secret"))
			}

			grantType := form.Get("grant_type")
			switch grantType {
			case "authorization_code":
				if form.Get("code") != "auth-code" {
					t.Fatalf("unexpected code: %s", form.Get("code"))
				}
				if form.Get("code_verifier") == "" {
					t.Fatal("expected code_verifier to be set")
				}
			case "refresh_token":
				if form.Get("refresh_token") != "refresh-123" {
					t.Fatalf("unexpected refresh_token: %s", form.Get("refresh_token"))
				}
			default:
				t.Fatalf("unexpected grant_type: %s", grantType)
			}

			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(map[string]any{
				"access_token":  "access-token-value",
				"refresh_token": "refresh-token-value",
				"token_type":    "Bearer",
				"expires_in":    300,
			})
		default:
			http.NotFound(rw, req)
		}
	}))

	return server
}
