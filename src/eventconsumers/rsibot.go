package eventconsumers

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"

	models2 "github.com/jiaming2012/slack-trading/src/models"

	"github.com/jiaming2012/slack-trading/src/indicators"

	pubsub "github.com/jiaming2012/slack-trading/src/eventpubsub"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type RsiBot struct {
	wg      *sync.WaitGroup
	rsiM5   *indicators.Rsi
	prevRsi float64
}

func (r *RsiBot) update(candle eventmodels.Candle) {
	rsi := r.rsiM5.Update(models2.Candle{
		Timestamp:   candle.Timestamp,
		LastUpdated: candle.LastUpdated,
		Open:        candle.Open,
		High:        candle.High,
		Low:         candle.Low,
		Close:       candle.Close,
	})

	log.Debugf("rsi: %v, prevRsi: %v", rsi, r.prevRsi)

	if rsi > 0 {
		if rsi <= 30 && r.prevRsi > 30 {
			pubsub.PublishEvent("RsiBot.update", eventmodels.RsiTradeSignalEventName, eventmodels.RsiTradeSignal{
				Value:          rsi,
				IsBuy:          true,
				RequestedPrice: candle.Close,
			})
		}

		if rsi >= 70 && r.prevRsi < 70 {
			pubsub.PublishEvent("RsiBot.update", eventmodels.RsiTradeSignalEventName, eventmodels.RsiTradeSignal{
				Value:          rsi,
				IsBuy:          false,
				RequestedPrice: candle.Close,
			})
		}
	}

	r.prevRsi = rsi
}

func (r *RsiBot) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("RsiBot", eventmodels.NewCandleEventName, r.update)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping RsiBot consumer")
				return
			}
		}
	}()
}

func NewRsiBotClient(wg *sync.WaitGroup) *RsiBot {
	rsi := indicators.NewRsi(14)

	return &RsiBot{
		wg:    wg,
		rsiM5: rsi,
	}
}
