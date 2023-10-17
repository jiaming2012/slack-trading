package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/worker"
	"sync"
	"time"
)

type TradeExecutor struct {
	wg *sync.WaitGroup
}

func (r *TradeExecutor) executeTrade(request models.TradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	pubsub.Publish("TradeExecutor.executeTrade", pubsub.TradeFulfilledEvent, models.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Symbol,
		RequestedPrice: request.Price,
		ExecutedPrice:  btcPrice,
		Volume:         request.Volume,
		ResponseURL:    request.ResponseURL,
	})
}

func (r *TradeExecutor) executeBotTrade(request models.BotTradeRequestEvent) {
	log.Debugf("TradeExecutor.executeBotTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	// todo: this should go to Coinbase
	request.Trade.Execute(btcPrice, request.Trade.ExecutedVolume)

	pubsub.Publish("TradeExecutor.executeBotTrade", pubsub.TradeFulfilledEvent, models.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Trade.Symbol,
		RequestedPrice: request.Trade.RequestedPrice,
		ExecutedPrice:  btcPrice,
		Volume:         request.Trade.RequestedVolume,
		ResponseURL:    WebhookURL,
	})
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("TradeExecutor", pubsub.TradeRequestEvent, r.executeTrade)
	pubsub.Subscribe("TradeExecutor", pubsub.BotTradeRequestEvent, r.executeBotTrade)

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
