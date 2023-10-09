package eventconsumers

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"os"
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

func (w *AccountWorker) monitorTrades() {

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

func (w *AccountWorker) getAccountsRequestHandler(request *eventmodels.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.Publish("AccountWorker", pubsub.GetAccountsResponseEvent, &eventmodels.GetAccountsResponseEvent{
		RequestID: request.RequestID,
		Accounts:  w.getAccounts(),
	})
}

// todo: test this
func (w *AccountWorker) checkTradeCloseParameters() ([]*models.CloseTradesRequest, []*models.CloseTradeRequestV2, error) {
	var closeTradesRequests []*models.CloseTradesRequest
	var closeTradeRequests []*models.CloseTradeRequestV2

	for _, account := range w.accounts {
		tick := account.Datafeed.Tick()
		stopOutRequests, err := account.CheckStopOut(*tick)
		stopLossRequests, err := account.CheckStopLoss(*tick)

		if err != nil {
			return nil, nil, fmt.Errorf("checkStopOut failed: %w", err)
		}

		closeTradesRequests = append(closeTradesRequests, stopOutRequests...)
		closeTradeRequests = append(closeTradeRequests, stopLossRequests...)
	}

	return closeTradesRequests, closeTradeRequests, nil
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

	// todo: current timing out. might need to implement caching
	closeTradesRequests, closeTradeRequests, err := w.checkTradeCloseParameters()
	if err != nil {
		log.Errorf("AccountWorker.update: check for stop out failed: %v", err)
		return
	}

	for _, req := range closeTradesRequests {
		requestID := uuid.New()

		// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
		eventmodels.RegisterResultCallback(requestID)

		pubsub.Publish("AccountWorker.update", pubsub.ExecuteCloseTradesRequest, eventmodels.ExecuteCloseTradesRequest{
			RequestID:          uuid.New(),
			CloseTradesRequest: req,
		})
	}

	for _, req := range closeTradeRequests {
		requestID := uuid.New()

		// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
		eventmodels.RegisterResultCallback(requestID)

		pubsub.Publish("AccountWorker.update", pubsub.ExecuteCloseTradeRequest, &eventmodels.ExecuteCloseTradeRequest{
			RequestID: requestID,
			Timeframe: req.Timeframe,
			Trade:     req.Trade,
			Percent:   req.Percent,
		})
	}
}

// todo: make this the model: NewCloseTradeRequest -> ExecuteCloseTradesRequest
func (w *AccountWorker) handleCloseTradesRequest(event *eventmodels.CloseTradeRequest) {
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

	closeTradesRequest, err := models.NewCloseTradesRequest(strategy, event.Timeframe, event.PriceLevelIndex, event.Percent, strategy.Name)
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

func (w *AccountWorker) handleExecuteCloseTradeRequest(event *eventmodels.ExecuteCloseTradeRequest) {
	tradeID := uuid.New()
	now := time.Now()

	strategy := event.Trade.PriceLevel.Strategy
	datafeed := strategy.Account.Datafeed

	var requestPrc float64
	if strategy.Direction == models.Up {
		requestPrc = datafeed.LastBid
	} else if strategy.Direction == models.Down {
		requestPrc = datafeed.LastOffer
	}

	trade, _, err := strategy.NewCloseTrade(tradeID, event.Timeframe, now, requestPrc, event.Percent, event.Trade)
	if err != nil {
		if errors.Is(err, models.DuplicateCloseTradeErr) {
			log.Debugf("duplicate close: skip closing %v", event.Trade.ID)
			return
		}

		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new close trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	pubsub.Publish("AccountWorker.handleExecuteCloseTradesRequest", pubsub.AutoExecuteTrade, &eventmodels.AutoExecuteTrade{
		RequestID: event.RequestID,
		Trade:     trade,
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

	// todo: unify models: partialCloseRequestItems. Strategy.AutoExecuteTrade handles differently than trade.AutoExecuteTrade
	trade, _, err := clsTradeReq.Strategy.NewCloseTrades(tradeID, clsTradeReq.Timeframe, now, requestPrc, clsTradeReq.PriceLevelIndex, clsTradeReq.Percent)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new close trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	pubsub.Publish("AccountWorker.handleExecuteCloseTradesRequest", pubsub.AutoExecuteTrade, &eventmodels.AutoExecuteTrade{
		RequestID: event.RequestID,
		Trade:     trade,
	})
}

func (w *AccountWorker) handleAutoExecuteTrade(event *eventmodels.AutoExecuteTrade) {
	strategy := event.Trade.PriceLevel.Strategy
	result, err := strategy.AutoExecuteTrade(event.Trade)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to place execute trade: %w", err))
		pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	executeCloseTradesResult := &eventmodels.ExecuteCloseTradesResult{
		RequestID: event.RequestID,
		Side:      strategy.GetTradeType(true).String(),
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

	trade, _, err := req.Strategy.NewOpenTrade(tradeID, req.Timeframe, now, requestPrc)
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

	currentTick := account.Datafeed.Tick()

	statsResult, err := eventservices.GetStats(event.RequestID, account, currentTick)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetStatsRequest", event, err)
		return
	}

	pubsub.Publish("AccountWorker.handleGetStatsRequest", pubsub.GetStatsResult, statsResult)
}

func (w *AccountWorker) handleExitConditionsSatisfied(exitConditionsSatisfied []*models.ExitConditionsSatisfied) ([]*eventmodels.CloseTradeRequest, error) {
	var clsTradeRequests []*eventmodels.CloseTradeRequest

	for _, exitCondition := range exitConditionsSatisfied {
		// todo: should be able to only pass the price level to the request
		priceLevel := exitCondition.PriceLevel
		strategy := priceLevel.Strategy
		account := strategy.Account

		req, closeTradeReqErr := eventmodels.NewCloseTradeRequest(uuid.New(), account.Name, strategy.Name, exitCondition.PriceLevelIndex, nil, float64(exitCondition.PercentClose), exitCondition.Reason)
		if closeTradeReqErr != nil {
			return nil, closeTradeReqErr
		}

		clsTradeRequests = append(clsTradeRequests, req)
	}

	return clsTradeRequests, nil
}

func (w *AccountWorker) handleEntryConditionsSatisfied(entryConditionsSatisfied []*models.EntryConditionsSatisfied) ([]*eventmodels.OpenTradeRequest, error) {
	var openTradeRequests []*eventmodels.OpenTradeRequest

	for _, entryConditions := range entryConditionsSatisfied {
		req, openTradeReqErr := eventmodels.NewOpenTradeRequest(uuid.New(), entryConditions.Account.Name, entryConditions.Strategy.Name, nil) // todo: timeframe should come from signal
		if openTradeReqErr != nil {
			return nil, openTradeReqErr
		}

		openTradeRequests = append(openTradeRequests, req)
	}

	return openTradeRequests, nil
}

func (w *AccountWorker) handleNewSignalRequest(event *models.SignalRequest) {
	// handle exit conditions
	exitConditionsSatisfied, updateErr := eventservices.UpdateExitConditions(w.getAccounts(), event)
	if updateErr == nil {
		if exitConditionsSatisfied != nil {
			clsTradeRequests, err := w.handleExitConditionsSatisfied(exitConditionsSatisfied)
			if err == nil {
				for _, req := range clsTradeRequests {
					// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
					eventmodels.RegisterResultCallback(req.RequestID)

					pubsub.Publish("AccountWorker.handleNewSignalRequest", pubsub.CloseTradesRequest, req)
				}
			} else {
				pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
			}
		}
	} else {
		pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, updateErr)
	}

	if entryConditionsSatisfied := eventservices.UpdateEntryConditions(w.getAccounts(), event); entryConditionsSatisfied != nil {
		openTradeRequests, err := w.handleEntryConditionsSatisfied(entryConditionsSatisfied)
		if err == nil {
			for _, req := range openTradeRequests {
				// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove
				// as the request didn't originate from an api call but is still picked up by the lister
				eventmodels.RegisterResultCallback(req.RequestID)

				pubsub.Publish("AccountWorker.handleNewSignalRequest", pubsub.NewOpenTradeRequest, *req)
			}
		} else {
			pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
		}
	}

	// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove
	// as the request didn't originate from an api call but is still picked up by the lister
	eventmodels.RegisterResultCallback(event.RequestID)

	pubsub.Publish("AccountWorker.handleNewSignalRequest", pubsub.NewSignalsResult, &eventmodels.NewSignalResult{
		Name:      event.Name,
		RequestID: event.RequestID,
	})
}

func (w *AccountWorker) handleManualDatafeedUpdateRequest(ev *eventmodels.ManualDatafeedUpdateRequest) {
	ts := time.Now()
	tick := models.Tick{
		Timestamp: ts,
		Bid:       ev.Bid,
		Ask:       ev.Ask,
	}

	w.manualDatafeed.Update(tick)

	result := eventmodels.NewManualDatafeedUpdateResult(ev.RequestID, ts, tick)

	pubsub.Publish("AccountWorker.handleManualDatafeedUpdateRequest", pubsub.ManualDatafeedUpdateResult, result)
}

func (w *AccountWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, w.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.GetAccountsRequestEvent, w.getAccountsRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.NewTickEvent, w.updateTickMachine)
	pubsub.Subscribe("AccountWorker", pubsub.NewOpenTradeRequest, w.handleNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteOpenTradeRequest, w.handleExecuteNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.CloseTradesRequest, w.handleCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteCloseTradesRequest, w.handleExecuteCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteCloseTradeRequest, w.handleExecuteCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.FetchTradesRequest, w.handleFetchTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewGetStatsRequest, w.handleGetStatsRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewSignalsRequest, w.handleNewSignalRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ManualDatafeedUpdateRequest, w.handleManualDatafeedUpdateRequest)
	pubsub.Subscribe("AccountWorker", pubsub.AutoExecuteTrade, w.handleAutoExecuteTrade)

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
		if os.Getenv("ENV") == "PRODUCTION" {
			log.Fatalf("cannot use manual datafeed in production")
		}

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
