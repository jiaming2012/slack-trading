package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
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
	probabilityTransitionUp := 0.05
	probabilityTransitionDown := 0.1
	probabilityCandleUp := 0.55
	jumpSize := 5.0
	startTimeStr := "2021-01-04 9:30:00"
	endTimeStr := "2021-01-31 16:00:00"
	timeDelta := time.Minute

	// Initial Time
	startTime, err := time.Parse("2006-01-02 15:04:05", startTimeStr)
	if err != nil {
		log.Fatalf("Error parsing start time: %v", err)
		return
	}

	// End Time
	endTime, err := time.Parse("2006-01-02 15:04:05", endTimeStr)
	if err != nil {
		log.Fatalf("Error parsing end time: %v", err)
		return
	}

	// Fetch calendar
	calendar, err := services.FetchCalendar(eventmodels.PolygonDate{
		Year:  startTime.Year(),
		Month: int(startTime.Month()),
		Day:   startTime.Day(),
	}, eventmodels.PolygonDate{
		Year:  endTime.Year(),
		Month: int(endTime.Month()),
		Day:   endTime.Day(),
	})

	if err != nil {
		log.Fatalf("Error fetching calendar: %v", err)
	}

	// Print the calendar
	for _, c := range calendar {
		fmt.Printf("Date: %s, Market Open: %s, Market Close: %s\n", c.Date, c.MarketOpen, c.MarketClose)
	}

	// Slice to hold prices
	var times []time.Time
	var candles []Candle

	// Initial price + change this!!!!!
	initialDiff := getNextPriceDifference(probabilityCandleUp, candleVolatility)
	// candles = append(candles, generateCandle(minStockPrice, initialPrice, initialPrice+initialDiff, candleVolatility, probabilityCandleUp))
	// times = append(times, startTime)
	initialCandle := generateCandle(minStockPrice, initialPrice, initialPrice+initialDiff, candleVolatility, probabilityCandleUp)

	// Simulate the range-bound period
	for _, c := range calendar {
		tstamp := time.Date(c.MarketOpen.Year(), c.MarketOpen.Month(), c.MarketOpen.Day(), c.MarketOpen.Hour(), c.MarketOpen.Minute(), c.MarketOpen.Second(), c.MarketOpen.Nanosecond(), c.MarketOpen.Location())
		for tstamp.Before(c.MarketClose) {
			transitionJump := transitionJump(probabilityTransitionUp, probabilityTransitionDown, jumpSize)
			diff := getNextPriceDifference(probabilityCandleUp, candleVolatility)

			var prevClose float64
			if len(candles) > 0 {
				prevClose = candles[len(candles)-1].Close + transitionJump
			} else {
				prevClose = initialCandle.Close
			}

			nextCandle := generateCandle(minStockPrice, prevClose, prevClose+diff, candleVolatility, probabilityCandleUp)
			candles = append(candles, nextCandle)
			times = append(times, tstamp)

			tstamp = tstamp.Add(timeDelta)
		}
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
