package services

import (
	"fmt"
	"sync"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

var (
	databaseMutex    = sync.Mutex{}
	liveRepositories = map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository{}
)

// func AppendToLiveRepository(symbol eventmodels.StockSymbol, interval eventmodels.TradierInterval, bars []*eventmodels.AggregateBarWithIndicators) error {
// 	databaseMutex.Lock()
// 	defer databaseMutex.Unlock()

// 	// query the liveRepository
// 	symbolRepo, ok := liveRepositories[symbol]
// 	if !ok {
// 		return fmt.Errorf("UpdateLiveRepository: symbol %s not found", symbol)
// 	}

// 	repo, ok := symbolRepo[interval]
// 	if !ok {
// 		return fmt.Errorf("UpdateLiveRepository: interval %s not found", interval)
// 	}

// 	// append the bars to the repository
// 	if err := repo.AppendBars(bars); err != nil {
// 		return fmt.Errorf("UpdateLiveRepository: failed to append bars: %w", err)
// 	}

// 	return nil
// }

func FetchAllLiveRepositories() (repositories []*models.CandleRepository, releaseLockFn func(), err error) {
	databaseMutex.Lock()

	repositories = []*models.CandleRepository{}
	for _, symbolRepo := range liveRepositories {
		for _, periodRepos := range symbolRepo {
			repositories = append(repositories, periodRepos...)
		}
	}

	return repositories, func() {
		databaseMutex.Unlock()
	}, nil
}

func SaveLiveRepository(repo *models.CandleRepository) error {
	databaseMutex.Lock()
	defer databaseMutex.Unlock()

	symbolRepo, ok := liveRepositories[repo.GetSymbol()]
	if !ok {
		symbolRepo = map[time.Duration][]*models.CandleRepository{}
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		periodRepos = []*models.CandleRepository{}
	}

	// append the repo to the periodRepos
	periodRepos = append(periodRepos, repo)
	symbolRepo[repo.GetPeriod()] = periodRepos
	liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
}

func CreateRepository(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, bars []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle], historyInDays uint32, source eventmodels.CandleRepositorySource) (*models.CandleRepository, error) {
	return CreateRepositoryWithPosition(symbol, timespan, bars, indicators, newCandlesQueue, 0, historyInDays, source)
}

func CreateRepositoryWithPosition(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, bars []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle], startingPosition int, historyInDays uint32, source eventmodels.CandleRepositorySource) (*models.CandleRepository, error) {
	period := timespan.ToDuration()
	repo, err := models.NewCandleRepository(symbol, period, bars, indicators, newCandlesQueue, historyInDays, source)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return repo, nil
}
