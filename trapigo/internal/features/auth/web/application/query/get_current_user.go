package query

import "github.com/karljucutan/trapigo/trapigo/internal/features/auth/domain"

type CurrentUser struct {
	Subject  string   `json:"subject"`
	Username string   `json:"username"`
	Email    string   `json:"email,omitempty"`
	Issuer   string   `json:"issuer"`
	Audience []string `json:"audience,omitempty"`
}

type GetCurrentUserQuery struct{}

func (q *GetCurrentUserQuery) Execute(claims *domain.Claims) *CurrentUser {
	if claims == nil {
		return nil
	}
	return &CurrentUser{
		Subject:  claims.Subject,
		Username: claims.Username,
		Email:    claims.Email,
		Issuer:   claims.Issuer,
		Audience: claims.Audience,
	}
}
