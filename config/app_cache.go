package config

import (
	"context"
	"os"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	pkgredis "github.com/Yeba-Technologies/go-api-foundry/pkg/redis"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/utils"
	"github.com/go-redis/redis/v8"
)

type Cache interface {
	// Get returns ("", nil) when a key is not found.
	Get(ctx context.Context, key string) (string, error)
	// Set uses ttl=0 for no expiry.
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Ping(ctx context.Context) error
	Close() error
}

// RedisClientProvider is an optional interface for caches that expose the underlying Redis client.
// This is useful for advanced operations like rate limiting with Lua scripts.
type RedisClientProvider interface {
	GetClient() *redis.Client
}

type CacheConfig struct {
	Host     string
	Port     string
	Password string
}

func NewCacheConfig() *CacheConfig {
	return &CacheConfig{
		Host:     os.Getenv("REDIS_HOST"),
		Port:     utils.GetEnvOrDefault("REDIS_PORT", "6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
	}
}

func (cc *CacheConfig) IsConfigured() bool {
	return cc.Host != ""
}

func (cc *CacheConfig) NewCache(logger *log.Logger) (Cache, error) {
	if !cc.IsConfigured() {
		logger.Error("Cache (Redis) configuration is missing")
		return nil, ErrCacheNotConfigured
	}

	cfg := &pkgredis.Config{
		Host:     cc.Host,
		Port:     cc.Port,
		Password: cc.Password,
		DB:       0, // Always use DB 0 for cache
	}

	cache, err := pkgredis.NewRedisCache(cfg)
	if err != nil {
		logger.Error("Failed to create Cache (Redis)", "error", err)
		return nil, err
	}

	logger.Info("Cache (Redis) connected successfully")
	return cache, nil
}

func (cc *CacheConfig) NewCacheOrNil(logger *log.Logger) Cache {
	if !cc.IsConfigured() {
		logger.Info("Cache (Redis) is not configured; proceeding without external cache")
		return nil
	}

	cache, err := cc.NewCache(logger)

	if err != nil {
		// Log error but don't fail - allow fallback to in-memory
		logger.Error("Failed to create Cache (Redis)", "error", err)
		return nil
	}

	return cache
}

func GetRedisClient(cache Cache) *redis.Client {
	if cache == nil {
		return nil
	}

	if provider, ok := cache.(RedisClientProvider); ok {
		return provider.GetClient()
	}

	return nil
}

func CloseCache(cache Cache, logger *log.Logger) error {
	if cache == nil {
		logger.Info("No cache provided; skipping cache close")
		return nil
	}

	if err := cache.Close(); err != nil {
		logger.Error("Failed to close cache", "error", err)
		return err
	}

	logger.Info("Cache connection closed")
	return nil
}

var ErrCacheNotConfigured = &CacheError{Message: "cache host is not configured"}

type CacheError struct {
	Message string
}

func (e *CacheError) Error() string {
	return e.Message
}
