package worker

import (
	"context"
	log "github.com/sirupsen/logrus"
	"math"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"strconv"
	"sync"
	"time"
)

const (
	timerFrequencyInSeconds = 6
)

var currentPrice float64
var mu sync.Mutex

func FetchCurrentPrice() chan float64 {
	result := make(chan float64)

	go func() {
		defer mu.Unlock()

		for {
			mu.Lock()

			if currentPrice > 0 {
				result <- currentPrice
				return
			}
			mu.Unlock()
			time.Sleep(200 * time.Millisecond)
		}
	}()

	return result
}

func FiveMinuteTimer() *time.Timer {
	// Current time
	now := time.Now()

	// Get the number of seconds until the next minute
	var d time.Duration
	minutes := 4 - math.Mod(float64(time.Now().Minute()), 5)
	// todo: remove this
	d = (time.Second * time.Duration(60-now.Second())) + (time.Minute * time.Duration(minutes))
	//d = 10 * time.Second

	// Time of the next tick
	nextTick := now.Add(d)

	// Subtract next tick from now
	diff := nextTick.Sub(time.Now())

	// Return new ticker
	return time.NewTimer(diff)
}

func Run(ctx context.Context, tickerCh chan CoinbaseDTO) {

	go WsTick(ctx, tickerCh)

	// todo: figure out if we need this?
	//go strategy.Worker()

	for {
		select {
		case <-ctx.Done():
			log.Info("stopping Coinbase producer")
			return
		case t := <-tickerCh:
			if len(t.Events) > 0 && len(t.Events[0].Tickers) > 0 {
				priceStr := t.Events[0].Tickers[0].Price
				price, err := strconv.ParseFloat(priceStr, 64)
				if err != nil {
					panic(err)
				}

				eventpubsub.PublishWithFlags("Coinbase.worker", eventpubsub.NewTickEvent, eventmodels.Tick{
					Timestamp: time.Now(),
					Price:     price,
				}, false)

				// todo: should this be moved to a separate service?
				mu.Lock()
				currentPrice = price
				mu.Unlock()
			}
		}
	}
}
