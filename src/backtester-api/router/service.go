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
		return nil, eventmodels.NewWebError(404, "handleCandles: playground not found", nil)
	}

	candles, err := playground.FetchCandles(symbol, period, from, to)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to fetch candles", err)
	}

	return candles, nil
}

func nextTick(playgroundID uuid.UUID, duration time.Duration, isPreview bool) (*models.TickDelta, error) {
	playground, found := playgrounds[playgroundID]
	if !found {
		return nil, fmt.Errorf("playground not found")
	}

	tickDelta, err := playground.Tick(duration, isPreview)
	if err != nil {
		return nil, fmt.Errorf("failed to tick: %v", err)
	}

	if playground.GetMeta().Environment == models.PlaygroundEnvironmentLive {
		if err := saveEquityPlotRecord(playgroundID, tickDelta.EquityPlot.Timestamp, tickDelta.EquityPlot.Value); err != nil {
			return nil, fmt.Errorf("failed to save equity plot record: %v", err)
		}
	}

	return tickDelta, nil
}

func getOpenOrders(playgroundID uuid.UUID, symbol eventmodels.Instrument) ([]*models.BacktesterOrder, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	// todo: add mutex for playground

	orders := playground.GetOpenOrders(symbol)

	return orders, nil
}

func GetPlaygrounds() []models.IPlayground {
	var playgroundsSlice []models.IPlayground
	for _, playground := range playgrounds {
		playgroundsSlice = append(playgroundsSlice, playground)
	}

	return playgroundsSlice
}

func getPlaygroundByClientId(clientId string) models.IPlayground {
	for _, playground := range playgrounds {
		cId := playground.GetClientId()
		if cId != nil && *cId == clientId {
			return playground
		}
	}

	return nil
}

func getPlayground(playgroundID uuid.UUID) (models.IPlayground, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	return playground, nil
}

func deletePlayground(playgroundID uuid.UUID) error {
	_, ok := playgrounds[playgroundID]
	if !ok {
		return eventmodels.NewWebError(404, "playground not found", nil)
	}

	delete(playgrounds, playgroundID)

	return nil
}

func getAccountStatsEquity(playgroundID uuid.UUID) ([]*eventmodels.EquityPlot, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	plot := playground.GetEquityPlot()
	return plot, nil
}

func getAccountInfo(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []models.BacktesterOrderStatus, sides []models.TradierOrderSide) (*GetAccountResponse, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	positions, err := playground.GetPositions()
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to get positions", nil)
	}

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
		filterOrders := from != nil || to != nil || len(status) > 0 || len(sides) > 0
		if filterOrders {
			filteredOrders := []*models.BacktesterOrder{}
			for _, order := range response.Orders {
				if from != nil && order.CreateDate.Before(*from) {
					continue
				}

				if to != nil && order.CreateDate.After(*to) {
					continue
				}

				if len(status) > 0 {
					found := false
					for _, s := range status {
						if order.Status == s {
							found = true
							break
						}
					}

					if !found {
						continue
					}
				}

				if len(sides) > 0 {
					found := false
					for _, s := range sides {
						if order.Side == s {
							found = true
							break
						}
					}

					if !found {
						continue
					}
				}

				filteredOrders = append(filteredOrders, order)
			}

			response.Orders = filteredOrders
		}
	}

	return &response, nil
}

func placeOrder(playgroundID uuid.UUID, req *CreateOrderRequest) (*models.BacktesterOrder, error) {
	playground, ok := playgrounds[playgroundID]
	if !ok {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	if err := req.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid request", err)
	}

	createdOn := playground.GetCurrentTime()

	var playgroundEnv models.PlaygroundEnvironment
	playgroundMeta := playground.GetMeta()
	if playgroundMeta != nil {
		playgroundEnv = playgroundMeta.Environment
	} else {
		return nil, eventmodels.NewWebError(500, "failed to get playground environment", nil)
	}

	liveOrderTempId := uint(0)
	if playgroundEnv == models.PlaygroundEnvironmentLive {
		req.Id = &liveOrderTempId
	}

	order, err := makeBacktesterOrder(playground, req, createdOn)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to place order", err)
	}

	return order, nil
}

func createNewReconcilePlaygroundAndLiveAccount(source *models.CreateAccountRequestSource, createdAt time.Time) (*models.LiveAccount, error) {
	createPlaygroundReq := &CreatePlaygroundRequest{
		Env: string(models.PlaygroundEnvironmentReconcile),
		Account: CreateAccountRequest{
			Source: &models.CreateAccountRequestSource{
				Broker:      source.Broker,
				AccountType: source.AccountType,
				AccountID:   source.AccountID,
			},
		},
		Repositories: nil,
		SaveToDB:     true,
		CreatedAt:    createdAt,
	}

	_playground, playgroundSession, err := CreatePlayground(createPlaygroundReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	playground, ok := _playground.(*models.Playground)
	if !ok {
		return nil, fmt.Errorf("failed to cast playground to reconcile playground")
	}

	reconcilePlayground, err := models.NewReconcilePlayground(playground)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to create new reconcile playground", err)
	}

	liveAccount, err := services.CreateLiveAccount(createPlaygroundReq.Account.Source.Broker, createPlaygroundReq.Account.Source.AccountType, reconcilePlayground)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to create live account", err)
	}

	// update playground balance
	response, err := liveAccount.Source.FetchEquity()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch equity: %w", err)
	}
	balance := response.Equity

	playground.SetBalance(balance)
	playground.Meta.InitialBalance = balance
	playground.Meta.SourceBroker = liveAccount.Source.GetBroker()
	playground.Meta.SourceAccountId = liveAccount.Source.GetAccountID()
	playground.Meta.LiveAccountType = liveAccount.Source.GetAccountType()

	playgroundSession.Balance = balance
	playgroundSession.StartingBalance = balance
	playgroundSession.Broker = &playground.Meta.SourceBroker
	playgroundSession.AccountID = &playground.Meta.SourceAccountId

	liveAccountType := string(liveAccount.Source.GetAccountType())
	playgroundSession.LiveAccountType = &liveAccountType
	if err := db.Save(playgroundSession).Error; err != nil {
		return nil, fmt.Errorf("failed to save playground session: %v", err)
	}

	return liveAccount, nil
}

// todo: this should be refactored to a service
// todo: refactor to use interfaces
func CreatePlayground(req *CreatePlaygroundRequest) (models.IPlayground, *models.PlaygroundSession, error) {
	env := models.PlaygroundEnvironment(req.Env)

	// validations
	if err := env.Validate(); err != nil {
		return nil, nil, eventmodels.NewWebError(400, "invalid playground environment", err)
	}

	if req.Env != string(models.PlaygroundEnvironmentReconcile) {
		if len(req.Repositories) == 0 {
			return nil, nil, eventmodels.NewWebError(400, "missing repositories", nil)
		}
	}

	// create playground
	var playground models.IPlayground
	var playgroundSession *models.PlaygroundSession

	if env == models.PlaygroundEnvironmentReconcile {
		var err error
		now := req.CreatedAt
		playground, err = models.NewPlayground(req.ID, req.ClientID, req.Account.Balance, req.InitialBalance, nil, req.BackfillOrders, env, now, req.Tags)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create reconcile playground", err)
		}

		if req.SaveToDB {
			if playgroundSession, err = savePlaygroundSession(playground); err != nil {
				return nil, nil, fmt.Errorf("failed to save reconcile playground: %v", err)
			}
		}

	} else if env == models.PlaygroundEnvironmentLive {
		now := time.Now()

		// capture all candles up to tomorrow
		tomorrow := now.AddDate(0, 0, 1)
		tomorrowStr := tomorrow.Format("2006-01-02")
		from, err := eventmodels.NewPolygonDate(tomorrowStr)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(400, "failed to parse clock.startDate", err)
		}

		// fetch or create live repositories
		newCandlesQueue := eventmodels.NewFIFOQueue[*models.BacktesterCandle]("newCandlesQueue", 999)

		newTradesFilledQueue := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesFilledQueue", 999)

		repos, webErr := createRepos(req.Repositories, from, nil, newCandlesQueue)
		if webErr != nil {
			return nil, nil, webErr
		}

		// save live repositories
		for _, repo := range repos {
			if err := services.SaveLiveRepository(repo); err != nil {
				// fatal as partial save is not allowed
				log.Fatalf("failed to save live repository: %v", err)
			}
		}

		// get live account
		account, found, err := FetchLiveAccount(req.Account.Source)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to fetch live account", err)
		}

		var liveAccount *models.LiveAccount
		if account != nil {
			var ok bool
			liveAccount, ok = account.(*models.LiveAccount)
			if !ok {
				return nil, nil, eventmodels.NewWebError(500, "failed to cast account to live account", nil)
			}
		}

		if !found {
			log.Debugf("failed to create live account: %v. Creating a new one ...", err)
			liveAccount, err = createNewReconcilePlaygroundAndLiveAccount(req.Account.Source, now)
			if err != nil {
				return nil, nil, eventmodels.NewWebError(500, "failed to create new reconcile playground and live account", err)
			}

			if err := saveLiveAccount(req.Account.Source, liveAccount); err != nil {
				return nil, nil, fmt.Errorf("failed to save live account: %v", err)
			}
		}

		// fetch live orders
		// todo: really should fetch positions instead of orders
		// orders if fetched, should be fetched from the DB

		// create live playground
		database := NewDatabaseService()
		playground, err = models.NewLivePlayground(req.ID, database, req.ClientID, liveAccount, req.InitialBalance, repos, newCandlesQueue, newTradesFilledQueue, req.BackfillOrders, req.CreatedAt, req.Tags)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create live playground", err)
		}

		// always save live playgrounds if flag is set
		if req.SaveToDB {
			if playgroundSession, err = savePlaygroundSession(playground); err != nil {
				return nil, nil, fmt.Errorf("failed to save playground: %v", err)
			}
		}

	} else if env == models.PlaygroundEnvironmentSimulator {
		// validations
		from, err := eventmodels.NewPolygonDate(req.Clock.StartDate)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(400, "failed to parse clock.startDate", err)
		}

		to, err := eventmodels.NewPolygonDate(req.Clock.StopDate)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(400, "failed to parse clock.stopDate", err)
		}

		// create clock
		clock, err := createClock(from, to)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create clock", err)
		}

		// create backtester repositories
		repos, webErr := createRepos(req.Repositories, from, to, nil)
		if webErr != nil {
			return nil, nil, webErr
		}

		// create playground
		now := clock.CurrentTime
		playground, err = models.NewPlayground(req.ID, req.ClientID, req.Account.Balance, req.InitialBalance, clock, req.BackfillOrders, env, now, req.Tags, repos...)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create playground", err)
		}
	} else {
		return nil, nil, eventmodels.NewWebError(400, "invalid playground environment", nil)
	}

	playground.SetEquityPlot(req.EquityPlotRecords)

	playground.SetOpenOrdersCache()

	playgrounds[playground.GetId()] = playground

	return playground, playgroundSession, nil
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

	return nil, eventmodels.NewWebError(500, errMsg, nil)
}

func createRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle]) ([]*models.CandleRepository, *eventmodels.WebError) {
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
			bars, err = client.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, from, to)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars", err)
			}
		} else if repo.Source.Type == eventmodels.RepositorySourceCSV {
			if repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename", nil)
			}

			sourceDir := path.Join(projectsDirectory, "slack-trading", "src", "backtester-api", "data", *repo.Source.CSVFilename)

			bars, err = utils.ImportCandlesFromCsv(sourceDir)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to import candles from CSV", err)
			}
		} else {
			return nil, eventmodels.NewWebError(400, "invalid repository source", nil)
		}

		if from != nil {
			pastBars, err = fetchPastCandles(eventmodels.StockSymbol(repo.Symbol), timespan, int(repo.HistoryInDays), from)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch past candles", err)
			}
		}

		aggregateBars := append(pastBars, bars...)

		source := eventmodels.CandleRepositorySource{
			Type: string(repo.Source.Type),
		}

		startingPosition := len(pastBars)
		repository, err := services.CreateRepositoryWithPosition(eventmodels.StockSymbol(repo.Symbol), timespan, aggregateBars, repo.Indicators, newCandlesQueue, startingPosition, repo.HistoryInDays, source)
		if err != nil {
			log.Errorf("failed to create repository: %v", err)
			return nil, eventmodels.NewWebError(500, "failed to create repository", err)
		}

		feeds = append(feeds, repository)
	}

	return feeds, nil
}
