package worker

import (
	"context"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventmodels"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/models"
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

func MinuteTimer(minuteInterval int) *time.Timer {
	// Current time
	now := time.Now().UTC()

	// Get the number of seconds until the next minute
	var d time.Duration
	minutes := float64(minuteInterval-1) - math.Mod(float64(time.Now().UTC().Minute()), float64(minuteInterval))

	// todo: remove this
	d = (time.Second * time.Duration(60-now.Second())) + (time.Minute * time.Duration(minutes))
	//d = 10 * time.Second

	// Timestamp of the next tick
	nextTick := now.Add(d)

	// Subtract next tick from now
	diff := nextTick.Sub(time.Now().UTC())

	// Return new ticker
	return time.NewTimer(diff)
}

func Run(ctx context.Context, tickerCh chan CoinbaseDTO, c *websocket.Conn) {
	go WsTick(ctx, tickerCh, c)

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

				eventpubsub.PublishEventResult("Coinbase.worker", eventpubsub.NewTickEvent, eventmodels.NewTick(
					time.Now().UTC(),
					price,
					models.CoinbaseDatafeed,
				))

				// todo: should this be moved to a separate service? or send the current price to a channel to be consumed by pubsub subscribers
				mu.Lock()
				currentPrice = price
				mu.Unlock()
			}
		}
	}
}
