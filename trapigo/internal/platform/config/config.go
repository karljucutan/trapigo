package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP HTTPConfig `yaml:"http"`
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

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
