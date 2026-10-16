package transporthttp

import (
	"encoding/json"
	"net/http"
	"strings"
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
	rw.Header().Set("Cache-Control", "no-store")
	returnURL, err := validateReturnURL(req.URL.Query().Get("return_to"), h.FrontendURL)
	if err != nil {
		http.Error(rw, "invalid return URL", http.StatusBadRequest)
		return
	}
	redirectURL, err := h.LoginCommand.Execute(req.Context(), returnURL)
	if err != nil {
		http.Error(rw, "login initialization failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, req, redirectURL, http.StatusFound)
}

func (h *WebAuthHandler) HandleCallback(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Cache-Control", "no-store")
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")

	result, err := h.CallbackCommand.Execute(req.Context(), code, state)
	if err != nil {
		http.Error(rw, "callback validation failed", http.StatusUnauthorized)
		return
	}

	target := h.FrontendURL
	if result.ReturnURL != "" {
		target, err = validateReturnURL(result.ReturnURL, h.FrontendURL)
		if err != nil {
			http.Error(rw, "invalid return URL", http.StatusBadRequest)
			return
		}
	}
	if target == "" {
		target = "/"
	}
	idTokenCookie := h.CookieManager.IDTokenCookie(result.IDToken, 30*24*time.Hour)

	ttl := time.Until(result.Token.Expiry)
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	http.SetCookie(rw, h.CookieManager.AccessTokenCookie(result.Token.AccessToken, ttl))
	if result.Token.RefreshToken != "" {
		http.SetCookie(rw, h.CookieManager.RefreshTokenCookie(result.Token.RefreshToken, 30*24*time.Hour))
	}
	http.SetCookie(rw, idTokenCookie)

	http.Redirect(rw, req, target, http.StatusFound)
}

func (h *WebAuthHandler) HandleLogout(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Cache-Control", "no-store")
	if strings.TrimSpace(req.Header.Get("Origin")) == "" {
		http.Error(rw, "Origin header required", http.StatusForbidden)
		return
	}
	var idTokenHint string
	if h.CookieManager != nil {
		idTokenHint, _ = h.CookieManager.ExtractIDToken(req)
		http.SetCookie(rw, h.CookieManager.ClearAccessTokenCookie())
		http.SetCookie(rw, h.CookieManager.ClearRefreshTokenCookie())
		http.SetCookie(rw, h.CookieManager.ClearIDTokenCookie())
	}

	logoutURL, err := h.LogoutCommand.Execute(req.Context(), h.FrontendURL, idTokenHint)
	if err != nil {
		http.Error(rw, "could not prepare Keycloak logout; local cookies cleared", http.StatusBadGateway)
		return
	}

	http.Redirect(rw, req, logoutURL, http.StatusSeeOther)
}

func (h *WebAuthHandler) HandleMe(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Cache-Control", "private, no-store")
	rw.Header().Set("Vary", "Cookie, Authorization")
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
