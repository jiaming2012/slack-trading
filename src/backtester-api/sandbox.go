package main

import (
	"fmt"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

func main() {
	playground := models.NewPlayground(1000.0)
	initialBalance := playground.GetAccountBalance()

	fmt.Printf("Initial balance: %.2f\n", initialBalance)
}
