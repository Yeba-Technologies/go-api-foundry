package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
)

func mountTestController(rs *RouterService) {
	ctrl := NewRESTController("TestController", "/", func(rs *RouterService, c *RESTController) {
		rs.AddGetHandler(c, nil, "ip", func(ctx *RequestContext) *ServiceResult {
			return OKResult(ctx.ClientIP(), "ok")
		})

		rs.AddPostHandler(c, nil, "echo", func(ctx *RequestContext) *ServiceResult {
			var payload map[string]any
			if err := ctx.ShouldBindJSON(&payload); err != nil {
				return BadRequestResult("bad", nil)
			}
			return OKResult(payload, "ok")
		})
	})

	rs.MountController(ctrl)
}

func newTestRouterService(t *testing.T) *RouterService {
	t.Helper()

	logger := log.NewLoggerWithJSONOutput()
	return CreateRouterService(logger, nil, &RouterConfig{
		RateLimitRequests: 1000,
		RateLimitWindow:   time.Minute,
		RequestTimeout:    5 * time.Second,
	})
}

func TestTrustedProxies_DisabledByDefault(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")

	rs := newTestRouterService(t)
	mountTestController(rs)

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	w := httptest.NewRecorder()
	rs.GetEngine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int    `json:"code"`
		Data    string `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Data != "10.0.0.2" {
		t.Fatalf("expected ClientIP to use RemoteAddr when trusted proxies disabled; got %q", resp.Data)
	}
}

func TestTrustedProxies_StarTrustsForwardedFor(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "*")

	rs := newTestRouterService(t)
	mountTestController(rs)

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	w := httptest.NewRecorder()
	rs.GetEngine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Code    int    `json:"code"`
		Data    string `json:"data"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Data != "1.1.1.1" {
		t.Fatalf("expected ClientIP to use X-Forwarded-For when trusted proxies enabled; got %q", resp.Data)
	}
}

func TestMaxBodySize_Returns413(t *testing.T) {
	t.Setenv("MAX_REQUEST_BODY_BYTES", "10")

	rs := newTestRouterService(t)
	mountTestController(rs)

	body := bytes.Repeat([]byte{'a'}, 50)
	req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	rs.GetEngine().ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}
