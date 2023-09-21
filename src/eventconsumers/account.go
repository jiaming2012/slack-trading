package eventconsumers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
	"slack-trading/src/models"
	"sync"
	"time"
)

type AccountWorker struct {
	wg               *sync.WaitGroup
	accounts         []*models.Account
	coinbaseDatafeed *models.Datafeed
	manualDatafeed   *models.Datafeed
}

func (w *AccountWorker) getAccounts() []models.Account {
	var accounts []models.Account

	for _, acc := range w.accounts {
		accounts = append(accounts, *acc)
	}

	return accounts
}

func (w *AccountWorker) addAccount(account *models.Account, balance float64, priceLevels []*models.PriceLevel) error {
	strategy, err := models.NewStrategy("trendline-break", "BTC-USD", "down", balance, priceLevels, account)
	if err != nil {
		return err
	}

	account.AddStrategy(*strategy)

	w.accounts = append(w.accounts, account)

	return nil
}

//func (r *AccountWorker) newOpenTradeRequest(accountName string, strategyName string, tradeType models.TradeType) (*models.OpenTradeRequest, error) {
//	account, err := r.findAccount(accountName)
//	if err != nil {
//		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: could not find account: %w", err)
//	}
//
//	currentTick := r.coinbaseDatafeed.Query()
//
//	openTradeRequest, err := account.PlaceOpenTradeRequest(strategyName, currentTick.Bid)
//	if err != nil {
//		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: PlaceOrderOpen: %w", err)
//	}
//
//	return openTradeRequest, nil
//}

func (w *AccountWorker) findAccount(name string) (*models.Account, error) {
	for _, a := range w.accounts {
		if name == a.Name {
			return a, nil
		}
	}

	return nil, fmt.Errorf("AccountWorker.findAccount: could not find account with name %v", name)
}

func (w *AccountWorker) addAccountRequestHandler(request eventmodels.AddAccountRequestEvent) {
	log.Debug("<- AccountWorker.addAccountRequestHandler")

	var priceLevels []*models.PriceLevel

	for _, input := range request.PriceLevelsInput {
		priceLevels = append(priceLevels, &models.PriceLevel{
			Price:             input[0],
			MaxNoOfTrades:     int(input[1]),
			AllocationPercent: input[2],
		})
	}

	account, err := models.NewAccount(request.Name, request.Balance, nil)
	if err != nil {
		pubsub.PublishError("AccountWorker.addAccountHandler", err)
		return
	}

	err = w.addAccount(account, request.Balance, priceLevels)
	if err != nil {
		pubsub.PublishError("AccountWorker.NewStrategy", err)
		return
	}

	pubsub.Publish("AccountWorker.addAccountHandler", pubsub.AddAccountResponseEvent, eventmodels.AddAccountResponseEvent{
		Account: *account,
	})
}

func (w *AccountWorker) getAccountsRequestHandler(request eventmodels.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.Publish("AccountWorker", pubsub.GetAccountsResponseEvent, eventmodels.GetAccountsResponseEvent{
		Accounts: w.getAccounts(),
	})
}

func (w *AccountWorker) checkForStopOut(tick models.Tick) (*models.Strategy, error) {
	// todo: analyze if calling PL() so many times on each tick causes a bottleneck
	for _, account := range w.accounts {
		for _, strategy := range account.Strategies {
			stats, err := strategy.GetTrades().GetTradeStats(tick)
			if err != nil {
				return nil, fmt.Errorf("AccountWorker.checkForStopOut: GetTradeStats failed: %w", err)
			}

			if stats.RealizedPL+stats.FloatingPL >= strategy.Balance {
				return &strategy, nil
			}
		}
	}

	return nil, nil
}

func (w *AccountWorker) updateTickMachine(tick eventmodels.Tick) {
	// todo: eventually update based off level 2 quotes to get bid and ask
	w.coinbaseDatafeed.Update(models.Tick{
		Timestamp: tick.Timestamp,
		Bid:       tick.Price,
		Ask:       tick.Price,
	})
}

func (w *AccountWorker) update() {
	tick := w.coinbaseDatafeed.Tick()
	strategy, err := w.checkForStopOut(*tick)
	if err != nil {
		log.Errorf("AccountWorker.update: check for stop out failed: %v", err)
		return
	}

	if strategy != nil {
		// send for event

	}
}

// todo: make this the model: NewCloseTradeRequest -> ExecuteCloseTradesRequest
func (w *AccountWorker) handleNewCloseTradeRequest(event eventmodels.CloseTradeRequest) {
	log.Debug("<- AccountWorker.handleNewCloseTradeRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find account: %w", err))
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	closeTradesRequest, err := models.NewCloseTradesRequestV2(strategy, event.Timeframe, event.PriceLevelIndex, event.Percent)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("new close trades request failed: %w", err))
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	pubsub.Publish("AccountWorker.handleCloseTradeRequest", pubsub.ExecuteCloseTradesRequest, eventmodels.ExecuteCloseTradesRequest{
		RequestID:          event.RequestID,
		CloseTradesRequest: closeTradesRequest,
	})
}

func (w *AccountWorker) handleExecuteCloseTradesRequest(event eventmodels.ExecuteCloseTradesRequest) {
	log.Debug("<- AccountWorker.handleExecuteCloseTradesRequest")

	clsTradeReq := event.CloseTradesRequest
	tradeID := uuid.New()
	now := time.Now()
	datafeed := event.CloseTradesRequest.Strategy.Account.Datafeed

	var requestPrc float64
	if event.CloseTradesRequest.Strategy.Direction == models.Up {
		requestPrc = datafeed.LastBid
	} else if event.CloseTradesRequest.Strategy.Direction == models.Down {
		requestPrc = datafeed.LastOffer
	}

	trade, err := clsTradeReq.Strategy.NewCloseTrades(tradeID, clsTradeReq.Timeframe, now, requestPrc, clsTradeReq.PriceLevelIndex, clsTradeReq.Percent)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new close trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	result, err := clsTradeReq.Strategy.AutoExecuteTrade(trade)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to place execute trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	executeCloseTradesResult := &eventmodels.ExecuteCloseTradesResult{
		RequestID: event.RequestID,
		Side:      clsTradeReq.Strategy.GetTradeType(true).String(),
		Result:    result,
	}

	pubsub.Publish("AccountWorker.handleExecuteCloseTradesRequest", pubsub.ExecuteCloseTradesResult, executeCloseTradesResult)
}

func (w *AccountWorker) getMarketPrice(strategy *models.Strategy, isClose bool) float64 {
	tick := w.coinbaseDatafeed.Tick()
	var requestPrc float64
	if strategy.Direction == models.Up {
		if isClose {
			requestPrc = tick.Bid
		} else {
			requestPrc = tick.Ask
		}
	} else if strategy.Direction == models.Down {
		if isClose {
			requestPrc = tick.Ask
		} else {
			requestPrc = tick.Bid
		}
	}

	return requestPrc
}

func (w *AccountWorker) handleExecuteNewOpenTradeRequest(event eventmodels.ExecuteOpenTradeRequest) {
	log.Debug("<- AccountWorker.handleExecuteNewOpenTradeRequest")

	req := event.OpenTradeRequest
	tradeID := uuid.New()
	now := time.Now()
	datafeed := event.OpenTradeRequest.Strategy.Account.Datafeed

	var requestPrc float64
	if event.OpenTradeRequest.Strategy.Direction == models.Up {
		requestPrc = datafeed.LastOffer
	} else if event.OpenTradeRequest.Strategy.Direction == models.Down {
		requestPrc = datafeed.LastBid
	}

	trade, err := req.Strategy.NewOpenTrade(tradeID, req.Timeframe, now, requestPrc)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteNewOpenTradeRequest", requestErr)
		return
	}

	result, err := req.Strategy.AutoExecuteTrade(trade)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to place execute trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteNewOpenTradeRequest", requestErr)
		return
	}

	executeOpenTradeResult := &eventmodels.ExecuteOpenTradeResult{
		RequestID: event.RequestID,
		Side:      req.Strategy.GetTradeType(false).String(),
		Result:    result,
	}

	pubsub.Publish("AccountWorker.handleExecuteNewOpenTradeRequest", pubsub.ExecuteOpenTradeResult, executeOpenTradeResult)
}

func (w *AccountWorker) handleNewOpenTradeRequest(event eventmodels.OpenTradeRequest) {
	log.Debug("<- AccountWorker.handleNewOpenTradeRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find findAccount: %w", err))
		pubsub.PublishError("AccountWorker.handleNewOpenTradeRequest", requestErr)
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
		pubsub.PublishError("AccountWorker.handleNewOpenTradeRequest", requestErr)
		return
	}

	// todo: refactor - if another method already has the strategy, e.g. - eventservices.UpdateConditions, could
	// it just invoke execute open trade request directly?
	// Furthermore, is there a difference between a request originating from outside of the system - e.g. NewOpenTradeRequest
	// and inside of the system - e.g. ExecuteOpenTradeRequest
	openTradeReq, err := models.NewOpenTradeRequest(
		event.Timeframe,
		strategy,
	)

	pubsub.Publish("AccountWorker.handleNewOpenTradeRequest", pubsub.ExecuteOpenTradeRequest, eventmodels.ExecuteOpenTradeRequest{
		RequestID:        event.RequestID,
		OpenTradeRequest: openTradeReq,
	})
}

// todo:: this is the model! Refactor services to be standardized. Ideally in its own directory of sorts
// todo: TEST THIS !!! And reproduce the issue of closing 50% of one trade in postman
func (w *AccountWorker) fetchTrades(event *eventmodels.FetchTradesRequest) (*eventmodels.FetchTradesResult, error) {
	account, err := w.findAccount(event.AccountName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.fetchTradesRequest: failed to find findAccount: %w", err)
	}

	fetchTradesResult := eventservices.FetchTrades(event.RequestID, account)

	return fetchTradesResult, nil
}

func (w *AccountWorker) handleFetchTradesRequest(event *eventmodels.FetchTradesRequest) {
	log.Debug("<- AccountWorker.handleFetchTradesRequest")

	fetchTradesResult, err := w.fetchTrades(event)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleFetchTradesRequest", event, err)
		return
	}

	pubsub.Publish("AccountWorker.handleFetchTradesRequest", pubsub.FetchTradesResult, fetchTradesResult)
}

func (w *AccountWorker) handleGetStatsRequest(event *eventmodels.GetStatsRequest) {
	log.Debug("<- AccountWorker.handleGetStatsRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetStatsRequest", event, fmt.Errorf("failed to find findAccount: %w", err))
		return
	}

	currentTick := w.coinbaseDatafeed.Tick()

	statsResult, err := eventservices.GetStats(event.RequestID, account, currentTick)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetStatsRequest", event, err)
		return
	}

	pubsub.Publish("AccountWorker.handleGetStatsRequest", pubsub.GetStatsResult, statsResult)
}

func (w *AccountWorker) handleNewSignalRequest(event *eventmodels.SignalRequest) {
	newSignalResult, entryConditionsSatisfied := eventservices.UpdateConditions(w.getAccounts(), event)

	if entryConditionsSatisfied != nil {
		for _, satisfiedConditions := range entryConditionsSatisfied {
			id := uuid.New()

			// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove
			// as the request didn't originate from an api call but is still picked up by the lister
			eventmodels.RegisterResultCallback(id)

			req, err := eventmodels.NewOpenTradeRequest(id, satisfiedConditions.Account.Name, satisfiedConditions.Strategy.Name, 5) // todo: timeframe should come from signal
			if err != nil {
				pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
				return
			}

			pubsub.Publish("AccountWorker.handleNewSignalRequest", pubsub.NewOpenTradeRequest, *req)
		}
	}

	pubsub.Publish("AccountWorker.handleNewSignalRequest", pubsub.NewSignalsResult, newSignalResult)
}

func (w *AccountWorker) handleManualDatafeedUpdateRequest(ev *eventmodels.ManualDatafeedUpdateRequest) {
	ts := time.Now()
	w.manualDatafeed.Update(models.Tick{
		Timestamp: ts,
		Bid:       ev.Bid,
		Ask:       ev.Ask,
	})

	result := eventmodels.NewManualDatafeedUpdateResult(ev.RequestID, ts)

	pubsub.Publish("AccountWorker.handleManualDatafeedUpdateRequest", pubsub.ManualDatafeedUpdateResult, result)
}

func (w *AccountWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, w.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.GetAccountsRequestEvent, w.getAccountsRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.NewTickEvent, w.updateTickMachine)
	pubsub.Subscribe("AccountWorker", pubsub.NewOpenTradeRequest, w.handleNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteOpenTradeRequest, w.handleExecuteNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewCloseTradesRequest, w.handleNewCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteCloseTradesRequest, w.handleExecuteCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.FetchTradesRequest, w.handleFetchTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewGetStatsRequest, w.handleGetStatsRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewSignalsRequest, w.handleNewSignalRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ManualDatafeedUpdateRequest, w.handleManualDatafeedUpdateRequest)

	go func() {
		defer w.wg.Done()
		// todo: investigate why we had to increase from 500ms -> 5 seconds
		ticker := time.NewTicker(5 * time.Second)

		for {
			select {
			case <-ticker.C:
				w.update()
			case <-ctx.Done():
				log.Info("stopping AccountWorker consumer")
				return
			}
		}
	}()
}

func NewAccountWorkerClientFromFixtures(wg *sync.WaitGroup, accounts []*models.Account, datafeedName models.DatafeedName) *AccountWorker {
	coinbaseDatafeed := models.NewDatafeed(models.CoinbaseDatafeed)
	manualDatafeed := models.NewDatafeed(models.ManualDatafeed)

	switch datafeedName {
	case models.CoinbaseDatafeed:
		for _, acc := range accounts {
			acc.Datafeed = coinbaseDatafeed
		}
	case models.ManualDatafeed:
		for _, acc := range accounts {
			acc.Datafeed = manualDatafeed
		}
	default:
		log.Fatalf("unknown datafeedName: %v", datafeedName)
	}

	return &AccountWorker{
		wg:               wg,
		accounts:         accounts,
		coinbaseDatafeed: coinbaseDatafeed,
		manualDatafeed:   manualDatafeed,
	}
}

func NewAccountWorkerClient(wg *sync.WaitGroup) *AccountWorker {
	return &AccountWorker{
		wg:               wg,
		accounts:         make([]*models.Account, 0),
		coinbaseDatafeed: models.NewDatafeed(models.CoinbaseDatafeed),
		manualDatafeed:   models.NewDatafeed(models.ManualDatafeed),
	}
}
