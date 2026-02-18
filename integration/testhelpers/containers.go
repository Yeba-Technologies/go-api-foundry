package testhelpers

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func requireDocker(t *testing.T) {
	t.Helper()
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("Docker is not available, skipping integration test: %v", err)
	}
}

type PostgresContainer struct {
	*tcpostgres.PostgresContainer
	ConnStr string
}

type RedisContainer struct {
	testcontainers.Container
	Host string
	Port string
}

// StartPostgres spins up an ephemeral Postgres container and returns a
// connected *gorm.DB and a *PostgresContainer (including its connection string).
// Container teardown is registered via t.Cleanup and runs automatically when the test completes.
func StartPostgres(ctx context.Context, t *testing.T) (*gorm.DB, *PostgresContainer) {
	t.Helper()
	requireDocker(t)

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:18.1-alpine",
		tcpostgres.WithDatabase("test_db"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get postgres connection string: %v", err)
	}

	db, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to postgres container: %v", err)
	}

	return db, &PostgresContainer{PostgresContainer: pgContainer, ConnStr: connStr}
}

// StartRedis spins up an ephemeral Redis container and returns host:port info.
// The container is removed automatically when the test completes.
func StartRedis(ctx context.Context, t *testing.T) *RedisContainer {
	t.Helper()
	requireDocker(t)

	redisContainer, err := tcredis.Run(ctx,
		"redis:8.4-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections").
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}

	t.Cleanup(func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate redis container: %v", err)
		}
	})

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}

	mappedPort, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	return &RedisContainer{
		Container: redisContainer,
		Host:      host,
		Port:      mappedPort.Port(),
	}
}

// Addr returns the host:port string for connecting to the Redis container.
func (r *RedisContainer) Addr() string {
	return fmt.Sprintf("%s:%s", r.Host, r.Port)
}
