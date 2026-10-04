package domain

import "slices"

type AllowedOrigins struct {
	origins []string
}

func NewAllowedOrigins(origins []string) *AllowedOrigins {
	return &AllowedOrigins{
		origins: origins,
	}
}

func (a *AllowedOrigins) Validate(origin string) bool {
	return slices.Contains(a.origins, origin)
}

func (a *AllowedOrigins) List() []string {
	return a.origins
}
