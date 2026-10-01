package command

import (
	"context"
	"strings"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
)

type RefreshTokenRevoker interface {
	RevokeRefreshToken(ctx context.Context, refreshToken string) error
}

type LogoutCommand struct {
	CookieManager *infrastructure.CookieManager
	Revoker       RefreshTokenRevoker
}

func (c *LogoutCommand) Execute(ctx context.Context, refreshToken string) error {
	if c.Revoker != nil && strings.TrimSpace(refreshToken) != "" {
		return c.Revoker.RevokeRefreshToken(ctx, refreshToken)
	}
	return nil
}
