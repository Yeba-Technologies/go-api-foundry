package utils

import "os"

// GetEnvOrDefault returns the value of the environment variable with the given key,
// or the default value if the environment variable is not set or empty.
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
