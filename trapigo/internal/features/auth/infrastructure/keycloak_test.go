package infrastructure

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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

	client, err := NewClient(Config{
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

func TestExchangeAuthorizationCode_SendsExpectedPayload(t *testing.T) {
	assertion := tokenAssertion{}
	server := newMockOIDCServer(t, assertion)
	defer server.Close()

	client, err := NewClient(Config{
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

	client, err := NewClient(Config{
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
			})
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
