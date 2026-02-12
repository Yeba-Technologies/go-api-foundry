package migrations

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
)

type testLogger struct {
	infos  []string
	warns  []string
	errors []string
}

func (l *testLogger) Info(msg string, _ ...any)  { l.infos = append(l.infos, msg) }
func (l *testLogger) Warn(msg string, _ ...any)  { l.warns = append(l.warns, msg) }
func (l *testLogger) Error(msg string, _ ...any) { l.errors = append(l.errors, msg) }

type fakeMigrator struct {
	upErr error
}

func (m *fakeMigrator) Up() error { return m.upErr }
func (m *fakeMigrator) Close() (error, error) {
	return nil, nil
}

func TestUp_NilDB(t *testing.T) {
	if err := Up(context.Background(), nil, Config{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUp_ErrNoChange_ReturnsNil(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	logger := &testLogger{}
	driverFactory = func(_ *sql.DB, _ Config) (database.Driver, error) { return nil, nil }
	migratorFactory = func(_ string, _ database.Driver) (migrator, error) {
		return &fakeMigrator{upErr: migrate.ErrNoChange}, nil
	}

	err := Up(context.Background(), &sql.DB{}, Config{Dir: t.TempDir(), Logger: logger})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	found := false
	for _, msg := range logger.infos {
		if msg == "No migrations to apply" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'No migrations to apply' log")
	}
}

func TestUp_Success_LogsApplied(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	logger := &testLogger{}
	driverFactory = func(_ *sql.DB, _ Config) (database.Driver, error) { return nil, nil }
	migratorFactory = func(_ string, _ database.Driver) (migrator, error) {
		return &fakeMigrator{upErr: nil}, nil
	}

	err := Up(context.Background(), &sql.DB{}, Config{Dir: t.TempDir(), Logger: logger})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	found := false
	for _, msg := range logger.infos {
		if msg == "Migrations applied successfully" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'Migrations applied successfully' log")
	}
}

func TestUp_BuildsFileSourceURL(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	tmp := t.TempDir()
	var gotSourceURL string

	driverFactory = func(_ *sql.DB, cfg Config) (database.Driver, error) {
		if cfg.MigrationsTable == "" {
			t.Fatalf("expected migrations table to be defaulted")
		}
		return nil, nil
	}
	migratorFactory = func(sourceURL string, _ database.Driver) (migrator, error) {
		gotSourceURL = sourceURL
		return &fakeMigrator{upErr: migrate.ErrNoChange}, nil
	}

	err := Up(context.Background(), &sql.DB{}, Config{Dir: tmp})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	abs, _ := filepath.Abs(tmp)
	expected := "file://" + abs

	// Normalize path separators for Windows.
	if runtime.GOOS == "windows" {
		expected = strings.ReplaceAll(expected, "\\", "/")
		gotSourceURL = strings.ReplaceAll(gotSourceURL, "\\", "/")
	}

	if gotSourceURL != expected {
		t.Fatalf("expected sourceURL %q, got %q", expected, gotSourceURL)
	}
}

func TestUp_MigratorInitError(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	driverFactory = func(_ *sql.DB, _ Config) (database.Driver, error) { return nil, nil }
	migratorFactory = func(_ string, _ database.Driver) (migrator, error) {
		return nil, errors.New("boom")
	}

	err := Up(context.Background(), &sql.DB{}, Config{Dir: t.TempDir()})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "migrations: init") {
		t.Fatalf("expected wrapped init error, got %v", err)
	}
}
