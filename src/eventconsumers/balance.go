package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"math"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	models2 "slack-trading/src/models"
	"slack-trading/src/sheets"
	"slack-trading/src/worker"
	"sync"
)

type BalanceWorker struct {
	wg *sync.WaitGroup
}

func (r *BalanceWorker) calculateBalance(symbol string) {
	log.Debugf("BalanceWorker.calculateBalance <- %v", symbol)

	trades, fetchErr := sheets.FetchTrades(context.Background())
	if fetchErr != nil {
		pubsub.PublishError("BalanceWorker.calculateBalance", fetchErr)
		return
	}

	// todo: make price and FetchTrades fetches async
	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	profit, statsErr := trades.GetTradeStats(models2.Tick{Bid: btcPrice, Ask: btcPrice})
	if statsErr != nil {
		pubsub.PublishError("BalanceWorker.calculateBalance", statsErr)
		return
	}

	vwap, volume, realizedPL := trades.GetTradeStatsItems()

	// todo: remove profit.RequestedVolume in favor of volume
	if math.Abs(float64(profit.Volume)-float64(volume)) > 0.001 {
		log.Warnf("Unexpected different volumes: %v, %v", profit.Volume, volume)
	}

	pubsub.Publish("BalanceWorker", pubsub.BalanceResultEvent, models.Balance{
		Floating: profit.Floating,
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
