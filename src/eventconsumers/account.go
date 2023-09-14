package eventconsumers

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
	"sync"
	"time"
)

type AccountWorker struct {
	wg          *sync.WaitGroup
	accounts    []*models.Account
	tickMachine *models.TickMachine
}

func (r *AccountWorker) getAccounts() []models.Account {
	var accounts []models.Account

	for _, acc := range r.accounts {
		accounts = append(accounts, *acc)
	}

	return accounts
}

func (r *AccountWorker) addAccount(account *models.Account, balance float64, priceLevels []*models.PriceLevel) error {
	strategy, err := models.NewStrategy("trendline-break", "BTC-USD", "down", balance, priceLevels)
	if err != nil {
		return err
	}

	account.AddStrategy(*strategy)

	r.accounts = append(r.accounts, account)

	return nil
}

//func (r *AccountWorker) newOpenTradeRequest(accountName string, strategyName string, tradeType models.TradeType) (*models.OpenTradeRequest, error) {
//	account, err := r.findAccount(accountName)
//	if err != nil {
//		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: could not find account: %w", err)
//	}
//
//	currentTick := r.tickMachine.Query()
//
//	openTradeRequest, err := account.PlaceOpenTradeRequest(strategyName, currentTick.Bid)
//	if err != nil {
//		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: PlaceOrderOpen: %w", err)
//	}
//
//	return openTradeRequest, nil
//}

func (r *AccountWorker) findAccount(name string) (*models.Account, error) {
	for _, a := range r.accounts {
		if name == a.Name {
			return a, nil
		}
	}

	return nil, fmt.Errorf("AccountWorker.findAccount: could not find account with name %v", name)
}

func (r *AccountWorker) addAccountRequestHandler(request eventmodels.AddAccountRequestEvent) {
	log.Debug("<- AccountWorker.addAccountRequestHandler")

	var priceLevels []*models.PriceLevel

	for _, input := range request.PriceLevelsInput {
		priceLevels = append(priceLevels, &models.PriceLevel{
			Price:             input[0],
			MaxNoOfTrades:     int(input[1]),
			AllocationPercent: input[2],
		})
	}

	account, err := models.NewAccount(request.Name, request.Balance)
	if err != nil {
		pubsub.PublishError("AccountWorker.addAccountHandler", err)
		return
	}

	err = r.addAccount(account, request.Balance, priceLevels)
	if err != nil {
		pubsub.PublishError("AccountWorker.NewStrategy", err)
		return
	}

	pubsub.Publish("AccountWorker.addAccountHandler", pubsub.AddAccountResponseEvent, eventmodels.AddAccountResponseEvent{
		Account: *account,
	})
}

func (r *AccountWorker) getAccountsRequestHandler(request eventmodels.GetAccountsRequestEvent) {
	log.Debugf("<- AccountWorker.getAccountsRequestHandler")

	pubsub.Publish("AccountWorker", pubsub.GetAccountsResponseEvent, eventmodels.GetAccountsResponseEvent{
		Accounts: r.getAccounts(),
	})
}

func (r *AccountWorker) checkForStopOut(tick models.Tick) (*models.Strategy, error) {
	// todo: analyze if calling PL() so many times on each tick causes a bottleneck
	for _, account := range r.accounts {
		for _, strategy := range account.Strategies {
			stats, err := strategy.GetTrades().GetTradeStats(tick)
			if err != nil {
				return nil, fmt.Errorf("AccountWorker.checkForStopOut: GetTradeStats failed: %w", err)
			}

			if stats.Realized+stats.Floating >= strategy.Balance {
				return &strategy, nil
			}
		}
	}

	return nil, nil
}

func (r *AccountWorker) updateTickMachine(tick eventmodels.Tick) {
	log.Debug("<- AccountWorker.updateTickMachine")

	// todo: eventually update based off level 2 quotes to get bid and ask
	r.tickMachine.Update(models.Tick{
		Timestamp: tick.Timestamp,
		Bid:       tick.Price,
		Ask:       tick.Price,
	})
}

func (r *AccountWorker) update() {
	tick := r.tickMachine.Query()
	strategy, err := r.checkForStopOut(*tick)
	if err != nil {
		log.Errorf("AccountWorker.update: check for stop out failed: %v", err)
		return
	}

	if strategy != nil {
		// send for event

	}
}

// todo: make this the model: NewCloseTradeRequest -> ExecuteCloseTradesRequest
func (r *AccountWorker) handleNewCloseTradeRequest(event eventmodels.CloseTradeRequest) {
	log.Debug("<- AccountWorker.handleNewCloseTradeRequest")

	account, err := r.findAccount(event.AccountName)
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

func (r *AccountWorker) handleExecuteCloseTradesRequest(event eventmodels.ExecuteCloseTradesRequest) {
	log.Debug("<- AccountWorker.handleExecuteCloseTradesRequest")

	clsTradeReq := event.CloseTradesRequest
	tradeID := uuid.New()
	now := time.Now()
	requestPrc := r.getMarketPrice(clsTradeReq.Strategy, true)

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

func (r *AccountWorker) getMarketPrice(strategy *models.Strategy, isClose bool) float64 {
	tick := r.tickMachine.Query()
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

func (r *AccountWorker) handleExecuteNewOpenTradeRequest(event eventmodels.ExecuteOpenTradeRequest) {
	log.Debug("<- AccountWorker.handleExecuteNewOpenTradeRequest")

	req := event.OpenTradeRequest
	tradeID := uuid.New()
	now := time.Now()
	requestPrc := r.getMarketPrice(req.Strategy, false)

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

func (r *AccountWorker) handleNewOpenTradeRequest(event eventmodels.OpenTradeRequest) {
	log.Debug("<- AccountWorker.handleNewOpenTradeRequest")

	account, err := r.findAccount(event.AccountName)
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

	openTradeReq, err := models.NewOpenTradeRequest(
		event.Timeframe,
		strategy,
	)

	pubsub.Publish("AccountWorker.handleNewOpenTradeRequest", pubsub.ExecuteOpenTradeRequest, eventmodels.ExecuteOpenTradeRequest{
		RequestID:        event.RequestID,
		OpenTradeRequest: openTradeReq,
	})
}

func (r *AccountWorker) handleFetchTradesRequest(event *eventmodels.FetchTradesRequest) {
	log.Debug("<- AccountWorker.handleFetchTradesRequest")

	account, err := r.findAccount(event.AccountName)
	if err != nil {
		requestErr := eventmodels.NewRequestError(event.RequestID, fmt.Errorf("failed to find findAccount: %w", err))
		pubsub.PublishError("AccountWorker.handleFetchTradesRequest", requestErr)
		return
	}

	priceLevelTrades := account.GetPriceLevelTrades()

	resultEvent := eventmodels.NewFetchTradesResult(event.RequestID, priceLevelTrades)

	pubsub.Publish("AccountWorker.handleFetchTradesRequest", pubsub.FetchTradesResult, resultEvent)
}

func (r *AccountWorker) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, r.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.GetAccountsRequestEvent, r.getAccountsRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.NewTickEvent, r.updateTickMachine)
	pubsub.Subscribe("AccountWorker", pubsub.NewOpenTradeRequest, r.handleNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteOpenTradeRequest, r.handleExecuteNewOpenTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.NewCloseTradesRequest, r.handleNewCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteCloseTradesRequest, r.handleExecuteCloseTradesRequest)
	pubsub.Subscribe("AccountWorker", pubsub.FetchTradesRequest, r.handleFetchTradesRequest)

	go func() {
		defer r.wg.Done()
		// todo: investigate why we had to increase from 500ms -> 5 seconds
		ticker := time.NewTicker(5 * time.Second)

		for {
			select {
			case <-ticker.C:
				r.update()
			case <-ctx.Done():
				log.Info("stopping AccountWorker consumer")
				return
			}
		}
	}()
}

func NewAccountWorkerClientFromFixtures(wg *sync.WaitGroup, accounts []*models.Account) *AccountWorker {
	return &AccountWorker{
		wg:          wg,
		accounts:    accounts,
		tickMachine: models.NewTickMachine(),
	}
}

func NewAccountWorkerClient(wg *sync.WaitGroup) *AccountWorker {
	return &AccountWorker{
		wg:          wg,
		accounts:    make([]*models.Account, 0),
		tickMachine: models.NewTickMachine(),
	}
}
