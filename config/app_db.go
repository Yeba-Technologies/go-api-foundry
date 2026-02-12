package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DBConfig struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	SSLMode         string // Default: "require" for prod safety
}

func NewDatabase(logger *log.Logger, cfg *DBConfig) (*gorm.DB, error) {
	if cfg == nil {
		cfg = &DBConfig{
			MaxIdleConns:    10,
			MaxOpenConns:    100,
			ConnMaxLifetime: time.Minute,
			SSLMode:         "require",
		}
	}

	appDatabaseURL := sanitizeEnv(GetValueFromEnvironmentVariable("APP_DATABASE_URL", ""))

	dsn, db, err, done := buildDSNFromEnv(appDatabaseURL, logger, cfg)
	if done {
		return db, err
	}

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		logger.Error("Failed to get database instance", "error", err)
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := sqlDB.Ping(); err != nil {
		logger.Error("Database ping failed", "error", err)
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	logger.Info("Database connection established successfully")
	return gdb, nil
}

func buildDSNFromEnv(appDatabaseURL string, logger *log.Logger, cfg *DBConfig) (string, *gorm.DB, error, bool) {
	if strings.TrimSpace(appDatabaseURL) != "" {
		logger.Info("Using APP_DATABASE_URL for database connection")
		return appDatabaseURL, nil, nil, false
	}

	host, portStr, user, pass, dbName, ssl := getDatabaseEnvParams()
	if ssl == "" {
		ssl = cfg.SSLMode
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		logger.Error("Invalid POSTGRES_PORT", "error", err)
		return "", nil, fmt.Errorf("invalid POSTGRES_PORT %q: %w", portStr, err), true
	}

	missing := []string{}

	if host == "" {
		missing = append(missing, "POSTGRES_HOST")
	}

	if portStr == "" {
		missing = append(missing, "POSTGRES_PORT")
	}

	if user == "" {
		missing = append(missing, "POSTGRES_USER")
	}

	if dbName == "" {
		missing = append(missing, "POSTGRES_DB_NAME")
	}

	if len(missing) > 0 {
		logger.Error("Missing required database environment variables", "missing_vars", strings.Join(missing, ", "))

		return "", nil, fmt.Errorf("missing required database env vars: %s", strings.Join(missing, ", ")), true
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbName, ssl,
	)

	logger.Info("Connecting to database",
		"host", host,
		"port", port,
		"user", user,
		"dbname", dbName,
		"sslmode", ssl,
	)
	return dsn, nil, nil, false
}

func getDatabaseEnvParams() (host, port, user, pass, dbName, ssl string) {
	host = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_HOST", ""))
	port = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_PORT", ""))
	user = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_USER", ""))
	pass = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_PASSWORD", ""))
	dbName = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_DB_NAME", ""))
	ssl = sanitizeEnv(GetValueFromEnvironmentVariable("POSTGRES_SSLMODE", ""))

	return host, port, user, pass, dbName, ssl
}

func sanitizeEnv(v string) string {
	s := strings.TrimSpace(v)

	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		s = s[1 : len(s)-1]
	}

	return s
}

func AutoMigrate(logger *log.Logger, db *gorm.DB, models ...interface{}) error {
	if db == nil {
		logger.Error("Cannot migrate: db is empty")
		return fmt.Errorf("cannot migrate: db is empty")
	}

	if err := db.AutoMigrate(models...); err != nil {
		logger.Error("Database migration failed", "error", err)
		return fmt.Errorf("auto-migrate failed: %w", err)
	}

	logger.Info("Database migration completed successfully")

	return nil
}

func CloseDatabase(db *gorm.DB, logger *log.Logger) {
	if db == nil {
		return
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Error("Failed to get SQL DB instance", "error", err)
		return
	}

	if err := sqlDB.Close(); err != nil {
		logger.Error("Failed to close database", "error", err)
	} else {
		logger.Info("Database closed successfully")
	}
}
