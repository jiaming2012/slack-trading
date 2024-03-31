package eventconsumers

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/worker"
)

type TradeExecutor struct {
	wg *sync.WaitGroup
}

func (r *TradeExecutor) executeTrade(request eventmodels.TradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	pubsub.PublishResult4("TradeExecutor.executeTrade", eventmodels.TradeFulfilledEventName, &eventmodels.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Symbol,
		RequestedPrice: request.Price,
		ExecutedPrice:  btcPrice,
		Volume:         request.Volume,
		ResponseURL:    request.ResponseURL,
	}, request.Meta)
}

func (r *TradeExecutor) executeBotTrade(request eventmodels.BotTradeRequestEvent) {
	log.Debugf("TradeExecutor.executeBotTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	// todo: this should go to Coinbase
	// todo: add a requestID
	request.Trade.Execute(btcPrice, request.Trade.ExecutedVolume)

	pubsub.PublishResult4("TradeExecutor.executeBotTrade", eventmodels.TradeFulfilledEventName, &eventmodels.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Trade.Symbol,
		RequestedPrice: request.Trade.RequestedPrice,
		ExecutedPrice:  btcPrice,
		Volume:         request.Trade.RequestedVolume,
		ResponseURL:    WebhookURL,
	}, request.Meta)
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("TradeExecutor", eventmodels.TradeRequestEventName, r.executeTrade)
	pubsub.Subscribe("TradeExecutor", eventmodels.BotTradeRequestEventName, r.executeBotTrade)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping TradeExecutor consumer")
				return
			}
		}
	}()
}

func NewTradeExecutorClient(wg *sync.WaitGroup) *TradeExecutor {
	return &TradeExecutor{
		wg: wg,
	}
}
