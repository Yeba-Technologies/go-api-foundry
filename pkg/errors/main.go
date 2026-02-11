package errors

import (
	"errors"
	"fmt"
	"strings"
)

const (
	StatusOK                  = 200
	StatusCreated             = 201
	StatusNoContent           = 204
	StatusBadRequest          = 400
	StatusUnauthorized        = 401
	StatusForbidden           = 403
	StatusNotFound            = 404
	StatusRequestTimeout      = 408
	StatusMethodNotAllowed    = 405
	StatusConflict            = 409
	StatusTooManyRequests     = 429
	StatusInternalServerError = 500
)

const (
	ErrorTypeDatabaseError       = "DATABASE_ERROR"
	ErrorTypeNotFound            = "NOT_FOUND"
	ErrorTypeInvalidRequest      = "INVALID_REQUEST"
	ErrorTypeUnauthorized        = "UNAUTHORIZED"
	ErrorTypeForbidden           = "FORBIDDEN"
	ErrorTypeConflict            = "CONFLICT"
	ErrorTypeInternalServerError = "INTERNAL_SERVER_ERROR"
	ErrorTypeUnknown             = "UNKNOWN_ERROR"
	ErrorTypeNoContent           = "NO_CONTENT"
	ErrorTypeTooManyRequests     = "TOO_MANY_REQUESTS"
	ErrorTypeRateLimitExceeded   = "RATE_LIMIT_EXCEEDED"
	ErrorTypeRequestTimeout      = "REQUEST_TIMEOUT"
	ErrorTypeMethodNotAllowed    = "METHOD_NOT_ALLOWED"
)

type AppError struct {
	Type    string
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(errType, message string, err error) *AppError {
	return &AppError{
		Type:    errType,
		Message: message,
		Err:     err,
	}
}

func NewNotFoundError(message string, err error) *AppError {
	return NewAppError(ErrorTypeNotFound, message, err)
}

func NewInvalidRequestError(message string, err error) *AppError {
	return NewAppError(ErrorTypeInvalidRequest, message, err)
}

func NewDatabaseError(message string, err error) *AppError {
	return NewAppError(ErrorTypeDatabaseError, message, err)
}

func NewConflictError(message string, err error) *AppError {
	return NewAppError(ErrorTypeConflict, message, err)
}

func NewUnauthorizedError(message string, err error) *AppError {
	return NewAppError(ErrorTypeUnauthorized, message, err)
}

func NewForbiddenError(message string, err error) *AppError {
	return NewAppError(ErrorTypeForbidden, message, err)
}

func NewInternalServerError(message string, err error) *AppError {
	return NewAppError(ErrorTypeInternalServerError, message, err)
}

func NewNoContentError(message string, err error) *AppError {
	return NewAppError(ErrorTypeNoContent, message, err)
}

func GetErrorType(err error) string {
	if err == nil {
		return ""
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type
	}

	return ErrorTypeUnknown
}

func DeduceErrorTypeFromErrorString(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()
	switch {
	case errMsg == "":
		return ""
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("not found")):
		return ErrorTypeNotFound
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("unauthorized")):
		return ErrorTypeUnauthorized
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("forbidden")):
		return ErrorTypeForbidden
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("conflict")):
		return ErrorTypeConflict
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("database")):
		return ErrorTypeDatabaseError
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("invalid request")):
		return ErrorTypeInvalidRequest
	case strings.Contains(strings.ToLower(errMsg), strings.ToLower("no content")):
		return ErrorTypeNoContent
	}

	return ErrorTypeUnknown
}

func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return DeduceErrorTypeFromErrorString(err) == ErrorTypeConflict ||
		strings.Contains(strings.ToLower(errMsg), strings.ToLower("duplicate")) ||
		strings.Contains(strings.ToLower(errMsg), strings.ToLower("unique constraint")) ||
		strings.Contains(strings.ToLower(errMsg), strings.ToLower("UNIQUE constraint failed"))
}
