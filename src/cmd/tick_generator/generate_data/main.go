package main

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

func transitionJump(probabilityUp float64, jumpSize float64) float64 {
	// Transition event after period X

	if rand.Float64() < probabilityUp {
		// Move up to the new range
		return jumpSize
	} else {
		// Move down to the new range
		return jumpSize * -1
	}
}

func main() {
	// Parameters
	initialPrice := 1000.0
	minStockPrice := 500.0
	lowerRangeDifferential := -300.0
	upperRangeDifferential := 300.0
	initialRangeMin := initialPrice + lowerRangeDifferential
	initialRangeMax := initialPrice + upperRangeDifferential
	durationHoursInDay := 24 // duration in hours, for simplicity treated as steps
	durationDays := 90       // duration in days, for simplicity treated as steps
	probabilityUp := 0.55
	jumpSize := 200.0
	startTimeStr := "2024-01-01 9:00:00"

	// Initial Time
	startTime, err := time.Parse("2006-01-02 15:04:05", startTimeStr)
	if err != nil {
		log.Fatalf("Error parsing start time: %v", err)
		return
	}

	// Slice to hold prices
	var times []time.Time
	var prices []float64

	// Initial price
	prices = append(prices, initialPrice)
	times = append(times, startTime)
	var j = 1

	// Simulate the range-bound period
	for i := 0; i < durationDays; i++ {
		for ; j < durationHoursInDay; j++ {
			nextPrice := initialRangeMin + rand.Float64()*(initialRangeMax-initialRangeMin)
			prices = append(prices, nextPrice)
			times = append(times, startTime.Add(time.Duration(j+i*durationHoursInDay)*time.Hour))
		}

		transitionJump := transitionJump(probabilityUp, jumpSize)
		initialRangeMin += transitionJump
		if initialRangeMin < minStockPrice {
			initialRangeMin = minStockPrice
		}

		initialRangeMax += transitionJump
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
	writer.Write([]string{"Time", "Stock Price"})

	for i, price := range prices {
		writer.Write([]string{times[i].Format("2006-01-02 15:04:05"), fmt.Sprintf("%.2f", price)})
	}
}
