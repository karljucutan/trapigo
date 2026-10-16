package command

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	"golang.org/x/oauth2"
)

func TestCallbackCommand_RejectsNonceMismatch(t *testing.T) {
	idToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"nonce": "different"})
	signedIDToken, err := idToken.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	oauthToken := (&oauth2.Token{
		AccessToken: "access-token",
		Expiry:      time.Now().Add(time.Minute),
	}).WithExtra(map[string]any{"id_token": signedIDToken})

	cmd := &CallbackCommand{
		IdentityProvider: callbackClientStub{token: oauthToken},
		StateStore: callbackStateStoreStub{state: &domain.OAuthState{
			State:        "s1",
			Nonce:        "expected",
			CodeVerifier: "ver",
			CreatedAt:    time.Now(),
		}},
		Validator:        callbackValidatorStub{claims: &domain.Claims{Subject: "user-1"}},
		IDTokenValidator: callbackIDTokenValidatorStub{err: domain.ErrInvalidJWT},
	}

	_, err = cmd.Execute(context.Background(), "code", "s1")
	if err == nil {
		t.Fatal("expected nonce mismatch to fail callback")
	}
}

func TestCallbackCommand_ReturnsStoredReturnURL(t *testing.T) {
	const returnURL = "http://localhost:3000/dashboard"
	cmd := &CallbackCommand{
		IdentityProvider: callbackClientStub{token: (&oauth2.Token{AccessToken: "access-token"}).WithExtra(map[string]any{"id_token": "validated-id-token"})},
		StateStore: callbackStateStoreStub{state: &domain.OAuthState{
			State: "s1", Nonce: "nonce", CodeVerifier: "verifier",
			CreatedAt: time.Now(), ReturnURL: returnURL,
		}},
		Validator:        callbackValidatorStub{claims: &domain.Claims{Subject: "user-1"}},
		IDTokenValidator: callbackIDTokenValidatorStub{},
	}
	result, err := cmd.Execute(context.Background(), "code", "s1")
	if err != nil {
		t.Fatal(err)
	}
	if result.ReturnURL != returnURL {
		t.Fatalf("got return URL %q, want %q", result.ReturnURL, returnURL)
	}
}

type callbackClientStub struct {
	token *oauth2.Token
}

func (s callbackClientStub) ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	return s.token, nil
}

func (callbackClientStub) BuildAuthorizationURL(context.Context, infrastructure.AuthorizationURLParams) (string, error) {
	return "", nil
}

func (callbackClientStub) BuildLogoutURL(context.Context, string, string) (string, error) {
	return "", nil
}

type callbackStateStoreStub struct {
	state *domain.OAuthState
}

func (s callbackStateStoreStub) Get(ctx context.Context, stateValue string) (*domain.OAuthState, error) {
	return s.state, nil
}

func (s callbackStateStoreStub) Delete(ctx context.Context, stateValue string) error {
	return nil
}

type callbackValidatorStub struct {
	claims *domain.Claims
}

type callbackIDTokenValidatorStub struct {
	err error
}

func (s callbackIDTokenValidatorStub) ValidateIDToken(context.Context, string, string) error {
	return s.err
}

func (s callbackValidatorStub) Validate(ctx context.Context, token string) (*domain.Claims, error) {
	return s.claims, nil
}
