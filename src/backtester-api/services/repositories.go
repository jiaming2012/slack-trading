package services

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

var (
	databaseMutex    = sync.Mutex{}
	liveRepositories = map[eventmodels.Instrument]map[eventmodels.TradierInterval]*models.LiveCandleRepository{}
)

func FetchAllLiveRepositories() (repositories []*models.LiveCandleRepository, releaseLockFn func(), err error) {
	databaseMutex.Lock()

	repositories = []*models.LiveCandleRepository{}
	for _, symbolRepo := range liveRepositories {
		for _, repo := range symbolRepo {
			repositories = append(repositories, repo)
		}
	}

	return repositories, func() {
		databaseMutex.Unlock()
	}, nil
}

func FetchOrCreateLiveRepository(symbol eventmodels.StockSymbol, timespan eventmodels.TradierInterval) (*models.LiveCandleRepository, error) {
	databaseMutex.Lock()
	defer databaseMutex.Unlock()

	symbolRepo, ok := liveRepositories[symbol]
	if !ok {
		symbolRepo = map[eventmodels.TradierInterval]*models.LiveCandleRepository{}
	}

	repo, ok := symbolRepo[timespan]
	if !ok {
		repo = createLiveRepository(symbol, timespan)
		symbolRepo[timespan] = repo
	}

	// save the symbolRepo back to the liveRepositories
	liveRepositories[symbol] = symbolRepo

	return repo, nil
}

func createLiveRepository(symbol eventmodels.Instrument, timespan eventmodels.TradierInterval) *models.LiveCandleRepository {
	return &models.LiveCandleRepository{
		Instrument: symbol,
		Period:     timespan,
	}
}
