package config

import (
	"os"
	"path/filepath"
	"testing"

	configuration "github.com/karljucutan/trapigo/trapigo/pkg"
)

func TestLoadConfig_GatewayConfig(t *testing.T) {
	configPath := writeConfigFile(t, gatewayConfigFixture())

	t.Setenv("TRAPIGO_GATEWAY_CONFIG", configPath)
	cfg, err := LoadConfig(configuration.GetEnv("TRAPIGO_GATEWAY_CONFIG", "configs/trapigo-gateway.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}

	if cfg.HTTP.RateLimit == nil {
		t.Fatal("expected http.rate-limit to be loaded")
	}
	if !cfg.HTTP.RateLimit.Enabled {
		t.Fatal("expected http.rate-limit.enabled to be true")
	}
	if cfg.HTTP.RateLimit.TokenBucket.Capacity != 100 {
		t.Fatalf("unexpected capacity: %d", cfg.HTTP.RateLimit.TokenBucket.Capacity)
	}
	if cfg.HTTP.RateLimit.TokenBucket.RefillRate != 10 {
		t.Fatalf("unexpected refill-rate: %d", cfg.HTTP.RateLimit.TokenBucket.RefillRate)
	}
	if cfg.HTTP.RateLimit.TokenBucket.RefillIntervalInSec != 1 {
		t.Fatalf("unexpected refill-interval-in-seconds: %d", cfg.HTTP.RateLimit.TokenBucket.RefillIntervalInSec)
	}
	if cfg.HTTP.Routers == nil {
		t.Fatal("expected http.routers to be loaded")
	}
	if _, ok := cfg.HTTP.Routers["users-router"]; !ok {
		t.Fatal("expected users-router to be present")
	}
	if _, ok := cfg.HTTP.Routers["orders-router"]; !ok {
		t.Fatal("expected orders-router to be present")
	}
	if _, ok := cfg.HTTP.Services["users-service"]; !ok {
		t.Fatal("expected users-service to be present")
	}
	if _, ok := cfg.HTTP.Services["orders-service"]; !ok {
		t.Fatal("expected orders-service to be present")
	}
}

func TestLoadConfig_ServiceBackends(t *testing.T) {
	configPath := writeConfigFile(t, gatewayConfigFixture())

	t.Setenv("TRAPIGO_GATEWAY_CONFIG", configPath)
	cfg, err := LoadConfig(configuration.GetEnv("TRAPIGO_GATEWAY_CONFIG", "configs/trapigo-gateway.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}

	if cfg.HTTP.Services["users-service"].LoadBalancer.Servers[0].URL != "http://app-one:8080" {
		t.Fatalf("unexpected users-service upstream: %s", cfg.HTTP.Services["users-service"].LoadBalancer.Servers[0].URL)
	}
	if cfg.HTTP.Services["orders-service"].LoadBalancer.Servers[0].URL != "http://app-two:8080" {
		t.Fatalf("unexpected orders-service upstream: %s", cfg.HTTP.Services["orders-service"].LoadBalancer.Servers[0].URL)
	}
}

func gatewayConfigFixture() string {
	return `http:
  rate-limit:
    enabled: true
    token-bucket:
      capacity: 100
      refill-rate: 10
      refill-interval-in-seconds: 1

  routers:
    users-router:
      path-prefix: /api/v1/users
      service: users-service

    orders-router:
      path-prefix: /api/v1/orders
      service: orders-service

  services:
    users-service:
      load-balancer:
        servers:
          - url: http://app-one:8080

    orders-service:
      load-balancer:
        servers:
          - url: http://app-two:8080
`
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "trapigo-gateway.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile returned an error: %v", err)
	}
	return configPath
}
