package eventconsumers

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	pubsub "github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/worker"
)

type TradeExecutor struct {
	wg         *sync.WaitGroup
	webHookURL string
}

func (r *TradeExecutor) executeTrade(request models.TradeRequestEvent) {
	log.Debugf("TradeExecutor.executeTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	pubsub.PublishResponse("TradeExecutor.executeTrade", models.TradeFulfilledEventName, &models.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Symbol,
		RequestedPrice: request.Price,
		ExecutedPrice:  btcPrice,
		Volume:         request.Volume,
		ResponseURL:    request.ResponseURL,
	}, &request.Meta)
}

func (r *TradeExecutor) executeBotTrade(request models.BotTradeRequestEvent) {
	log.Debugf("TradeExecutor.executeBotTrade <- %v", request)

	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	// todo: this should go to Coinbase
	// todo: add a requestID
	request.Trade.Execute(btcPrice, request.Trade.ExecutedVolume)

	pubsub.PublishResponse("TradeExecutor.executeBotTrade", models.TradeFulfilledEventName, &models.TradeFulfilledEvent{
		Timestamp:      time.Now().UTC(),
		Symbol:         request.Trade.Symbol,
		RequestedPrice: request.Trade.RequestedPrice,
		ExecutedPrice:  btcPrice,
		Volume:         request.Trade.RequestedVolume,
		ResponseURL:    r.webHookURL,
	}, &request.Meta)
}

func (r *TradeExecutor) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("TradeExecutor", models.TradeRequestEventName, r.executeTrade)
	pubsub.Subscribe("TradeExecutor", models.BotTradeRequestEventName, r.executeBotTrade)

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

func NewTradeExecutorClient(wg *sync.WaitGroup, webHookURL string) *TradeExecutor {
	return &TradeExecutor{
		wg:         wg,
		webHookURL: webHookURL,
	}
}
