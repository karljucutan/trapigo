package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
)

// PKCE functions from oauth2 package are used directly

// Config contains the Keycloak and OAuth client configuration used by this gateway.
type Config struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// DiscoveryDocument is the subset of OIDC discovery fields required by this service.
type DiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// KeycloakClient handles OIDC discovery and token-related calls to Keycloak.
type KeycloakClient struct {
	cfg        Config
	httpClient *http.Client

	mu        sync.RWMutex
	discovery *DiscoveryDocument
}

func NewKeycloakClient(cfg Config, httpClient *http.Client) (*KeycloakClient, error) {
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return nil, errors.New("issuer URL is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("client ID is required")
	}
	if strings.TrimSpace(cfg.RedirectURI) == "" {
		return nil, errors.New("redirect URI is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &KeycloakClient{
		cfg:        cfg,
		httpClient: httpClient,
	}, nil
}

func (c *KeycloakClient) Discover(ctx context.Context) (*DiscoveryDocument, error) {
	c.mu.RLock()
	if c.discovery != nil {
		doc := *c.discovery
		c.mu.RUnlock()
		return &doc, nil
	}
	c.mu.RUnlock()

	discoveryURL := strings.TrimRight(c.cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build discovery request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call discovery endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("discovery endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var doc DiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode discovery response: %w", err)
	}

	if strings.TrimSpace(doc.AuthorizationEndpoint) == "" || strings.TrimSpace(doc.TokenEndpoint) == "" {
		return nil, errors.New("discovery response missing required endpoints")
	}

	c.mu.Lock()
	c.discovery = &doc
	c.mu.Unlock()

	cloned := doc
	return &cloned, nil
}

// AuthorizationURLParams defines values required for Authorization Code + PKCE redirection.
type AuthorizationURLParams struct {
	State         string
	Nonce         string
	CodeChallenge string
	Scopes        []string
}

func (c *KeycloakClient) BuildAuthorizationURL(ctx context.Context, params AuthorizationURLParams) (string, error) {
	if strings.TrimSpace(params.State) == "" {
		return "", errors.New("state is required")
	}
	if strings.TrimSpace(params.Nonce) == "" {
		return "", errors.New("nonce is required")
	}
	if strings.TrimSpace(params.CodeChallenge) == "" {
		return "", errors.New("code challenge is required")
	}

	cfg, err := c.oauthConfig(ctx)
	if err != nil {
		return "", err
	}
	if len(params.Scopes) > 0 {
		cfg.Scopes = params.Scopes
	}

	// Note: CodeChallenge is already base64url-encoded from S256ChallengeFromVerifier
	return cfg.AuthCodeURL(
		params.State,
		oauth2.SetAuthURLParam("nonce", params.Nonce),
		oauth2.SetAuthURLParam("code_challenge", params.CodeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	), nil
}

func (c *KeycloakClient) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("authorization code is required")
	}
	if strings.TrimSpace(codeVerifier) == "" {
		return nil, errors.New("code verifier is required")
	}

	cfg, err := c.oauthConfig(ctx)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	token, err := cfg.Exchange(
		ctx,
		code,
		oauth2.VerifierOption(codeVerifier),
	)
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}

	return token, nil
}

func (c *KeycloakClient) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, errors.New("refresh token is required")
	}

	cfg, err := c.oauthConfig(ctx)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)

	token, err := cfg.TokenSource(ctx, &oauth2.Token{
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
	}).Token()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}

	return token, nil
}

// EndSession ends the Keycloak SSO session bound to the refresh token.
func (c *KeycloakClient) EndSession(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return errors.New("refresh token is required")
	}

	doc, err := c.Discover(ctx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(doc.EndSessionEndpoint) == "" {
		return errors.New("discovery response missing end_session_endpoint")
	}

	form := url.Values{}
	form.Set("client_id", c.cfg.ClientID)
	if strings.TrimSpace(c.cfg.ClientSecret) != "" {
		form.Set("client_secret", c.cfg.ClientSecret)
	}
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.EndSessionEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build logout request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call end session endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	// An expired or already-revoked refresh token means there is no session left to end.
	if resp.StatusCode == http.StatusBadRequest {
		var oauthErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &oauthErr) == nil && oauthErr.Error == "invalid_grant" {
			return nil
		}
	}
	return fmt.Errorf("end session endpoint returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *KeycloakClient) oauthConfig(ctx context.Context) (*oauth2.Config, error) {
	doc, err := c.Discover(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := oauth2.Endpoint{
		AuthURL:  doc.AuthorizationEndpoint,
		TokenURL: doc.TokenEndpoint,
	}
	if strings.TrimSpace(c.cfg.ClientSecret) != "" {
		endpoint.AuthStyle = oauth2.AuthStyleInParams
	}

	return &oauth2.Config{
		ClientID:     c.cfg.ClientID,
		ClientSecret: c.cfg.ClientSecret,
		RedirectURL:  c.cfg.RedirectURI,
		Endpoint:     endpoint,
		Scopes:       []string{"openid"},
	}, nil
}
