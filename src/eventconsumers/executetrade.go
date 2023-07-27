package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/coingecko"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"sync"
	"time"
)

type TradeExecutor struct {
	wg *sync.WaitGroup
}

func (r *TradeExecutor) executeTrade(tradeRequestEvent models.TradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade -> %v", tradeRequestEvent)

	btcPrice, err := coingecko.FetchCoinbaseBTCPrice()
	if err != nil {
		panic(err)
		//return models.TradeRequestEvent{}, fmt.Errorf("failed to fetch coinbase btc price: %v", err)
	}

	pubsub.Publish(pubsub.TradeFulfilledEvent, models.TradeFulfilledEvent{
		Timestamp:      time.Now(),
		Symbol:         tradeRequestEvent.Symbol,
		RequestedPrice: tradeRequestEvent.Price,
		ExecutedPrice:  btcPrice,
		Volume:         tradeRequestEvent.Volume,
	})
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe(pubsub.TradeRequestEvent, r.executeTrade)

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
