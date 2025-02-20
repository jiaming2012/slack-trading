package dbutils

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

func InitPostgresWithUrl(url string) (*gorm.DB, error) {
	log.Infof("connecting to postgres @ %s", url)

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&models.LiveAccount{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.LiveAccountPlot{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.PlaygroundSession{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.OrderRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.TradeRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.EquityPlotRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func InitPostgres(host, port, user, password, dbName string) (*gorm.DB, error) {
	url := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", host, user, password, dbName, port)
	return InitPostgresWithUrl(url)
}
