package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/go/models"
	pubsub "github.com/jiaming2012/slack-trading/src/go/pubsub"
)

type TradingBot struct {
	wg       *sync.WaitGroup
	strategy *models.Strategy
}

func (b *TradingBot) placeTrade(signal models.RsiTradeSignal) {
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
	//pubsub.Publish("TradingBot", pubsub.BotTradeRequestEvent, models.BotTradeRequestEvent{
	//	PriceLevel: t,
	//})
}

func (b *TradingBot) handleSupportBreakSignal(signal models.SupportBreakSignal) {
	log.Infof("TradingBot.handleSupportBreakSignal: %v", signal)
}

func (b *TradingBot) handleResistanceBreakSignal(signal models.ResistanceBreakSignal) {
	log.Infof("TradingBot.handleResistanceBreakSignal: %v", signal)
}

func (b *TradingBot) handleTrendlineBreakSignal(signal models.TrendlineBreakSignal) {
	log.Infof("TradingBot.handleTrendlineBreakSignal: %v", signal)
}

func (b *TradingBot) handleAddStrategy(ev models.AddStrategyRequest) {
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
	//	signal = models.NewTrendlineBreakSignal(ev.Symbol, timeframe, ev.ExecutedPrice, ev.Direction, ev.PriceActionEvent)
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

	pubsub.Subscribe("TradingBot", models.SupportBreakSignalEventName, b.handleSupportBreakSignal)
	pubsub.Subscribe("TradingBot", models.ResistanceBreakSignalEventName, b.handleResistanceBreakSignal)
	pubsub.Subscribe("TradingBot", models.TrendlineBreakSignalEventName, b.handleTrendlineBreakSignal)
	pubsub.Subscribe("TradingBot", models.AddStrategyRequestEventName, b.handleAddStrategy)

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
