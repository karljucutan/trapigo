package transporthttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitMiddleware_AllowsRequestsWithinCapacity(t *testing.T) {
	limit := &RateLimitPolicy{
		Enabled: true,
		TokenBucket: TokenBucket{
			Capacity:            2,
			RefillRate:          1,
			RefillIntervalInSec: 60,
		},
	}
	handler := RateLimitMiddleware(limit)(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "192.168.0.10:1234"
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)
		if res.Code != http.StatusOK {
			t.Fatalf("request %d: expected status %d, got %d", i+1, http.StatusOK, res.Code)
		}
	}
}

func TestRateLimitMiddleware_RejectsRequestsAfterCapacity(t *testing.T) {
	limit := &RateLimitPolicy{
		Enabled: true,
		TokenBucket: TokenBucket{
			Capacity:            2,
			RefillRate:          1,
			RefillIntervalInSec: 60,
		},
	}
	handler := RateLimitMiddleware(limit)(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "192.168.0.10:1234"
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "192.168.0.10:1234"
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d after limit is exhausted, got %d", http.StatusTooManyRequests, res.Code)
	}
}

func TestRateLimitMiddleware_IgnoresDisabledConfig(t *testing.T) {
	handler := RateLimitMiddleware(&RateLimitPolicy{Enabled: false})(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "192.168.0.10:1234"
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d when rate limit is disabled, got %d", http.StatusOK, res.Code)
	}
}
