package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

const DEV_ENV_FILENAME = ".env.development"
const PROD_ENV_FILENAME = ".env.production"

// CustomFormatter is a custom log formatter for Logrus
type LogFormatter struct{}

// Format formats the log entry to include a custom timestamp format
func (f *LogFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestamp := entry.Time.Format("01-02-2006 15:04:05")
	log := fmt.Sprintf("[%s] %s %s\n", timestamp, entry.Level, entry.Message)
	return []byte(log), nil
}

func GetEnv(key string) (string, error) {
	envVar := os.Getenv(key)
	if len(envVar) == 0 {
		return "", fmt.Errorf("environment variable %s not set", key)
	}

	return strings.Trim(envVar, `"`), nil
}

func InitEnvironmentVariables(projectsDir string, goEnvironment string) error {
	// In production, environment variables are set in the environment
	if os.Getenv("ENV") == "production" {
		log.Info("Running in production environment")

		level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
		if err != nil {
			return fmt.Errorf("failed to parse log level: %v", err)
		}

		log.SetLevel(level)
		log.SetFormatter(&log.JSONFormatter{})

		return nil
	}

	envDir := filepath.Join(projectsDir, "slack-trading")

	log.Infof("Using go environment: %s", goEnvironment)

	// Determine which .env file to load
	envFile := filepath.Join(envDir, DEV_ENV_FILENAME) // default to development environment
	if goEnvironment == "production" {
		envFile = filepath.Join(envDir, PROD_ENV_FILENAME)
	}

	// Load the specified .env file
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("failed to load %s file: %v", envFile, err)
	}

	// Set logger
	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	if goEnvironment == "production" {
		log.SetFormatter(&log.JSONFormatter{})
	} else {
		log.SetFormatter(&LogFormatter{})
	}

	return nil
}
