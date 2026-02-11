package router

import (
	"net/http"
	"strconv"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
)

func GetLogger(ctx *RequestContext) *log.Logger {
	if logger := ctx.Request.Context().Value(log.LoggerKeyForContext); logger != nil {
		if l, ok := logger.(*log.Logger); ok {
			return l
		}
	}

	baseLogger := log.NewLoggerWithJSONOutput()
	return baseLogger.WithCorrelationID(ctx.Request.Context())
}

func OKResult(data any, message string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusOK,
		Data:       data,
		Message:    message,
	}
}

func CreatedResult(data any, resourceName string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusCreated,
		Data:       data,
		Message:    resourceName + " created successfully",
	}
}

func TooManyRequestsResult(data RateLimitResponse) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusTooManyRequests,
		Data:       data,
		Message:    "Too Many Requests",
	}
}

func BadRequestResult(message string, payload any) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusBadRequest,
		Data:       payload,
		Message:    message,
	}
}

func UnauthorizedResult(message string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusUnauthorized,
		Data:       nil,
		Message:    message,
	}
}

func NotFoundResult(message string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusNotFound,
		Data:       nil,
		Message:    message,
	}
}

func InternalServerErrorResult(message string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusInternalServerError,
		Data:       nil,
		Message:    message,
	}
}

func ConflictResult(message string) *ServiceResult {
	return &ServiceResult{
		StatusCode: http.StatusConflict,
		Data:       nil,
		Message:    message,
	}
}

func ErrorResult(statusCode int, message string, data any) *ServiceResult {
	return &ServiceResult{
		StatusCode: statusCode,
		Data:       data,
		Message:    message,
	}
}

func ParseIDParam(ctx *RequestContext, paramName string) (uint, *ServiceResult) {
	logger := GetLogger(ctx)

	idParam := ctx.Param(paramName)
	id, err := strconv.ParseUint(idParam, 10, 32)

	if err != nil {
		logger.Error("Invalid ID parameter", "param", paramName, "value", idParam, "error", err)
		return 0, BadRequestResult("Invalid ID parameter", nil)
	}

	return uint(id), nil
}
