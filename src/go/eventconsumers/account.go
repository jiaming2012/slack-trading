package eventconsumers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	pubsub "github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
)

type AccountWorker struct {
	wg               *sync.WaitGroup
	accounts         []*models.Account
	coinbaseDatafeed *models.Datafeed
	ibDatafeed       *models.Datafeed
	manualDatafeed   *models.Datafeed
}

func (w *AccountWorker) getAccounts() []*models.Account {
	accounts := []*models.Account{}

	accounts = append(accounts, w.accounts...)

	return accounts
}

// todo: add a mutex
func (w *AccountWorker) addAccountWithoutStrategy(account *models.Account) error {

	for _, acc := range w.accounts {
		if acc.Name == account.Name {
			return fmt.Errorf("AccountWorker.addAccountWithoutStrategy: account with name %v already exists", account.Name)
		}
	}

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

func (w *AccountWorker) createAccountRequestHandler(request *models.CreateAccountRequestEventV1, env string) {
	log.Debug("<- AccountWorker.createAccountRequestHandler")

	var datafeed *models.Datafeed
	switch request.DatafeedName {
	case models.CoinbaseDatafeed:
		datafeed = w.coinbaseDatafeed
	case models.IBDatafeed:
		datafeed = w.ibDatafeed
	case models.ManualDatafeed:
		datafeed = w.manualDatafeed
	default:
		pubsub.PublishRequestError("AccountWorker.createAccountRequestHandler", fmt.Errorf("datafeed source: %v", request.DatafeedName), &request.Meta)
		return
	}

	account, err := models.NewAccount(request.Name, request.Balance, datafeed, env)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountRequestHandler", err, &request.Meta)
		return
	}

	err = w.addAccountWithoutStrategy(account)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.addAccountWithoutStrategy", err, &request.Meta)
		return
	}

	pubsub.PublishCompletedResponse("AccountWorker.createAccountRequestHandler", &models.CreateAccountResponseEvent{
		Account: account,
	}, &request.Meta)
}

func (w *AccountWorker) handleGetAccountsRequestEvent(request *models.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.PublishCompletedResponse("AccountWorker", &models.GetAccountsResponseEvent{
		Accounts: w.getAccounts(),
	}, &request.Meta)
}

// todo: test this
func (w *AccountWorker) checkTradeCloseParameters() ([]*models.CloseTradesRequest, []*models.CloseTradeRequestV2, error) {
	var closeTradesRequests []*models.CloseTradesRequest
	var closeTradeRequests []*models.CloseTradeRequestV2

	for _, account := range w.accounts {
		tick := account.Datafeed.Tick()
		stopOutRequests, err := account.CheckStopOut(*tick)
		if err != nil {
			return nil, nil, fmt.Errorf("checkStopOut failed: %w", err)
		}

		stopLossRequests, err := account.CheckStopLoss(*tick)
		if err != nil {
			return nil, nil, fmt.Errorf("CheckStopLoss failed: %w", err)
		}

		closeTradesRequests = append(closeTradesRequests, stopOutRequests...)
		closeTradeRequests = append(closeTradeRequests, stopLossRequests...)
	}

	return closeTradesRequests, closeTradeRequests, nil
}

func (w *AccountWorker) updateTickMachine(tick *models.Tick) {
	// todo: eventually update based off level 2 quotes to get bid and ask
	t := models.Tick{
		Timestamp: tick.Timestamp,
		Price:     tick.Price,
	}

	switch tick.Source {
	case models.CoinbaseDatafeed:
		w.coinbaseDatafeed.Update(t)
	case models.IBDatafeed:
		w.ibDatafeed.Update(t)
	// todo: current updates in different section of code. Should be refactored to update in the same place
	// case models.ManualDatafeed:
	// 	w.manualDatafeed.Update(t)
	default:
		log.Fatalf("unknown datafeed source: %v", tick.Source)
	}
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
		models.RegisterResultCallback(requestID)

		pubsub.PublishResponse("AccountWorker.update", models.ExecuteCloseTradesRequestEventName, &models.ExecuteCloseTradesRequest{
			CloseTradesRequest: req,
		}, &req.Meta)
	}

	for _, req := range closeTradeRequests {
		requestID := uuid.New()

		// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
		models.RegisterResultCallback(requestID)

		pubsub.PublishResponse("AccountWorker.update", models.ExecuteCloseTradeRequestEventName, &models.ExecuteCloseTradeRequest{
			Timeframe: req.Timeframe,
			Trade:     req.Trade,
			Percent:   req.Percent,
		}, &req.Meta)
	}
}

// todo: make this the model: NewCloseTradeRequest -> ExecuteCloseTradesRequest
func (w *AccountWorker) handleCloseTradesRequest(event *models.CloseTradeRequest) {
	log.Debug("<- AccountWorker.handleNewCloseTradeRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleNewCloseTradeRequest", err, &event.Meta)
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleNewCloseTradeRequest", err, &event.Meta)
		return
	}

	closeTradesRequest, err := models.NewCloseTradesRequest(strategy, event.Timeframe, event.PriceLevelIndex, event.Percent, strategy.Name)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleNewCloseTradeRequest", err, &event.Meta)
		return
	}

	pubsub.PublishCompletedResponse("AccountWorker.handleCloseTradeRequest", &models.ExecuteCloseTradesRequest{
		CloseTradesRequest: closeTradesRequest,
	}, &event.Meta)
}

func (w *AccountWorker) executeCloseTradesRequest(req *models.CloseTradeRequest) (*models.AutoExecuteTrade, error) {
	tradeID := uuid.New()
	now := time.Now().UTC()

	account, err := w.findAccount(req.AccountName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.executeCloseTradesRequest: %w", err)
	}

	strategy, err := account.FindStrategy(req.StrategyName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.executeCloseTradesRequest: %w", err)
	}

	datafeed := strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	trade, _, err := strategy.NewCloseTrades(tradeID, req.Timeframe, now, requestPrc, req.PriceLevelIndex, req.Percent)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateCloseTrade) {
			log.Debugf("duplicate close: skipping")
			return nil, nil
		}

		return nil, fmt.Errorf("executeCloseTradesRequest: unable to create new close trade: %w", err)
	}

	if _, err = strategy.AutoExecuteTrade(trade); err != nil {
		return nil, fmt.Errorf("AccountWorker.executeCloseTradesRequest: %w", err)
	}

	return &models.AutoExecuteTrade{
		Trade: trade,
	}, nil
}

func (w *AccountWorker) executeCloseTradeRequest(req *models.ExecuteCloseTradeRequest) (*models.AutoExecuteTrade, error) {
	return nil, fmt.Errorf("AccountWorker.executeCloseTradeRequest: not implemented")
}

func (w *AccountWorker) handleExecuteCloseTradeRequest(event *models.ExecuteCloseTradeRequest) {
	trade, err := w.executeCloseTradeRequest(event)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleExecuteCloseTradeRequest", err, &event.Meta)
		return
	}

	pubsub.PublishCompletedResponse("AccountWorker.handleExecuteCloseTradeRequest", trade, &event.Meta)
}

func (w *AccountWorker) handleExecuteCloseTradesRequest(event *models.ExecuteCloseTradesRequest) {
	log.Debug("<- AccountWorker.handleExecuteCloseTradesRequest")

	clsTradeReq := event.CloseTradesRequest
	tradeID := uuid.New()
	now := time.Now().UTC()
	datafeed := event.CloseTradesRequest.Strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	// todo: unify models: partialCloseRequestItems. Strategy.AutoExecuteTrade handles differently than trade.AutoExecuteTrade
	trade, _, err := clsTradeReq.Strategy.NewCloseTrades(tradeID, clsTradeReq.Timeframe, now, requestPrc, clsTradeReq.PriceLevelIndex, clsTradeReq.Percent)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleExecuteCloseTradesRequest", fmt.Errorf("unable to create new close trade: %w", err), &event.Meta)
		return
	}

	pubsub.PublishCompletedResponse("AccountWorker.handleExecuteCloseTradesRequest", &models.AutoExecuteTrade{
		Trade: trade,
	}, &event.Meta)
}

func (w *AccountWorker) handleAutoExecuteTrade(event *models.AutoExecuteTrade) {
	strategy := event.Trade.PriceLevel.Strategy
	_, err := strategy.AutoExecuteTrade(event.Trade)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleExecuteCloseTradesRequest", err, &event.Meta)
		return
	}

	executeCloseTradesResult := &models.ExecuteCloseTradesResult{
		Trade: event.Trade,
	}

	pubsub.PublishResponse("AccountWorker.handleExecuteCloseTradesRequest", models.ExecuteCloseTradesResultEventName, executeCloseTradesResult, &event.Meta)
}

func (w *AccountWorker) executeOpenTradeRequest(req *models.CreateTradeRequest) (*models.ExecuteOpenTradeResult, error) {
	tradeID := uuid.New()
	now := time.Now().UTC()

	account, err := w.findAccount(req.AccountName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.handleNewOpenTradeRequest: %w", err)
	}

	strategy, err := account.FindStrategy(req.StrategyName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.handleNewOpenTradeRequest: %w", err)
	}

	datafeed := strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	trade, _, err := strategy.NewOpenTrade(tradeID, req.Timeframe, now, requestPrc)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.handleNewOpenTradeRequest: %w", err)
	}

	result, err := strategy.AutoExecuteTrade(trade)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.handleNewOpenTradeRequest: %w", err)
	}

	return &models.ExecuteOpenTradeResult{
		PriceLevelIndex: result.PriceLevelIndex,
		Trade:           trade,
	}, nil
}

func (w *AccountWorker) handleExecuteOpenTradeRequest(event *models.ExecuteOpenTradeRequest) {
	log.Debug("<- AccountWorker.handleExecuteNewOpenTradeRequest")

	executeOpenTradeResult, err := w.executeOpenTradeRequest(event.OpenTradeRequest)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleExecuteNewOpenTradeRequest", fmt.Errorf("failed to execute open trade request: %w", err), &event.Meta)
		return
	}

	// terminates process
	pubsub.PublishCompletedResponse("AccountWorker.handleExecuteNewOpenTradeRequest", executeOpenTradeResult, &event.Meta)
}

// todo: remove isOpen
func (w *AccountWorker) processCreateTradeRequest(event *models.CreateTradeRequest, isOpen bool) {
	// perform a lookup to find the trade, or create an execute trade request

	if isOpen {
		w.executeOpenTradeRequest(event)
	} else {
		// w.executeCloseTradeRequest()
	}
}

func (w *AccountWorker) handleCreateTradeRequest(event models.CreateTradeRequest) {
	log.Debug("<- AccountWorker.handleCreateTradeRequest")

	// todo: refactor - can i remove this??

	// account, err := w.findAccount(event.AccountName)
	// if err != nil {
	// 	requestErr := models.NewRequestError(event.RequestID, fmt.Errorf("failed to find findAccount: %w", err))
	// 	pubsub.PublishRequestError("AccountWorker.handleCreateTradeRequest", &event, requestErr)
	// 	return
	// }

	// strategy, err := account.FindStrategy(event.StrategyName)
	// if err != nil {
	// 	requestErr := models.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
	// 	pubsub.PublishRequestError("AccountWorker.handleCreateTradeRequest", &event, requestErr)
	// 	return
	// }

	// todo: refactor - if another method already has the strategy, e.g. - eventservices.UpdateConditions, could
	// it just invoke execute open trade request directly?
	// Furthermore, is there a difference between a request originating from outside of the system - e.g. NewOpenTradeRequest
	// and inside of the system - e.g. ExecuteOpenTradeRequest
	// openTradeReq, err := models.NewOpenTradeRequest(
	// 	event.Timeframe,
	// 	strategy,
	// )

	pubsub.PublishResponse("AccountWorker.handleCreateTradeRequest", models.ExecuteOpenTradeRequestEventName, &models.ExecuteOpenTradeRequest{
		OpenTradeRequest: &event,
	}, &event.Meta)
}

// todo:: this is the model! Refactor services to be standardized. Ideally in its own directory of sorts
// todo: TEST THIS !!! And reproduce the issue of closing 50% of one trade in postman
func (w *AccountWorker) fetchTrades(event *models.FetchTradesRequest) (*models.FetchTradesResult, error) {
	account, err := w.findAccount(event.AccountName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.fetchTradesRequest: failed to find findAccount: %w", err)
	}

	if &event.Meta == nil {
		return nil, fmt.Errorf("AccountWorker.fetchTradesRequest: meta is nil")
	}

	fetchTradesResult := eventservices.FetchTrades(event.Meta.RequestID, account)

	return fetchTradesResult, nil
}

func (w *AccountWorker) handleFetchTradesRequest(event *models.FetchTradesRequest) {
	log.Debug("<- AccountWorker.handleFetchTradesRequest")

	fetchTradesResult, err := w.fetchTrades(event)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleFetchTradesRequest", err, &event.Meta)
		return
	}

	pubsub.PublishResponse("AccountWorker.handleFetchTradesRequest", models.FetchTradesResultEventName, fetchTradesResult, &event.Meta)
}

func (w *AccountWorker) handleGetAccountStatsRequest(event *models.GetStatsRequest) {
	log.Debug("<- AccountWorker.handleGetAccountStatsRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetAccountStatsRequest", fmt.Errorf("failed to find findAccount: %w", err), &event.Meta)
		return
	}

	currentTick := account.Datafeed.Tick()

	statsResult, err := eventservices.GetStats(event.GetMetaData().RequestID, account, currentTick)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetAccountStatsRequest", err, &event.Meta)
		return
	}

	pubsub.PublishCompletedResponse("AccountWorker.handleGetAccountStatsRequest", statsResult, &event.Meta)
}

func (w *AccountWorker) handleExitConditionsSatisfied(exitConditionsSatisfied []*models.ExitConditionsSatisfied) ([]*models.CloseTradeRequest, error) {
	var clsTradeRequests []*models.CloseTradeRequest

	for _, exitCondition := range exitConditionsSatisfied {
		// todo: should be able to only pass the price level to the request
		priceLevel := exitCondition.PriceLevel
		strategy := priceLevel.Strategy
		account := strategy.Account

		req, closeTradeReqErr := models.NewCloseTradeRequest(uuid.New(), account.Name, strategy.Name, exitCondition.PriceLevelIndex, nil, float64(exitCondition.PercentClose), exitCondition.Reason)
		if closeTradeReqErr != nil {
			return nil, closeTradeReqErr
		}

		clsTradeRequests = append(clsTradeRequests, req)
	}

	return clsTradeRequests, nil
}

func (w *AccountWorker) handleEntryConditionsSatisfied(entryConditionsSatisfied []*models.EntryConditionsSatisfied) ([]*models.CreateTradeRequest, error) {
	var openTradeRequests []*models.CreateTradeRequest

	for _, entryConditions := range entryConditionsSatisfied {
		req, openTradeReqErr := models.NewOpenTradeRequest(uuid.New(), entryConditions.Account.Name, entryConditions.Strategy.Name, nil) // todo: timeframe should come from signal
		if openTradeReqErr != nil {
			return nil, openTradeReqErr
		}

		openTradeRequests = append(openTradeRequests, req)
	}

	return openTradeRequests, nil
}

func (w *AccountWorker) handleExitConditions(event *models.CreateSignalRequestEventV1DTO) error {
	exitConditionsSatisfied, updateErr := eventservices.UpdateExitConditions(w.getAccounts(), event)
	if updateErr != nil {
		return fmt.Errorf("AccountWorker.handleExitConditions: failed to update exit conditions: %w", updateErr)
	}

	clsTradeRequests, err := w.handleExitConditionsSatisfied(exitConditionsSatisfied)
	if err != nil {
		return fmt.Errorf("AccountWorker.handleExitConditions: failed to handle exit conditions: %w", err)
	}

	for _, req := range clsTradeRequests {
		// todo: return the close trade requests
		_, err := w.executeCloseTradesRequest(req)
		if err != nil {
			return fmt.Errorf("AccountWorker.handleExitConditions: failed to execute close trade request: %w", err)
		}
	}

	return nil
}

func (w *AccountWorker) handleOpenConditions(event *models.CreateSignalRequestEventV1DTO) error {
	entryConditionsSatisfied := eventservices.UpdateEntryConditions(w.getAccounts(), event)
	openTradeRequests, err := w.handleEntryConditionsSatisfied(entryConditionsSatisfied)
	if err != nil {
		return fmt.Errorf("AccountWorker.handleOpenConditions: failed to handle entry conditions: %w", err)
	}

	for _, req := range openTradeRequests {
		// todo: return the open trade requests
		_, err := w.executeOpenTradeRequest(req)
		if err != nil {
			return fmt.Errorf("AccountWorker.handleOpenConditions: failed to execute open trade request: %w", err)
		}
	}

	return nil
}

func (w *AccountWorker) handleCreateSignalResponse(event *models.CreateSignalRequestEventV1DTO) error {
	log.Infof("received %v", event)

	if err := w.handleExitConditions(event); err != nil {
		return fmt.Errorf("AccountWorker.handleCreateSignalResponse: failed to handle exit conditions: %w", err)
	}

	if err := w.handleOpenConditions(event); err != nil {
		return fmt.Errorf("AccountWorker.handleCreateSignalResponse: failed to handle open conditions: %w", err)
	}

	// todo: publish any newly created trades

	return nil
}

func (w *AccountWorker) handleCreateSignalRequest(event *models.CreateSignalRequestEventV1DTO) {
	log.Infof("received %v", event)

	if err := w.handleCreateSignalResponse(event); err != nil {
		pubsub.PublishRequestError("AccountWorker.handleCreateSignalRequest", fmt.Errorf("failed to handle signal request: %w", err), &event.Meta)
		return
	}

	pubsub.PublishEvent("AccountWorker.handleCreateSignalRequest", models.CreateSignalRequestProcessedByAccount, nil)
}

func (w *AccountWorker) handleManualDatafeedUpdateRequest(ev *models.ManualDatafeedUpdateRequest) {
	ts := time.Now().UTC()
	tick := models.Tick{
		Timestamp: ts,
		Price:     ev.Bid,
	}

	w.manualDatafeed.Update(tick)

	result := models.NewManualDatafeedUpdateResult(ev.Meta.RequestID, ts, tick)

	pubsub.PublishResponse("AccountWorker.handleManualDatafeedUpdateRequest", models.ManualDatafeedUpdateResultEventName, result, &ev.Meta)
}

func (w *AccountWorker) createAccountStrategyRequestHandler(ev *models.CreateAccountStrategyRequestEvent) {
	account, err := w.findAccount(ev.AccountName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to find account: %w", err), &ev.Meta)
		return
	}

	strategy, err := models.NewStrategy(ev.Strategy.Name, ev.Strategy.Symbol, ev.Strategy.Direction, ev.Strategy.Balance, ev.Strategy.EntryConditions, ev.Strategy.ExitConditions, ev.Strategy.PriceLevels, account)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to create strategy: %w", err), &ev.Meta)
		return
	}

	if err := account.AddStrategy(strategy); err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to add strategy: %w", err), &ev.Meta)
		return
	}

	// todo: add the requestID as a parameter when dispatching to the event bus, instead of in the event itself
	pubsub.PublishCompletedResponse("AccountWorker.createAccountStrategyRequestHandler", &models.CreateAccountStrategyResponseEvent{
		AccountsRequestHeader: models.AccountsRequestHeader{
			AccountName: ev.AccountName,
		},
		Strategy: strategy,
	}, &ev.Meta)
}

func (w *AccountWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	// task: *** create an AccountManager to hold each account worker and subscribe to events

	// pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, w.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", models.GetAccountsRequestEventName, w.handleGetAccountsRequestEvent)
	pubsub.Subscribe("AccountWorker", models.NewTickEventName, w.updateTickMachine)
	pubsub.Subscribe("AccountWorker", models.CreateTradeRequestEventName, w.handleCreateTradeRequest)
	pubsub.Subscribe("AccountWorker", models.ExecuteOpenTradeRequestEventName, w.handleExecuteOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", models.CloseTradesRequestEventName, w.handleCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", models.ExecuteCloseTradesRequestEventName, w.handleExecuteCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", models.ExecuteCloseTradeRequestEventName, w.handleExecuteCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", models.FetchTradesRequestEventName, w.handleFetchTradesRequest)
	pubsub.Subscribe("AccountWorker", models.NewGetStatsRequestEventName, w.handleGetAccountStatsRequest)
	pubsub.Subscribe("AccountWorker", models.NewSavedEvent(models.CreateSignalRequestEventName), w.handleCreateSignalRequest)
	pubsub.Subscribe("AccountWorker", models.ManualDatafeedUpdateRequestEventName, w.handleManualDatafeedUpdateRequest)
	pubsub.Subscribe("AccountWorker", models.AutoExecuteTradeEventName, w.handleAutoExecuteTrade)
	pubsub.Subscribe("AccountWorker", models.NewSavedEvent(models.CreateAccountStrategyRequestEventName), w.createAccountStrategyRequestHandler)
	pubsub.Subscribe("AccountWorker", models.NewSavedEvent(models.CreateAccountRequestEventName), w.createAccountRequestHandler)

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

func NewAccountWorkerClient(wg *sync.WaitGroup) *AccountWorker {
	return &AccountWorker{
		wg:               wg,
		accounts:         make([]*models.Account, 0),
		coinbaseDatafeed: models.NewDatafeed(models.CoinbaseDatafeed),
		ibDatafeed:       models.NewDatafeed(models.IBDatafeed),
		manualDatafeed:   models.NewDatafeed(models.ManualDatafeed),
	}
}
