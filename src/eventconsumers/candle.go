package eventconsumers

import (
	"context"
	"math"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	models "slack-trading/src/eventmodels"
	pubsub "slack-trading/src/eventpubsub"
	models2 "slack-trading/src/models"
	"slack-trading/src/sheets"
	"slack-trading/src/worker"
)

const (
	CandleInterval = 1
)

type CandleWorker struct {
	wg     *sync.WaitGroup
	candle *models2.Candle
	timer  *time.Timer
	mu     sync.Mutex
}

func (w *CandleWorker) calculateBalance(symbol string) {
	log.Debugf("CandleWorker.calculateBalance <- %v", symbol)

	trades, fetchErr := sheets.FetchTrades(context.Background())
	if fetchErr != nil {
		pubsub.PublishEventError("CandleWorker.calculateBalance: fetchErr:", fetchErr)
		return
	}

	// todo: make price and FetchTrades fetches async
	btcPriceCh := worker.FetchCurrentPrice()
	btcPrice := <-btcPriceCh

	profit, statsErr := trades.GetTradeStats(models2.Tick{Bid: btcPrice, Ask: btcPrice})
	if statsErr != nil {
		pubsub.PublishEventError("CandleWorker.calculateBalance: statsErr", statsErr)
		return
	}

	vwap, volume, realizedPL := trades.GetTradeStatsItems()

	// todo: remove profit.RequestedVolume in favor of volume
	if math.Abs(float64(profit.Volume)-float64(volume)) > 0.001 {
		log.Warnf("Unexpected different volumes: %v, %v", profit.Volume, volume)
	}

	pubsub.PublishEventResult("CandleWorker", eventmodels.BalanceResultEventName, models.Balance{
		Floating: profit.FloatingPL,
		Realized: realizedPL,
		Vwap:     vwap,
		Volume:   volume,
	})
}

func (w *CandleWorker) Update(tick models.Tick) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.candle == nil {
		w.candle = models2.NewCandle(tick.Price)
		return
	}

	w.candle.Update(tick.Price)
}

func (w *CandleWorker) resetParams() {
	w.candle = nil
	w.timer = worker.MinuteTimer(CandleInterval)
}

func (w *CandleWorker) CreateNewCandle() {
	log.Debug("CandleWorker:: CreateNewCandle")

	if w.candle == nil {
		log.Debug("CreateNewCandle::short circuit. candle not created")
		return
	}

	w.mu.Lock()

	newCandle := models.Candle{
		Timestamp:   w.candle.Timestamp,
		LastUpdated: w.candle.LastUpdated,
		Open:        w.candle.Open,
		High:        w.candle.High,
		Low:         w.candle.Low,
		Close:       w.candle.Close,
	}

	w.resetParams()

	w.mu.Unlock()

	pubsub.PublishEventResult("CandleWorker.CreateNewCandle", eventmodels.NewCandleEventName, newCandle)
}

func (w *CandleWorker) Start(ctx context.Context) {
	w.wg.Add(1)

	pubsub.Subscribe("CandleWorker", eventmodels.NewTickEventName, w.Update)

	go func() {
		defer w.wg.Done()
		for {
			select {
			case <-w.timer.C:
				w.CreateNewCandle()
			case <-ctx.Done():
				log.Info("stopping CandleWorker consumer")
				return
			}
		}
	}()
}

func NewCandleWorkerClient(wg *sync.WaitGroup) *CandleWorker {
	timer := worker.MinuteTimer(CandleInterval)

	return &CandleWorker{
		wg:     wg,
		candle: nil,
		timer:  timer,
	}
}
