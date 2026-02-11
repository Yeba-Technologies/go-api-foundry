package waitlist

import (
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	"github.com/Yeba-Technologies/go-api-foundry/pkg/constants"
)

type CreateWaitlistEntryRequest struct {
	Email     string `json:"email" binding:"required,email,lowercase,trim,max=255"`
	FirstName string `json:"first_name" binding:"required,min=1,max=255"`
	LastName  string `json:"last_name" binding:"required,min=1,max=255"`
}

type UpdateWaitlistEntryRequest struct {
	Email     string `json:"email" binding:"omitempty,email,lowercase,trim,max=255"`
	FirstName string `json:"first_name" binding:"omitempty,min=1,max=255"`
	LastName  string `json:"last_name" binding:"omitempty,min=1,max=255"`
}

type WaitlistEntryResponse struct {
	ID        uint   `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	CreatedAt string `json:"created_at"`
}

// ========================================
// Mappers
// ========================================

func ToWaitlistEntryModel(req *CreateWaitlistEntryRequest) *models.WaitlistEntry {
	if req == nil {
		return nil
	}
	return &models.WaitlistEntry{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}
}

func ToWaitlistEntryResponse(waitList *models.WaitlistEntry) WaitlistEntryResponse {
	if waitList == nil {
		return WaitlistEntryResponse{}
	}
	return WaitlistEntryResponse{
		ID:        waitList.ID,
		Email:     waitList.Email,
		FirstName: waitList.FirstName,
		LastName:  waitList.LastName,
		CreatedAt: waitList.CreatedAt.Format(constants.RFC3339DateTimeFormat),
	}
}
