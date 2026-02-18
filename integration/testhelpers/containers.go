package testhelpers

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tclocalstack "github.com/testcontainers/testcontainers-go/modules/localstack"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
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

// LocalStackContainer wraps a testcontainers LocalStack instance.
type LocalStackContainer struct {
	*tclocalstack.LocalStackContainer
	Host string
	Port string
}

// RabbitMQContainer wraps a testcontainers RabbitMQ instance.
type RabbitMQContainer struct {
	*tcrabbitmq.RabbitMQContainer
	Host     string
	AmqpPort string
	Username string
	Password string
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

// StartLocalStack spins up an ephemeral LocalStack container with SQS enabled.
// The container is removed automatically when the test completes.
func StartLocalStack(ctx context.Context, t *testing.T) *LocalStackContainer {
	t.Helper()
	requireDocker(t)

	lsContainer, err := tclocalstack.Run(ctx,
		"localstack/localstack:4.13",
		testcontainers.WithEnv(map[string]string{
			"SERVICES":              "sqs",
			"AWS_ACCESS_KEY_ID":     "test",
			"AWS_SECRET_ACCESS_KEY": "test",
			"AWS_DEFAULT_REGION":    "us-east-1",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForHTTP("/_localstack/health").
				WithPort("4566/tcp").
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start localstack container: %v", err)
	}

	t.Cleanup(func() {
		if err := lsContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate localstack container: %v", err)
		}
	})

	host, err := lsContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get localstack host: %v", err)
	}

	mappedPort, err := lsContainer.MappedPort(ctx, "4566")
	if err != nil {
		t.Fatalf("failed to get localstack port: %v", err)
	}

	return &LocalStackContainer{
		LocalStackContainer: lsContainer,
		Host:                host,
		Port:                mappedPort.Port(),
	}
}

// Endpoint returns the HTTP endpoint URL for the LocalStack container.
func (l *LocalStackContainer) Endpoint() string {
	return fmt.Sprintf("http://%s:%s", l.Host, l.Port)
}

// StartRabbitMQ spins up an ephemeral RabbitMQ container with the management
// plugin. The container is removed automatically when the test completes.
func StartRabbitMQ(ctx context.Context, t *testing.T) *RabbitMQContainer {
	t.Helper()
	requireDocker(t)

	const username = "test"
	const password = "test"

	rmqContainer, err := tcrabbitmq.Run(ctx,
		"rabbitmq:4.2-management-alpine",
		tcrabbitmq.WithAdminUsername(username),
		tcrabbitmq.WithAdminPassword(password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Server startup complete").
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start rabbitmq container: %v", err)
	}

	t.Cleanup(func() {
		if err := rmqContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate rabbitmq container: %v", err)
		}
	})

	host, err := rmqContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get rabbitmq host: %v", err)
	}

	mappedPort, err := rmqContainer.MappedPort(ctx, "5672")
	if err != nil {
		t.Fatalf("failed to get rabbitmq amqp port: %v", err)
	}

	return &RabbitMQContainer{
		RabbitMQContainer: rmqContainer,
		Host:              host,
		AmqpPort:          mappedPort.Port(),
		Username:          username,
		Password:          password,
	}
}

// AmqpURL returns the AMQP connection URL for the RabbitMQ container.
func (r *RabbitMQContainer) AmqpURL() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%s/", r.Username, r.Password, r.Host, r.AmqpPort)
}
