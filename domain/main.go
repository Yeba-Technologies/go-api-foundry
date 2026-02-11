package domain

import (
	"github.com/Yeba-Technologies/go-api-foundry/config"
	"github.com/Yeba-Technologies/go-api-foundry/domain/monitoring"
	"github.com/Yeba-Technologies/go-api-foundry/domain/waitlist"
)

func SetupCoreDomain(appConfig *config.ApplicationConfig) {
	// Use factory to create controllers
	monitoringFactory := monitoring.NewMonitoringControllerFactory(appConfig.DB, appConfig.Logger, appConfig.Cache)
	appConfig.RouterService.MountController(monitoringFactory.CreateController())
	waitlistFactory := waitlist.NewWaitlistServiceFactory(appConfig.DB, appConfig.Logger)
	appConfig.RouterService.MountController(waitlistFactory.CreateController())
}
