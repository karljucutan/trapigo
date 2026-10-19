package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/karljucutan/trapigo/trapigo/internal/features/origin/domain"
)

func TestOriginValidationCORS(t *testing.T) {
	const origin = "http://localhost:3000"
	tests := []struct {
		name       string
		method     string
		origin     string
		preflight  string
		headers    string
		wantStatus int
		wantNext   bool
		wantCORS   bool
	}{
		{"direct get retains auth", "GET", origin, "", "", 401, true, true},
		{"preflight skips auth", "OPTIONS", origin, "GET", "traceparent", 204, false, true},
		{"case insensitive header", "OPTIONS", origin, "GET", "TraceParent", 204, false, true},
		{"no extra headers", "OPTIONS", origin, "GET", "", 204, false, true},
		{"untrusted origin", "OPTIONS", "https://untrusted.example", "GET", "traceparent", 403, false, false},
		{"untrusted get", "GET", "https://untrusted.example", "", "", 403, false, false},
		{"unsupported method", "OPTIONS", origin, "POST", "traceparent", 403, false, true},
		{"unsupported header", "OPTIONS", origin, "GET", "traceparent, authorization", 403, false, true},
		{"no origin unchanged", "GET", "", "", "", 401, true, false},
		{"ordinary options retains auth", "OPTIONS", origin, "", "", 401, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			handler := NewOriginValidationMiddleware(domain.NewAllowedOrigins([]string{origin}))(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					http.Error(w, "Unauthorized", http.StatusUnauthorized)
				}),
			)
			req := httptest.NewRequest(tt.method, "/api/v1/go-orders", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", tt.preflight)
			req.Header.Set("Access-Control-Request-Headers", tt.headers)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tt.wantStatus || called != tt.wantNext {
				t.Fatalf("status=%d next=%t, want status=%d next=%t", res.Code, called, tt.wantStatus, tt.wantNext)
			}
			if tt.wantCORS {
				for key, want := range map[string]string{
					"Access-Control-Allow-Origin":      origin,
					"Access-Control-Allow-Credentials": "true",
					"Access-Control-Expose-Headers":    "Retry-After",
				} {
					if got := res.Header().Get(key); got != want {
						t.Errorf("%s=%q, want %q", key, got, want)
					}
				}
			} else if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("unexpected CORS allow-origin %q", got)
			}
			if tt.wantStatus == http.StatusNoContent {
				if res.Header().Get("Access-Control-Allow-Methods") != "GET" ||
					res.Header().Get("Access-Control-Allow-Headers") != "traceparent" {
					t.Error("missing preflight permissions")
				}
				vary := strings.Join(res.Header().Values("Vary"), ",")
				for _, name := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
					if !strings.Contains(vary, name) {
						t.Errorf("Vary missing %s", name)
					}
				}
			}
		})
	}
}
