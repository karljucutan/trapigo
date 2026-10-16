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
	"time"

	"golang.org/x/oauth2"
)

const logoutDiscoveryHTTPTimeout = 10 * time.Second

// PKCE functions from oauth2 package are used directly

// Config contains the Keycloak and OAuth client configuration used by this gateway.
type Config struct {
	IssuerURL        string
	InternalRealmURL string
	ClientID         string
	ClientSecret     string
	RedirectURI      string
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
	if strings.TrimSpace(cfg.InternalRealmURL) == "" {
		cfg.InternalRealmURL = cfg.IssuerURL
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

	discoveryURL := strings.TrimRight(c.cfg.InternalRealmURL, "/") + "/.well-known/openid-configuration"
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
	doc.AuthorizationEndpoint, err = rebaseEndpointURL(doc.AuthorizationEndpoint, c.cfg.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("configure public authorization endpoint: %w", err)
	}
	doc.TokenEndpoint, err = rebaseEndpointURL(doc.TokenEndpoint, c.cfg.InternalRealmURL)
	if err != nil {
		return nil, fmt.Errorf("configure internal token endpoint: %w", err)
	}
	if doc.EndSessionEndpoint != "" {
		doc.EndSessionEndpoint, err = rebaseEndpointURL(doc.EndSessionEndpoint, c.cfg.IssuerURL)
		if err != nil {
			return nil, fmt.Errorf("configure public end-session endpoint: %w", err)
		}
	}
	if doc.JWKSURI != "" {
		doc.JWKSURI, err = rebaseEndpointURL(doc.JWKSURI, c.cfg.InternalRealmURL)
		if err != nil {
			return nil, fmt.Errorf("configure internal JWKS endpoint: %w", err)
		}
	}

	c.mu.Lock()
	c.discovery = &doc
	c.mu.Unlock()

	cloned := doc
	return &cloned, nil
}

func rebaseEndpointURL(endpointURL, baseURL string) (string, error) {
	endpoint, err := url.Parse(endpointURL)
	if err != nil {
		return "", err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if endpoint.Scheme == "" || endpoint.Host == "" || base.Scheme == "" || base.Host == "" {
		return "", errors.New("endpoint and base URL must be absolute URLs")
	}
	endpoint.Scheme = base.Scheme
	endpoint.Host = base.Host
	return endpoint.String(), nil
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

// BuildLogoutURL prepares browser RP-Initiated Logout without ending the session itself.
func (c *KeycloakClient) BuildLogoutURL(ctx context.Context, postLogoutRedirectURI, idTokenHint string) (string, error) {
	returnURL, err := url.Parse(postLogoutRedirectURI)
	if err != nil || returnURL == nil || (returnURL.Scheme != "http" && returnURL.Scheme != "https") ||
		returnURL.Host == "" || returnURL.User != nil || returnURL.Fragment != "" {
		return "", errors.New("post-logout redirect URI must be an absolute HTTP(S) URL without credentials or fragment")
	}

	ctx, cancel := context.WithTimeout(ctx, logoutDiscoveryHTTPTimeout)
	defer cancel()

	doc, err := c.Discover(ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(doc.EndSessionEndpoint) == "" {
		return "", errors.New("discovery response missing end_session_endpoint")
	}
	endpoint, err := url.Parse(doc.EndSessionEndpoint)
	if err != nil {
		return "", fmt.Errorf("parse end-session endpoint: %w", err)
	}
	if (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" ||
		endpoint.User != nil || endpoint.Fragment != "" {
		return "", errors.New("end-session endpoint must be an absolute HTTP(S) URL without credentials or fragment")
	}
	params := endpoint.Query()
	params.Set("client_id", c.cfg.ClientID)
	params.Set("post_logout_redirect_uri", returnURL.String())
	if strings.TrimSpace(idTokenHint) != "" {
		params.Set("id_token_hint", idTokenHint)
	}
	endpoint.RawQuery = params.Encode()
	return endpoint.String(), nil
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
