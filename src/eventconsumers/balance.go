package eventconsumers

import (
	"context"
	log "github.com/sirupsen/logrus"
	"math"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
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
		panic(fetchErr)
	}

	// todo: make fetches async
	var btcPriceCh chan float64
	worker.FetchCurrentPrice(btcPriceCh)
	btcPrice := <-btcPriceCh

	profit := trades.PL(btcPrice)
	vwap, volume, realizedPL := trades.Vwap()

	// todo: remove profit.Volume in favor of volume
	if math.Abs(float64(profit.Volume)-float64(volume)) > 0.001 {
		log.Warnf("Unexpected different volumes: %v, %v", profit.Volume, volume)
	}

	pubsub.Publish("BalanceWorker", pubsub.BalanceResultEvent, models.Balance{
		Floating: profit,
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
