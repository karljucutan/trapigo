package transporthttp

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/infrastructure"
	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/command"
	"github.com/karljucutan/trapigo/trapigo/internal/features/auth/web/application/query"
)

type WebAuthHandler struct {
	LoginCommand    *command.LoginCommand
	CallbackCommand *command.CallbackCommand
	LogoutCommand   *command.LogoutCommand
	MeQuery         *query.GetCurrentUserQuery
	CookieManager   *infrastructure.CookieManager
	FrontendURL     string
}

func (h *WebAuthHandler) HandleLogin(rw http.ResponseWriter, req *http.Request) {
	redirectURL, err := h.LoginCommand.Execute(req.Context())
	if err != nil {
		http.Error(rw, "login initialization failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, req, redirectURL, http.StatusFound)
}

func (h *WebAuthHandler) HandleCallback(rw http.ResponseWriter, req *http.Request) {
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

func (h *WebAuthHandler) HandleLogout(rw http.ResponseWriter, req *http.Request) {
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

func (h *WebAuthHandler) HandleMe(rw http.ResponseWriter, req *http.Request) {
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
