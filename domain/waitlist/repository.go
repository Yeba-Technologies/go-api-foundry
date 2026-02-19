package waitlist

import (
	"context"
	"errors"

	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
	apperrors "github.com/Yeba-Technologies/go-api-foundry/pkg/errors"
	"gorm.io/gorm"
)

type WaitlistRepository interface {
	CreateEntry(ctx context.Context, entry *models.WaitlistEntry) (*models.WaitlistEntry, error)
	FindEntryByID(ctx context.Context, id uint) (*models.WaitlistEntry, error)
	UpdateEntry(ctx context.Context, id uint, updates map[string]interface{}) error
	GetAllEntries(ctx context.Context) ([]*models.WaitlistEntry, error)
	DeleteEntry(ctx context.Context, id uint) error
}

type waitlistRepository struct {
	db *gorm.DB
}

func NewWaitlistRepository(db *gorm.DB) WaitlistRepository {
	return &waitlistRepository{db: db}
}

func (wr *waitlistRepository) CreateEntry(ctx context.Context, entry *models.WaitlistEntry) (*models.WaitlistEntry, error) {
	if err := wr.db.WithContext(ctx).Create(entry).Error; err != nil {
		if isDuplicateKey(err) {
			return nil, apperrors.NewConflictError("waitlist entry with this email already exists", err)
		}
		return nil, apperrors.NewDatabaseError("unable to create waitlist entry", err)
	}

	return entry, nil
}

func (wr *waitlistRepository) FindEntryByID(ctx context.Context, id uint) (*models.WaitlistEntry, error) {
	var entry models.WaitlistEntry

	if err := wr.db.WithContext(ctx).First(&entry, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewNotFoundError("waitlist entry not found", err)
		}
		return nil, apperrors.NewDatabaseError("failed to fetch waitlist entry", err)
	}

	return &entry, nil
}

func (wr *waitlistRepository) UpdateEntry(ctx context.Context, id uint, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return apperrors.NewInvalidRequestError("no fields to update", nil)
	}

	result := wr.db.WithContext(ctx).
		Model(&models.WaitlistEntry{}).
		Where("id = ?", id).
		Updates(updates)

	if result.Error != nil {
		if isDuplicateKey(result.Error) {
			return apperrors.NewConflictError("waitlist entry with this email already exists", result.Error)
		}
		return apperrors.NewDatabaseError("unable to update waitlist entry", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperrors.NewNotFoundError("waitlist entry not found", nil)
	}

	return nil
}

func (wr *waitlistRepository) GetAllEntries(ctx context.Context) ([]*models.WaitlistEntry, error) {
	var entries []*models.WaitlistEntry

	if err := wr.db.WithContext(ctx).Find(&entries).Error; err != nil {
		return nil, apperrors.NewDatabaseError("unable to fetch waitlist entries", err)
	}

	return entries, nil
}

func (wr *waitlistRepository) DeleteEntry(ctx context.Context, id uint) error {
	result := wr.db.WithContext(ctx).Delete(&models.WaitlistEntry{}, id)

	if result.Error != nil {
		return apperrors.NewDatabaseError("unable to delete waitlist entry", result.Error)
	}

	if result.RowsAffected == 0 {
		return apperrors.NewNotFoundError("waitlist entry not found", nil)
	}

	return nil
}

func isDuplicateKey(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || apperrors.IsDuplicateKeyError(err)
}
