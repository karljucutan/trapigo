package gateway

import (
	"net/http"

	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
)

// AddAuthorizationHeader forwards the authenticated access token to downstream services.
func AddAuthorizationHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		accessToken, ok := authmiddleware.AccessTokenFromContext(req.Context())
		if ok && accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}
		next.ServeHTTP(rw, req)
	})
}
