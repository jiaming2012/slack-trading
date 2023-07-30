package worker

import (
	"context"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
	"sync"
	"time"
)

const (
	timerFrequencyInSeconds = 6
)

var currentPrice float64
var mu sync.Mutex

func FetchCurrentPrice(curPrice chan float64) {
	go func() {
		defer mu.Unlock()

		for {
			mu.Lock()

			if currentPrice > 0 {
				curPrice <- currentPrice
				return
			}
			mu.Unlock()
			time.Sleep(200 * time.Millisecond)
		}
	}()
}

func fiveMinuteTimer() *time.Timer {
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
	//go strategy.Worker()

	//timer := fiveMinuteTimer()
	//ev := <-tickerCh
	//initialPriceStr := ev.Events[0].Tickers[0].Price
	//initialPrice, err := strconv.ParseFloat(initialPriceStr, 64)
	//if err != nil {
	//	panic(err)
	//}

	//candle := models.NewCandle(initialPrice)

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

				mu.Lock()
				currentPrice = price
				mu.Unlock()
			}

			//	candle.Update(price)
			//case <-timer.C:
			//	ev2 := <-tickerCh
			//	priceStr := ev2.Events[0].Tickers[0].Price
			//	price, err := strconv.ParseFloat(priceStr, 64)
			//	if err != nil {
			//		panic(err)
			//	}
			//
			//	err = sheets.AppendCandle(ctx, candle)
			//	if err != nil {
			//		log.Error(err)
			//	}
			//
			//	// emit event
			//	events.Emit(models.NewM5Candle, candle)
			//
			//	log.Info("recorded a new candle")
			//	candle = models.NewCandle(price)
			//	timer = fiveMinuteTimer()
		}
	}
}
