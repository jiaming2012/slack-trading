package router

import (
	"fmt"
	"path"
	"time"

	"github.com/google/uuid"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func fetchCandles(playgroundID uuid.UUID, symbol eventmodels.StockSymbol, period time.Duration, from, to time.Time) ([]*eventmodels.PolygonAggregateBarV2, error) {
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

func createPlayground(req *CreatePlaygroundRequest) (*models.Playground, error) {
	env := models.PlaygroundEnvironment(req.Env)
	
	if err := env.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid playground environment")
	}

	var playground *models.Playground

	if env == models.PlaygroundEnvironmentLive {

	} else {
		// create clock
		from, err := eventmodels.NewPolygonDate(req.Clock.StartDate)
		if err != nil {
			return nil, eventmodels.NewWebError(400, "failed to parse clock.startDate")
		}

		to, err := eventmodels.NewPolygonDate(req.Clock.StopDate)
		if err != nil {
			return nil, eventmodels.NewWebError(400, "failed to parse clock.stopDate")
		}

		clock, err := createClock(from, to)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to create clock")
		}

		// create repositories
		if len(req.Repositories) == 0 {
			return nil, eventmodels.NewWebError(400, "missing repositories")
		}

		var feeds []models.BacktesterDataFeed
		for _, repo := range req.Repositories {
			if repo.Source.Type == RepositorySourceCSV && repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename")
			}
		
			timespan := eventmodels.PolygonTimespan{
				Multiplier: repo.Timespan.Multiplier,
				Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
			}

			var bars []*eventmodels.PolygonAggregateBarV2
			if repo.Source.Type == RepositorySourcePolygon {
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

			repository, err := createRepository(eventmodels.StockSymbol(repo.Symbol), timespan, bars)
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

	playgrounds[playground.ID] = playground

	return playground, nil
}
