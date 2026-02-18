package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupLoggingRouter() *gin.Engine {
	logger := log.NewLoggerWithJSONOutput()
	engine := gin.New()
	engine.Use(NewCorrelationIDMiddleware())
	engine.Use(NewLoggerInjectionMiddleware(logger))
	engine.Use(NewRequestLoggingMiddleware(logger))
	engine.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

func TestLoggerInjection_LoggerInContext(t *testing.T) {
	logger := log.NewLoggerWithJSONOutput()
	engine := gin.New()
	engine.Use(NewCorrelationIDMiddleware())
	engine.Use(NewLoggerInjectionMiddleware(logger))

	var ctxLogger *log.Logger
	engine.GET("/test", func(c *gin.Context) {
		ctxLogger = log.GetLoggerInstanceFromContext(c.Request.Context(), nil)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, ctxLogger)
}

func TestRequestLogging_CompletesWithoutPanic(t *testing.T) {
	engine := setupLoggingRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	assert.NotPanics(t, func() {
		engine.ServeHTTP(w, req)
	})
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestLogging_MultipleRequests(t *testing.T) {
	engine := setupLoggingRouter()

	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		engine.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}
