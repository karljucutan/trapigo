package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
	Routers  map[string]Router  `yaml:"routers"`
	Services map[string]Service `yaml:"services"`
}

type Router struct {
	PathPrefix string `yaml:"path-prefix"`
	Service    string `yaml:"service"`
}

type Service struct {
	LoadBalancer LoadBalancer `yaml:"load-balancer"`
}

type LoadBalancer struct {
	Servers []Server `yaml:"servers"`
}

type Server struct {
	URL string `yaml:"url"`
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
