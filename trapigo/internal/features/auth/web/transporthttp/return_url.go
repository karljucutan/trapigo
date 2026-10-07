package transporthttp

import (
	"fmt"
	"net/url"
	"strings"
)

func validateReturnURL(raw, frontendURL string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > 4096 || strings.ContainsAny(raw, "\\\r\n\t") || strings.HasPrefix(raw, "//") {
		return "", fmt.Errorf("invalid return URL")
	}
	base, err := url.Parse(frontendURL)
	if err != nil || base == nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" || base.User != nil {
		return "", fmt.Errorf("frontend URL is not configured")
	}
	target, err := url.Parse(raw)
	if err != nil || target.User != nil || target.Opaque != "" {
		return "", fmt.Errorf("invalid return URL")
	}
	if !target.IsAbs() && (!strings.HasPrefix(raw, "/") || target.Host != "") {
		return "", fmt.Errorf("return URL must be absolute or root-relative")
	}
	target = base.ResolveReference(target)
	if target.Scheme != base.Scheme || !strings.EqualFold(target.Host, base.Host) ||
		strings.HasPrefix(target.Path, "//") || strings.ContainsAny(target.Path, "\\\r\n\t") {
		return "", fmt.Errorf("return URL must use the frontend origin")
	}
	return target.String(), nil
}
