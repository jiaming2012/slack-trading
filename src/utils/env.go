package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

func InitEnvironmentVariablesDefault() error {
	pathToDevEnvFile := "../.env.development"
	pathToProdEnvFile := "../.env.production"

	return InitEnvironmentVariables(pathToDevEnvFile, pathToProdEnvFile)
}

func InitEnvironmentVariables(pathToDevEnvFile string, pathToProdEnvFile string) error {
	// Currently, we use heroku for production which doesn't support .env files
	if os.Getenv("ENV") == "production" {
		log.Info("Running in production environment")
		return nil
	}

	// Determine which .env file to load
	envFile := pathToDevEnvFile // default to development environment

	if os.Getenv("GO_ENV") == "production" {
		envFile = pathToProdEnvFile
	}

	// Load the specified .env file
	err := godotenv.Load(envFile)
	if err != nil {
		return fmt.Errorf("failed to load %s file: %v", envFile, err)
	}

	return nil
}
