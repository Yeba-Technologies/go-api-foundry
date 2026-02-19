package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
)

const DefaultTimeoutDuration = 30 * time.Second

type TimeoutConfig struct {
	Duration time.Duration
	Logger   *log.Logger
}

func NewTimeoutMiddleware(cfg TimeoutConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), cfg.Duration)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		c.Next()

		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			correlatedLogger := cfg.Logger.WithCorrelationID(c.Request.Context())
			correlatedLogger.Warn("Request timeout detected")
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
				"code":    http.StatusRequestTimeout,
				"data":    nil,
				"message": "Request timeout",
			})
			return
		}
	}
}
