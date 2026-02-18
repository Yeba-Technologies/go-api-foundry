package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
)

type CORSConfig struct {
	AllowedOrigins []string
	Logger         *log.Logger
}

func ResolveCORSConfig(logger *log.Logger) CORSConfig {
	allowedOriginsStr := os.Getenv("CORS_ALLOWED_ORIGIN")
	var origins []string
	if allowedOriginsStr != "" {
		parts := strings.Split(allowedOriginsStr, ",")
		for _, o := range parts {
			o = strings.TrimSpace(o)
			if o != "" {
				origins = append(origins, o)
			}
		}
	}
	return CORSConfig{
		AllowedOrigins: origins,
		Logger:         logger,
	}
}

func NewCORSMiddleware(cfg CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if len(cfg.AllowedOrigins) == 0 {
			cfg.Logger.Warn("CORS_ALLOWED_ORIGIN not set, denying cross-origin request", "origin", origin)
			c.Next()
			return
		}

		originAllowed := false
		for _, allowedOrigin := range cfg.AllowedOrigins {
			if allowedOrigin == "*" || allowedOrigin == origin {
				originAllowed = true
				break
			}
		}

		if !originAllowed {
			cfg.Logger.Warn("CORS origin not allowed", "origin", origin, "allowed_origins", cfg.AllowedOrigins)
			c.Next()
			return
		}

		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
