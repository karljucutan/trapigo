package domain

import "time"

// Claims describes the subset of validated JWT data the gateway needs.
type Claims struct {
	Subject   string
	Issuer    string
	Audience  []string
	Scopes    []string
	Email     string
	Username  string
	ExpiresAt time.Time
	NotBefore *time.Time
	Raw       string
}
