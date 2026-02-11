package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Yeba-Technologies/go-api-foundry/config"
	"github.com/Yeba-Technologies/go-api-foundry/domain"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
)

func main() {
	logger := log.NewLoggerWithJSONOutput()

	logger.Info("V2 Backend Server initialized ✅")

	autoMigrate := false

	for _, arg := range os.Args[1:] {
		if strings.ToLower(arg) == "--auto-migrate" || strings.ToLower(arg) == "-m" {
			autoMigrate = true
			break
		}
	}

	appConfig, err := config.LoadApplicationConfiguration(logger, autoMigrate)
	if err != nil {
		logger.Error("Failed to load application configuration", "error", err.Error())
		os.Exit(1)
	}

	domain.SetupCoreDomain(appConfig)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTP server...")
		if err := appConfig.RouterService.RunHTTPServer(); err != nil {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		logger.Error("Server error", "error", err)
		appConfig.Cleanup()
		os.Exit(1)
	case <-quit:
		logger.Info("Shutdown signal received, shutting down gracefully...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := appConfig.RouterService.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown error", "error", err)
		} else {
			logger.Info("HTTP server shut down gracefully")
		}
		appConfig.Cleanup()

		logger.Info("Graceful shutdown completed")
	}
}
