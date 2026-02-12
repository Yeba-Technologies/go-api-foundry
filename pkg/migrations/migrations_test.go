package migrations

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

type blockingMigrator struct {
	closeCh   chan struct{}
	closeOnce sync.Once
	closed    atomic.Bool
}

func newBlockingMigrator() *blockingMigrator {
	return &blockingMigrator{closeCh: make(chan struct{})}
}

func (m *blockingMigrator) Up() error {
	<-m.closeCh
	return nil
}

func (m *blockingMigrator) Close() (error, error) {
	m.closeOnce.Do(func() {
		m.closed.Store(true)
		close(m.closeCh)
	})
	return nil, nil
}

func TestUp_NilDB(t *testing.T) {
	if err := Up(context.Background(), nil, Config{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUp_ContextAlreadyCancelled_ReturnsCtxErr(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	called := atomic.Bool{}
	driverFactory = func(_ *sql.DB, _ Config) (database.Driver, error) {
		called.Store(true)
		return nil, nil
	}
	migratorFactory = func(_ string, _ database.Driver) (migrator, error) {
		called.Store(true)
		return &fakeMigrator{upErr: nil}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Up(ctx, &sql.DB{}, Config{Dir: t.TempDir()})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if called.Load() {
		t.Fatalf("expected no driver/migrator creation when ctx already cancelled")
	}
}

func TestUp_ContextDeadlineExceeded_ReturnsCtxErr_AndCloses(t *testing.T) {
	origDriverFactory := driverFactory
	origMigratorFactory := migratorFactory
	t.Cleanup(func() {
		driverFactory = origDriverFactory
		migratorFactory = origMigratorFactory
	})

	block := newBlockingMigrator()
	driverFactory = func(_ *sql.DB, _ Config) (database.Driver, error) { return nil, nil }
	migratorFactory = func(_ string, _ database.Driver) (migrator, error) {
		return block, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := Up(ctx, &sql.DB{}, Config{Dir: t.TempDir()})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if !block.closed.Load() {
		t.Fatalf("expected migrator.Close to be attempted on ctx cancellation")
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
