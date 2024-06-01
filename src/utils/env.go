package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

const DEV_ENV_FILENAME = ".env.development"
const PROD_ENV_FILENAME = ".env.production"

func InitEnvironmentVariables(projectsDir string, goEnvironment string) error {
	// Currently, we use heroku for production which doesn't support .env files
	if os.Getenv("ENV") == "production" {
		log.Info("Running in production environment")
		return nil
	}

	envDir := filepath.Join(projectsDir, "slack-trading", "src")

	log.Infof("Using go environment: %s", goEnvironment)

	// Determine which .env file to load
	envFile := filepath.Join(envDir, DEV_ENV_FILENAME) // default to development environment
	if goEnvironment == "production" {
		envFile = filepath.Join(envDir, PROD_ENV_FILENAME)
	}

	// Load the specified .env file
	err := godotenv.Load(envFile)
	if err != nil {
		return fmt.Errorf("failed to load %s file: %v", envFile, err)
	}

	return nil
}
