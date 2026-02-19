package router

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config/router/middleware"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

type Cache interface {
	Ping(ctx context.Context) error
}

type RedisClientProvider interface {
	GetClient() *redis.Client
}

type RouterService struct {
	engine            *gin.Engine
	server            *http.Server
	logger            *log.Logger
	rateLimiter       ratelimit.RateLimiter
	rateLimitRequests int
	rateLimitWindow   time.Duration
	redisClient       *redis.Client

	handlerToControllerMap map[string]*RESTController
	rateLimitOverrides     map[string]ratelimit.RateLimiter
}

type RouterConfig struct {
	RateLimitRequests int
	RateLimitWindow   time.Duration
	RequestTimeout    time.Duration
}

func CreateRouterService(logger *log.Logger, cache Cache, cfg *RouterConfig) *RouterService {
	configureGinMode(logger)

	engine := gin.New()
	engine.Use(gin.Recovery())

	registerCustomValidators()
	configureTracing(engine, logger)
	configureTrustedProxies(engine, logger)

	rs := &RouterService{
		engine:                 engine,
		logger:                 logger,
		rateLimitRequests:      cfg.RateLimitRequests,
		rateLimitWindow:        cfg.RateLimitWindow,
		redisClient:            extractRedisClient(cache),
		rateLimitOverrides:     make(map[string]ratelimit.RateLimiter),
		handlerToControllerMap: make(map[string]*RESTController),
	}

	rs.initRateLimiting()
	rs.mountMetrics()
	rs.registerMiddleware(engine, logger, cfg.RequestTimeout)
	configureFallbackRoutes(engine, logger)
	rs.server = newHTTPServer(engine, cfg.RequestTimeout)

	logger.Info("Router service initialized")
	return rs
}

func configureGinMode(logger *log.Logger) {
	if mode, ok := os.LookupEnv("GIN_MODE"); ok && mode != "" {
		logger.Info("Setting Gin mode", "mode", mode)
		gin.SetMode(mode)
	}
}

func registerCustomValidators() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok || v == nil {
		return
	}
	_ = v.RegisterValidation("trim", func(fl validator.FieldLevel) bool {
		if fl.Field().Kind() != reflect.String {
			return true
		}
		val := fl.Field().String()
		return val == strings.TrimSpace(val)
	})
}

func configureTracing(engine *gin.Engine, logger *log.Logger) {
	if !utils.IsTracingEnabled() {
		return
	}
	engine.Use(otelgin.Middleware(utils.OTelServiceName()))
	logger.Info("Tracing middleware enabled")
}

func configureTrustedProxies(engine *gin.Engine, logger *log.Logger) {
	proxies := parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err := engine.SetTrustedProxies(proxies); err != nil {
		logger.Error("Invalid TRUSTED_PROXIES; disabling", "error", err)
		_ = engine.SetTrustedProxies(nil)
	} else if proxies == nil {
		logger.Info("Trusted proxies disabled (TRUSTED_PROXIES not set)")
	}
}

func extractRedisClient(cache Cache) *redis.Client {
	if cache == nil {
		return nil
	}
	if provider, ok := cache.(RedisClientProvider); ok {
		return provider.GetClient()
	}
	return nil
}

func (rs *RouterService) registerMiddleware(engine *gin.Engine, logger *log.Logger, timeout time.Duration) {
	engine.Use(middleware.NewSecurityHeadersMiddleware(middleware.SecurityConfig{HSTS: middleware.ResolveHSTSConfig()}))
	engine.Use(middleware.NewMaxBodySizeMiddleware(middleware.ResolveMaxBodySize()))
	engine.Use(middleware.NewCORSMiddleware(middleware.ResolveCORSConfig(logger)))
	engine.Use(rs.rateLimitMiddleware())
	engine.Use(middleware.NewTimeoutMiddleware(middleware.TimeoutConfig{Duration: timeout, Logger: logger}))
	engine.Use(middleware.NewCorrelationIDMiddleware())
	engine.Use(middleware.NewLoggerInjectionMiddleware(logger))
	engine.Use(middleware.NewRequestLoggingMiddleware(logger))

	engine.HandleMethodNotAllowed = true
	engine.RedirectTrailingSlash = true
}

func configureFallbackRoutes(engine *gin.Engine, logger *log.Logger) {
	engine.NoRoute(func(c *gin.Context) {
		logger.WithCorrelationID(c.Request.Context()).Error("Route not found")
		c.JSON(http.StatusNotFound, gin.H{
			"code":    apperrors.StatusNotFound,
			"message": "Route not found",
			"data":    nil,
		})
	})

	engine.NoMethod(func(c *gin.Context) {
		logger.WithCorrelationID(c.Request.Context()).Error("Method not allowed")
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"code":    apperrors.StatusMethodNotAllowed,
			"message": "Method not allowed",
			"data":    nil,
		})
	})
}

func newHTTPServer(handler http.Handler, requestTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       requestTimeout,
		WriteTimeout:      requestTimeout,
		IdleTimeout:       60 * time.Second,
	}
}

func (rs *RouterService) GetDefaultRateLimitConfig() (int, time.Duration) {
	return rs.rateLimitRequests, rs.rateLimitWindow
}

func (rs *RouterService) GetEngine() *gin.Engine {
	return rs.engine
}

func (rs *RouterService) GetLogger(c *RequestContext) *log.Logger {
	return rs.logger.WithCorrelationID(c.Request.Context())
}

func (rs *RouterService) Cleanup() {
	if rs.rateLimiter != nil {
		if err := rs.rateLimiter.Close(); err != nil {
			rs.logger.Error("Failed to close rate limiter", "error", err)
		}
	}
	rs.logger.Info("Router service cleanup completed")
}

func (rs *RouterService) MountController(controller *RESTController) {
	rs.logger.Info("Mounting controller",
		"name", controller.name,
		"path", controller.mountPoint,
		"version", controller.version,
	)

	controller.prepare(rs, controller)

	rs.logger.Info("Controller mounted",
		"name", controller.name,
		"handlers", controller.handlerCount,
	)
}

func (rs *RouterService) RunHTTPServer() error {
	port, ok := os.LookupEnv("APP_PORT")
	if !ok || port == "" {
		port = "8080"
	}
	addr := ":" + port
	rs.server.Addr = addr

	rs.logger.Info("Starting HTTP server", "addr", addr)

	if err := rs.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		rs.logger.Error("Failed to start HTTP server", "error", err)
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (rs *RouterService) Shutdown(ctx context.Context) error {
	rs.logger.Info("Shutting down HTTP server gracefully...")
	return rs.server.Shutdown(ctx)
}
