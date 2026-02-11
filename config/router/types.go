package router

import (
	"github.com/gin-gonic/gin"
)

type RequestContext = gin.Context

type MiddlewareFunc = gin.HandlerFunc

type ServiceResult struct {
	StatusCode int    `json:"code"`
	Data       any    `json:"data"`
	Message    string `json:"message"`
}

type RateLimitResponse struct {
	Limit      int    `json:"limit"`
	Window     string `json:"window"`
	RetryAfter string `json:"retry_after"`
}

type HandlerFunction func(*RequestContext) *ServiceResult

type RESTController struct {
	name         string
	mountPoint   string
	version      string
	handlerCount int
	prepare      func(*RouterService, *RESTController)
}

func (result *ServiceResult) ToJSON() gin.H {
	return gin.H{
		"code":    result.StatusCode,
		"data":    result.Data,
		"message": result.Message,
	}
}

func (result *ServiceResult) IsSuccess() bool {
	return result.StatusCode >= 200 && result.StatusCode < 300
}

func (result *ServiceResult) IsError() bool {
	return result.StatusCode >= 400
}
