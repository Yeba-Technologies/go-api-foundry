package config

import (
	"os"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/joho/godotenv"
)

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
