package infrastructure

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidator_AllowsMissingNotBefore(t *testing.T) {
	privateKey, kid, serverURL, client := newJWKSValidatorTestServer(t, nil)

	validator := NewJWTValidatorWithCacheTTL(serverURL, "trapigo", time.Hour, client)
	token := buildValidatorToken(t, serverURL, "trapigo", kid, privateKey, time.Now().Add(5*time.Minute), nil)

	claims, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if claims.Subject != "user-1" {
		t.Fatalf("unexpected subject: %s", claims.Subject)
	}
}

func TestJWTValidator_RefreshesJWKSOnUnknownKeyID(t *testing.T) {
	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate old RSA key: %v", err)
	}
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate new RSA key: %v", err)
	}

	var calls int64
	_, _, serverURL, client := newJWKSValidatorTestServer(t, func(call int64) jwksDocument {
		atomic.StoreInt64(&calls, call)
		if call == 1 {
			return jwksDocument{Keys: []jwk{publicJWKFromKey(t, "old-key", &oldKey.PublicKey)}}
		}
		return jwksDocument{Keys: []jwk{publicJWKFromKey(t, "new-key", &newKey.PublicKey)}}
	})

	validator := NewJWTValidatorWithCacheTTL(serverURL, "trapigo", time.Hour, client)
	primeToken := buildValidatorToken(t, serverURL, "trapigo", "old-key", oldKey, time.Now().Add(5*time.Minute), timePtr(time.Now().Add(-time.Minute)))
	if _, err := validator.Validate(context.Background(), primeToken); err != nil {
		t.Fatalf("prime validation failed: %v", err)
	}

	rotatedToken := buildValidatorToken(t, serverURL, "trapigo", "new-key", newKey, time.Now().Add(5*time.Minute), timePtr(time.Now().Add(-time.Minute)))
	if _, err := validator.Validate(context.Background(), rotatedToken); err != nil {
		t.Fatalf("expected JWKS refresh on unknown kid, got error: %v", err)
	}
	if atomic.LoadInt64(&calls) < 2 {
		t.Fatalf("expected JWKS endpoint to be called again for unknown kid, got %d calls", atomic.LoadInt64(&calls))
	}
}

func newJWKSValidatorTestServer(t *testing.T, docForCall func(call int64) jwksDocument) (*rsa.PrivateKey, string, string, *http.Client) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	kid := "test-key"
	var calls int64

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/protocol/openid-connect/certs" {
			http.NotFound(rw, req)
			return
		}
		call := atomic.AddInt64(&calls, 1)
		rw.Header().Set("Content-Type", "application/json")
		doc := jwksDocument{Keys: []jwk{publicJWKFromKey(t, kid, &privateKey.PublicKey)}}
		if docForCall != nil {
			doc = docForCall(call)
		}
		if err := json.NewEncoder(rw).Encode(doc); err != nil {
			t.Fatalf("encode JWKS response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return privateKey, kid, server.URL, server.Client()
}

func TestJWTValidator_ExtractsKeycloakExtrasFromStandardClaims(t *testing.T) {
	privateKey, kid, serverURL, client := newJWKSValidatorTestServer(t, nil)

	validator := NewJWTValidatorWithCacheTTL(serverURL, "trapigo", time.Hour, client)
	token := buildValidatorTokenWithExtras(t, serverURL, "trapigo", kid, privateKey, time.Now().Add(5*time.Minute), nil, map[string]any{
		"email":              "alice@example.com",
		"preferred_username": "alice",
		"scope":              "openid profile email",
	})

	claims, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if claims.Email != "alice@example.com" {
		t.Fatalf("unexpected email: %s", claims.Email)
	}
	if claims.Username != "alice" {
		t.Fatalf("unexpected username: %s", claims.Username)
	}
	if len(claims.Scopes) != 3 || claims.Scopes[0] != "openid" || claims.Scopes[1] != "profile" || claims.Scopes[2] != "email" {
		t.Fatalf("unexpected scopes: %#v", claims.Scopes)
	}
}

func buildValidatorToken(t *testing.T, issuer, clientID, kid string, key *rsa.PrivateKey, expiresAt time.Time, notBefore *time.Time) string {
	return buildValidatorTokenWithExtras(t, issuer, clientID, kid, key, expiresAt, notBefore, nil)
}

func buildValidatorTokenWithExtras(t *testing.T, issuer, clientID, kid string, key *rsa.PrivateKey, expiresAt time.Time, notBefore *time.Time, extraClaims map[string]any) string {
	t.Helper()

	issuedAt := time.Now().Add(-time.Minute)
	claims := jwt.MapClaims{
		"iss": issuer,
		"sub": "user-1",
		"aud": clientID,
		"exp": expiresAt.Unix(),
		"iat": issuedAt.Unix(),
	}
	if notBefore != nil {
		claims["nbf"] = notBefore.Unix()
	}
	for name, value := range extraClaims {
		claims[name] = value
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	str, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return str
}

func publicJWKFromKey(t *testing.T, kid string, publicKey *rsa.PublicKey) jwk {
	t.Helper()
	return jwk{
		Kid: kid,
		Kty: "RSA",
		Use: "sig",
		N:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
