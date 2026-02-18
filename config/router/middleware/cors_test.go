package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newTestLogger() *log.Logger {
	return log.NewLoggerWithJSONOutput()
}

func setupCORSRouter(cfg CORSConfig) *gin.Engine {
	engine := gin.New()
	engine.Use(NewCORSMiddleware(cfg))
	engine.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	engine.OPTIONS("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return engine
}

func TestCORS_NoOriginsConfigured_DeniesAndContinues(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: nil, Logger: newTestLogger()}
	engine := setupCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_OriginAllowed_SetsHeaders(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}, Logger: newTestLogger()}
	engine := setupCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORS_OriginNotAllowed_NoHeaders(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}, Logger: newTestLogger()}
	engine := setupCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://evil.com")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardAllowsAnyOrigin(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"*"}, Logger: newTestLogger()}
	engine := setupCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://anything.example.com")
	engine.ServeHTTP(w, req)

	assert.Equal(t, "http://anything.example.com", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_MultipleAllowedOrigins(t *testing.T) {
	cfg := CORSConfig{
		AllowedOrigins: []string{"http://localhost:3000", "http://localhost:4000"},
		Logger:         newTestLogger(),
	}
	engine := setupCORSRouter(cfg)

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req1.Header.Set("Origin", "http://localhost:4000")
	engine.ServeHTTP(w1, req1)
	assert.Equal(t, "http://localhost:4000", w1.Header().Get("Access-Control-Allow-Origin"))

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Origin", "http://localhost:5000")
	engine.ServeHTTP(w2, req2)
	assert.Empty(t, w2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightOPTIONS_Returns204(t *testing.T) {
	cfg := CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}, Logger: newTestLogger()}
	engine := setupCORSRouter(cfg)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}
