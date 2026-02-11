package errors

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"github.com/go-playground/validator/v10"
)

type ValidationErrorResponse struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func msgForTag(tag string) string {
	switch tag {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Value is too short or too small"
	case "max":
		return "Value is too long or too large"
	case "len":
		return "Value must be exact length"
	case "numeric":
		return "Value must be numeric"
	case "alpha":
		return "Value must contain only letters"
	case "alphanum":
		return "Value must contain only letters and numbers"
	case "url":
		return "Invalid URL format"
	case "uri":
		return "Invalid URI format"
	case "eqfield":
		return "Value must match the referenced field"
	case "nefield":
		return "Value must not match the referenced field"
	case "gt":
		return "Value must be greater than specified"
	case "gte":
		return "Value must be greater than or equal to specified"
	case "lt":
		return "Value must be less than specified"
	case "lte":
		return "Value must be less than or equal to specified"
	default:
		return "Invalid value"
	}
}

func getJSONFieldName(structType reflect.Type, fieldName string) string {
	field, found := structType.FieldByName(fieldName)
	if !found {
		return fieldName
	}

	jsonTag := field.Tag.Get("json")
	if jsonTag == "" {
		return fieldName
	}

	parts := strings.Split(jsonTag, ",")
	return parts[0]
}

func FormatValidationErrors(err error, model interface{}) []ValidationErrorResponse {
	var errorsList []ValidationErrorResponse

	if err == nil {
		return errorsList
	}

	if jsonErr, ok := err.(*json.UnmarshalTypeError); ok {
		return []ValidationErrorResponse{
			{
				Field:   jsonErr.Field,
				Message: fmt.Sprintf("Invalid type for field %s. Expected %s, got %s", jsonErr.Field, jsonErr.Type, jsonErr.Value),
			},
		}
	}

	if validationErrors, ok := err.(validator.ValidationErrors); ok {

		var structType reflect.Type
		if model != nil {
			structType = reflect.TypeOf(model)
			if structType.Kind() == reflect.Ptr {
				structType = structType.Elem()
			}
		}

		errorsList = make([]ValidationErrorResponse, len(validationErrors))

		for i, fieldError := range validationErrors {
			jsonField := fieldError.Field()
			if model != nil {
				jsonField = getJSONFieldName(structType, fieldError.Field())
			}

			message := msgForTag(fieldError.Tag())

			if fieldError.Param() != "" {
				switch fieldError.Tag() {
				case "min":
					message = fmt.Sprintf("Must be at least %s characters", fieldError.Param())
				case "max":
					message = fmt.Sprintf("Must not exceed %s characters", fieldError.Param())
				case "len":
					message = fmt.Sprintf("Must be exactly %s characters", fieldError.Param())
				case "gt":
					message = fmt.Sprintf("Must be greater than %s", fieldError.Param())
				case "gte":
					message = fmt.Sprintf("Must be greater than or equal to %s", fieldError.Param())
				case "lt":
					message = fmt.Sprintf("Must be less than %s", fieldError.Param())
				case "lte":
					message = fmt.Sprintf("Must be less than or equal to %s", fieldError.Param())
				}
			}

			errorsList[i] = ValidationErrorResponse{
				Field:   jsonField,
				Message: message,
			}
		}
	}

	return errorsList
}
