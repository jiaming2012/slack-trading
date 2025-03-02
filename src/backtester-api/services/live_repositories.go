package services

import (
	"fmt"
	"path"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func (s *BacktesterApiService) FetchAllLiveRepositories() (repositories []*models.CandleRepository, releaseLockFn func(), err error) {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	repositories = []*models.CandleRepository{}
	for _, symbolRepo := range s.liveRepositories {
		for _, periodRepos := range symbolRepo {
			repositories = append(repositories, periodRepos...)
		}
	}

	return repositories, func() {
		s.liveAccountsMutex.Unlock()
	}, nil
}

func (s *BacktesterApiService) RemoveLiveRepository(repo *models.CandleRepository) error {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	symbolRepo, ok := s.liveRepositories[repo.GetSymbol()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: symbol %s not found", repo.GetSymbol())
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: period %s not found", repo.GetPeriod())
	}

	foundRepo := false
	for i, r := range periodRepos {
		if r == repo {
			periodRepos = append(periodRepos[:i], periodRepos[i+1:]...)
			foundRepo = true
			break
		}
	}

	if !foundRepo {
		return fmt.Errorf("DeleteLiveRepository: repository not found")
	}

	symbolRepo[repo.GetPeriod()] = periodRepos
	s.liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
}

func (s *BacktesterApiService) SaveLiveRepository(repo *models.CandleRepository) error {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	symbolRepo, ok := s.liveRepositories[repo.GetSymbol()]
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
	s.liveRepositories[repo.GetSymbol()] = symbolRepo

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

func (s *BacktesterApiService) CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle]) ([]*models.CandleRepository, *eventmodels.WebError) {
	var feeds []*models.CandleRepository
	for _, repo := range repoRequests {
		var bars, pastBars []*eventmodels.PolygonAggregateBarV2
		var err error

		timespan := eventmodels.PolygonTimespan{
			Multiplier: repo.Timespan.Multiplier,
			Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
		}

		if repo.Source.Type == eventmodels.RepositorySourceTradier {
			// pass
		} else if repo.Source.Type == eventmodels.RepositorySourcePolygon {
			bars, err = s.polygonClient.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, from, to)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars", err)
			}
		} else if repo.Source.Type == eventmodels.RepositorySourceCSV {
			if repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename", nil)
			}

			sourceDir := path.Join(s.projectsDir, "slack-trading", "src", "backtester-api", "data", *repo.Source.CSVFilename)

			bars, err = utils.ImportCandlesFromCsv(sourceDir)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to import candles from CSV", err)
			}
		} else {
			return nil, eventmodels.NewWebError(400, "invalid repository source", nil)
		}

		if from != nil {
			pastBars, err = s.polygonClient.FetchPastCandles(eventmodels.StockSymbol(repo.Symbol), timespan, int(repo.HistoryInDays), from)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch past candles", err)
			}
		}

		aggregateBars := append(pastBars, bars...)

		source := eventmodels.CandleRepositorySource{
			Type: string(repo.Source.Type),
		}

		startingPosition := len(pastBars)
		repository, err := CreateRepositoryWithPosition(eventmodels.StockSymbol(repo.Symbol), timespan, aggregateBars, repo.Indicators, newCandlesQueue, startingPosition, repo.HistoryInDays, source)
		if err != nil {
			log.Errorf("failed to create repository: %v", err)
			return nil, eventmodels.NewWebError(500, "failed to create repository", err)
		}

		feeds = append(feeds, repository)
	}

	return feeds, nil
}
