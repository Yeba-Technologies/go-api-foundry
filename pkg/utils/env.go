package utils

import (
	"os"
	"strings"
)

func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

func GetEnvTrimmed(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func GetEnvTrimmedOrDefault(key, defaultValue string) string {
	v := strings.TrimSpace(os.Getenv(key))

	if v == "" {
		return defaultValue
	}

	return v
}
