package middleware

import (
	"context"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
)

func NewLoggerInjectionMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		correlatedLogger := logger.WithCorrelationID(c.Request.Context())
		ctx := context.WithValue(c.Request.Context(), log.LoggerKeyForContext, correlatedLogger)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func NewRequestLoggingMiddleware(logger *log.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)

		correlatedLogger := logger.WithCorrelationID(c.Request.Context())
		correlatedLogger.Info("HTTP request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
			"remote_addr", c.ClientIP(),
		)
	}
}
