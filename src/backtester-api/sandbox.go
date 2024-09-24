package main

import (
	"fmt"
	"time"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

func main() {
	clock := models.NewClock(time.Date(2023, time.November, 3, 0, 0, 0, 0, time.UTC), time.Date(2023, time.December, 3, 0, 0, 0, 0, time.UTC))
	playground := models.NewPlayground(1000.0, clock, nil)
	initialBalance := playground.GetAccountBalance()

	fmt.Printf("Initial balance: %.2f\n", initialBalance)
}
