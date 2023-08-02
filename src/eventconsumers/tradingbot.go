package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
	"slack-trading/src/service"
	"sync"
)

type TradingBot struct {
	account *models.Account
	wg      *sync.WaitGroup
}

func (b *TradingBot) placeTrade(signal eventmodels.RsiTradeSignal) {
	var t *models.Trade
	var err error
	stopLossDistance := 3000.0

	tradesRemainingCount, tradeType := b.account.TradesRemaining(signal.RequestedPrice)
	log.Debugf("(tradesRemainingCount, side) = : (%v, %v)", tradesRemainingCount, tradeType)

	if signal.IsBuy {
		t, err = service.PlaceBuy(b.account, signal.RequestedPrice, signal.RequestedPrice-stopLossDistance)
		if err != nil {
			pubsub.PublishError("TradingBot.placeTrade.IsBuy", err)
			return
		}
	} else {
		t, err = service.PlaceSell(b.account, signal.RequestedPrice, signal.RequestedPrice+stopLossDistance)
		if err != nil {
			pubsub.PublishError("TradingBot.placeTrade.IsSell", err)
			return
		}
	}

	pubsub.Publish("TradingBot", pubsub.BotTradeRequestEvent, eventmodels.BotTradeRequestEvent{
		Trade: t,
	})
}

func (b *TradingBot) Start(ctx context.Context) {
	b.wg.Add(1)

	pubsub.Subscribe("TradingBot", pubsub.RsiTradeSignal, b.placeTrade)

	go func() {
		defer b.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping RsiBot consumer")
				return
			}
		}
	}()
}

func NewTradingBot(wg *sync.WaitGroup) *TradingBot {
	account, err := models.NewAccount(150000, 0.2, models.PriceLevels{
		Values: []*models.PriceLevel{
			{
				Price:             28500.0,
				NoOfTrades:        4,
				AllocationPercent: 0.6,
			},
			{
				Price:             29500.0,
				NoOfTrades:        3,
				AllocationPercent: 0.4,
			},
			{
				Price: 30500.0,
			},
		},
	})

	if err != nil {
		panic(err)
	}

	return &TradingBot{
		wg:      wg,
		account: account,
	}
}
