package config

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/constants"
	"gorm.io/gorm"
)

type ApplicationConfig struct {
	DB            *gorm.DB
	RouterService *router.RouterService
	Logger        *log.Logger
	Cache         Cache
	Config        *AppConfig
	TracingShutdown func(context.Context) error
}

type AppConfig struct {
	RateLimitRequests int
	RateLimitWindow   time.Duration
	RequestTimeout    time.Duration
}

func NewAppConfig() *AppConfig {
	config := &AppConfig{
		RateLimitRequests: constants.DefaultRateLimitRequests,
		RateLimitWindow:   constants.DefaultRateLimitWindow(),
		RequestTimeout:    30 * time.Second, // Default request timeout
	}

	// Override from environment variables
	if reqStr := os.Getenv("RATE_LIMIT_REQUESTS"); reqStr != "" {
		if parsed, err := strconv.Atoi(reqStr); err == nil && parsed > 0 {
			config.RateLimitRequests = parsed
		}
	}

	if winStr := os.Getenv("RATE_LIMIT_WINDOW"); winStr != "" {
		if parsed, err := time.ParseDuration(winStr); err == nil && parsed > 0 {
			config.RateLimitWindow = parsed
		}
	}

	if timeoutStr := os.Getenv("REQUEST_TIMEOUT"); timeoutStr != "" {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil && parsed > 0 {
			config.RequestTimeout = parsed
		}
	}

	return config
}

func (ac *ApplicationConfig) Cleanup() {
	if ac.TracingShutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := ac.TracingShutdown(ctx); err != nil {
			ac.Logger.Error("Failed to shutdown tracer provider", "error", err.Error())
		}
	}

	if ac.DB != nil {
		CloseDatabase(ac.DB, ac.Logger)
	}

	if ac.RouterService != nil {
		ac.RouterService.Cleanup()
	}

	if ac.Cache != nil {
		CloseCache(ac.Cache, ac.Logger)
	}

	ac.Logger.Info("Application cleanup completed")
}

func LoadApplicationConfiguration(logger *log.Logger, autoMigrate bool) (*ApplicationConfig, error) {
	InitializeEnvFile(logger)

	tracingShutdown, err := SetupTracing(logger)
	if err != nil {
		return nil, err
	}

	dbCfg := &DBConfig{}
	db, err := NewDatabase(logger, dbCfg)
	if err != nil {
		return nil, err
	}

	if autoMigrate {
		if err := AutoMigrate(logger, db, models.ModelRegistry...); err != nil {
			return nil, err
		}
	}

	appConfig := NewAppConfig()
	cache := NewCacheConfig().NewCacheOrNil(logger)

	routerService := router.CreateRouterService(logger, cache, &router.RouterConfig{
		RateLimitRequests: appConfig.RateLimitRequests,
		RateLimitWindow:   appConfig.RateLimitWindow,
		RequestTimeout:    appConfig.RequestTimeout,
	})

	logger.Info("Application configuration loaded successfully")

	return &ApplicationConfig{
		DB:            db,
		RouterService: routerService,
		Logger:        logger,
		Cache:         cache,
		Config:        appConfig,
		TracingShutdown: tracingShutdown,
	}, nil
}
