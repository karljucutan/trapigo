package domain

import "time"

// OAuthState represents the temporary browser state exchanged during the OIDC login flow.
type OAuthState struct {
	State        string
	Nonce        string
	CodeVerifier string
	CreatedAt    time.Time
}

func (s *OAuthState) Valid() bool {
	if s == nil {
		return false
	}
	return s.State != "" && s.Nonce != "" && s.CodeVerifier != "" && !s.CreatedAt.IsZero()
}

func (s *OAuthState) IsExpired(now time.Time, ttl time.Duration) bool {
	if s == nil || s.CreatedAt.IsZero() {
		return true
	}
	return now.Sub(s.CreatedAt) > ttl
}
