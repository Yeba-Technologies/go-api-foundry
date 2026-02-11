package waitlist

import (
	"github.com/Yeba-Technologies/go-api-foundry/config/router"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"gorm.io/gorm"
)

type WaitlistServiceFactory interface {
	CreateService() WaitlistService
	CreateController() *router.RESTController
}

type DefaultWaitlistServiceFactory struct {
	db     *gorm.DB
	logger *log.Logger
}

func NewWaitlistServiceFactory(db *gorm.DB, logger *log.Logger) WaitlistServiceFactory {
	return &DefaultWaitlistServiceFactory{
		db:     db,
		logger: logger,
	}
}

func (f *DefaultWaitlistServiceFactory) CreateService() WaitlistService {
	repository := NewWaitlistRepository(f.db)
	return NewWaitlistService(f.logger, repository)
}

func (f *DefaultWaitlistServiceFactory) CreateController() *router.RESTController {
	return NewWaitlistController(f.db, f.logger)
}
