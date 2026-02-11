package errors

import (
	"errors"
)

func HTTPStatusCode(err error) int {
	if err == nil {
		return StatusInternalServerError
	}

	errorType := GetErrorType(err)

	switch errorType {
	case ErrorTypeNotFound:
		return StatusNotFound
	case ErrorTypeInvalidRequest:
		return StatusBadRequest
	case ErrorTypeConflict:
		return StatusConflict
	case ErrorTypeUnauthorized:
		return StatusUnauthorized
	case ErrorTypeForbidden:
		return StatusForbidden
	case ErrorTypeTooManyRequests, ErrorTypeRateLimitExceeded:
		return StatusTooManyRequests
	case ErrorTypeRequestTimeout:
		return StatusRequestTimeout
	case ErrorTypeMethodNotAllowed:
		return StatusMethodNotAllowed
	case ErrorTypeNoContent:
		return StatusNoContent
	case ErrorTypeDatabaseError:
		return StatusInternalServerError
	case ErrorTypeInternalServerError:
		return StatusInternalServerError
	default:
		return StatusInternalServerError
	}
}

func GetHumanReadableMessage(err error) string {
	if err == nil {
		return "An unexpected error occurred"
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Message
	}

	// SECURITY: avoid leaking internal error strings (DB errors, stack messages, etc.)
	return "An unexpected error occurred"
}
