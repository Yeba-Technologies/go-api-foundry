package waitlist

import (
	"context"
	"net/mail"
	"strings"

	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
)

type WaitlistService interface {
	// CreateEntry creates a new waitlist entry based on the provided request.
	CreateEntry(ctx context.Context, req *CreateWaitlistEntryRequest) (*WaitlistEntryResponse, error)

	// FindEntryByID retrieves a waitlist entry by its unique ID.
	FindEntryByID(ctx context.Context, id uint) (*WaitlistEntryResponse, error)

	// UpdateEntry updates an existing waitlist entry identified by ID with the provided data.
	UpdateEntry(ctx context.Context, id uint, req *UpdateWaitlistEntryRequest) error

	// GetAllEntries retrieves all waitlist entries.
	GetAllEntries(ctx context.Context) ([]WaitlistEntryResponse, error)

	// DeleteEntry removes a waitlist entry identified by its ID.
	DeleteEntry(ctx context.Context, id uint) error
}

type waitlistService struct {
	logger     *log.Logger
	repository WaitlistRepository
}

func NewWaitlistService(logger *log.Logger, repository WaitlistRepository) WaitlistService {
	return &waitlistService{logger: logger, repository: repository}
}

func (s *waitlistService) CreateEntry(ctx context.Context, req *CreateWaitlistEntryRequest) (*WaitlistEntryResponse, error) {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if req == nil {
		logger.Error("CreateEntry received empty request")
		return nil, apperrors.NewInvalidRequestError("request cannot be nil", nil)
	}

	entryModel := ToWaitlistEntryModel(req)

	entry, err := s.repository.CreateEntry(ctx, entryModel)
	if err != nil {
		logger.Error("Failed to create waitlist entry", "error", err)
		return nil, err
	}

	response := ToWaitlistEntryResponse(entry)
	return &response, nil
}

func (s *waitlistService) FindEntryByID(ctx context.Context, id uint) (*WaitlistEntryResponse, error) {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if id == 0 {
		logger.Error("FindEntryByID received invalid ID")
		return nil, apperrors.NewInvalidRequestError("invalid entry ID", nil)
	}

	entry, err := s.repository.FindEntryByID(ctx, id)
	if err != nil {
		logger.Error("Failed to find waitlist entry", "id", id, "error", err)
		return nil, err
	}

	response := ToWaitlistEntryResponse(entry)
	return &response, nil
}

func (s *waitlistService) UpdateEntry(ctx context.Context, id uint, req *UpdateWaitlistEntryRequest) error {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if id == 0 {
		logger.Error("UpdateEntry received invalid ID")
		return apperrors.NewInvalidRequestError("invalid entry ID", nil)
	}

	if req == nil {
		logger.Error("UpdateEntry received empty request")
		return apperrors.NewInvalidRequestError("request cannot be nil", nil)
	}

	if strings.TrimSpace(req.Email) == "" && strings.TrimSpace(req.FirstName) == "" && strings.TrimSpace(req.LastName) == "" {
		logger.Error("UpdateEntry received request with no fields to update")
		return apperrors.NewInvalidRequestError("at least one field must be provided for update", nil)
	}

	fieldsToUpdate := make(map[string]interface{})
	if req.Email != "" {
		if _, err := mail.ParseAddress(req.Email); err != nil {
			logger.Error("UpdateEntry received invalid email format", "email", req.Email)
			return apperrors.NewInvalidRequestError("invalid email format", nil)
		}
		fieldsToUpdate["email"] = req.Email
	}
	if req.FirstName != "" {
		fieldsToUpdate["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		fieldsToUpdate["last_name"] = req.LastName
	}

	err := s.repository.UpdateEntry(ctx, id, fieldsToUpdate)
	if err != nil {
		logger.Error("Failed to update waitlist entry", "id", id, "error", err)
		return err
	}

	return nil
}

func (s *waitlistService) GetAllEntries(ctx context.Context) ([]WaitlistEntryResponse, error) {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	entries, err := s.repository.GetAllEntries(ctx)
	if err != nil {
		logger.Error("Failed to get all waitlist entries", "error", err)
		return nil, err
	}

	responses := make([]WaitlistEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, ToWaitlistEntryResponse(entry))
	}

	return responses, nil
}

func (s *waitlistService) DeleteEntry(ctx context.Context, id uint) error {
	logger := log.GetLoggerInstanceFromContext(ctx, s.logger)

	if id == 0 {
		logger.Error("DeleteEntry received invalid ID")
		return apperrors.NewInvalidRequestError("invalid entry ID", nil)
	}

	err := s.repository.DeleteEntry(ctx, id)
	if err != nil {
		logger.Error("Failed to delete waitlist entry", "id", id, "error", err)
		return err
	}

	return nil
}
