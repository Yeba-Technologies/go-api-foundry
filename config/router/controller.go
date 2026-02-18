package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
	"github.com/gin-gonic/gin"
)

func normalizePath(controller *RESTController, relativePath string) string {
	path := controller.mountPoint
	if relativePath != "" {
		path = path + "/" + relativePath
	}
	if path[0] != '/' {
		path = "/" + path
	}
	if len(path) > 1 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}
	return strings.ReplaceAll(path, "//", "/")
}

func (rs *RouterService) routeKey(path, method string) string {
	return method + "-" + path
}

func (ctrl *RESTController) bindToRouter(rs *RouterService, path, method string) {
	key := rs.routeKey(path, method)
	if existing, found := rs.handlerToControllerMap[key]; found {
		panic(fmt.Sprintf("handler already registered for '%s' by controller '%s'", path, existing.name))
	}
	rs.handlerToControllerMap[key] = ctrl
}

func (rs *RouterService) registerRateLimitOverride(path string, limiter ratelimit.RateLimiter) {
	if limiter == nil {
		return
	}
	if _, exists := rs.rateLimitOverrides[path]; exists {
		panic(fmt.Sprintf("rate limiter already registered for '%s'", path))
	}
	rs.rateLimitOverrides[path] = limiter
}

func wrapHandler(handler HandlerFunction) MiddlewareFunc {
	return func(c *RequestContext) {
		result := handler(c)
		if result == nil {
			c.JSON(http.StatusInternalServerError, InternalServerErrorResult("handler returned nil result").ToJSON())
			return
		}
		c.JSON(result.StatusCode, result.ToJSON())
	}
}

func (rs *RouterService) addHandler(
	method string,
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	route := normalizePath(controller, path)
	controller.bindToRouter(rs, route, method)
	rs.registerRateLimitOverride(rs.routeKey(route, method), limiter)

	handlers := append(middlewares, wrapHandler(handler))

	var register func(string, ...gin.HandlerFunc) gin.IRoutes
	switch method {
	case http.MethodGet:
		register = rs.engine.GET
	case http.MethodPost:
		register = rs.engine.POST
	case http.MethodPut:
		register = rs.engine.PUT
	case http.MethodDelete:
		register = rs.engine.DELETE
	case http.MethodPatch:
		register = rs.engine.PATCH
	case http.MethodHead:
		register = rs.engine.HEAD
	default:
		panic(fmt.Sprintf("unsupported HTTP method: %s", method))
	}

	register(route, handlers...)
	rs.logger.Debug("Handler registered", "method", method, "path", route)
}

func NewRESTController(name, mountPoint string, prepare func(*RouterService, *RESTController)) *RESTController {
	return &RESTController{
		name:       name,
		mountPoint: strings.ReplaceAll("/"+mountPoint, "//", "/"),
		prepare:    prepare,
	}
}

func NewVersionedRESTController(name, version, mountPoint string, prepare func(*RouterService, *RESTController)) *RESTController {
	return &RESTController{
		name:       name,
		mountPoint: strings.ReplaceAll("/"+version+"/"+mountPoint, "//", "/"),
		version:    version,
		prepare:    prepare,
	}
}

func (ctrl *RESTController) RateLimitWith(rs *RouterService, limiter ratelimit.RateLimiter) *RESTController {
	rs.registerRateLimitOverride(ctrl.mountPoint, limiter)
	return ctrl
}

func (rs *RouterService) AddPostHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodPost, ctrl, limiter, path, handler, mw...)
}

func (rs *RouterService) AddGetHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodGet, ctrl, limiter, path, handler, mw...)
}

func (rs *RouterService) AddPutHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodPut, ctrl, limiter, path, handler, mw...)
}

func (rs *RouterService) AddDeleteHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodDelete, ctrl, limiter, path, handler, mw...)
}

func (rs *RouterService) AddPatchHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodPatch, ctrl, limiter, path, handler, mw...)
}

func (rs *RouterService) AddHeadHandler(ctrl *RESTController, limiter ratelimit.RateLimiter, path string, handler HandlerFunction, mw ...MiddlewareFunc) {
	rs.addHandler(http.MethodHead, ctrl, limiter, path, handler, mw...)
}
