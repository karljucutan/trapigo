package infrastructure

import (
	"net/url"
	"strings"

	"github.com/karljucutan/trapigo/trapigo/internal/features/origin/domain"
)

func ParseAllowedOrigins(raw []string, fallbackFrontendURL string) *domain.AllowedOrigins {
	values := []string{}
	for _, origin := range raw {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}

	if len(values) == 0 {
		fallback := strings.TrimSpace(fallbackFrontendURL)
		if fallback != "" {
			values = append(values, fallback)
		}
	}

	normalized := normalizeOrigins(values)
	return domain.NewAllowedOrigins(normalized)
}

func normalizeOrigins(origins []string) []string {
	normalized := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, raw := range origins {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		value := parsed.Scheme + "://" + parsed.Host
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}
