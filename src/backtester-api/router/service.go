package router

import (
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func fetchCandles(playgroundID uuid.UUID, symbol eventmodels.StockSymbol, period time.Duration, from, to time.Time) ([]*eventmodels.AggregateBarWithIndicators, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "handleCandles: playground not found")
	}

	candles, err := playground.FetchCandles(symbol, period, from, to)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "handleCandles: failed to fetch candles")
	}

	return candles, nil
}

func nextTick(playgroundID uuid.UUID, duration time.Duration, isPreview bool) (*models.TickDelta, error) {
	playground, found := playgrounds[playgroundID]
	if !found {
		return nil, fmt.Errorf("playground not found")
	}

	tick, err := playground.Tick(duration, isPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	return tick, nil
}

func getOpenOrders(playgroundID uuid.UUID, symbol eventmodels.Instrument) ([]*models.BacktesterOrder, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found")
	}

	// todo: add mutex for playground

	orders := playground.GetOpenOrders(symbol)

	return orders, nil
}

func getAccountInfo(playgroundID uuid.UUID, fetchOrders bool) (*GetAccountResponse, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found")
	}

	positions := playground.GetPositions()

	positionsKV := map[string]*models.Position{}
	for k, v := range positions {
		positionsKV[k.GetTicker()] = v
	}

	response := GetAccountResponse{
		Meta:       playground.GetMeta(),
		Balance:    playground.GetBalance(),
		Equity:     playground.GetEquity(positions),
		FreeMargin: playground.GetFreeMarginFromPositionMap(positions),
		Positions:  positionsKV,
	}

	if fetchOrders {
		response.Orders = playground.GetOrders()
	}

	return &response, nil
}

func placeOrder(playgroundID uuid.UUID, req *CreateOrderRequest) (*models.BacktesterOrder, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found")
	}

	if err := req.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid request")
	}

	createdOn := playground.GetCurrentTime()

	order, err := makeBacktesterOrder(playground, req, createdOn)
	if err != nil {
		return nil, eventmodels.NewWebError(500, fmt.Sprintf("failed to place order: %v", err))
	}

	return order, nil
}

func createPlayground(req *CreatePlaygroundRequest) (models.IPlayground, error) {
	env := models.PlaygroundEnvironment(req.Env)

	// validations
	if err := env.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid playground environment")
	}

	if len(req.Repositories) == 0 {
		return nil, eventmodels.NewWebError(400, "missing repositories")
	}

	// create playground
	var playground models.IPlayground

	if env == models.PlaygroundEnvironmentLive {
		// create live account
		liveAccount, err := services.CreateLiveAccount(req.Account.Balance, req.Account.Source.AccountID, req.Account.Source.Broker, req.Account.Source.ApiKeyName)
		if err != nil {
			log.Errorf("failed to create live account: %v", err)
			return nil, err
		}

		fmt.Printf("liveAccount: %v\n", liveAccount)

		// capture all candles up to tomorrow
		tomorrow := time.Now().AddDate(0, 0, 1)
		tomorrowStr := tomorrow.Format("2006-01-02")
		from, err := eventmodels.NewPolygonDate(tomorrowStr)
		if err != nil {
			return nil, eventmodels.NewWebError(400, "failed to parse clock.startDate")
		}

		// fetch or create live repositories
		newCandlesQueue := eventmodels.NewFIFOQueue[*models.BacktesterCandle](1000)

		repos, webErr := createRepos(req.Repositories, from, nil, newCandlesQueue)
		if webErr != nil {
			return nil, webErr
		}

		// save live repositories
		for _, repo := range repos {
			if err := services.SaveLiveRepository(repo); err != nil {
				// fatal as partial save is not allowed
				log.Fatalf("failed to save live repository: %v", err)
			}
		}

		// create live playground
		playground, err = models.NewLivePlayground(liveAccount, repos, newCandlesQueue)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create live playground")
		}

	} else {
		// validations
		from, err := eventmodels.NewPolygonDate(req.Clock.StartDate)
		if err != nil {
			return nil, eventmodels.NewWebError(400, "failed to parse clock.startDate")
		}

		to, err := eventmodels.NewPolygonDate(req.Clock.StopDate)
		if err != nil {
			return nil, eventmodels.NewWebError(400, "failed to parse clock.stopDate")
		}

		// create clock
		clock, err := createClock(from, to)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create clock")
		}

		// create backtester repositories
		repos, webErr := createRepos(req.Repositories, from, to, nil)
		if webErr != nil {
			return nil, webErr
		}

		// create playground
		playground, err = models.NewPlayground(req.Balance, clock, env, repos...)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create playground")
		}
	}

	playgrounds[playground.GetId()] = playground

	return playground, nil
}

func fetchPastCandles(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, daysPast int, end *eventmodels.PolygonDate) ([]*eventmodels.PolygonAggregateBarV2, error) {
	to := end.GetPreviousDay(1)
	from := to.GetPreviousDay(daysPast)
	maxAttempts := 5

	errMsg := ""
	for i := 0; true; i++ {
		pastBars, err := client.FetchAggregateBars(eventmodels.StockSymbol(symbol), timespan, from, to)
		if err != nil {
			if i == maxAttempts-1 {
				errMsg = fmt.Sprintf("failed to fetch past candles from %s to %s: %v", from.ToString(), to.ToString(), err)
				break
			}

			from = from.GetPreviousDay(1)
			time.Sleep(10 * time.Millisecond)

			continue
		}

		return pastBars, nil
	}

	return nil, eventmodels.NewWebError(500, errMsg)
}

func createRepos(repoRequests []CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle]) ([]*models.CandleRepository, *eventmodels.WebError) {
	var feeds []*models.CandleRepository
	for _, repo := range repoRequests {
		var bars, pastBars []*eventmodels.PolygonAggregateBarV2
		var err error

		timespan := eventmodels.PolygonTimespan{
			Multiplier: repo.Timespan.Multiplier,
			Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
		}

		if repo.Source.Type == RepositorySourceTradier {
			// pass
		} else if repo.Source.Type == RepositorySourcePolygon {
			bars, err = client.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, from, to)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars")
			}
		} else if repo.Source.Type == RepositorySourceCSV {
			if repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename")
			}

			sourceDir := path.Join(projectsDirectory, "slack-trading", "src", "backtester-api", "data", *repo.Source.CSVFilename)

			bars, err = utils.ImportCandlesFromCsv(sourceDir)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to import candles from CSV")
			}
		} else {
			return nil, eventmodels.NewWebError(400, "invalid repository source")
		}

		if len(repo.Indicators) > 0 {
			pastBars, err = fetchPastCandles(eventmodels.StockSymbol(repo.Symbol), timespan, int(repo.HistoryInDays), from)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch past candles")
			}
		}

		aggregateBars := append(pastBars, bars...)

		startingPosition := len(pastBars)
		repository, err := services.CreateRepositoryWithPosition(eventmodels.StockSymbol(repo.Symbol), timespan, aggregateBars, repo.Indicators, newCandlesQueue, startingPosition)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create repository")
		}

		feeds = append(feeds, repository)
	}

	return feeds, nil
}
