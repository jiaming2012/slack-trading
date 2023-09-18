package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/models"
	"sync"
)

type TradingBot struct {
	wg       *sync.WaitGroup
	strategy *models.Strategy
}

func (b *TradingBot) placeTrade(signal eventmodels.RsiTradeSignal) {
	//var t *models.PriceLevel
	//var err error
	//stopLossDistance := 3000.0
	//
	//tradesRemainingCount, tradeType := b.account.TradesRemaining(signal.RequestedPrice)
	//log.Debugf("(tradesRemainingCount, side) = : (%v, %v)", tradesRemainingCount, tradeType)
	//
	//if signal.IsBuy {
	//	t, err = service.PlaceBuy(b.account, signal.RequestedPrice, signal.RequestedPrice-stopLossDistance)
	//	if err != nil {
	//		pubsub.PublishError("TradingBot.placeTrade.IsBuy", err)
	//		return
	//	}
	//} else {
	//	t, err = service.PlaceSell(b.account, signal.RequestedPrice, signal.RequestedPrice+stopLossDistance)
	//	if err != nil {
	//		pubsub.PublishError("TradingBot.placeTrade.IsSell", err)
	//		return
	//	}
	//}
	//
	//pubsub.Publish("TradingBot", pubsub.BotTradeRequestEvent, eventmodels.BotTradeRequestEvent{
	//	PriceLevel: t,
	//})
}

func (b *TradingBot) handleSupportBreakSignal(signal eventmodels.SupportBreakSignal) {
	log.Infof("TradingBot.handleSupportBreakSignal: %v", signal)
}

func (b *TradingBot) handleResistanceBreakSignal(signal eventmodels.ResistanceBreakSignal) {
	log.Infof("TradingBot.handleResistanceBreakSignal: %v", signal)
}

func (b *TradingBot) handleTrendlineBreakSignal(signal eventmodels.TrendlineBreakSignal) {
	log.Infof("TradingBot.handleTrendlineBreakSignal: %v", signal)
}

func (b *TradingBot) handleAddStrategy(ev eventmodels.AddStrategyRequest) {
	//var signal models.Signal
	//
	//timeframe, err := ev.Timeframe.Validate()
	//if err != nil {
	//	log.Errorf("TradingBot.handleAddStrategy: failed to validate timeframe: %v", err)
	//}
	//
	//switch ev.Header.Signal {
	//case "support-break":
	//	log.Error("TradingBot.handleAddStrategy::support-break: not yet implemented")
	//	return
	//case "resistance-break":
	//	log.Error("TradingBot.handleAddStrategy::resistance-break: not yet implemented")
	//	return
	//case "trendline-break":
	//	signal = eventmodels.NewTrendlineBreakSignal(ev.Symbol, timeframe, ev.Price, ev.Direction, ev.PriceActionEvent)
	//default:
	//	log.Errorf("TradingBot.handleAddStrategy: unknown signal %v", ev.Header.Signal)
	//	return
	//}

	//if err = b.strategy.AddCondition(signal, ); err != nil {
	//	log.Errorf("failed to add strategy: %v", err)
	//}
}

func (b *TradingBot) Start(ctx context.Context) {
	b.wg.Add(1)

	pubsub.Subscribe("TradingBot", pubsub.SupportBreakSignal, b.handleSupportBreakSignal)
	pubsub.Subscribe("TradingBot", pubsub.ResistanceBreakSignal, b.handleResistanceBreakSignal)
	pubsub.Subscribe("TradingBot", pubsub.TrendlineBreakSignal, b.handleTrendlineBreakSignal)
	pubsub.Subscribe("TradingBot", pubsub.AddStrategyRequest, b.handleAddStrategy)

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

//func NewTradingBot(wg *sync.WaitGroup) *TradingBot {
//	return &TradingBot{
//		wg:       wg,
//		strategy: models.NewStrategy("main"),
//	}
//}
