package middleware

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/pkg/utils"
	"github.com/gin-gonic/gin"
)

type HSTSConfig struct {
	Enabled           bool
	MaxAge            int64
	IncludeSubdomains bool
}

type SecurityConfig struct {
	HSTS HSTSConfig
}

func ResolveHSTSConfig() HSTSConfig {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))

	enabled := false
	if raw := utils.GetEnvTrimmed("HSTS_ENABLED"); raw != "" {
		if b, err := strconv.ParseBool(raw); err == nil {
			enabled = b
		}
	} else {
		enabled = appEnv == "production" || appEnv == "prod"
	}

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

	return HSTSConfig{
		Enabled:           enabled,
		MaxAge:            maxAge,
		IncludeSubdomains: includeSubdomains,
	}
}

func NewSecurityHeadersMiddleware(cfg SecurityConfig) gin.HandlerFunc {
	hstsValue := buildHSTSHeaderValue(cfg.HSTS)

	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		if cfg.HSTS.Enabled && isEffectivelyHTTPS(c) {
			h.Set("Strict-Transport-Security", hstsValue)
		}
		c.Next()
	}
}

func isEffectivelyHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")))
	return proto == "https"
}

func buildHSTSHeaderValue(cfg HSTSConfig) string {
	value := fmt.Sprintf("max-age=%d", cfg.MaxAge)
	if cfg.IncludeSubdomains {
		value += "; includeSubDomains"
	}
	return value
}
