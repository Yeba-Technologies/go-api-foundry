package monitoring

import (
	"context"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/version"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"gorm.io/gorm"
)

type Cache interface {
	Ping(ctx context.Context) error
}

type HealthStatus struct {
	Version      version.Info `json:"version"`
	Database     int          `json:"database"`      // 1 = healthy, 0 = unhealthy
	Cache        int          `json:"cache"`         // 1 = healthy, 0 = unhealthy/not configured
	MessageQueue int          `json:"message_queue"` // 1 = healthy, 0 = not implemented
	Storage      int          `json:"storage"`       // 1 = healthy, 0 = not implemented
	Uptime       int          `json:"uptime"`        // uptime in seconds
}

type MonitoringController struct {
	db        *gorm.DB
	logger    *log.Logger
	cache     Cache
	startTime time.Time
}

func NewMonitoringController(db *gorm.DB, logger *log.Logger, cache Cache) *router.RESTController {
	ctrl := &MonitoringController{
		db:        db,
		logger:    logger,
		cache:     cache,
		startTime: time.Now(),
	}

	return router.NewRESTController(
		"MonitoringController",
		"/",
		func(routerService *router.RouterService, controller *router.RESTController) {

			monitoringRateLimiter := createMonitoringRateLimiter(routerService)

			routerService.AddGetHandler(controller, monitoringRateLimiter, "", func(c *router.RequestContext) *router.ServiceResult {
				return ctrl.monitor(c)
			})

			routerService.AddGetHandler(controller, monitoringRateLimiter, "health", func(c *router.RequestContext) *router.ServiceResult {
				return ctrl.healthCheck(routerService, c)
			})

			routerService.AddGetHandler(controller, monitoringRateLimiter, "version", func(c *router.RequestContext) *router.ServiceResult {
				return router.OKResult(version.Get(), "Version info")
			})

			routerService.AddGetHandler(controller, nil, "extras/greet", func(c *router.RequestContext) *router.ServiceResult {
				return ctrl.greet(c)
			})
		},
	)
}

func createMonitoringRateLimiter(routerService *router.RouterService) ratelimit.RateLimiter {

	const monitoringRequestsPerMinute = 10 // More restrictive than default 100

	config := &ratelimit.RateLimitConfig{
		Requests: monitoringRequestsPerMinute,
		Window:   time.Minute, // 1 minute window
		Redis:    nil,         // For now, use in-memory (could be enhanced to use Redis)
		Logger:   nil,         // Logger not needed for in-memory limiter
	}

	return ratelimit.NewRateLimiter(config)
}

func (ctrl *MonitoringController) healthCheck(
	routerService *router.RouterService,
	c *router.RequestContext,
) *router.ServiceResult {
	logger := routerService.GetLogger(c)
	logger.Info("Health check endpoint called")
	healthStatus := ctrl.performHealthChecks(context.Background(), logger)

	return &router.ServiceResult{
		StatusCode: 200,
		Data:       healthStatus,
		Message:    "go-api-foundry health check completed",
	}
}

func (ctrl *MonitoringController) greet(
	c *router.RequestContext,
) *router.ServiceResult {
	return &router.ServiceResult{
		StatusCode: 200,
		Data:       "Hello, welcome to the go-api-foundry!",
		Message:    "Greeting successful",
	}
}

func (ctrl *MonitoringController) monitor(
	c *router.RequestContext,
) *router.ServiceResult {
	return &router.ServiceResult{
		StatusCode: 200,
		Data:       "Monitoring endpoint is operational.",
		Message:    "Monitoring successful",
	}
}

func (ctrl *MonitoringController) performHealthChecks(ctx context.Context, logger *log.Logger) HealthStatus {
	status := HealthStatus{
		Version: version.Get(),
		Uptime:  int(time.Since(ctrl.startTime).Seconds()),
	}

	checkDatabaseConnectivity(ctx, ctrl, &status, logger)

	checkCacheConnectivity(ctx, ctrl, &status, logger)

	status.MessageQueue = 0 // Not implemented
	status.Storage = 0      // Not implemented

	logger.Info("Message queue and storage health checks not implemented")

	return status
}

func checkCacheConnectivity(ctx context.Context, ctrl *MonitoringController, status *HealthStatus, logger *log.Logger) {
	if ctrl.cache != nil {
		if ctrl.checkCache(ctx) {
			status.Cache = 1
			logger.Info("Cache health check passed")
		} else {
			status.Cache = 0
			logger.Error("Cache health check failed")
		}
	} else {
		status.Cache = 0 // Cache not configured
		logger.Info("Cache not configured, cache health check skipped")
	}
}

func checkDatabaseConnectivity(ctx context.Context, ctrl *MonitoringController, status *HealthStatus, logger *log.Logger) {
	if ctrl.checkDatabase(ctx) {
		status.Database = 1
		logger.Info("Database health check passed")
	} else {
		status.Database = 0
		logger.Error("Database health check failed")
	}
}

func (ctrl *MonitoringController) checkDatabase(ctx context.Context) bool {
	sqlDB, err := ctrl.db.DB()
	if err != nil {
		return false
	}

	// Ping the database
	return sqlDB.PingContext(ctx) == nil
}

func (ctrl *MonitoringController) checkCache(ctx context.Context) bool {
	// Ping the cache
	return ctrl.cache.Ping(ctx) == nil
}
