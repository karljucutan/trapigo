package infrastructure

import (
	"net/http"
	"strings"
	"time"
)

const (
	AccessTokenCookieName  = "web_access_token"
	RefreshTokenCookieName = "web_refresh_token"
)

type CookieManager struct {
	secure   bool
	sameSite http.SameSite
	path     string
}

func NewCookieManager(secure bool, sameSite http.SameSite, path string) *CookieManager {
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	return &CookieManager{secure: secure, sameSite: sameSite, path: path}
}

func (m *CookieManager) AccessTokenCookie(token string, ttl time.Duration) *http.Cookie {
	return m.newCookie(AccessTokenCookieName, token, ttl)
}

func (m *CookieManager) RefreshTokenCookie(token string, ttl time.Duration) *http.Cookie {
	return m.newCookie(RefreshTokenCookieName, token, ttl)
}

func (m *CookieManager) ClearAccessTokenCookie() *http.Cookie {
	return m.newClearCookie(AccessTokenCookieName)
}

func (m *CookieManager) ClearRefreshTokenCookie() *http.Cookie {
	return m.newClearCookie(RefreshTokenCookieName)
}

func (m *CookieManager) ExtractToken(r *http.Request, name string) (string, bool) {
	if r == nil {
		return "", false
	}
	cookie, err := r.Cookie(name)
	if err != nil || cookie == nil {
		return "", false
	}
	if strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	return cookie.Value, true
}

func (m *CookieManager) newCookie(name, value string, ttl time.Duration) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     m.path,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: m.sameSite,
	}
	if ttl > 0 {
		cookie.MaxAge = int(ttl.Seconds())
		cookie.Expires = time.Now().Add(ttl)
	}
	return cookie
}

func (m *CookieManager) newClearCookie(name string) *http.Cookie {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     m.path,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: m.sameSite,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	}
	return cookie
}
