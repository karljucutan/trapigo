package command

import (
	"context"
	"strings"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/application"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
)

type LogoutCommand struct {
	CookieManager    *infrastructure.CookieManager
	IdentityProvider application.IdentityProvider
}

func (c *LogoutCommand) Execute(ctx context.Context, refreshToken string) error {
	if c.IdentityProvider != nil && strings.TrimSpace(refreshToken) != "" {
		return c.IdentityProvider.EndSession(ctx, refreshToken)
	}
	return nil
}
