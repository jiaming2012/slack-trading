package worker

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"math"
	"slack-trading/src/coingecko"
	"slack-trading/src/models"
	"slack-trading/src/sheets"
	"strconv"
	"time"
)

const (
	timerFrequencyInSeconds = 6
)

func fetchPrice() (float64, error) {
	btcPrice, fetchErr := coingecko.FetchCoinbaseBTCPrice()
	if fetchErr != nil {
		return 0, fmt.Errorf("failed to fetch coinbase btc price: %w", fetchErr)
	}

	return btcPrice, nil
}

func fiveMinuteTimer() *time.Timer {
	// Current time
	now := time.Now()

	// Get the number of seconds until the next minute
	var d time.Duration
	minutes := 4 - math.Mod(float64(time.Now().Minute()), 5)
	d = (time.Second * time.Duration(60-now.Second())) + (time.Minute * time.Duration(minutes))

	// Time of the next tick
	nextTick := now.Add(d)

	// Subtract next tick from now
	diff := nextTick.Sub(time.Now())

	// Return new ticker
	return time.NewTimer(diff)
}

func Run(tickerCh chan CoinbaseDTO) {
	go WsTick(tickerCh)

	ctx := context.Background()
	timer := fiveMinuteTimer()
	ev := <-tickerCh
	initialPriceStr := ev.Events[0].Tickers[0].Price
	initialPrice, err := strconv.ParseFloat(initialPriceStr, 64)
	if err != nil {
		panic(err)
	}

	candle := models.NewCandle(initialPrice)

	for {
		select {
		case t := <-tickerCh:
			priceStr := t.Events[0].Tickers[0].Price
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				panic(err)
			}

			candle.Update(price)
		case <-timer.C:
			ev2 := <-tickerCh
			priceStr := ev2.Events[0].Tickers[0].Price
			price, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				panic(err)
			}

			err = sheets.AppendCandle(ctx, candle)
			if err != nil {
				log.Error(err)
			}

			log.Info("recorded a new candle")
			candle = models.NewCandle(price)
			timer = fiveMinuteTimer()
		}
	}
}
