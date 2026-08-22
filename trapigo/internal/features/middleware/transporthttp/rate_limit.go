package transporthttp

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/karljucutan/trapigo/trapigo/internal/platform/config"
)

type RateLimitPolicy struct {
	Enabled bool
	TokenBucket
}

type TokenBucket struct {
	Capacity            int
	RefillRate          int
	RefillIntervalInSec int
}

type tokenBucketLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*tokenBucketState
	tokenBucket TokenBucket
}

type tokenBucketState struct {
	tokens     float64
	lastRefill time.Time
}

func NewRateLimitPolicy(cfg *config.RateLimit) *RateLimitPolicy {
	if cfg == nil {
		return nil
	}

	return &RateLimitPolicy{
		Enabled: cfg.Enabled,
		TokenBucket: TokenBucket{
			Capacity:            cfg.TokenBucket.Capacity,
			RefillRate:          cfg.TokenBucket.RefillRate,
			RefillIntervalInSec: cfg.TokenBucket.RefillIntervalInSec,
		},
	}
}

// RateLimitMiddleware builds a middleware for the provided policy.
// It returns a function that accepts an http.Handler and returns a wrapped http.Handler.
// This pattern lets us configure the limiter once, then apply it to any handler.
func RateLimitMiddleware(policy *RateLimitPolicy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if policy == nil || !policy.Enabled || policy.TokenBucket.Capacity <= 0 {
			return next
		}

		limiter := &tokenBucketLimiter{
			buckets:     make(map[string]*tokenBucketState),
			tokenBucket: policy.TokenBucket,
		}

		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if !limiter.consume(clientKey(req)) {
				rw.Header().Set("Retry-After", "1")
				http.Error(rw, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(rw, req)
		})
	}
}

// consume attempts to take a token for the client. If the bucket is empty,
// the request is rejected. This limiter is in-memory and works for a single
// app instance only.
func (tb *tokenBucketLimiter) consume(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now().UTC()
	state, ok := tb.buckets[key]
	if !ok {
		state = &tokenBucketState{
			tokens:     float64(tb.tokenBucket.Capacity),
			lastRefill: now,
		}
		tb.buckets[key] = state
	}

	refillInterval := time.Duration(tb.tokenBucket.RefillIntervalInSec) * time.Second
	if refillInterval > 0 {
		elapsed := now.Sub(state.lastRefill)
		if elapsed > 0 {
			tokensToAdd := float64(elapsed) / float64(refillInterval) * float64(tb.tokenBucket.RefillRate)
			if tokensToAdd > 0 {
				state.tokens = math.Min(float64(tb.tokenBucket.Capacity), state.tokens+tokensToAdd)
				state.lastRefill = now
			}
		}
	}

	if state.tokens >= 1 {
		state.tokens -= 1
		return true
	}

	return false
}

// TODO: Distributed bucket implementation using Redis

func clientKey(req *http.Request) string {
	if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		return host
	}

	return req.RemoteAddr
}
