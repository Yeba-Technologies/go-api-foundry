package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"sync"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type migrator interface {
	Up() error
	Close() (sourceErr error, databaseErr error)
}

var driverFactory = func(db *sql.DB, cfg Config) (database.Driver, error) {
	return postgres.WithInstance(db, &postgres.Config{MigrationsTable: cfg.MigrationsTable})
}

var migratorFactory = func(sourceURL string, driver database.Driver) (migrator, error) {
	return migrate.NewWithDatabaseInstance(sourceURL, "postgres", driver)
}

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
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
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
	
	// Build a proper file:// URL with correct escaping and path separators.
	// Use ToSlash to normalize Windows backslashes to forward slashes.
	sourceURL := (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absDir),
	}).String()

	driver, err := driverFactory(db, cfg)
	if err != nil {
		return fmt.Errorf("migrations: postgres driver: %w", err)
	}

	m, err := migratorFactory(sourceURL, driver)
	if err != nil {
		return fmt.Errorf("migrations: init: %w", err)
	}
	closeOnce := sync.Once{}
	closeMigrator := func() {
		closeOnce.Do(func() {
			srcErr, dbErr := m.Close()
			if cfg.Logger != nil {
				if srcErr != nil {
					cfg.Logger.Warn("Migrations source close error", "error", srcErr)
				}
				if dbErr != nil {
					cfg.Logger.Warn("Migrations db close error", "error", dbErr)
				}
			}
		})
	}
	defer closeMigrator()

	if cfg.Logger != nil {
		cfg.Logger.Info("Running SQL migrations", "dir", absDir, "table", cfg.MigrationsTable)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- m.Up()
	}()

	select {
	case <-ctx.Done():
		// Best-effort interruption. migrate doesn't accept a context directly.
		closeMigrator()
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			if err == migrate.ErrNoChange {
				if cfg.Logger != nil {
					cfg.Logger.Info("No migrations to apply")
				}
				return nil
			}
			return fmt.Errorf("migrations: up: %w", err)
		}
	}

	if cfg.Logger != nil {
		cfg.Logger.Info("Migrations applied successfully")
	}
	return nil
}
