package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

func transitionJump(probabilityUp, probabilityDown float64, jumpSize float64) float64 {
	// Transition event after period X

	if rand.Float64() < probabilityUp {
		return jumpSize
	} else if rand.Float64() < probabilityDown {
		return -jumpSize
	} else {
		return 0
	}
}

type Candle struct {
	Open   float64 `json:"open"`
	Close  float64 `json:"close"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Volume float64 `json:"volume"`
}

func generateCandle(minStockPrice, open, close, volatility, probabilityCandleUp float64) Candle {
	mid := (open + close) / 2

	high := mid + rand.Float64()*volatility
	if open <= close {
		high = math.Max(high, close)
	} else {
		high = math.Max(high, open)
	}

	low := mid - rand.Float64()*volatility
	if open >= close {
		low = math.Min(low, close)
	} else {
		low = math.Min(low, open)
	}

	if low < minStockPrice {
		low = minStockPrice
	}

	if open < minStockPrice {
		open = minStockPrice
	}

	if close < minStockPrice {
		close = minStockPrice
	}

	if high < minStockPrice {
		high = minStockPrice
	}

	return Candle{
		Open:   open,
		Close:  close,
		High:   high,
		Low:    low,
		Volume: rand.Float64() * 1000,
	}
}

func getNextPriceDifference(probabilityCandleUp, volatility float64) float64 {
	if rand.Float64() < probabilityCandleUp {
		return rand.Float64() * volatility
	} else {
		return rand.Float64() * volatility * -1
	}
}

func main() {
	// Parameters
	initialPrice := 1000.0
	minStockPrice := 500.0
	candleVolatility := 10.0
	durationHoursInDay := 24 // duration in hours, for simplicity treated as steps
	durationDays := 90       // duration in days, for simplicity treated as steps
	probabilityTransitionUp := 0.05
	probabilityTransitionDown := 0.1
	probabilityCandleUp := 0.55
	jumpSize := 5.0
	startTimeStr := "2021-01-04 9:30:00"

	// Initial Time
	startTime, err := time.Parse("2006-01-02 15:04:05", startTimeStr)
	if err != nil {
		log.Fatalf("Error parsing start time: %v", err)
		return
	}

	// Slice to hold prices
	var times []time.Time
	var candles []Candle

	// Initial price
	initialDiff := getNextPriceDifference(probabilityCandleUp, candleVolatility)
	candles = append(candles, generateCandle(minStockPrice, initialPrice, initialPrice+initialDiff, candleVolatility, probabilityCandleUp))
	times = append(times, startTime)
	var j = 1

	// Simulate the range-bound period
	for i := 0; i < durationDays; i++ {
		for ; j < durationHoursInDay; j++ {
			transitionJump := transitionJump(probabilityTransitionUp, probabilityTransitionDown, jumpSize)
			diff := getNextPriceDifference(probabilityCandleUp, candleVolatility)

			prevClose := candles[len(candles)-1].Close + transitionJump
			nextCandle := generateCandle(minStockPrice, prevClose, prevClose+diff, candleVolatility, probabilityCandleUp)
			candles = append(candles, nextCandle)
			times = append(times, startTime.Add(time.Duration(j+i*durationHoursInDay)*time.Hour))
		}

		j = 0
	}

	// Export the prices
	file, err := os.Create("stock_data.csv")
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"timestamp", "open", "high", "low", "close", "volume"})

	for i, c := range candles {
		writer.Write([]string{times[i].Format("2006-01-02 15:04:05"), fmt.Sprintf("%.2f", c.Open), fmt.Sprintf("%.2f", c.High), fmt.Sprintf("%.2f", c.Low), fmt.Sprintf("%.2f", c.Close), fmt.Sprintf("%.2f", c.Volume)})
	}
}
