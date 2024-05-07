package main

import (
	"encoding/csv"
	"fmt"
	"math/rand"
	"os"
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
	lowerRangeDifferential := -100.0
	upperRangeDifferential := 100.0
	initialRangeMin := initialPrice + lowerRangeDifferential
	initialRangeMax := initialPrice + upperRangeDifferential
	durationX := 24 // duration in hours, for simplicity treated as steps
	durationY := 7  // duration in days, for simplicity treated as steps
	probabilityUp := 0.60
	jumpSize := 200.0

	// Slice to hold prices
	var prices []float64
	prices = append(prices, initialPrice)

	// Simulate the range-bound period
	for i := 0; i < durationY; i++ {
		for j := 0; j < durationX; j++ {
			nextPrice := initialRangeMin + rand.Float64()*(initialRangeMax-initialRangeMin)
			prices = append(prices, nextPrice)
		}

		transitionJump := transitionJump(probabilityUp, jumpSize)
		initialRangeMin += transitionJump
		initialRangeMax += transitionJump
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
	writer.Write([]string{"Time Step", "Stock Price"})

	for i, price := range prices {
		writer.Write([]string{fmt.Sprintf("%d", i), fmt.Sprintf("%.2f", price)})
	}
}
