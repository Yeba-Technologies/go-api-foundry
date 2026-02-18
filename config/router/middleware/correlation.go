package middleware

import (
	"context"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
)

func NewCorrelationIDMiddleware() gin.HandlerFunc {
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
