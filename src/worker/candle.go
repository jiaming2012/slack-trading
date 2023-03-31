package worker

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slack-trading/src/coingecko"
	"slack-trading/src/models"
	"slack-trading/src/sheets"
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

func minuteTicker() *time.Ticker {
	// Current time
	now := time.Now()

	// Get the number of seconds until the next minute
	var d time.Duration
	d = time.Second * time.Duration(60-now.Second())

	// Time of the next tick
	nextTick := now.Add(d)

	// Subtract next tick from now
	diff := nextTick.Sub(time.Now())

	// Return new ticker
	return time.NewTicker(diff)
}

func Run(initialPrice float64) {
	ctx := context.Background()
	timer := time.NewTicker(timerFrequencyInSeconds * time.Second)
	ticker := minuteTicker()
	candle := models.NewCandle(initialPrice)

	for {
		select {
		case <-timer.C:
			price, err := fetchPrice()
			if err != nil {
				log.Error(fmt.Errorf("failed to fetch price during candle tick: %w", err))
				continue
			}

			fmt.Println("price: ", price)
			candle.Update(price)
		case <-ticker.C:
			price, err := fetchPrice()
			if err != nil {
				log.Error(fmt.Errorf("failed to fetch price during candle upload: %w", err))
			}

			err = sheets.AppendCandle(ctx, candle)
			if err != nil {
				log.Error(err)
			}

			candle = models.NewCandle(price)
			ticker = minuteTicker()
		}
	}
}
