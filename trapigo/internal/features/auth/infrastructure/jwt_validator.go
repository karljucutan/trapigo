package infrastructure

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
)

// JWTValidator validates access tokens issued by Keycloak using JWKS.
type JWTValidator struct {
	issuerURL    string
	clientID     string
	jwksCacheTTL time.Duration
	httpClient   *http.Client

	mu                sync.RWMutex
	verificationKeys  map[string]jwt.VerificationKey
	lastJWKSFetchTime time.Time
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewJWTValidator(issuerURL, clientID string, httpClient *http.Client) *JWTValidator {
	return NewJWTValidatorWithJWKSCacheTTL(issuerURL, clientID, time.Hour, httpClient)
}

func NewJWTValidatorWithJWKSCacheTTL(issuerURL, clientID string, jwksCacheTTL time.Duration, httpClient *http.Client) *JWTValidator {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if jwksCacheTTL <= 0 {
		jwksCacheTTL = time.Hour
	}
	trimmedIssuerURL := strings.TrimRight(issuerURL, "/")
	return &JWTValidator{
		issuerURL:        trimmedIssuerURL,
		clientID:         clientID,
		jwksCacheTTL:     jwksCacheTTL,
		httpClient:       httpClient,
		verificationKeys: map[string]jwt.VerificationKey{},
	}
}

// Deprecated: use NewJWTValidatorWithJWKSCacheTTL.
func NewJWTValidatorWithCacheTTL(issuerURL, clientID string, cacheTTL time.Duration, httpClient *http.Client) *JWTValidator {
	return NewJWTValidatorWithJWKSCacheTTL(issuerURL, clientID, cacheTTL, httpClient)
}

func (v *JWTValidator) Validate(ctx context.Context, tokenString string) (*domain.Claims, error) {
	if strings.TrimSpace(tokenString) == "" {
		return nil, domain.ErrInvalidJWT
	}

	if err := v.ensureJWKS(ctx); err != nil {
		return nil, err
	}

	// Build parser options for validation using jwt/v5 built-in validators
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuerURL),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30 * time.Second), // Allow 30s clock skew between servers
	}

	// Add audience validation if clientID is configured
	if v.clientID != "" {
		parserOpts = append(parserOpts, jwt.WithAudience(v.clientID))
	}

	// Parse and validate the token using Keycloak's JWKS:
	//   1. Read the JWT kid from the header
	//   2. Fetch the matching RSA public key from the cached JWKS
	//   3. Verify the JWT signature cryptographically
	//   4. Validate standard claims: alg, iss, aud, exp, nbf, etc.
	parsed, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, domain.ErrInvalidJWT
		}
		key, ok := v.lookupVerificationKey(kid)
		if !ok {
			if refreshErr := v.refreshJWKS(ctx, true); refreshErr != nil {
				return nil, refreshErr
			}
			key, ok = v.lookupVerificationKey(kid)
			if !ok {
				return nil, fmt.Errorf("%w: unknown key id %s", domain.ErrInvalidJWT, kid)
			}
		}
		return key, nil
	}, parserOpts...)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrExpiredToken
		}
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidJWT, err)
	}

	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || claims == nil {
		return nil, domain.ErrInvalidJWT
	}

	email, username, scopes := extractJWTExtras(tokenString)
	result := &domain.Claims{
		Subject:  claims.Subject,
		Issuer:   claims.Issuer,
		Email:    email,
		Username: username,
		Raw:      tokenString,
	}
	if result.Username == "" {
		result.Username = claims.Subject
	}
	if claims.ExpiresAt != nil {
		result.ExpiresAt = claims.ExpiresAt.Time
	}
	if claims.NotBefore != nil {
		result.NotBefore = &claims.NotBefore.Time
	}
	for _, aud := range claims.Audience {
		result.Audience = append(result.Audience, aud)
	}
	if scopes != "" {
		for scope := range strings.FieldsSeq(scopes) {
			result.Scopes = append(result.Scopes, scope)
		}
	}
	return result, nil
}

func extractJWTExtras(tokenString string) (string, string, string) {
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil || token == nil {
		return "", "", ""
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", ""
	}

	email, _ := claims["email"].(string)
	username, _ := claims["preferred_username"].(string)
	scope, _ := claims["scope"].(string)
	return email, username, scope
}

func (v *JWTValidator) ensureJWKS(ctx context.Context) error {
	return v.refreshJWKS(ctx, false)
}

func (v *JWTValidator) refreshJWKS(ctx context.Context, force bool) error {
	v.mu.RLock()
	if !force && len(v.verificationKeys) > 0 && time.Since(v.lastJWKSFetchTime) < v.jwksCacheTTL {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	jwksURL := v.issuerURL + "/protocol/openid-connect/certs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return fmt.Errorf("%w: build JWKS request: %v", domain.ErrKeycloakConnection, err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: fetch JWKS: %v", domain.ErrKeycloakConnection, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("%w: JWKS returned %d: %s", domain.ErrKeycloakConnection, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("%w: decode JWKS: %v", domain.ErrKeycloakConnection, err)
	}

	verificationKeys := map[string]jwt.VerificationKey{}
	for _, key := range doc.Keys {
		if key.Kty != "RSA" || key.N == "" || key.E == "" {
			continue
		}
		rsaKey, err := parseRSAPublicKey(key)
		if err != nil {
			continue
		}
		verificationKeys[key.Kid] = rsaKey
	}
	if len(verificationKeys) == 0 {
		return fmt.Errorf("%w: no RSA keys found in JWKS", domain.ErrKeycloakConnection)
	}

	v.mu.Lock()
	v.verificationKeys = verificationKeys
	v.lastJWKSFetchTime = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *JWTValidator) lookupVerificationKey(kid string) (jwt.VerificationKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.verificationKeys[kid]
	return key, ok
}

func parseRSAPublicKey(j jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, err
	}

	// RFC 7518 exponent is a big-endian integer, but we can convert to int.
	eInt := 0
	for _, b := range eBytes {
		eInt = eInt<<8 | int(b)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}

func (v *JWTValidator) ValidateString(tokenString string) (*domain.Claims, error) {
	return v.Validate(context.Background(), tokenString)
}

func (v *JWTValidator) ParseTokenString(tokenString string) (*jwt.Token, error) {
	return jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid")
		}
		key, ok := v.lookupVerificationKey(kid)
		if !ok {
			return nil, fmt.Errorf("unknown key id %s", kid)
		}
		return key, nil
	})
}
