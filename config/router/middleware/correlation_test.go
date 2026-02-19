package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func setupCorrelationRouter() *gin.Engine {
	engine := gin.New()
	engine.Use(NewCorrelationIDMiddleware())
	engine.GET("/test", func(c *gin.Context) {
		id := c.Request.Context().Value(log.CorrelatedIDKey)
		c.String(http.StatusOK, id.(string))
	})
	return engine
}

func TestCorrelation_GeneratesIDWhenMissing(t *testing.T) {
	engine := setupCorrelationRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	responseID := w.Header().Get("X-Correlation-ID")
	assert.NotEmpty(t, responseID)
	assert.Len(t, responseID, 36) // UUID format

	assert.Equal(t, responseID, w.Body.String())
}

func TestCorrelation_PreservesIncomingID(t *testing.T) {
	engine := setupCorrelationRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Correlation-ID", "my-trace-123")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "my-trace-123", w.Header().Get("X-Correlation-ID"))
	assert.Equal(t, "my-trace-123", w.Body.String())
}

func TestCorrelation_EachRequestGetsUniqueID(t *testing.T) {
	engine := setupCorrelationRouter()

	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(w2, req2)

	id1 := w1.Header().Get("X-Correlation-ID")
	id2 := w2.Header().Get("X-Correlation-ID")
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
}
