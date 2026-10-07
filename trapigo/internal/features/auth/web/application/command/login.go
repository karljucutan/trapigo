package command

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/application"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	"golang.org/x/oauth2"
)

type LoginStateStore interface {
	Save(ctx context.Context, state *domain.OAuthState) error
}

type LoginCommand struct {
	IdentityProvider application.IdentityProvider
	StateStore       LoginStateStore
	StateTTL         time.Duration
}

func (c *LoginCommand) Execute(ctx context.Context, returnURL string) (string, error) {
	if c.IdentityProvider == nil || c.StateStore == nil {
		return "", fmt.Errorf("login command misconfigured")
	}

	state, err := randomString(32)
	if err != nil {
		return "", err
	}
	nonce, err := randomString(32)
	if err != nil {
		return "", err
	}
	codeVerifier := oauth2.GenerateVerifier()
	codeChallenge := oauth2.S256ChallengeFromVerifier(codeVerifier)

	oauthState := &domain.OAuthState{
		State:        state,
		Nonce:        nonce,
		CodeVerifier: codeVerifier,
		ReturnURL:    returnURL,
		CreatedAt:    time.Now().UTC(),
	}
	if err := c.StateStore.Save(ctx, oauthState); err != nil {
		return "", err
	}

	redirectURL, err := c.IdentityProvider.BuildAuthorizationURL(ctx, infrastructure.AuthorizationURLParams{
		State:         state,
		Nonce:         nonce,
		CodeChallenge: codeChallenge,
		Scopes:        []string{"openid", "profile", "email"},
	})
	if err != nil {
		return "", err
	}
	return redirectURL, nil
}

func randomString(byteLength int) (string, error) {
	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
