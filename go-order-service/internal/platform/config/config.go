package config

import (
	"fmt"
	"os"
)

type Config struct {
	DSN      string
	Host     string
	Port     string
	Env      string
	LogLevel string
}

func Load() Config {
	return Config{
		DSN:      getenv("DB_DSN", "postgres://postgres:postgres@localhost:5432/orders_db"),
		Host:     getenv("HOST", "0.0.0.0"),
		Port:     getenv("PORT", "8080"),
		Env:      getenv("APP_ENV", "local"),
		LogLevel: getenv("LOG_LEVEL", "info"),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func (c Config) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}
