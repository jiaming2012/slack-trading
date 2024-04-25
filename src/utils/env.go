package utils

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

func InitEnvironmentVariables() error {
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
