package factory

import (
	"context"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type Cache interface {
	Ping(ctx context.Context) error
}

type RedisClientProvider interface {
	GetClient() *redis.Client
}

type RateLimitConfig struct {
	Requests int
	Window   time.Duration
	Logger   ratelimit.Logger
}

type RateLimiterFactory interface {
	CreateRateLimiter() ratelimit.RateLimiter
}

type DefaultRateLimiterFactory struct {
	config *ratelimit.RateLimitConfig
}

func NewDefaultRateLimiterFactory(requests int, window time.Duration, cache Cache, logger ratelimit.Logger) *DefaultRateLimiterFactory {

	var redisClient *redis.Client
	if cache != nil {
		if provider, ok := cache.(RedisClientProvider); ok {
			redisClient = provider.GetClient()
		}
	}

	return &DefaultRateLimiterFactory{
		config: &ratelimit.RateLimitConfig{
			Requests: requests,
			Window:   window,
			Redis:    redisClient,
			Logger:   logger,
		},
	}
}

func (f *DefaultRateLimiterFactory) CreateRateLimiter() ratelimit.RateLimiter {
	return ratelimit.NewRateLimiter(f.config)
}

type FactoryContainer struct {
	RateLimiterFactory RateLimiterFactory
}

func NewFactoryContainer(db *gorm.DB, logger *log.Logger, rateLimitConfig *RateLimitConfig, cache Cache) *FactoryContainer {
	rateLimiterFactory := NewDefaultRateLimiterFactory(rateLimitConfig.Requests, rateLimitConfig.Window, cache, rateLimitConfig.Logger)

	return &FactoryContainer{
		RateLimiterFactory: rateLimiterFactory,
	}
}
