package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	"slack-trading/src/indicators"
	models2 "slack-trading/src/models"
	"sync"
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
			pubsub.Publish("RsiBot.update", pubsub.RsiTradeSignal, eventmodels.RsiTradeSignal{
				Value: rsi,
				IsBuy: true,
			})
		}

		if rsi >= 70 && r.prevRsi < 70 {
			pubsub.Publish("RsiBot.update", pubsub.RsiTradeSignal, eventmodels.RsiTradeSignal{
				Value: rsi,
				IsBuy: false,
			})
		}
	}

	r.prevRsi = rsi
}

func (r *RsiBot) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("RsiBot", pubsub.NewCandleEvent, r.update)

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
