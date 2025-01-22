package main

import (
	"context"
	"fmt"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	r "github.com/jiaming2012/slack-trading/src/backtester-api/router"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

var db *gorm.DB

func initDB() error {
	var err error
	dsn := "host=localhost user=grodt password=test747 dbname=playground port=5432 sslmode=disable TimeZone=UTC"
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect database: %w", err)
	}

	// Migrate the schema
	if err := db.AutoMigrate(&models.PlaygroundSession{}); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

func savePlaygroundSession(playground models.IPlayground) error {
	meta := playground.GetMeta()

	if err := meta.Validate(); err != nil {
		return fmt.Errorf("savePlaygroundSession: invalid playground meta: %w", err)
	}

	repos := playground.GetRepositories()
	var repoDTOs []models.CandleRepositoryDTO
	for _, repo := range repos {
		repoDTOs = append(repoDTOs, repo.ToDTO())
	}

	store := &models.PlaygroundSession{
		ID:              playground.GetId(),
		StartAt:         meta.StartAt,
		EndAt:           meta.EndAt,
		StartingBalance: meta.StartingBalance,
		Repositories:    repoDTOs,
		Env:             string(meta.Environment),
	}

	if meta.Environment == models.PlaygroundEnvironmentLive {
		store.Broker = &meta.SourceBroker
		store.AccountID = &meta.SourceAccountId
		store.ApiKeyName = &meta.SourceApiKeyName
	}

	if err := db.Create(store).Error; err != nil {
		return fmt.Errorf("failed to save playground: %w", err)
	}

	return nil
}

func main() {
	fmt.Printf("Hello, Postgres!\n")

	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Fatalf("PROJECTS_DIR not set: %v", err)
	}

	ctx := context.Background()
	goEnv := "development"

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	if err != nil {
		log.Fatalf("$POLYGON_API_KEY not set: %v", err)
	}

	if err := initDB(); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}

	router := mux.NewRouter()
	liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*eventmodels.TradierOrderUpdateEvent](999)
	r.SetupHandler(ctx, router.PathPrefix("/playground").Subrouter(), projectsDir, polygonApiKey, liveOrdersUpdateQueue, db)

	req := &r.CreatePlaygroundRequest{
		Account: r.CreateAccountRequest{
			Balance: 10000.0,
			Source: &r.CreateAccountRequestSource{
				Broker:     "tradier",
				AccountID:  "VA12962195",
				ApiKeyName: "TRADIER_TRADES_BEARER_TOKEN",
			},
		},
		Repositories: []eventmodels.CreateRepositoryRequest{
			{
				Symbol: "AAPL",
				Timespan: eventmodels.PolygonTimespanRequest{
					Multiplier: 1,
					Unit:       "minute",
				},
				Source: eventmodels.RepositorySource{
					Type: eventmodels.RepositorySourceTradier,
				},
				Indicators:    []string{"supertrend"},
				HistoryInDays: 10,
			},
		},
		Env:       "live",
		SaveToDB: false,
		CreatedAt: time.Now(),
	}

	playground, webErr := r.CreatePlayground(req)
	if webErr != nil {
		log.Fatalf("failed to create playground: %v", webErr)
	}

	fmt.Printf("playground: %v\n", playground.GetId())

	fmt.Printf("saving playground...\n")

	if err := savePlaygroundSession(playground); err != nil {
		log.Fatalf("failed to save playground: %v", err)
	}

	fmt.Printf("playground saved\n")
}
