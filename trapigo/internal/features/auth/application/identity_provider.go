package application

import (
	"context"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	"golang.org/x/oauth2"
)

type IdentityProvider interface {
	BuildAuthorizationURL(ctx context.Context, params infrastructure.AuthorizationURLParams) (string, error)
	ExchangeAuthorizationCode(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error)
	EndSession(ctx context.Context, refreshToken string) error
}
