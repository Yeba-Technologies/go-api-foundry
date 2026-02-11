package router

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/pkg/ratelimit"
)

func normalizePath(controller *RESTController, relativePath string) string {
	var path string = controller.mountPoint

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

func (routerService *RouterService) keyForPathAndMethod(path, method string) string {
	return fmt.Sprintf("%s-%s", method, path)
}

func (controller *RESTController) bindHandlerToController(routerService *RouterService, path, method string) {
	key := routerService.keyForPathAndMethod(path, method)
	otherController, foundPrevious := routerService.handlerToControllerMap[key]

	if foundPrevious {
		panic(fmt.Sprintf("A handler is already registered for path '%s' by a different controller '%s'", path, otherController.name))
	}

	routerService.handlerToControllerMap[key] = controller
}

func (routerService *RouterService) bindOverrideRateLimiter(path string, limiter ratelimit.RateLimiter) {
	if limiter == nil {
		return
	}

	_, foundPrevious := routerService.rateLimitOverrides[path]
	if foundPrevious {
		panic(fmt.Sprintf("A rate limiter is already registered for path '%s'", path))
	}

	routerService.rateLimitOverrides[path] = limiter
}

func (routerService *RouterService) bindHandlerRateLimiter(path, method string, limiter ratelimit.RateLimiter) {
	key := routerService.keyForPathAndMethod(path, method)
	routerService.bindOverrideRateLimiter(key, limiter)
}

func createHandler(handler HandlerFunction) MiddlewareFunc {
	return func(c *RequestContext) {
		result := handler(c)

		if result == nil {
			c.JSON(http.StatusInternalServerError, InternalServerErrorResult("A handler returned an undefined result. This typically indicates a bug in a handler's implementation.").ToJSON())
			return
		}

		c.JSON(result.StatusCode, result.ToJSON())
	}
}

func NewRESTController(name, mountPoint string, prepare func(*RouterService, *RESTController)) *RESTController {
	mountPoint = strings.ReplaceAll("/"+mountPoint, "//", "/")

	return &RESTController{
		name:       name,
		mountPoint: mountPoint,
		version:    "",
		prepare:    prepare,
	}
}

func NewVersionedRESTController(name, version, mountPoint string, prepare func(*RouterService, *RESTController)) *RESTController {
	// Prefixing the version to the mount point at controller creation clarifies routing and leaves no room for ambiguity.
	finalPath := strings.ReplaceAll("/"+version+"/"+mountPoint, "//", "/")

	return &RESTController{
		name:       name,
		mountPoint: finalPath,
		version:    version,
		prepare:    prepare,
	}
}

func (controller *RESTController) RateLimitWith(routerService *RouterService, limiter ratelimit.RateLimiter) *RESTController {
	routerService.bindOverrideRateLimiter(controller.mountPoint, limiter)
	return controller
}

func (routerService *RouterService) AddPostHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "POST")
	routerService.bindHandlerRateLimiter(mountPoint, "POST", limiter)
	routerService.engine.POST(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "POST", "path", mountPoint)
}

func (routerService *RouterService) AddGetHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "GET")
	routerService.bindHandlerRateLimiter(mountPoint, "GET", limiter)
	routerService.engine.GET(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "GET", "path", mountPoint)
}

func (routerService *RouterService) AddPutHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "PUT")
	routerService.bindHandlerRateLimiter(mountPoint, "PUT", limiter)
	routerService.engine.PUT(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "PUT", "path", mountPoint)
}

func (routerService *RouterService) AddDeleteHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "DELETE")
	routerService.bindHandlerRateLimiter(mountPoint, "DELETE", limiter)
	routerService.engine.DELETE(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "DELETE", "path", mountPoint)
}

func (routerService *RouterService) AddPatchHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "PATCH")
	routerService.bindHandlerRateLimiter(mountPoint, "PATCH", limiter)
	routerService.engine.PATCH(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "PATCH", "path", mountPoint)
}

func (routerService *RouterService) AddHeadHandler(
	controller *RESTController,
	limiter ratelimit.RateLimiter,
	path string,
	handler HandlerFunction,
	middlewares ...MiddlewareFunc,
) {
	controller.handlerCount++
	mountPoint := normalizePath(controller, path)
	controller.bindHandlerToController(routerService, mountPoint, "HEAD")
	routerService.bindHandlerRateLimiter(mountPoint, "HEAD", limiter)
	routerService.engine.HEAD(mountPoint, append(middlewares, createHandler(handler))...)
	routerService.logger.Debug("Handler registered", "method", "HEAD", "path", mountPoint)
}
