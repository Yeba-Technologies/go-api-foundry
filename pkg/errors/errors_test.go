package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jsonUnmarshalTypeErrorStub struct {
	Field        string
	ExpectedType string
	GotValue     string
}

func makeUnmarshalTypeError(field, expectedType, gotValue string) *json.UnmarshalTypeError {
	var t reflect.Type
	switch expectedType {
	case "int":
		t = reflect.TypeOf(0)
	default:
		t = reflect.TypeOf("")
	}
	return &json.UnmarshalTypeError{
		Value: gotValue,
		Type:  t,
		Field: field,
	}
}

func TestAppError_ErrorWithWrapped(t *testing.T) {
	inner := errors.New("connection refused")
	e := NewAppError(ErrorTypeDatabaseError, "query failed", inner)
	assert.Equal(t, "DATABASE_ERROR: query failed: connection refused", e.Error())
}

func TestAppError_ErrorWithoutWrapped(t *testing.T) {
	e := NewAppError(ErrorTypeNotFound, "user not found", nil)
	assert.Equal(t, "NOT_FOUND: user not found", e.Error())
}

func TestAppError_Unwrap(t *testing.T) {
	inner := errors.New("root cause")
	e := NewAppError(ErrorTypeInternalServerError, "something broke", inner)
	assert.ErrorIs(t, e, inner)
}

func TestAppError_UnwrapNil(t *testing.T) {
	e := NewAppError(ErrorTypeNotFound, "missing", nil)
	assert.Nil(t, e.Unwrap())
}

func TestAppError_ErrorsAs(t *testing.T) {
	inner := errors.New("db timeout")
	e := NewAppError(ErrorTypeDatabaseError, "query failed", inner)
	wrapped := fmt.Errorf("service layer: %w", e)

	var appErr *AppError
	require.True(t, errors.As(wrapped, &appErr))
	assert.Equal(t, ErrorTypeDatabaseError, appErr.Type)
	assert.Equal(t, "query failed", appErr.Message)
}

func TestNewNotFoundError(t *testing.T) {
	e := NewNotFoundError("user not found", nil)
	assert.Equal(t, ErrorTypeNotFound, e.Type)
	assert.Equal(t, "user not found", e.Message)
}

func TestNewInvalidRequestError(t *testing.T) {
	e := NewInvalidRequestError("bad payload", nil)
	assert.Equal(t, ErrorTypeInvalidRequest, e.Type)
}

func TestNewDatabaseError(t *testing.T) {
	e := NewDatabaseError("insert failed", errors.New("pq: unique"))
	assert.Equal(t, ErrorTypeDatabaseError, e.Type)
	assert.NotNil(t, e.Err)
}

func TestNewConflictError(t *testing.T) {
	e := NewConflictError("duplicate email", nil)
	assert.Equal(t, ErrorTypeConflict, e.Type)
}

func TestNewUnauthorizedError(t *testing.T) {
	e := NewUnauthorizedError("token expired", nil)
	assert.Equal(t, ErrorTypeUnauthorized, e.Type)
}

func TestNewForbiddenError(t *testing.T) {
	e := NewForbiddenError("insufficient role", nil)
	assert.Equal(t, ErrorTypeForbidden, e.Type)
}

func TestNewInternalServerError(t *testing.T) {
	e := NewInternalServerError("panic recovered", nil)
	assert.Equal(t, ErrorTypeInternalServerError, e.Type)
}

func TestNewNoContentError(t *testing.T) {
	e := NewNoContentError("empty result", nil)
	assert.Equal(t, ErrorTypeNoContent, e.Type)
}

func TestGetErrorType_NilError(t *testing.T) {
	assert.Equal(t, "", GetErrorType(nil))
}

func TestGetErrorType_AppError(t *testing.T) {
	e := NewNotFoundError("missing", nil)
	assert.Equal(t, ErrorTypeNotFound, GetErrorType(e))
}

func TestGetErrorType_WrappedAppError(t *testing.T) {
	e := fmt.Errorf("outer: %w", NewConflictError("dup", nil))
	assert.Equal(t, ErrorTypeConflict, GetErrorType(e))
}

func TestGetErrorType_PlainError(t *testing.T) {
	assert.Equal(t, ErrorTypeUnknown, GetErrorType(errors.New("raw")))
}

func TestIsDuplicateKeyError_Nil(t *testing.T) {
	assert.False(t, IsDuplicateKeyError(nil))
}

func TestIsDuplicateKeyError_ConflictAppError(t *testing.T) {
	e := NewConflictError("duplicate email", nil)
	assert.True(t, IsDuplicateKeyError(e))
}

func TestIsDuplicateKeyError_WrappedConflictAppError(t *testing.T) {
	e := fmt.Errorf("repo: %w", NewConflictError("dup key", nil))
	assert.True(t, IsDuplicateKeyError(e))
}

func TestIsDuplicateKeyError_NonConflictAppError(t *testing.T) {
	e := NewNotFoundError("missing", nil)
	assert.False(t, IsDuplicateKeyError(e))
}

func TestIsDuplicateKeyError_DBStringFallback(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{"duplicate", "ERROR: duplicate key value violates unique constraint", true},
		{"unique constraint", "unique constraint violation", true},
		{"unique_violation", "pq: unique_violation", true},
		{"duplicate key value", "duplicate key value violates constraint", true},
		{"case insensitive", "DUPLICATE KEY VALUE", true},
		{"unrelated", "connection refused", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsDuplicateKeyError(errors.New(tt.msg)))
		})
	}
}

func TestHTTPStatusCode_NilError(t *testing.T) {
	assert.Equal(t, StatusInternalServerError, HTTPStatusCode(nil))
}

func TestHTTPStatusCode_AllTypes(t *testing.T) {
	tests := []struct {
		errType    string
		wantStatus int
	}{
		{ErrorTypeNotFound, StatusNotFound},
		{ErrorTypeInvalidRequest, StatusBadRequest},
		{ErrorTypeConflict, StatusConflict},
		{ErrorTypeUnauthorized, StatusUnauthorized},
		{ErrorTypeForbidden, StatusForbidden},
		{ErrorTypeTooManyRequests, StatusTooManyRequests},
		{ErrorTypeRateLimitExceeded, StatusTooManyRequests},
		{ErrorTypeRequestTimeout, StatusRequestTimeout},
		{ErrorTypeMethodNotAllowed, StatusMethodNotAllowed},
		{ErrorTypeNoContent, StatusNoContent},
		{ErrorTypeDatabaseError, StatusInternalServerError},
		{ErrorTypeInternalServerError, StatusInternalServerError},
		{ErrorTypeUnknown, StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.errType, func(t *testing.T) {
			e := NewAppError(tt.errType, "test", nil)
			assert.Equal(t, tt.wantStatus, HTTPStatusCode(e))
		})
	}
}

func TestHTTPStatusCode_PlainError(t *testing.T) {
	assert.Equal(t, StatusInternalServerError, HTTPStatusCode(errors.New("boom")))
}

func TestGetHumanReadableMessage_NilError(t *testing.T) {
	assert.Equal(t, "An unexpected error occurred", GetHumanReadableMessage(nil))
}

func TestGetHumanReadableMessage_AppError(t *testing.T) {
	e := NewNotFoundError("User could not be found", nil)
	assert.Equal(t, "User could not be found", GetHumanReadableMessage(e))
}

func TestGetHumanReadableMessage_WrappedAppError(t *testing.T) {
	e := fmt.Errorf("svc: %w", NewDatabaseError("Database unavailable", errors.New("pq: conn refused")))
	assert.Equal(t, "Database unavailable", GetHumanReadableMessage(e))
}

func TestGetHumanReadableMessage_PlainError_HidesInternals(t *testing.T) {
	// SECURITY: raw errors must NOT leak to clients
	e := errors.New("pq: password authentication failed for user postgres")
	assert.Equal(t, "An unexpected error occurred", GetHumanReadableMessage(e))
}

func TestFormatValidationErrors_NilError(t *testing.T) {
	result := FormatValidationErrors(nil, nil)
	assert.Empty(t, result)
}

func TestFormatValidationErrors_UnmarshalTypeError(t *testing.T) {
	err := &jsonUnmarshalTypeErrorStub{Field: "age", ExpectedType: "int", GotValue: "string"}
	// FormatValidationErrors accepts generic error, use real json.UnmarshalTypeError
	result := FormatValidationErrors(makeUnmarshalTypeError("age", "int", "hello"), nil)
	_ = err
	require.Len(t, result, 1)
	assert.Equal(t, "age", result[0].Field)
	assert.Contains(t, result[0].Message, "Invalid type")
}

func TestMsgForTag_KnownTags(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"required", "This field is required"},
		{"email", "Invalid email format"},
		{"min", "Value is too short or too small"},
		{"max", "Value is too long or too large"},
		{"len", "Value must be exact length"},
		{"numeric", "Value must be numeric"},
		{"alpha", "Value must contain only letters"},
		{"alphanum", "Value must contain only letters and numbers"},
		{"url", "Invalid URL format"},
		{"uri", "Invalid URI format"},
		{"eqfield", "Value must match the referenced field"},
		{"nefield", "Value must not match the referenced field"},
		{"gt", "Value must be greater than specified"},
		{"gte", "Value must be greater than or equal to specified"},
		{"lt", "Value must be less than specified"},
		{"lte", "Value must be less than or equal to specified"},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			assert.Equal(t, tt.want, msgForTag(tt.tag))
		})
	}
}

func TestMsgForTag_UnknownTag(t *testing.T) {
	assert.Equal(t, "Invalid value", msgForTag("custom_whatever"))
}

type sampleModel struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	NoTag     string
}

func TestGetJSONFieldName_WithTag(t *testing.T) {
	st := reflect.TypeOf(sampleModel{})
	assert.Equal(t, "first_name", getJSONFieldName(st, "FirstName"))
}

func TestGetJSONFieldName_TagWithOptions(t *testing.T) {
	st := reflect.TypeOf(sampleModel{})
	assert.Equal(t, "last_name", getJSONFieldName(st, "LastName"))
}

func TestGetJSONFieldName_NoTag(t *testing.T) {
	st := reflect.TypeOf(sampleModel{})
	assert.Equal(t, "NoTag", getJSONFieldName(st, "NoTag"))
}

func TestGetJSONFieldName_MissingField(t *testing.T) {
	st := reflect.TypeOf(sampleModel{})
	assert.Equal(t, "Unknown", getJSONFieldName(st, "Unknown"))
}
