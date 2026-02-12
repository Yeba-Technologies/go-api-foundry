package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

type Config struct {
	Dir             string
	MigrationsTable string
	Logger          Logger
}

func Up(ctx context.Context, db *sql.DB, cfg Config) error {
	if db == nil {
		return fmt.Errorf("migrations: db is nil")
	}
	if strings.TrimSpace(cfg.Dir) == "" {
		cfg.Dir = "migrations"
	}
	if strings.TrimSpace(cfg.MigrationsTable) == "" {
		cfg.MigrationsTable = "schema_migrations"
	}

	absDir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return fmt.Errorf("migrations: resolve dir: %w", err)
	}
	sourceURL := "file://" + absDir

	driver, err := postgres.WithInstance(db, &postgres.Config{MigrationsTable: cfg.MigrationsTable})
	if err != nil {
		return fmt.Errorf("migrations: postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrations: init: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if cfg.Logger != nil {
			if srcErr != nil {
				cfg.Logger.Warn("Migrations source close error", "error", srcErr)
			}
			if dbErr != nil {
				cfg.Logger.Warn("Migrations db close error", "error", dbErr)
			}
		}
	}()

	// migrate doesn't use ctx directly for file source; keep signature for future.
	_ = ctx

	if cfg.Logger != nil {
		cfg.Logger.Info("Running SQL migrations", "dir", absDir, "table", cfg.MigrationsTable)
	}

	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			if cfg.Logger != nil {
				cfg.Logger.Info("No migrations to apply")
			}
			return nil
		}
		return fmt.Errorf("migrations: up: %w", err)
	}

	if cfg.Logger != nil {
		cfg.Logger.Info("Migrations applied successfully")
	}
	return nil
}
