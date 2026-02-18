package main

import (
	"context"
	"fmt"
	"net/http"
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
	autoMigrate := false

	for _, arg := range os.Args[1:] {
		switch strings.ToLower(arg) {
		case "--health", "-health":
			os.Exit(runHealthCheck())
		case "--auto-migrate", "-m":
			autoMigrate = true
		}
	}

	logger := log.NewLoggerWithJSONOutput()

	logger.Info("V2 Backend Server initialized ✅")

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

func runHealthCheck() int {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "health check failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return 0
	}
	fmt.Fprintf(os.Stderr, "health check returned status %d\n", resp.StatusCode)
	return 1
}
