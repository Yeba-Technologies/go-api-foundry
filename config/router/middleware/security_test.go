package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupSecurityRouter(cfg SecurityConfig) *gin.Engine {
	engine := gin.New()
	engine.Use(NewSecurityHeadersMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return engine
}

func TestSecurityHeaders_AlwaysSet(t *testing.T) {
	engine := setupSecurityRouter(SecurityConfig{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
}

func TestSecurityHeaders_HSTS_DisabledByDefault(t *testing.T) {
	engine := setupSecurityRouter(SecurityConfig{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_EnabledHTTPS_XForwardedProto(t *testing.T) {
	cfg := SecurityConfig{HSTS: HSTSConfig{Enabled: true, MaxAge: 31536000, IncludeSubdomains: true}}
	engine := setupSecurityRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	engine.ServeHTTP(w, req)

	assert.Equal(t, "max-age=31536000; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_EnabledHTTPS_DirectTLS(t *testing.T) {
	cfg := SecurityConfig{HSTS: HSTSConfig{Enabled: true, MaxAge: 3600, IncludeSubdomains: false}}
	engine := setupSecurityRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.TLS = &tls.ConnectionState{}
	engine.ServeHTTP(w, req)

	assert.Equal(t, "max-age=3600", w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_EnabledButHTTP_NoHeader(t *testing.T) {
	cfg := SecurityConfig{HSTS: HSTSConfig{Enabled: true, MaxAge: 31536000, IncludeSubdomains: true}}
	engine := setupSecurityRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_HSTS_CustomMaxAge(t *testing.T) {
	cfg := SecurityConfig{HSTS: HSTSConfig{Enabled: true, MaxAge: 600, IncludeSubdomains: true}}
	engine := setupSecurityRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	engine.ServeHTTP(w, req)

	assert.Equal(t, "max-age=600; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
}

func TestBuildHSTSHeaderValue(t *testing.T) {
	tests := []struct {
		name     string
		cfg      HSTSConfig
		expected string
	}{
		{"with subdomains", HSTSConfig{MaxAge: 31536000, IncludeSubdomains: true}, "max-age=31536000; includeSubDomains"},
		{"without subdomains", HSTSConfig{MaxAge: 3600, IncludeSubdomains: false}, "max-age=3600"},
		{"zero max-age", HSTSConfig{MaxAge: 0, IncludeSubdomains: false}, "max-age=0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildHSTSHeaderValue(tt.cfg))
		})
	}
}
