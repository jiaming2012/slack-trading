package services

import (
	"sync"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

var (
	databaseMutex    = sync.Mutex{}
	liveRepositories = map[eventmodels.Instrument]map[eventmodels.TradierInterval]*eventmodels.LiveCandleRepository{}
)

func FetchAllLiveRepositories() (repositories []*eventmodels.LiveCandleRepository, releaseLockFn func(), err error) {
	databaseMutex.Lock()

	repositories = []*eventmodels.LiveCandleRepository{}
	for _, symbolRepo := range liveRepositories {
		for _, repo := range symbolRepo {
			repositories = append(repositories, repo)
		}
	}

	return repositories, func() {
		databaseMutex.Unlock()
	}, nil
}

func FetchOrCreateLiveRepository(symbol eventmodels.StockSymbol, timespan eventmodels.TradierInterval) (*eventmodels.LiveCandleRepository, error) {
	databaseMutex.Lock()
	defer databaseMutex.Unlock()

	symbolRepo, ok := liveRepositories[symbol]
	if !ok {
		symbolRepo = map[eventmodels.TradierInterval]*eventmodels.LiveCandleRepository{}
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

func createLiveRepository(symbol eventmodels.Instrument, timespan eventmodels.TradierInterval) *eventmodels.LiveCandleRepository {
	return &eventmodels.LiveCandleRepository{
		Instrument: symbol,
		Period:     timespan,
	}
}
