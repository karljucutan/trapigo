package transporthttp

import "testing"

func TestValidateReturnURL(t *testing.T) {
	const frontend = "http://localhost:3000"
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"absent", "", ""},
		{"relative", "/dashboard", frontend + "/dashboard"},
		{"absolute", frontend + "/dashboard?tab=logs#latest", frontend + "/dashboard?tab=logs#latest"},
		{"external", "https://evil.example/dashboard", ""},
		{"wrong port", "http://localhost/dashboard", ""},
		{"wrong scheme", "https://localhost:3000/dashboard", ""},
		{"protocol relative", "//evil.example/dashboard", ""},
		{"credentials", "http://user@localhost:3000/dashboard", ""},
		{"backslash", "/\\evil.example", ""},
		{"encoded backslash", "/%5cevil.example", ""},
		{"encoded double slash", "/%2f/evil.example", ""},
		{"relative without slash", "dashboard", ""},
		{"invalid escape", "/%zz", ""},
		{"newline", "/dashboard\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateReturnURL(tc.raw, frontend)
			if tc.want == "" && tc.raw != "" {
				if err == nil {
					t.Fatalf("expected rejection, got %q", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
