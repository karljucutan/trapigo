package infrastructure

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCookieManager_CreatesAccessCookieWithSecurityFlags(t *testing.T) {
	manager := NewCookieManager(true, http.SameSiteLaxMode, "/")
	cookie := manager.AccessTokenCookie("token-123", 5*time.Minute)

	if cookie.Name != AccessTokenCookieName {
		t.Fatalf("unexpected cookie name: %s", cookie.Name)
	}
	if cookie.Value != "token-123" {
		t.Fatalf("unexpected cookie value: %s", cookie.Value)
	}
	if !cookie.HttpOnly || !cookie.Secure {
		t.Fatal("expected secure HttpOnly cookie")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected same-site value: %v", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Fatalf("unexpected cookie path: %s", cookie.Path)
	}
	if cookie.MaxAge <= 0 {
		t.Fatalf("expected max-age > 0, got %d", cookie.MaxAge)
	}
}

func TestCookieManager_ClearsCookies(t *testing.T) {
	manager := NewCookieManager(false, http.SameSiteLaxMode, "/")
	cookie := manager.ClearRefreshTokenCookie()

	if cookie.Name != RefreshTokenCookieName {
		t.Fatalf("unexpected cookie name: %s", cookie.Name)
	}
	if cookie.MaxAge >= 0 {
		t.Fatalf("expected negative max-age for clear cookie, got %d", cookie.MaxAge)
	}
	if cookie.Value != "" {
		t.Fatalf("expected empty cookie value, got %q", cookie.Value)
	}
}

func TestCookieManager_ExtractsToken(t *testing.T) {
	manager := NewCookieManager(false, http.SameSiteLaxMode, "/")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(manager.AccessTokenCookie("abc123", 5*time.Minute))

	token, ok := manager.ExtractToken(req, AccessTokenCookieName)
	if !ok {
		t.Fatal("expected token extraction to succeed")
	}
	if token != "abc123" {
		t.Fatalf("expected token abc123, got %s", token)
	}
}
