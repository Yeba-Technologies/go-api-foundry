package router

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/gin-gonic/gin"
)

func (rs *RouterService) initRateLimiting() {
	redisClient := rs.redisClient

	if redisClient != nil {
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			rs.logger.Warn("Redis unavailable for rate limiting, falling back to in-memory", "error", err)
			redisClient = nil
		}
	}

	rs.rateLimiter = ratelimit.NewRateLimiter(&ratelimit.RateLimitConfig{
		Requests: rs.rateLimitRequests,
		Window:   rs.rateLimitWindow,
		Redis:    redisClient,
		Logger:   rs.logger,
	})

	backend := "in-memory"
	if redisClient != nil {
		backend = "Redis"
	}
	rs.logger.Info("Rate limiting initialized",
		"backend", backend,
		"requests", rs.rateLimitRequests,
		"window", rs.rateLimitWindow,
	)
}

func (rs *RouterService) rateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s", clientIP)
		requestPath := c.Request.URL.Path
		handlerKey := rs.routeKey(c.FullPath(), c.Request.Method)

		controller, found := rs.handlerToControllerMap[handlerKey]
		if !found || controller == nil {
			rs.logger.Error("No controller mapping for handler", "path", requestPath)
			c.AbortWithStatusJSON(http.StatusNotFound, NotFoundResult(
				fmt.Sprintf("No handler configured for path %s", requestPath),
			).ToJSON())
			return
		}

		limiter := rs.resolveRateLimiter(handlerKey, controller.mountPoint)
		limit, window := limiter.GetLimitDetails()

		setRateLimitHeaders(c, limit, window.String())

		limited, err := limiter.IsLimited(key)
		if err != nil {
			rs.logger.Error("Rate limiter error", "error", err, "client_ip", clientIP)
			c.Next()
			return
		}

		if limited {
			rs.logger.Warn("Rate limit exceeded", "client_ip", clientIP)
			retryAfter := int(math.Ceil(window.Seconds()))
			if retryAfter < 1 {
				retryAfter = 1
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, TooManyRequestsResult(RateLimitResponse{
				Limit:      limit,
				Window:     window.String(),
				RetryAfter: strconv.Itoa(retryAfter),
			}).ToJSON())
			return
		}

		c.Next()
	}
}

func (rs *RouterService) resolveRateLimiter(handlerKey, controllerMount string) ratelimit.RateLimiter {
	if limiter, ok := rs.rateLimitOverrides[handlerKey]; ok {
		return limiter
	}
	if limiter, ok := rs.rateLimitOverrides[controllerMount]; ok {
		return limiter
	}
	return rs.rateLimiter
}

func setRateLimitHeaders(c *gin.Context, limit int, window string) {
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Window", window)
}
