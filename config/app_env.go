package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/joho/godotenv"
)

const AppEnvKey = "APP_ENV"

func InitializeEnvFile(logger *log.Logger) {
	logger.Info("Initializing environment variables from .env file if present")

	// Use explicit environment variable instead of fragile binary name detection
	if os.Getenv("SKIP_DOTENV") == "true" {
		logger.Info("Skipping .env file load (SKIP_DOTENV=true)")
		return
	}

	if err := godotenv.Load(); err != nil {
		logger.Warn("No .env file found or failed to load it", "error", err.Error())
		return
	}

	logger.Info("Environment variables loaded from .env file successfully")
}

func GetValueFromEnvironmentVariable(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultValue
}

func GetAppEnv() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(AppEnvKey)))
}

func ValidateAutoMigrateAllowed(appEnv string) error {
	env := strings.ToLower(strings.TrimSpace(appEnv))

	switch env {
	case "", "dev", "development", "local", "test", "testing":
		return nil
	default:
		return fmt.Errorf("--auto-migrate is not allowed when %s=%q (allowed: \"\", dev, development, local, test, testing)", AppEnvKey, env)
	}
}
