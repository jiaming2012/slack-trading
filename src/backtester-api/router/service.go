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
	"github.com/jiaming2012/slack-trading/src/eventservices"
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

		// fetch or create live repositories
		var repos []*models.LiveCandleRepository
		for _, repo := range req.Repositories {
			if repo.Source.Type != RepositorySourceTradier {
				return nil, eventmodels.NewWebError(400, "invalid repository source")
			}

			polygonTimespan := eventmodels.PolygonTimespan{
				Multiplier: repo.Timespan.Multiplier,
				Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
			}

			timespan, err := eventmodels.NewTradierInterval(polygonTimespan)
			if err != nil {
				return nil, eventmodels.NewWebError(400, "failed to create tradier interval")
			}

			liveRepository, err := services.FetchOrCreateLiveRepository(eventmodels.StockSymbol(repo.Symbol), timespan)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch or create live repository")
			}

			fmt.Printf("liveRepository: %v\n", liveRepository)

			repos = append(repos, liveRepository)
		}

		// create live playground
		playground, err = models.NewLivePlayground(liveAccount, repos)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create live playground")
		}

		// save live account
		services.SavePlaygroundAccount(playground, liveAccount)

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
		var feeds []*models.BacktesterCandleRepository
		for _, repo := range req.Repositories {
			if repo.Source.Type == RepositorySourceCSV && repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename")
			}

			timespan := eventmodels.PolygonTimespan{
				Multiplier: repo.Timespan.Multiplier,
				Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
			}

			var bars, pastBars []*eventmodels.PolygonAggregateBarV2
			if repo.Source.Type == RepositorySourcePolygon {
				bars, err = client.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, from, to)
				if err != nil {
					return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars")
				}

				if len(repo.Indicators) > 0 {
					_to := from.GetPreviousDay()
					// _from := _to.GetPreviousYear()
					_from := _to.GetPreviousDay()
					attempts := 5
					for i := 0; true; i++ {
						pastBars, err = client.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, _from, _to)
						if err == nil {
							break
						} else {
							if i == attempts-1 {
								return nil, eventmodels.NewWebError(500, "failed to fetch past aggregate bars")
							}

							_from = _from.GetPreviousDay()
							time.Sleep(10 * time.Millisecond)
						}
					}
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

			candles, err := eventservices.AddIndicatorsToCandles(bars, pastBars, repo.Indicators)
			if err != nil {
				log.Errorf("failed to add indicators to candles: %v", err)
				return nil, eventmodels.NewWebError(500, "failed to add indicators to candles")
			}

			startingPosition := len(pastBars)
			repository, err := createRepositoryWithPosition(eventmodels.StockSymbol(repo.Symbol), timespan, candles, startingPosition)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to create repository")
			}

			feeds = append(feeds, repository)
		}

		// create playground
		playground, err = models.NewPlayground(req.Balance, clock, env, feeds...)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create playground")
		}
	}

	playgrounds[playground.GetId()] = playground

	return playground, nil
}
