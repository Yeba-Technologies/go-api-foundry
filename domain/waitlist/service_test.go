package waitlist

import (
	"context"
	"testing"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestWaitlistService_CreateEntry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := NewMockWaitlistRepository(ctrl)
	logger := log.NewLoggerWithJSONOutput()
	service := NewWaitlistService(logger, mockRepo)

	t.Run("successful creation", func(t *testing.T) {
		req := &CreateWaitlistEntryRequest{
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		}

		expectedEntry := &models.WaitlistEntry{
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		}

		mockRepo.EXPECT().
			CreateEntry(gomock.Any(), gomock.Any()).
			Return(expectedEntry, nil)

		result, err := service.CreateEntry(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, req.Email, result.Email)
		assert.Equal(t, req.FirstName, result.FirstName)
		assert.Equal(t, req.LastName, result.LastName)
	})

	t.Run("repository error", func(t *testing.T) {
		req := &CreateWaitlistEntryRequest{
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		}

		mockRepo.EXPECT().
			CreateEntry(gomock.Any(), gomock.Any()).
			Return(nil, apperrors.NewDatabaseError("database error", nil))

		result, err := service.CreateEntry(context.Background(), req)

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
