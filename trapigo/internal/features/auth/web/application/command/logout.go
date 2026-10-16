package command

import (
	"context"
	"errors"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/application"
)

type LogoutCommand struct {
	IdentityProvider application.IdentityProvider
}

func (c *LogoutCommand) Execute(ctx context.Context, postLogoutRedirectURI, idTokenHint string) (string, error) {
	if c.IdentityProvider == nil {
		return "", errors.New("logout identity provider is not configured")
	}
	return c.IdentityProvider.BuildLogoutURL(ctx, postLogoutRedirectURI, idTokenHint)
}
