package router

import (
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
)

func TestTrimValidationTag_IsRegistered(t *testing.T) {
	logger := log.NewLoggerWithJSONOutput()
	_ = CreateRouterService(logger, nil, &RouterConfig{
		RateLimitRequests: 1000,
		RateLimitWindow:   time.Minute,
		RequestTimeout:    5 * time.Second,
	})

	type payload struct {
		Email string `json:"email" binding:"required,email,trim"`
	}

	p := payload{Email: "test@example.com"}

	require.NotPanics(t, func() {
		_ = binding.Validator.ValidateStruct(&p)
	})

	err := binding.Validator.ValidateStruct(&p)
	require.NoError(t, err)

	type trimOnlyPayload struct {
		Value string `json:"value" binding:"trim"`
	}

	p2 := trimOnlyPayload{Value: " test@example.com "}

	err = binding.Validator.ValidateStruct(&p2)
	require.Error(t, err)

	validationErrors, ok := err.(validator.ValidationErrors)
	require.True(t, ok, "expected validator.ValidationErrors, got %T", err)
	require.Len(t, validationErrors, 1)
	require.Equal(t, "Value", validationErrors[0].Field())
	require.Equal(t, "trim", validationErrors[0].Tag())
}
