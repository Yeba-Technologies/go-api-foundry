package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBodySizeRouter(maxBytes int64) *gin.Engine {
	engine := gin.New()
	engine.Use(NewMaxBodySizeMiddleware(maxBytes))
	engine.POST("/test", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, body)
	})
	return engine
}

func TestBodySize_UnderLimit_Passes(t *testing.T) {
	engine := setupBodySizeRouter(1024)
	payload := []byte(`{"name":"test"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBodySize_ContentLengthExceedsLimit_Returns413(t *testing.T) {
	maxBytes := int64(16)
	engine := setupBodySizeRouter(maxBytes)
	payload := []byte(`{"name":"this is a very long payload that exceeds the limit"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "Request payload too large", resp["message"])
}

func TestBodySize_StreamExceedsLimit_MaxBytesReaderCutsOff(t *testing.T) {
	maxBytes := int64(16)
	engine := setupBodySizeRouter(maxBytes)
	payload := strings.NewReader(`{"name":"long payload that exceeds the limit"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", payload)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1 // unknown content length — forces MaxBytesReader path
	engine.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusOK, w.Code)
}

func TestBodySize_NoBody_Passes(t *testing.T) {
	engine := setupBodySizeRouter(1024)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestBodySize_ExactLimit_Passes(t *testing.T) {
	payload := []byte(`{"a":"b"}`)
	engine := setupBodySizeRouter(int64(len(payload)))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
