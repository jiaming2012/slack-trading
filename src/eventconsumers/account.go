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
		pubsub.PublishRequestError("AccountWorker.createAccountRequestHandler", request, fmt.Errorf("datafeed source: %v", request.DatafeedName))
		return
	}

	account, err := eventmodels.NewAccount(request.Name, request.Balance, datafeed)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountRequestHandler", request, err)
		return
	}

	err = w.addAccountWithoutStrategy(account)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.addAccountWithoutStrategy", request, err)
		return
	}

	pubsub.PublishResult("AccountWorker.createAccountRequestHandler", eventmodels.CreateAccountResponseEventName, &eventmodels.CreateAccountResponseEvent{
		RequestID: request.RequestID,
		Account:   account,
	})
}

func (w *AccountWorker) handleGetAccountsRequestEvent(request *eventmodels.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.PublishResult("AccountWorker", eventmodels.GetAccountsResponseEventName, &eventmodels.GetAccountsResponseEvent{
		RequestID: request.RequestID,
		Accounts:  w.getAccounts(),
	})
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

		pubsub.PublishResult("AccountWorker.update", eventmodels.ExecuteCloseTradesRequestEventName, &eventmodels.ExecuteCloseTradesRequest{
			RequestID:          uuid.New(),
			CloseTradesRequest: req,
		})
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
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find account: %w", err))
		pubsub.PublishEventError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
		pubsub.PublishEventError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	closeTradesRequest, err := eventmodels.NewCloseTradesRequest(strategy, event.Timeframe, event.PriceLevelIndex, event.Percent, strategy.Name)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("new close trades request failed: %w", err))
		pubsub.PublishEventError("AccountWorker.handleNewCloseTradeRequest", requestErr)
		return
	}

	pubsub.PublishResult("AccountWorker.handleCloseTradeRequest", eventmodels.ExecuteCloseTradesRequestEventName, eventmodels.ExecuteCloseTradesRequest{
		RequestID:          event.RequestID,
		CloseTradesRequest: closeTradesRequest,
	})
}

func (w *AccountWorker) handleExecuteCloseTradeRequest(event *eventmodels.ExecuteCloseTradeRequest) {
	tradeID := uuid.New()
	now := time.Now().UTC()

	strategy := event.Trade.PriceLevel.Strategy
	datafeed := strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	trade, _, err := strategy.NewCloseTrade(tradeID, event.Timeframe, now, requestPrc, event.Percent, event.Trade)
	if err != nil {
		if errors.Is(err, eventmodels.DuplicateCloseTradeErr) {
			log.Debugf("duplicate close: skip closing %v", event.Trade.ID)
			return
		}

		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new close trade: %w", err))
		pubsub.PublishEventError("AccountWorker.handleExecuteCloseTradesRequest", requestErr)
		return
	}

	pubsub.PublishResult("AccountWorker.handleExecuteCloseTradesRequest", eventmodels.AutoExecuteTradeEventName, &eventmodels.AutoExecuteTrade{
		RequestID: event.RequestID,
		Trade:     trade,
	})
}

func (w *AccountWorker) handleExecuteCloseTradesRequest(event eventmodels.ExecuteCloseTradesRequest) {
	log.Debug("<- AccountWorker.handleExecuteCloseTradesRequest")

	clsTradeReq := event.CloseTradesRequest
	tradeID := uuid.New()
	now := time.Now().UTC()
	datafeed := event.CloseTradesRequest.Strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	// todo: unify models: partialCloseRequestItems. Strategy.AutoExecuteTrade handles differently than trade.AutoExecuteTrade
	trade, _, err := clsTradeReq.Strategy.NewCloseTrades(tradeID, clsTradeReq.Timeframe, now, requestPrc, clsTradeReq.PriceLevelIndex, clsTradeReq.Percent)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleExecuteCloseTradesRequest", event, fmt.Errorf("unable to create new close trade: %w", err))
		return
	}

	pubsub.PublishResult("AccountWorker.handleExecuteCloseTradesRequest", eventmodels.AutoExecuteTradeEventName, &eventmodels.AutoExecuteTrade{
		RequestID: event.RequestID,
		Trade:     trade,
	})
}

func (w *AccountWorker) handleAutoExecuteTrade(event *eventmodels.AutoExecuteTrade) {
	strategy := event.Trade.PriceLevel.Strategy
	_, err := strategy.AutoExecuteTrade(event.Trade)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to place execute trade: %w", err))
		pubsub.PublishRequestError("AccountWorker.handleExecuteCloseTradesRequest", event, requestErr)
		return
	}

	// executeCloseTradesResult := &eventmodels.ExecuteCloseTradesResult{
	// 	RequestID: event.RequestID,
	// 	Side:      strategy.GetTradeType(true).String(),
	// 	Result:    result,
	// }
	executeCloseTradesResult := &eventmodels.ExecuteCloseTradesResult{
		Trade: event.Trade,
	}

	pubsub.PublishResult("AccountWorker.handleExecuteCloseTradesRequest", eventmodels.ExecuteCloseTradesResultEventName, executeCloseTradesResult)
}

func (w *AccountWorker) handleExecuteOpenTradeRequest(event eventmodels.ExecuteOpenTradeRequest) {
	log.Debug("<- AccountWorker.handleExecuteNewOpenTradeRequest")

	req := event.OpenTradeRequest
	tradeID := uuid.New()
	now := time.Now().UTC()

	account, err := w.findAccount(req.AccountName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find findAccount: %w", err))
		pubsub.PublishRequestError("AccountWorker.handleExecuteNewOpenTradeRequest", event, requestErr)
		return
	}

	strategy, err := account.FindStrategy(req.StrategyName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find strategy: %w", err))
		pubsub.PublishRequestError("AccountWorker.handleExecuteNewOpenTradeRequest", event, requestErr)
		return
	}

	datafeed := strategy.Account.Datafeed

	requestPrc := datafeed.LastTick

	trade, _, err := strategy.NewOpenTrade(tradeID, req.Timeframe, now, requestPrc)
	if err != nil {
		// event.Meta.EndProcess(err)
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to create new trade: %w", err))
		pubsub.PublishRequestErrorInterface("AccountWorker.handleExecuteNewOpenTradeRequest", event, requestErr)
		return
	}

	result, err := strategy.AutoExecuteTrade(trade)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("unable to place execute trade: %w", err))
		pubsub.PublishRequestError("AccountWorker.handleExecuteNewOpenTradeRequest", event, requestErr)
		return
	}

	// todo: automatically insert the parent event
	executeOpenTradeResult := &eventmodels.ExecuteOpenTradeResult{
		Meta:            event.Meta,
		PriceLevelIndex: result.PriceLevelIndex,
		Trade:           trade,
	}
	// executeOpenTradeResult := &eventmodels.ExecuteOpenTradeResult{
	// 	Meta:      eventmodels.NewMetaData(event.Meta),
	// 	RequestID: event.RequestID,
	// 	Side:      strategy.GetTradeType(false).String(),
	// 	Result:    result,
	// }

	pubsub.PublishResult("AccountWorker.handleExecuteNewOpenTradeRequest", eventmodels.ExecuteOpenTradeResultEventName, executeOpenTradeResult)
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

	pubsub.PublishResult("AccountWorker.handleCreateTradeRequest", eventmodels.ExecuteOpenTradeRequestEventName, eventmodels.ExecuteOpenTradeRequest{
		ParentRequest:    &event,
		Meta:             event.Meta,
		RequestID:        event.RequestID,
		OpenTradeRequest: &event,
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

	pubsub.PublishResult("AccountWorker.handleFetchTradesRequest", eventmodels.FetchTradesResultEventName, fetchTradesResult)
}

func (w *AccountWorker) handleGetAccountStatsRequest(event *eventmodels.GetStatsRequest) {
	log.Debug("<- AccountWorker.handleGetAccountStatsRequest")

	event.Meta = &eventmodels.MetaData{
		ParentMeta:   nil,
		RequestError: make(chan eventmodels.RequestError2),
	}

	account, err := w.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetAccountStatsRequest", event, fmt.Errorf("failed to find findAccount: %w", err))
		return
	}

	currentTick := account.Datafeed.Tick()

	statsResult, err := eventservices.GetStats(event.RequestID, account, currentTick)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.handleGetAccountStatsRequest", event, err)
		return
	}

	statsResult.Meta = &eventmodels.MetaData{
		ParentMeta:   event.Meta,
		RequestError: make(chan eventmodels.RequestError2),
	}

	pubsub.PublishResult("AccountWorker.handleGetAccountStatsRequest", eventmodels.GetStatsResultEventName, statsResult)
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

// g="coinbase: initial connect failed:EOF"
func (w *AccountWorker) handleCreateSignalRequest(event *eventmodels.CreateSignalRequest) {
	log.Infof("received %v", event)

	meta := &eventmodels.MetaData{
		ParentMeta:   nil,
		RequestError: make(chan eventmodels.RequestError2),
	}

	// handle exit conditions
	exitConditionsSatisfied, updateErr := eventservices.UpdateExitConditions(w.getAccounts(), event)
	if updateErr == nil {
		if exitConditionsSatisfied != nil {
			var syncProcess pubsub.SyncProcess
			clsTradeRequests, err := w.handleExitConditionsSatisfied(exitConditionsSatisfied)
			if err == nil {
				for _, req := range clsTradeRequests {
					// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove, as the request didn't originate from an api call but is still picked up by the lister
					// eventmodels.RegisterResultCallback(req.RequestID)

					// todo: change to error
					syncProcess.Add(func(c chan error) {
						pubsub.PublishEventResult("AccountWorker.handleNewSignalRequest", eventmodels.CloseTradesRequestEventName, req)
					})
				}

				// todo: test error handling
				if err := syncProcess.Run(); err != nil {
					pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
				}
			} else {
				pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
			}
		}
	} else {
		pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, updateErr)
	}

	// 3/4/24: the current problem is that we need to consider the flow
	// Flow 1: apiRequest -> parsedRequest (globaldispatcher) -> parsedRequestDBWrite -> (mutex.Lock) dbRead -> processRequest -> (mutex.Unlock) processRequestComplete -> (globaldispatcher) publishResult
	// Flob 1b:  							  											                                                    -> (globaldispatcher) publishError vs pubsub.PublishRequestError (this should terminate the request)
	// Flow 2: 																	      -> (mutex.Lock) dbRead -> processRequest -> (mutex.Unlock) processRequestComplete -> (globaldispatcher) publishResult
	if entryConditionsSatisfied := eventservices.UpdateEntryConditions(w.getAccounts(), event); entryConditionsSatisfied != nil {
		openTradeRequests, err := w.handleEntryConditionsSatisfied(entryConditionsSatisfied)
		if err == nil {
			// var syncProcess pubsub.SyncProcess
			for _, req := range openTradeRequests {
				// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove
				// as the request didn't originate from an api call but is still picked up by the lister
				// eventmodels.RegisterResultCallback(req.RequestID)

				// syncProcess.Add(func(c chan error) {
				// req.Meta.RequestError = c
				reqErrCh := req.Wait()

				pubsub.PublishEventResult("AccountWorker.handleNewSignalRequest", eventmodels.CreateTradeRequestEventName, *req)

				for e := range reqErrCh {
					bFoundExecuteOpenTradeRequest := false
					log.Errorf("Trade error, %T: %v", e.Request, e.Error)
					switch e.Request.(type) {
					case eventmodels.ExecuteOpenTradeRequest:
						bFoundExecuteOpenTradeRequest = true
					}

					if bFoundExecuteOpenTradeRequest {
						pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, e.Error)
						break
					}
				}
			}
		} else {
			pubsub.PublishRequestError("AccountWorker.handleNewSignalRequest", event, err)
		}
	}

	// todo: there must be a more elegant way to handle this: stops error message from GlobalDispatcher.GetChannelAndRemove
	// as the request didn't originate from an api call but is still picked up by the lister
	// eventmodels.RegisterResultCallback(event.RequestID)

	pubsub.PublishResult("AccountWorker.handleNewSignalRequest", eventmodels.CreateSignalResponseEventName, &eventmodels.CreateSignalResponseEvent{
		Meta: &eventmodels.MetaData{
			ParentMeta:   meta,
			RequestError: make(chan eventmodels.RequestError2),
		},
		Name:      event.Name,
		RequestID: event.RequestID,
	})
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
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", ev, fmt.Errorf("failed to find account: %w", err))
		return
	}

	strategy, err := eventmodels.NewStrategy(ev.Strategy.Name, ev.Strategy.Symbol, ev.Strategy.Direction, ev.Strategy.Balance, ev.Strategy.EntryConditions, ev.Strategy.ExitConditions, ev.Strategy.PriceLevels, account)
	if err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", ev, fmt.Errorf("failed to create strategy: %w", err))
		return
	}

	if err := account.AddStrategy(strategy); err != nil {
		pubsub.PublishRequestError("AccountWorker.createAccountStrategyRequestHandler", ev, fmt.Errorf("failed to add strategy: %w", err))
		return
	}

	// todo: add the requestID as a parameter when dispatching to the event bus, instead of in the event itself
	pubsub.PublishResult("AccountWorker.createAccountStrategyRequestHandler", eventmodels.CreateStrategyResponseEventName, &eventmodels.CreateAccountStrategyResponseEvent{
		AccountsRequestHeader: eventmodels.AccountsRequestHeader{
			RequestHeader: eventmodels.RequestHeader{
				RequestID: ev.RequestHeader.RequestID,
			},
			AccountName: ev.AccountName,
		},
		Strategy: strategy,
	})
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
	pubsub.Subscribe("AccountWorker", eventmodels.CreateSignalRequestSavedEventName, w.handleCreateSignalRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.ManualDatafeedUpdateRequestEventName, w.handleManualDatafeedUpdateRequest)
	pubsub.Subscribe("AccountWorker", eventmodels.AutoExecuteTradeEventName, w.handleAutoExecuteTrade)
	pubsub.Subscribe("AccountWorker", eventmodels.CreateAccountStrategyRequestSavedEventName, w.createAccountStrategyRequestHandler)
	pubsub.Subscribe("AccountWorker", eventmodels.CreateAccountRequestEventSavedEventName, w.createAccountRequestHandler)

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
