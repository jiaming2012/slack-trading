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

func (r *TradeExecutor) executeTrade(tradeRequestEvent models.TradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade <- %v", tradeRequestEvent)

	btcPriceCh := make(chan float64)
	worker.FetchCurrentPrice(btcPriceCh)
	btcPrice := <-btcPriceCh

	pubsub.Publish("TradeExecutor", pubsub.TradeFulfilledEvent, models.TradeFulfilledEvent{
		Timestamp:      time.Now(),
		Symbol:         tradeRequestEvent.Symbol,
		RequestedPrice: tradeRequestEvent.Price,
		ExecutedPrice:  btcPrice,
		Volume:         tradeRequestEvent.Volume,
		ResponseURL:    tradeRequestEvent.ResponseURL,
	})
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("TradeExecutor", pubsub.TradeRequestEvent, r.executeTrade)

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
