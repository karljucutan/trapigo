package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	Keycloak KeycloakConfig `yaml:"keycloak"`
	Auth     AuthConfig     `yaml:"auth"`
}

type KeycloakConfig struct {
	IssuerURL    string `yaml:"issuer_url"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURI  string `yaml:"redirect_uri"`
}

type AuthConfig struct {
	CookieSecure           bool     `yaml:"cookie_secure"`
	CookieSameSite         string   `yaml:"cookie_same_site"`
	StateExpirationSec     int      `yaml:"state_expiration_seconds"`
	JWTCacheTTLSec         int      `yaml:"jwt_cache_ttl_seconds"`
	RequiredScopes         []string `yaml:"required_scopes"`
	FrontendRedirectURL    string   `yaml:"frontend_redirect_url"`
	FrontendAllowedOrigins []string `yaml:"frontend_allowed_origins"`
}

type HTTPConfig struct {
	Routers   map[string]Router  `yaml:"routers"`
	Services  map[string]Service `yaml:"services"`
	RateLimit *RateLimit         `yaml:"rate-limit,omitempty"`
}

type Router struct {
	PathPrefix string `yaml:"path-prefix"`
	Service    string `yaml:"service"`
}

type Service struct {
	LoadBalancer LoadBalancer `yaml:"load-balancer"`
	RateLimit    *RateLimit   `yaml:"rate-limit,omitempty"`
}

type LoadBalancer struct {
	Servers []Server `yaml:"servers"`
}

type Server struct {
	URL string `yaml:"url"`
}

type RateLimit struct {
	Enabled     bool        `yaml:"enabled"`
	TokenBucket TokenBucket `yaml:"token-bucket"`
}

type TokenBucket struct {
	Capacity            int `yaml:"capacity"`
	RefillRate          int `yaml:"refill-rate"`
	RefillIntervalInSec int `yaml:"refill-interval-in-seconds"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
