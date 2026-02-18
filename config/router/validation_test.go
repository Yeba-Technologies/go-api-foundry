package router

import (
	"testing"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/gin-gonic/gin/binding"
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
}
