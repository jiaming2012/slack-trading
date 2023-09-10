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
	strategy, err := models.NewStrategy("Trendline Break", "BTC-USD", "down", balance, priceLevels)
	if err != nil {
		return err
	}

	account.AddStrategy(*strategy)

	r.accounts = append(r.accounts, account)

	return nil
}

func (r *AccountWorker) newOpenTradeRequest(accountName string, strategyName string, tradeType models.TradeType) (*models.OpenTradeRequest, error) {
	account, err := r.findAccount(accountName)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: could not find account: %w", err)
	}

	currentTick := r.tickMachine.Query()

	openTradeRequest, err := account.PlaceOpenTradeRequest(strategyName, currentTick.Bid)
	if err != nil {
		return nil, fmt.Errorf("AccountWorker.placeOpenTradeRequest: PlaceOrderOpen: %w", err)
	}

	return openTradeRequest, nil
}

func (r *AccountWorker) findAccount(name string) (*models.Account, error) {
	for _, a := range r.accounts {
		if name == a.Name {
			return a, nil
		}
	}

	return nil, fmt.Errorf("AccountWorker.findAccount: could not find account with name %v", name)
}

func (r *AccountWorker) addAccountRequestHandler(request eventmodels.AddAccountRequestEvent) {
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
	log.Debugf("AccountWorker.getAccountsRequestHandler")

	pubsub.Publish("AccountWorker", pubsub.GetAccountsResponseEvent, eventmodels.GetAccountsResponseEvent{
		Accounts: r.getAccounts(),
	})
}

func (r *AccountWorker) checkForStopOut(tick models.Tick) *models.Strategy {
	// todo: analyze if calling PL() so many times on each tick causes a bottleneck
	for _, account := range r.accounts {
		for _, strategy := range account.Strategies {
			pl := strategy.GetTrades().PL(tick)
			if pl.Realized+pl.Floating >= strategy.Balance {
				return &strategy
			}
		}
	}

	return nil
}

func (r *AccountWorker) updateTickMachine(tick eventmodels.Tick) {
	// todo: eventually update based off level 2 quotes to get bid and ask
	r.tickMachine.Update(models.Tick{
		Timestamp: tick.Timestamp,
		Bid:       tick.Price,
		Ask:       tick.Price,
	})
}

func (r *AccountWorker) update() {
	tick := r.tickMachine.Query()
	if strategy := r.checkForStopOut(*tick); strategy != nil {
		// send for event

	}
}

// todo: make this the model: NewCloseTradeRequest -> ExecuteCloseTradesRequest
func (r *AccountWorker) handleNewCloseTradeRequest(event eventmodels.CloseTradeRequest) {
	account, err := r.findAccount(event.AccountName)
	if err != nil {
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", fmt.Errorf("failed to find account: %w", err))
		return
	}

	strategy, err := account.FindStrategy(event.StrategyName)
	if err != nil {
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", fmt.Errorf("failed to find strategy: %w", err))
		return
	}

	priceLevel, err := strategy.GetPriceLevelByIndex(event.PriceLevelIndex)
	if err != nil {
		pubsub.PublishError("AccountWorker.handleNewCloseTradeRequest", fmt.Errorf("failed to get price level by index: %w", err))
		return
	}

	closeTradesRequest, err := account.PlaceOrderClose(priceLevel, event.Percent, event.Reason)

	pubsub.Publish("AccountWorker.handleCloseTradeRequest", pubsub.ExecuteCloseTradesRequest, eventmodels.ExecuteCloseTradeRequest{
		PriceLevel:         priceLevel,
		CloseTradesRequest: closeTradesRequest,
	})
}

func (r *AccountWorker) handleExecuteCloseTradesRequest(event eventmodels.ExecuteCloseTradeRequest) {
	for _, req := range event.CloseTradesRequest {
		marketPrice := r.tickMachine.Query().Bid

		offsetTrade, err := models.NewCloseTrade(uuid.New(), []*models.Trade{req.Trade}, req.Timeframe, time.Now(), marketPrice, req.Volume)
		if err != nil {
			pubsub.PublishError("AccountWorker.handleExecuteCloseTradesRequest", err)
			return
		}

		event.PriceLevel.Add(offsetTrade)
	}
}

func (r *AccountWorker) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("AccountWorker", pubsub.AddAccountRequestEvent, r.addAccountRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.GetAccountsRequestEvent, r.getAccountsRequestHandler)
	pubsub.Subscribe("AccountWorker", pubsub.NewTickEvent, r.updateTickMachine)
	pubsub.Subscribe("AccountWorker", pubsub.NewCloseTradesRequest, r.handleNewCloseTradeRequest)
	pubsub.Subscribe("AccountWorker", pubsub.ExecuteCloseTradesRequest, r.handleExecuteCloseTradesRequest)

	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)

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
