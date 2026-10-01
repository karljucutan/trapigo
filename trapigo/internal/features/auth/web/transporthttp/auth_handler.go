package transporthttp

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/command"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/query"
)

type AuthHandler struct {
	LoginCommand    *command.LoginCommand
	CallbackCommand *command.CallbackCommand
	LogoutCommand   *command.LogoutCommand
	MeQuery         *query.GetCurrentUserQuery
	CookieManager   *infrastructure.CookieManager
	FrontendURL     string
	AllowedOrigins  []string
}

func (h *AuthHandler) HandleLogin(rw http.ResponseWriter, req *http.Request) {
	redirectURL, err := h.LoginCommand.Execute(req.Context())
	if err != nil {
		http.Error(rw, "login initialization failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, req, redirectURL, http.StatusFound)
}

func (h *AuthHandler) HandleCallback(rw http.ResponseWriter, req *http.Request) {
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")

	result, err := h.CallbackCommand.Execute(req.Context(), code, state)
	if err != nil {
		http.Error(rw, "callback validation failed", http.StatusUnauthorized)
		return
	}

	ttl := time.Until(result.Token.Expiry)
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	http.SetCookie(rw, h.CookieManager.AccessTokenCookie(result.Token.AccessToken, ttl))
	if result.Token.RefreshToken != "" {
		http.SetCookie(rw, h.CookieManager.RefreshTokenCookie(result.Token.RefreshToken, 30*24*time.Hour))
	}

	target := h.FrontendURL
	if target == "" {
		target = "/"
	}
	http.Redirect(rw, req, target, http.StatusFound)
}

func (h *AuthHandler) HandleLogout(rw http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.isAllowedOrigin(req.Header.Get("Origin")) {
		http.Error(rw, "forbidden", http.StatusForbidden)
		return
	}

	refreshToken, _ := h.CookieManager.ExtractToken(req, infrastructure.RefreshTokenCookieName)
	if err := h.LogoutCommand.Execute(req.Context(), refreshToken); err != nil {
		http.Error(rw, "logout failed", http.StatusUnauthorized)
		return
	}

	http.SetCookie(rw, h.CookieManager.ClearAccessTokenCookie())
	http.SetCookie(rw, h.CookieManager.ClearRefreshTokenCookie())
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte(`{"status":"logged_out"}`))
}

func (h *AuthHandler) HandleMe(rw http.ResponseWriter, req *http.Request) {
	claims, ok := authmiddleware.ClaimsFromContext(req.Context())
	if !ok || claims == nil {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}

	currentUser := h.MeQuery.Execute(claims)
	rw.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(rw).Encode(currentUser); err != nil {
		http.Error(rw, "failed to encode response", http.StatusInternalServerError)
	}
}

func (h *AuthHandler) isAllowedOrigin(origin string) bool {
	allowed := normalizeOrigins(h.AllowedOrigins)
	if len(allowed) == 0 {
		return true
	}
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	for _, candidate := range allowed {
		if origin == candidate {
			return true
		}
	}
	return false
}

func normalizeOrigins(origins []string) []string {
	normalized := make([]string, 0, len(origins))
	seen := map[string]struct{}{}
	for _, raw := range origins {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Scheme == "" || u.Host == "" {
			continue
		}
		value := u.Scheme + "://" + u.Host
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}
