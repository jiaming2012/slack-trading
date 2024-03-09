package eventconsumers

import (
	"context"
	"math"
	"sync"

	log "github.com/sirupsen/logrus"

	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	models2 "slack-trading/src/models"
	"slack-trading/src/sheets"
	"slack-trading/src/worker"
)

type BalanceWorker struct {
	wg *sync.WaitGroup
}

func (r *BalanceWorker) calculateBalance(symbol string) {
	log.Debugf("BalanceWorker.calculateBalance <- %v", symbol)

	trades, fetchErr := sheets.FetchTrades(context.Background())
	if fetchErr != nil {
		pubsub.PublishEventError("BalanceWorker.calculateBalance", fetchErr)
		return
	}

	// todo: make price and FetchTrades fetches async
	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	profit, statsErr := trades.GetTradeStats(models2.Tick{Bid: btcPrice, Ask: btcPrice})
	if statsErr != nil {
		pubsub.PublishEventError("BalanceWorker.calculateBalance", statsErr)
		return
	}

	vwap, volume, realizedPL := trades.GetTradeStatsItems()

	// todo: remove profit.RequestedVolume in favor of volume
	if math.Abs(float64(profit.Volume)-float64(volume)) > 0.001 {
		log.Warnf("Unexpected different volumes: %v, %v", profit.Volume, volume)
	}

	pubsub.PublishEventResult("BalanceWorker", pubsub.BalanceResultEvent, models.Balance{
		Floating: profit.FloatingPL,
		Realized: realizedPL,
		Vwap:     vwap,
		Volume:   volume,
	})
}

func (r *BalanceWorker) Start(ctx context.Context) {
	r.wg.Add(1)

	pubsub.Subscribe("BalanceWorker", pubsub.BalanceRequestEvent, r.calculateBalance)

	go func() {
		defer r.wg.Done()
		for {
			select {
			case <-ctx.Done():
				log.Info("stopping BalanceWorker consumer")
				return
			}
		}
	}()
}

func NewBalanceWorkerClient(wg *sync.WaitGroup) *BalanceWorker {
	return &BalanceWorker{
		wg: wg,
	}
}
