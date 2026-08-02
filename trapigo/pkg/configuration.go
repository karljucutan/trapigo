package pkg

import (
	"log"
	"os"
	"strconv"
	"time"
)

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func GetEnvAsInt(name string, defaultValue int) int {
	valueStr := GetEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func GetEnvDuration(key string, defaultValue int, unit time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return time.Duration(defaultValue) * unit
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid value for %s, using fallback %d", key, defaultValue)
		return time.Duration(defaultValue) * unit
	}
	return time.Duration(value) * unit
}
