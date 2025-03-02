package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

func (s *BacktesterApiService) CreatePlayground(req *models.CreatePlaygroundRequest) (models.IPlayground, *models.PlaygroundSession, error) {
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
			if playgroundSession, err = s.dbService.SavePlaygroundSession(playground); err != nil {
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

		repos, webErr := s.CreateRepos(req.Repositories, from, nil, newCandlesQueue)
		if webErr != nil {
			return nil, nil, webErr
		}

		// save live repositories
		for _, repo := range repos {
			if err := s.SaveLiveRepository(repo); err != nil {
				// fatal as partial save is not allowed
				log.Fatalf("failed to save live repository: %v", err)
			}
		}

		// get live account
		account, found, err := s.dbService.FetchLiveAccount(req.Account.Source)
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
			liveAccount, err = s.createNewReconcilePlaygroundAndLiveAccount(req.Account.Source, now)
			if err != nil {
				return nil, nil, eventmodels.NewWebError(500, "failed to create new reconcile playground and live account", err)
			}

			if err := s.dbService.SaveLiveAccount(req.Account.Source, liveAccount); err != nil {
				return nil, nil, fmt.Errorf("failed to save live account: %v", err)
			}
		}

		// fetch live orders
		// todo: really should fetch positions instead of orders
		// orders if fetched, should be fetched from the DB

		// create live playground
		playground, err = models.NewLivePlayground(req.ID, s.dbService, req.ClientID, liveAccount, req.InitialBalance, repos, newCandlesQueue, newTradesFilledQueue, req.BackfillOrders, req.CreatedAt, req.Tags)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create live playground", err)
		}

		// always save live playgrounds if flag is set
		if req.SaveToDB {
			if playgroundSession, err = s.dbService.SavePlaygroundSession(playground); err != nil {
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
		clock, err := s.CreateClock(from, to)
		if err != nil {
			return nil, nil, eventmodels.NewWebError(500, "failed to create clock", err)
		}

		// create backtester repositories
		repos, webErr := s.CreateRepos(req.Repositories, from, to, nil)
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

	if err := s.GetDbService().SaveInMemoryPlayground(playground); err != nil {
		return nil, nil, fmt.Errorf("failed to save in-memory playground: %w", err)
	}

	return playground, playgroundSession, nil
}

func (s *BacktesterApiService) createNewReconcilePlaygroundAndLiveAccount(source *models.CreateAccountRequestSource, createdAt time.Time) (*models.LiveAccount, error) {
	createPlaygroundReq := &models.CreatePlaygroundRequest{
		Env: string(models.PlaygroundEnvironmentReconcile),
		Account: models.CreateAccountRequest{
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

	_playground, playgroundSession, err := s.CreatePlayground(createPlaygroundReq)
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

	liveAccount, err := s.CreateLiveAccount(createPlaygroundReq.Account.Source.Broker, createPlaygroundReq.Account.Source.AccountType, reconcilePlayground)
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
	if err := s.dbService.UpdatePlaygroundSession(playgroundSession); err != nil {
		return nil, fmt.Errorf("failed to update playground session: %v", err)
	}

	return liveAccount, nil
}

func (s *BacktesterApiService) PopulatePlayground(p models.PlaygroundSession) (models.IPlayground, error) {
	orders := make([]*models.BacktesterOrder, len(p.Orders))

	pIDStr := p.ID.String()

	log.Infof("loading playground: %s", pIDStr)

	var err error
	for i, o := range p.Orders {
		orders[i], err = o.ToBacktesterOrder()
		if err != nil {
			return nil, fmt.Errorf("loadPlaygrounds: failed to convert order: %w", err)
		}
	}

	var source *models.CreateAccountRequestSource
	var clockRequest models.CreateClockRequest
	if p.Env == "simulator" {
		if p.EndAt == nil {
			return nil, fmt.Errorf("loadPlaygrounds: missing end date for simulator playground")
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
			StopDate:  p.EndAt.Format(time.RFC3339),
		}

	} else if p.Env == "live" {
		if p.Broker == nil || p.AccountID == nil || p.LiveAccountType == nil {
			return nil, fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for live playground")
		}

		liveAccountType := models.LiveAccountType(*p.LiveAccountType)
		if err := liveAccountType.Validate(); err != nil {
			return nil, fmt.Errorf("loadPlaygrounds: invalid live account type for live playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:      *p.Broker,
			AccountID:   *p.AccountID,
			AccountType: liveAccountType,
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
		}

	} else if p.Env == "reconcile" {
		if p.Broker == nil || p.AccountID == nil || p.LiveAccountType == nil {
			return nil, fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for reconcile playground")
		}

		liveAccountType := models.LiveAccountType(*p.LiveAccountType)
		if err := liveAccountType.Validate(); err != nil {
			return nil, fmt.Errorf("loadPlaygrounds: invalid live account type for reconcile playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:      *p.Broker,
			AccountID:   *p.AccountID,
			AccountType: liveAccountType,
		}

	} else {
		return nil, fmt.Errorf("loadPlaygrounds: unknown environment: %v", p.Env)
	}

	var createRepoRequests []eventmodels.CreateRepositoryRequest
	for _, r := range p.Repositories {
		req, err := r.ToCreateRepositoryRequest()
		if err != nil {
			return nil, fmt.Errorf("loadPlaygrounds: failed to convert repository: %w", err)
		}

		createRepoRequests = append(createRepoRequests, req)
	}

	var plot []*eventmodels.EquityPlot
	for _, r := range p.EquityPlotRecords {
		plot = append(plot, &eventmodels.EquityPlot{
			Timestamp: r.Timestamp,
			Value:     r.Equity,
		})
	}

	playground, _, err := s.CreatePlayground(&models.CreatePlaygroundRequest{
		ID:       &p.ID,
		ClientID: p.ClientID,
		Env:      p.Env,
		Account: models.CreateAccountRequest{
			Balance: p.Balance,
			Source:  source,
		},
		InitialBalance:    p.StartingBalance,
		Clock:             clockRequest,
		Repositories:      createRepoRequests,
		BackfillOrders:    orders,
		CreatedAt:         p.CreatedAt,
		EquityPlotRecords: plot,
		Tags:              p.Tags,
		SaveToDB:          false,
	})

	if err != nil {
		return nil, fmt.Errorf("loadPlaygrounds: failed to create playground: %w", err)
	}

	return playground, nil
}

func (s *BacktesterApiService) PlaceOrder(playgroundID uuid.UUID, req *models.CreateOrderRequest) (*models.BacktesterOrder, error) {
	playground, err := s.dbService.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", err)
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

	order, err := s.makeBacktesterOrder(playground, req, createdOn)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to place order", err)
	}

	return order, nil
}

func (s *BacktesterApiService) makeBacktesterOrder(playground models.IPlayground, req *models.CreateOrderRequest, createdOn time.Time) (*models.BacktesterOrder, error) {
	var orderId uint
	if req.Id != nil {
		orderId = *req.Id
	} else {
		orderId = playground.NextOrderID()
	}

	order := models.NewBacktesterOrder(
		orderId,
		req.Class,
		createdOn,
		eventmodels.StockSymbol(req.Symbol),
		req.Side,
		req.Quantity,
		req.OrderType,
		req.Duration,
		req.RequestedPrice,
		req.Price,
		req.StopPrice,
		models.BacktesterOrderStatusPending,
		req.Tag,
	)

	changes, err := playground.PlaceOrder(order)
	if err != nil {
		return nil, fmt.Errorf("placeOrder: failed to place order: %w", err)
	}

	switch playground.(type) {
	case *models.LivePlayground:
		liveAccountType := playground.GetLiveAccountType()
		if err := s.dbService.SaveOrderRecord(playground.GetId(), order, nil, liveAccountType); err != nil {
			return nil, fmt.Errorf("makeBacktesterOrder: failed to save live order record: %w", err)
		}
	}

	for _, change := range changes {
		change.Commit()
	}

	return order, nil
}

func (s *BacktesterApiService) GetAccountStatsEquity(playgroundID uuid.UUID) ([]*eventmodels.EquityPlot, error) {
	playground, err := s.dbService.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	plot := playground.GetEquityPlot()
	return plot, nil
}

func (s *BacktesterApiService) GetAccountInfo(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []models.BacktesterOrderStatus, sides []models.TradierOrderSide) (*models.GetAccountResponse, error) {
	playground, err := s.dbService.FetchPlayground(playgroundID)
	if err != nil {
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

	response := models.GetAccountResponse{
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

func (s *BacktesterApiService) getOpenOrders(playgroundID uuid.UUID, symbol eventmodels.Instrument) ([]*models.BacktesterOrder, error) {
	playground, err := s.dbService.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	// todo: add mutex for playground

	orders := playground.GetOpenOrders(symbol)

	return orders, nil
}
