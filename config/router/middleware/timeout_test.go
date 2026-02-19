package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupTimeoutRouter(duration time.Duration) *gin.Engine {
	logger := log.NewLoggerWithJSONOutput()
	engine := gin.New()
	engine.Use(NewCorrelationIDMiddleware())
	engine.Use(NewTimeoutMiddleware(TimeoutConfig{Duration: duration, Logger: logger}))
	engine.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	engine.GET("/slow", func(c *gin.Context) {
		select {
		case <-c.Request.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
			c.String(http.StatusOK, "done")
		}
	})
	return engine
}

func TestTimeout_FastRequest_Passes(t *testing.T) {
	engine := setupTimeoutRouter(5 * time.Second)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
}

func TestTimeout_ContextDeadlineSet(t *testing.T) {
	duration := 2 * time.Second
	logger := log.NewLoggerWithJSONOutput()
	engine := gin.New()
	engine.Use(NewTimeoutMiddleware(TimeoutConfig{Duration: duration, Logger: logger}))
	engine.GET("/test", func(c *gin.Context) {
		deadline, ok := c.Request.Context().Deadline()
		assert.True(t, ok)
		assert.WithinDuration(t, time.Now().Add(duration), deadline, 200*time.Millisecond)
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTimeout_SlowRequest_ContextCancelled(t *testing.T) {
	engine := setupTimeoutRouter(50 * time.Millisecond)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	engine.ServeHTTP(w, req)

	// The handler exits via context cancellation; timeout middleware
	// detects DeadlineExceeded and writes 408 if nothing was written yet.
	assert.Equal(t, http.StatusRequestTimeout, w.Code)
}
