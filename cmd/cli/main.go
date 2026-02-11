package main

import (
	"fmt"
	"os"

	"github.com/Yeba-Technologies/go-api-foundry/config"
	"github.com/Yeba-Technologies/go-api-foundry/internal/log"
	"github.com/Yeba-Technologies/go-api-foundry/internal/models"
)

func main() {
	logger := log.NewLoggerWithJSONOutput()

	config.InitializeEnvFile(logger) // Load envs early for CLI consistency

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "migrate":
		dbCfg := &config.DBConfig{}
		db, err := config.NewDatabase(logger, dbCfg)
		if err != nil {
			logger.Error("Failed to connect to database for migration", "error", err.Error())

			os.Exit(1)
		}
		defer func() {
			sqlDB, err := db.DB()
			if err == nil {
				sqlDB.Close()
			}
		}()

		if err := config.Migrate(logger, db, models.ModelRegistry...); err != nil {
			logger.Error("Database migration failed", "error", err.Error())

			os.Exit(1)
		}

		logger.Info("Database migrations completed")
		return

	case "generate-domain", "gendomain", "gen-domain":
		GenerateDomain()
		return

	case "help", "-h", "--help":
		printUsage()
		return

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: cli <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  migrate          Run database migrations and exit")
	fmt.Println("  generate-domain  Interactively scaffolds a new domain/module (repository, service, controller, routes)")
}
