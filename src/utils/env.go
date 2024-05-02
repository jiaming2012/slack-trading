package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func InitEnvironmentVariables() error {
	// Currently, we use heroku for production which doesn't support .env files
	if os.Getenv("GO_ENV") == "production" {
		log.Info("Running in production environment")
		return nil
	}

	// Determine which .env file to load
	envFile := "../.env.development" // default to development environment

	if os.Getenv("GO_ENV") == "production" {
		envFile = "../.env.production"
	}

	// Load the specified .env file
	err := godotenv.Load(envFile)
	if err != nil {
		return fmt.Errorf("failed to load %s file: %v", envFile, err)
	}

	return nil
}
