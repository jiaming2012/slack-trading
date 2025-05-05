package dbutils

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

func UpdateOrder(tx *gorm.DB, order *models.OrderRecord) error {
	if order.ID == 0 {
		return fmt.Errorf("updateOrder: order ID is 0")
	}

	var existing models.OrderRecord
	if err := tx.First(&existing, order.ID).Error; err != nil {
		return fmt.Errorf("updateOrder: failed to find existing order: %w", err)
	}

	if err := tx.Save(order).Error; err != nil {
		return fmt.Errorf("updateOrder: failed to update order: %w", err)
	}

	return nil
}

func InitPostgresWithUrl(url string) (*gorm.DB, error) {
	log.ParseLevel(os.Getenv("LOG_LEVEL"))
	var level logger.LogLevel
	switch os.Getenv("LOG_LEVEL") {
	// case "trace":
	// 	level = logger.Info
	// case "warn":
	// 	level = logger.Warn
	default:
		level = logger.Warn
	}

	log.Infof("Setting database log level to %v", level)

	postgresLogger := logger.New(
		log.StandardLogger(),
		logger.Config{
			SlowThreshold:             1 * time.Second,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: postgresLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := db.AutoMigrate(&models.LiveAccount{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.LiveAccountPlot{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.TradeRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.OrderRecord{}); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	if err := db.AutoMigrate(&models.Playground{}); err != nil {
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
