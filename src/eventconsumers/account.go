package eventconsumers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/eventservices"
)

type AccountWorker struct {
	wg               *sync.WaitGroup
	accounts         []*eventmodels.Account
	coinbaseDatafeed *eventmodels.Datafeed
	ibDatafeed       *eventmodels.Datafeed
	manualDatafeed   *eventmodels.Datafeed
}

func (w *AccountWorker) getAccounts() []*eventmodels.Account {
	accounts := []*eventmodels.Account{}

	accounts = append(accounts, w.accounts...)

	return accounts
}

// todo: add a mutex
func (w *AccountWorker) addAccountWithoutStrategy(account *eventmodels.Account) error {

	for _, acc := range w.accounts {
		if acc.Name == account.Name {
			return fmt.Errorf("AccountWorker.addAccountWithoutStrategy: account with name %v already exists", account.Name)
		}
	}

	w.accounts = append(w.accounts, account)

	return nil
}

func (w *AccountWorker) findAccount(name string) (*eventmodels.Account, error) {
	for _, a := range w.accounts {
		if name == a.Name {
			return a, nil
		}
	}

	return nil, fmt.Errorf("AccountWorker.findAccount: could not find account with name %v", name)
}

func (w *AccountWorker) createAccountRequestHandler(request *eventmodels.CreateAccountRequestEvent) {
	log.Debug("<- AccountWorker.createAccountRequestHandler")

	var datafeed *eventmodels.Datafeed
	switch request.DatafeedName {
	case eventmodels.CoinbaseDatafeed:
		datafeed = w.coinbaseDatafeed
	case eventmodels.IBDatafeed:
		datafeed = w.ibDatafeed
	case eventmodels.ManualDatafeed:
		datafeed = w.manualDatafeed
	default:
		pubsub.PublishTerminalError("AccountWorker.createAccountRequestHandler", fmt.Errorf("datafeed source: %v", request.DatafeedName), request.Meta)
		return
	}

	account, err := eventmodels.NewAccount(request.Name, request.Balance, datafeed)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.createAccountRequestHandler", err, request.Meta)
		return
	}

	err = w.addAccountWithoutStrategy(account)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.addAccountWithoutStrategy", err, request.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.createAccountRequestHandler", &eventmodels.CreateAccountResponseEvent{
		Account: account,
	}, request.Meta)
}

func (w *AccountWorker) handleGetAccountsRequestEvent(request *eventmodels.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.PublishResult3("AccountWorker", &eventmodels.GetAccountsResponseEvent{
		Accounts: w.getAccounts(),
	}, request.Meta)
}

// todo: test this
func (w *AccountWorker) checkTradeCloseParameters() ([]*eventmodels.CloseTradesRequest, []*eventmodels.CloseTradeRequestV2, error) {
	var closeTradesRequests []*eventmodels.CloseTradesRequest
	var closeTradeRequests []*eventmodels.CloseTradeRequestV2

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

func (w *AccountWorker) updateTickMachine(tick *eventmodels.Tick) {
	// todo: eventually update based off level 2 quotes to get bid and ask
	t := eventmodels.Tick{
		Timestamp: tick.Timestamp,
		Price:     tick.Price,
	}

	switch tick.Source {
	case eventmodels.CoinbaseDatafeed:
		w.coinbaseDatafeed.Update(t)
	case eventmodels.IBDatafeed:
		w.ibDatafeed.Update(t)
	// todo: current updates in different section of code. Should be refactored to update in the same place
	// case eventmodels.ManualDatafeed:
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
		eventmodels.RegisterResultCallback(requestID)

		pubsub.PublishResult4("AccountWorker.update", eventmodels.ExecuteCloseTradesRequestEventName, &eventmodels.ExecuteCloseTradesRequest{
			CloseTradesRequest: req,
		}, req.Meta)
	}

	for _, req := range closeTradeRequests {
		requestID := uuid.New()

		// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
		eventmodels.RegisterResultCallback(requestID)

		pubsub.PublishResult("AccountWorker.update", eventmodels.ExecuteCloseTradeRequestEventName, &eventmodels.ExecuteCloseTradeRequest{
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
		pubsub.PublishTerminalError("AccountWorker.handleNewCloseTradeRequest", err, event.Meta)
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleNewCloseTradeRequest", err, event.Meta)
		return
	}

	closeTradesRequest, err := eventmodels.NewCloseTradesRequest(strategy, event.Timeframe, event.PriceLevelIndex, event.Percent, strategy.Name)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleNewCloseTradeRequest", err, event.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.handleCloseTradeRequest", &eventmodels.ExecuteCloseTradesRequest{
		CloseTradesRequest: closeTradesRequest,
	}, event.Meta)
}

func (w *AccountWorker) executeCloseTradeRequest(event *eventmodels.ExecuteCloseTradeRequest) (*eventmodels.AutoExecuteTrade, error) {
	tradeID := uuid.New()
	now := time.Now().UTC()

	strategy := event.Trade.PriceLevel.Strategy
	datafeed := strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	trade, _, err := strategy.NewCloseTrade(tradeID, event.Timeframe, now, requestPrc, event.Percent, event.Trade)
	if err != nil {
		if errors.Is(err, eventmodels.DuplicateCloseTradeErr) {
			log.Debugf("duplicate close: skip closing %v", event.Trade.ID)
			return nil, nil
		}

		return nil, fmt.Errorf("unable to create new close trade: %w", err)
	}

	return &eventmodels.AutoExecuteTrade{
		Trade: trade,
	}, nil
}

func (w *AccountWorker) handleExecuteCloseTradeRequest(event *eventmodels.ExecuteCloseTradeRequest) {
	trade, err := w.executeCloseTradeRequest(event)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleExecuteCloseTradeRequest", err, event.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.handleExecuteCloseTradeRequest", trade, event.Meta)
}

func (w *AccountWorker) handleExecuteCloseTradesRequest(event *eventmodels.ExecuteCloseTradesRequest) {
	log.Debug("<- AccountWorker.handleExecuteCloseTradesRequest")

	clsTradeReq := event.CloseTradesRequest
	tradeID := uuid.New()
	now := time.Now().UTC()
	datafeed := event.CloseTradesRequest.Strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	// todo: unify models: partialCloseRequestItems. Strategy.AutoExecuteTrade handles differently than trade.AutoExecuteTrade
	trade, _, err := clsTradeReq.Strategy.NewCloseTrades(tradeID, clsTradeReq.Timeframe, now, requestPrc, clsTradeReq.PriceLevelIndex, clsTradeReq.Percent)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleExecuteCloseTradesRequest", fmt.Errorf("unable to create new close trade: %w", err), event.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.handleExecuteCloseTradesRequest", &eventmodels.AutoExecuteTrade{
		Trade: trade,
	}, event.Meta)
}

func (w *AccountWorker) handleAutoExecuteTrade(event *eventmodels.AutoExecuteTrade) {
	strategy := event.Trade.PriceLevel.Strategy
	_, err := strategy.AutoExecuteTrade(event.Trade)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleExecuteCloseTradesRequest", err, event.Meta)
		return
	}

	executeCloseTradesResult := &eventmodels.ExecuteCloseTradesResult{
		Trade: event.Trade,
	}

	pubsub.PublishResult("AccountWorker.handleExecuteCloseTradesRequest", eventmodels.ExecuteCloseTradesResultEventName, executeCloseTradesResult)
}

func (w *AccountWorker) executeOpenTradeRequest(req *eventmodels.CreateTradeRequest) (*eventmodels.ExecuteOpenTradeResult, error) {
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

	return &eventmodels.ExecuteOpenTradeResult{
		PriceLevelIndex: result.PriceLevelIndex,
		Trade:           trade,
	}, nil
}

func (w *AccountWorker) handleExecuteOpenTradeRequest(event *eventmodels.ExecuteOpenTradeRequest) {
	log.Debug("<- AccountWorker.handleExecuteNewOpenTradeRequest")

	executeOpenTradeResult, err := w.executeOpenTradeRequest(event.OpenTradeRequest)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleExecuteNewOpenTradeRequest", fmt.Errorf("failed to execute open trade request: %w", err), event.Meta)
		return
	}

	// terminates process
	pubsub.PublishResult3("AccountWorker.handleExecuteNewOpenTradeRequest", executeOpenTradeResult, event.Meta)
}

// todo: remove isOpen
func (w *AccountWorker) processCreateTradeRequest(event *eventmodels.CreateTradeRequest, isOpen bool) {
	// perform a lookup to find the trade, or create an execute trade request

	if isOpen {
		w.executeOpenTradeRequest(event)
	} else {
		// w.executeCloseTradeRequest()
	}
}

func (w *AccountWorker) handleCreateTradeRequest(event eventmodels.CreateTradeRequest) {
	log.Debug("<- AccountWorker.handleCreateTradeRequest")

	// todo: refactor - can i remove this??

	// account, err := w.findAccount(event.AccountName)
	// if err != nil {
	// 	requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find findAccount: %w", err))
	// 	pubsub.PublishRequestError("AccountWorker.handleCreateTradeRequest", &event, requestErr)
	// 	return
	// }

	// strategy, err := account.FindStrategy(event.StrategyName)
	// if err != nil {
	// 	requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
	// 	pubsub.PublishRequestError("AccountWorker.handleCreateTradeRequest", &event, requestErr)
	// 	return
	// }

	// todo: refactor - if another method already has the strategy, e.g. - eventservices.UpdateConditions, could
	// it just invoke execute open trade request directly?
	// Furthermore, is there a difference between a request originating from outside of the system - e.g. NewOpenTradeRequest
	// and inside of the system - e.g. ExecuteOpenTradeRequest
	// openTradeReq, err := eventmodels.NewOpenTradeRequest(
	// 	event.Timeframe,
	// 	strategy,
	// )

	pubsub.PublishResult4("AccountWorker.handleCreateTradeRequest", eventmodels.ExecuteOpenTradeRequestEventName, &eventmodels.ExecuteOpenTradeRequest{
		OpenTradeRequest: &event,
	}, event.Meta)
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

	pubsub.PublishResult("AccountWorker.handleFetchTradesRequest", eventmodels.FetchTradesResultEventName, fetchTradesResult)
}

func (w *AccountWorker) handleGetAccountStatsRequest(event *eventmodels.GetStatsRequest) {
	log.Debug("<- AccountWorker.handleGetAccountStatsRequest")

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleGetAccountStatsRequest", fmt.Errorf("failed to find findAccount: %w", err), event.Meta)
		return
	}

	currentTick := account.Datafeed.Tick()

	statsResult, err := eventservices.GetStats(event.GetMetaData().RequestID, account, currentTick)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleGetAccountStatsRequest", err, event.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.handleGetAccountStatsRequest", statsResult, event.Meta)
}

func (w *AccountWorker) handleExitConditionsSatisfied(exitConditionsSatisfied []*eventmodels.ExitConditionsSatisfied) ([]*eventmodels.CloseTradeRequest, error) {
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

func (w *AccountWorker) handleEntryConditionsSatisfied(entryConditionsSatisfied []*eventmodels.EntryConditionsSatisfied) ([]*eventmodels.CreateTradeRequest, error) {
	var openTradeRequests []*eventmodels.CreateTradeRequest

	for _, entryConditions := range entryConditionsSatisfied {
		req, openTradeReqErr := eventmodels.NewOpenTradeRequest(uuid.New(), entryConditions.Account.Name, entryConditions.Strategy.Name, nil) // todo: timeframe should come from signal
		if openTradeReqErr != nil {
			return nil, openTradeReqErr
		}

		openTradeRequests = append(openTradeRequests, req)
	}

	return openTradeRequests, nil
}

func (w *AccountWorker) handleExitConditions(event *eventmodels.CreateSignalRequestEvent) error {
	exitConditionsSatisfied, updateErr := eventservices.UpdateExitConditions(w.getAccounts(), event)
	if updateErr != nil {
		return fmt.Errorf("AccountWorker.handleExitConditions: failed to update exit conditions: %w", updateErr)
	}

	clsTradeRequests, err := w.handleExitConditionsSatisfied(exitConditionsSatisfied)
	if err != nil {
		return fmt.Errorf("AccountWorker.handleExitConditions: failed to handle exit conditions: %w", err)
	}

	for _, req := range clsTradeRequests {
		reqErrCh := req.Wait()

		pubsub.PublishEventResult("AccountWorker.handleExitConditions", eventmodels.CloseTradesRequestEventName, req)

		err := <-reqErrCh

		if err != nil {
			return fmt.Errorf("AccountWorker.handleExitConditions: failed to create trade: %w", err)
		}
	}

	return nil
}

func (w *AccountWorker) handleOpenConditions(event *eventmodels.CreateSignalRequestEvent) error {
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

func (w *AccountWorker) handleCreateSignalResponse(event *eventmodels.CreateSignalRequestEvent) (*eventmodels.CreateSignalResponseEvent, error) {
	log.Infof("received %v", event)

	if err := w.handleExitConditions(event); err != nil {
		return nil, fmt.Errorf("AccountWorker.handleCreateSignalResponse: failed to handle exit conditions: %w", err)
	}

	if err := w.handleOpenConditions(event); err != nil {
		return nil, fmt.Errorf("AccountWorker.handleCreateSignalResponse: failed to handle open conditions: %w", err)
	}

	// todo: publish any newly created trades

	return &eventmodels.CreateSignalResponseEvent{
		Name: event.Name,
	}, nil
}

func (w *AccountWorker) handleCreateSignalRequest(event *eventmodels.CreateSignalRequestEvent) {
	log.Infof("received %v", event)

	responseEvent, err := w.handleCreateSignalResponse(event)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.handleCreateSignalRequest", fmt.Errorf("failed to handle signal request: %w", err), event.Meta)
		return
	}

	pubsub.PublishResult3("AccountWorker.handleCreateSignalRequest", responseEvent, event.Meta)
}

func (w *AccountWorker) handleManualDatafeedUpdateRequest(ev *eventmodels.ManualDatafeedUpdateRequest) {
	ts := time.Now().UTC()
	tick := eventmodels.Tick{
		Timestamp: ts,
		Price:     ev.Bid,
	}

	w.manualDatafeed.Update(tick)

	result := eventmodels.NewManualDatafeedUpdateResult(ev.RequestID, ts, tick)

	pubsub.PublishResult("AccountWorker.handleManualDatafeedUpdateRequest", eventmodels.ManualDatafeedUpdateResultEventName, result)
}

func (w *AccountWorker) createAccountStrategyRequestHandler(ev *eventmodels.CreateAccountStrategyRequestEvent) {
	account, err := w.findAccount(ev.AccountName)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to find account: %w", err), ev.Meta)
		return
	}

	strategy, err := eventmodels.NewStrategy(ev.Strategy.Name, ev.Strategy.Symbol, ev.Strategy.Direction, ev.Strategy.Balance, ev.Strategy.EntryConditions, ev.Strategy.ExitConditions, ev.Strategy.PriceLevels, account)
	if err != nil {
		pubsub.PublishTerminalError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to create strategy: %w", err), ev.Meta)
		return
	}

	if err := account.AddStrategy(strategy); err != nil {
		pubsub.PublishTerminalError("AccountWorker.createAccountStrategyRequestHandler", fmt.Errorf("failed to add strategy: %w", err), ev.Meta)
		return
	}

	// todo: add the requestID as a parameter when dispatching to the event bus, instead of in the event itself
	pubsub.PublishResult3("AccountWorker.createAccountStrategyRequestHandler", &eventmodels.CreateAccountStrategyResponseEvent{
		AccountsRequestHeader: eventmodels.AccountsRequestHeader{
			AccountName: ev.AccountName,
		},
		Strategy: strategy,
	}, ev.Meta)
}

func (w *AccountWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	// task: *** create an AccountManager to hold each account worker and subscribe to events

	// pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, w.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", eventmodels.GetAccountsRequestEventName, w.handleGetAccountsRequestEvent)
	pubsub.Subscribe("AccountWorker", eventmodels.NewTickEventName, w.updateTickMachine)
	pubsub.Subscribe("AccountWorker", eventmodels.CreateTradeRequestEventName, w.handleCreateTradeRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.ExecuteOpenTradeRequestEventName, w.handleExecuteOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.CloseTradesRequestEventName, w.handleCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.ExecuteCloseTradesRequestEventName, w.handleExecuteCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.ExecuteCloseTradeRequestEventName, w.handleExecuteCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.FetchTradesRequestEventName, w.handleFetchTradesRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.NewGetStatsRequestEventName, w.handleGetAccountStatsRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.NewSavedEvent(eventmodels.CreateSignalRequestEventName), w.handleCreateSignalRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.ManualDatafeedUpdateRequestEventName, w.handleManualDatafeedUpdateRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.AutoExecuteTradeEventName, w.handleAutoExecuteTrade)
	pubsub.Subscribe("AccountWorker", eventmodels.NewSavedEvent(eventmodels.CreateAccountStrategyRequestEventName), w.createAccountStrategyRequestHandler)
	pubsub.Subscribe("AccountWorker", eventmodels.NewSavedEvent(eventmodels.CreateAccountRequestEventName), w.createAccountRequestHandler)

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

func NewAccountWorkerClientFromFixtures(wg *sync.WaitGroup, accounts []*eventmodels.Account, coinbaseDatafeed *eventmodels.Datafeed, ibDatafeed *eventmodels.Datafeed, manualDatafeed *eventmodels.Datafeed) *AccountWorker {
	return &AccountWorker{
		wg:               wg,
		accounts:         accounts,
		coinbaseDatafeed: coinbaseDatafeed,
		ibDatafeed:       ibDatafeed,
		manualDatafeed:   manualDatafeed,
	}
}

func NewAccountWorkerClient(wg *sync.WaitGroup) *AccountWorker {
	return &AccountWorker{
		wg:               wg,
		accounts:         make([]*eventmodels.Account, 0),
		coinbaseDatafeed: eventmodels.NewDatafeed(eventmodels.CoinbaseDatafeed),
		ibDatafeed:       eventmodels.NewDatafeed(eventmodels.IBDatafeed),
		manualDatafeed:   eventmodels.NewDatafeed(eventmodels.ManualDatafeed),
	}
}
