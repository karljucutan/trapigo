package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authmiddleware "github.com/karljucutan/trapigo/trapigo/internal/features/auth/middleware"
)

func TestAddAuthorizationHeader_AddsBearerToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	ctx := authmiddleware.WithAccessToken(req.Context(), "abc123")
	req = req.WithContext(ctx)
	res := httptest.NewRecorder()

	var observed string
	handler := AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		observed = req.Header.Get("Authorization")
		rw.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(res, req)

	if observed != "Bearer abc123" {
		t.Fatalf("expected bearer header, got %q", observed)
	}
}

func TestAddAuthorizationHeader_NoTokenNoHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	req = req.WithContext(context.Background())
	res := httptest.NewRecorder()

	var observed string
	handler := AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		observed = req.Header.Get("Authorization")
		rw.WriteHeader(http.StatusNoContent)
	}))

	handler.ServeHTTP(res, req)

	if observed != "" {
		t.Fatalf("expected empty authorization header, got %q", observed)
	}
}

func TestAddAuthorizationHeader_PassesThroughDownstream401(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	ctx := authmiddleware.WithAccessToken(req.Context(), "abc123")
	req = req.WithContext(ctx)
	res := httptest.NewRecorder()

	handler := AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Error(rw, "downstream unauthorized", http.StatusUnauthorized)
	}))

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestAddAuthorizationHeader_PassesThroughDownstream500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/go-orders", nil)
	ctx := authmiddleware.WithAccessToken(req.Context(), "abc123")
	req = req.WithContext(ctx)
	res := httptest.NewRecorder()

	handler := AddAuthorizationHeader(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		http.Error(rw, "downstream failure", http.StatusInternalServerError)
	}))

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}
