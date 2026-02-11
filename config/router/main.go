package router

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const (
	// DefaultTimeoutDuration is the default request timeout
	DefaultTimeoutDuration = 30 * time.Second
)

type MiddlewareConfig struct {
	TimeoutDuration time.Duration
}

func NewMiddlewareConfig() *MiddlewareConfig {
	return &MiddlewareConfig{
		TimeoutDuration: DefaultTimeoutDuration,
	}
}

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
	middlewareConfig  *MiddlewareConfig

	handlerToControllerMap map[string]*RESTController
	rateLimitOverrides     map[string]ratelimit.RateLimiter
}

type RouterConfig struct {
	RateLimitRequests int
	RateLimitWindow   time.Duration
	RequestTimeout    time.Duration
}

func CreateRouterService(logger *log.Logger, cache Cache, routerConfig *RouterConfig) *RouterService {
	if mode, ok := os.LookupEnv("GIN_MODE"); ok && mode != "" {
		logger.Info("Setting Gin mode", "mode", mode)
		gin.SetMode(mode)
	}

	ginRouter := gin.New()
	ginRouter.Use(gin.Recovery())

	// SECURITY: Gin trusts all proxies by default, which makes ClientIP() depend
	// on potentially spoofed X-Forwarded-For headers. Disable trust by default
	// and require explicit configuration via TRUSTED_PROXIES.
	trustedProxies := parseTrustedProxiesEnv(os.Getenv("TRUSTED_PROXIES"))
	if err := ginRouter.SetTrustedProxies(trustedProxies); err != nil {
		logger.Error("Invalid TRUSTED_PROXIES; disabling trusted proxies", "error", err)
		_ = ginRouter.SetTrustedProxies(nil)
	} else if trustedProxies == nil {
		logger.Info("Trusted proxies disabled (TRUSTED_PROXIES not set)")
	}

	// Extract Redis client from cache if available
	var redisClient *redis.Client

	if cache != nil {
		if provider, ok := cache.(RedisClientProvider); ok {
			redisClient = provider.GetClient()
		}
	}

	rs := &RouterService{
		engine:            ginRouter,
		logger:            logger,
		rateLimitRequests: routerConfig.RateLimitRequests,
		rateLimitWindow:   routerConfig.RateLimitWindow,
		redisClient:       redisClient,
		middlewareConfig:  &MiddlewareConfig{TimeoutDuration: routerConfig.RequestTimeout},

		// Maps to track controller-specific and handler-specific rate limit overrides
		rateLimitOverrides:     make(map[string]ratelimit.RateLimiter),
		handlerToControllerMap: make(map[string]*RESTController),
	}

	rs.initRateLimiting()

	// Observability (opt-out): /metrics
	rs.mountMetrics()

	ginRouter.Use(rs.securityHeadersMiddleware())
	ginRouter.Use(rs.maxBodySizeMiddleware())
	ginRouter.Use(rs.corsMiddleware())
	ginRouter.Use(rs.rateLimitMiddleware()) // Add rate limiting before other middleware
	ginRouter.Use(rs.timeoutMiddleware())

	ginRouter.Use(rs.correlationIDMiddleware())
	ginRouter.Use(rs.loggerInjectionMiddleware())
	ginRouter.Use(rs.requestLoggingMiddleware())

	ginRouter.HandleMethodNotAllowed = true
	ginRouter.RedirectTrailingSlash = true

	ginRouter.NoRoute(func(c *gin.Context) {
		correlatedLogger := logger.WithCorrelationID(c.Request.Context())
		correlatedLogger.Error("Route not found")
		c.JSON(http.StatusNotFound, gin.H{
			"code":    apperrors.StatusNotFound,
			"message": "Route not found",
			"data":    nil,
		})
	})

	ginRouter.NoMethod(func(c *gin.Context) {
		correlatedLogger := logger.WithCorrelationID(c.Request.Context())
		correlatedLogger.Error("Method not allowed")
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"code":    apperrors.StatusMethodNotAllowed,
			"message": "Method not allowed",
			"data":    nil,
		})
	})

	rs.server = &http.Server{
		Addr:    ":8080", // Default, will be overridden in RunHTTPServer
		Handler: ginRouter,

		// Server-side timeouts are the safe way to enforce request time limits.
		// Gin's Context is not goroutine-safe, so we avoid running handlers in
		// a separate goroutine to implement timeouts.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       routerConfig.RequestTimeout,
		WriteTimeout:      routerConfig.RequestTimeout,
		IdleTimeout:       60 * time.Second,
	}

	logger.Info("Router service initialized")
	return rs
}

func parseTrustedProxiesEnv(v string) []string {
	s := strings.TrimSpace(v)
	if s == "" {
		// Disable trusted proxies: ClientIP() will use RemoteAddr.
		return nil
	}
	if s == "*" {
		// Explicit escape hatch for local/dev.
		return []string{"0.0.0.0/0", "::/0"}
	}
	parts := strings.Split(s, ",")
	proxies := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			proxies = append(proxies, p)
		}
	}
	if len(proxies) == 0 {
		return nil
	}
	return proxies
}

func (routerService *RouterService) initRateLimiting() {
	requests := routerService.rateLimitRequests
	window := routerService.rateLimitWindow

	// Use the provided Redis client, fallback to in-memory if nil
	redisClient := routerService.redisClient

	if redisClient != nil {
		// Test connection if we have a client
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			routerService.logger.Warn("Failed to connect to Redis for rate limiting, falling back to in-memory", "error", err)
			redisClient = nil
		}
	}

	// Create rate limiter using strategy pattern
	config := &ratelimit.RateLimitConfig{
		Requests: requests,
		Window:   window,
		Redis:    redisClient,
		Logger:   routerService.logger,
	}

	routerService.rateLimiter = ratelimit.NewRateLimiter(config)

	if redisClient != nil {
		routerService.logger.Info("Rate limiting initialized with Redis",
			"requests", requests,
			"window", window)
	} else {
		routerService.logger.Info("Rate limiting initialized with in-memory limiter",
			"requests", requests,
			"window", window)
	}
}

func (routerService *RouterService) GetDefaultRateLimitConfig() (int, time.Duration) {
	return routerService.rateLimitRequests, routerService.rateLimitWindow
}

func (routerService *RouterService) GetEngine() *gin.Engine {
	return routerService.engine
}

func (routerService *RouterService) GetLogger(c *RequestContext) *log.Logger {
	return routerService.logger.WithCorrelationID(c.Request.Context())
}

func (routerService *RouterService) Cleanup() {
	if routerService.rateLimiter != nil {
		if err := routerService.rateLimiter.Close(); err != nil {
			routerService.logger.Error("Failed to close rate limiter", "error", err)
		}
	}
	routerService.logger.Info("Router service cleanup completed")
}

func (routerService *RouterService) MountController(controller *RESTController) {
	routerService.logger.Info("Mounting controller",
		"name", controller.name,
		"path", controller.mountPoint,
		"version", controller.version,
	)

	controller.prepare(routerService, controller)

	routerService.logger.Info("Controller mounted",
		"name", controller.name,
		"handlers", controller.handlerCount,
	)
}

func (routerService *RouterService) RunHTTPServer() error {
	appPort, ok := os.LookupEnv("APP_PORT")
	if !ok || appPort == "" {
		appPort = "8080"
	}
	addr := ":" + appPort

	// Update server address
	routerService.server.Addr = addr

	routerService.logger.Info("Starting HTTP server", "addr", addr)

	if err := routerService.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		routerService.logger.Error("Failed to start HTTP server", "error", err)
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

func (routerService *RouterService) Shutdown(ctx context.Context) error {
	routerService.logger.Info("Shutting down HTTP server gracefully...")
	return routerService.server.Shutdown(ctx)
}

// Middleware methods
func (routerService *RouterService) correlationIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Correlation-ID")
		if id == "" {
			id = log.GenerateCorrelationID()
		}
		ctx := context.WithValue(c.Request.Context(), log.CorrelatedIDKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Header("X-Correlation-ID", id)
		c.Next()
	}
}

func (routerService *RouterService) loggerInjectionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlatedLogger := routerService.logger.WithCorrelationID(c.Request.Context())
		ctx := context.WithValue(c.Request.Context(), log.LoggerKeyForContext, correlatedLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func (routerService *RouterService) requestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		correlatedLogger := routerService.logger.WithCorrelationID(c.Request.Context())
		correlatedLogger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"remote_addr", c.ClientIP(),
		)
	}
}

func (routerService *RouterService) securityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		// HSTS: only set when we believe the request is effectively HTTPS.
		// Enabled by default in production; can be overridden via HSTS_ENABLED.
		if shouldSetHSTS(c) {
			h.Set("Strict-Transport-Security", buildHSTSValue())
		}
		c.Next()
	}
}

func shouldSetHSTS(c *gin.Context) bool {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))

	enabled := false
	if strings.TrimSpace(os.Getenv("HSTS_ENABLED")) != "" {
		b, err := strconv.ParseBool(os.Getenv("HSTS_ENABLED"))
		if err == nil {
			enabled = b
		}
	} else {
		enabled = appEnv == "production" || appEnv == "prod"
	}

	if !enabled {
		return false
	}

	if c.Request.TLS != nil {
		return true
	}
	// Common setup when TLS is terminated at a reverse proxy.
	proto := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	return proto == "https"
}

func buildHSTSValue() string {
	maxAge := int64(31536000)
	if raw := strings.TrimSpace(os.Getenv("HSTS_MAX_AGE")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			maxAge = parsed
		}
	}

	includeSubdomains := true
	if raw := strings.TrimSpace(os.Getenv("HSTS_INCLUDE_SUBDOMAINS")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			includeSubdomains = parsed
		}
	}

	value := fmt.Sprintf("max-age=%d", maxAge)
	if includeSubdomains {
		value += "; includeSubDomains"
	}
	return value
}

func (routerService *RouterService) maxBodySizeMiddleware() gin.HandlerFunc {
	// Default: 1 MiB. Adjust via MAX_REQUEST_BODY_BYTES.
	maxBytes := int64(1 << 20)
	if raw := strings.TrimSpace(os.Getenv("MAX_REQUEST_BODY_BYTES")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			maxBytes = parsed
		}
	}

	return func(c *gin.Context) {
		// Fast-path for known-size bodies.
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, ErrorResult(
				http.StatusRequestEntityTooLarge,
				"Request payload too large",
				nil,
			).ToJSON())
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func (routerService *RouterService) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		allowedOriginsStr := os.Getenv("CORS_ALLOWED_ORIGIN")

		if allowedOriginsStr == "" {

			routerService.logger.Warn("CORS_ALLOWED_ORIGIN not set, denying cross-origin request", "origin", origin)

			c.Next()

			return
		}

		// Parse comma-separated origins
		allowedOrigins := strings.Split(allowedOriginsStr, ",")
		for i, o := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(o)
		}

		originAllowed := false
		for _, allowedOrigin := range allowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				originAllowed = true
				break
			}
		}

		if !originAllowed {
			routerService.logger.Warn("CORS origin not allowed", "origin", origin, "allowed_origins", allowedOrigins)

			c.Next()

			return
		}

		// Set CORS headers
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(apperrors.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (routerService *RouterService) timeoutMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a context with timeout from config
		timeout := routerService.middlewareConfig.TimeoutDuration
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Replace the request context
		c.Request = c.Request.WithContext(ctx)

		// Important: do NOT call c.Next() in a goroutine.
		// Gin's Context is not safe for concurrent use.
		c.Next()

		// If the handler chain completed but exceeded the deadline and nothing
		// was written, return a 408. Enforcement mid-flight is handled by the
		// http.Server Read/WriteTimeouts.
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			correlatedLogger := routerService.logger.WithCorrelationID(c.Request.Context())
			correlatedLogger.Warn("Request timeout detected")
			c.AbortWithStatusJSON(http.StatusRequestTimeout, ErrorResult(
				apperrors.StatusRequestTimeout,
				"Request timeout",
				nil,
			).ToJSON())
			return
		}
	}
}

func (routerService *RouterService) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s", clientIP)
		handlerPath := c.Request.URL.Path
		handlerKey := routerService.keyForPathAndMethod(c.FullPath(), c.Request.Method)
		handlerController, controllerFound := routerService.handlerToControllerMap[handlerKey]
		handlerOverride, handlerRouterFound := routerService.rateLimitOverrides[handlerKey]

		if !controllerFound || handlerController == nil {
			routerService.logger.Error("Possible development anomaly detected. A handler might have been configured without a controller mapping", "path", handlerPath, "cases", []string{
				"Incorrect mounting of controller, direct handler registration without controller, improper handler path normalization, or misconfiguration in route definitions",
				"Usage of a non-existent handler. Possible round robin brute force attack or incorrect utilization by an engineer",
			})
			c.AbortWithStatusJSON(http.StatusNotFound, NotFoundResult(fmt.Sprintf("There is no handler configured to handle any resource at the path %s", handlerPath)).ToJSON())
			return
		}

		controllerOverride, controllerRouterFound := routerService.rateLimitOverrides[handlerController.mountPoint]

		// Make room for the context to override the limiter
		var usedLimiter ratelimit.RateLimiter = routerService.rateLimiter

		// If there is a controller override, first use it.
		if controllerRouterFound {
			usedLimiter = controllerOverride
		}

		// If there is a handler override, use it. This trend guarantees that handler overrides have precedence
		// over controller overrides.
		if handlerRouterFound {
			usedLimiter = handlerOverride
		}

		limit, window := usedLimiter.GetLimitDetails()

		// Use strategy pattern to check rate limit
		if usedLimiter != nil {
			limited, err := usedLimiter.IsLimited(key)
			if err != nil {
				routerService.logger.Error("Rate limiter error", "error", err, "client_ip", clientIP)
				// On rate limiter error, allow request but log the issue
				// This prevents blocking legitimate traffic due to infrastructure issues
				c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				c.Header("X-RateLimit-Window", window.String())
				c.Next()
				return
			}
			if limited {
				routerService.logger.Warn("Rate limit exceeded", "client_ip", clientIP)
				c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
				c.Header("X-RateLimit-Window", window.String())
				retryAfterSeconds := int(math.Ceil(window.Seconds()))
				if retryAfterSeconds < 1 {
					retryAfterSeconds = 1
				}
				c.Header("Retry-After", strconv.Itoa(retryAfterSeconds))
				c.AbortWithStatusJSON(http.StatusTooManyRequests, TooManyRequestsResult(RateLimitResponse{
					Limit:      limit,
					Window:     window.String(),
					RetryAfter: strconv.Itoa(retryAfterSeconds),
				}).ToJSON())
				return
			}
		}

		// Add rate limit headers to successful requests
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Window", window.String())
		c.Next()
	}
}
