package main

import (
	"context"
	"log"
	"math"
	"time"

	polygon "github.com/polygon-io/client-go/rest"
	"github.com/polygon-io/client-go/rest/models"
)

func all_contracts() {
	c := polygon.New("7Z_3KjIHQeH3yW7RuTMGZH2kHwCK11QY")
	t := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
	params := models.ListOptionsContractsParams{}.
		WithUnderlyingTicker(models.EQ, "COIN").
		WithOrder(models.Order("asc")).
		WithLimit(10).
		WithSort(models.Sort("ticker")).
		WithExpired(true).
		WithAsOf(models.Date(t)).
		WithStrikePrice(models.GTE, 350).
		WithStrikePrice(models.LT, 400)

	iter := c.ListOptionsContracts(context.Background(), params)
	var contracts []models.OptionsContract
	for iter.Next() {
		contracts = append(contracts, iter.Item())
		if math.Mod(float64(len(contracts)), 1000) == 0 {
			log.Printf("Found %d contracts for COIN with ticker %s", len(contracts), contracts[len(contracts)-1].Ticker)
		}
	}

	if iter.Err() != nil {
		log.Fatal(iter.Err())
	}

	log.Printf("Found %d contracts for COIN as of %s", len(contracts), t.Format("2006-01-02"))
}

func ticks() {
	c := polygon.New("7Z_3KjIHQeH3yW7RuTMGZH2kHwCK11QY")

	from, err := time.Parse("2006-01-02", "2000-01-09")
	if err != nil {
		log.Fatalf("Error parsing 'from' date: %v", err)
	}
	to, err := time.Parse("2006-01-02", "2025-07-01")
	if err != nil {
		log.Fatalf("Error parsing 'to' date: %v", err)
	}

	params := models.ListAggsParams{
		Ticker:     "O:COIN240419C00360000",
		Multiplier: 1,
		Timespan:   "day",
		From:       models.Millis(from),
		To:         models.Millis(to),
	}.
		WithAdjusted(true).
		WithOrder(models.Order("asc")).
		WithLimit(5000)

	iter := c.ListAggs(context.Background(), params)

	for iter.Next() {
		log.Print(iter.Item())
	}
	if iter.Err() != nil {
		log.Fatal(iter.Err())
	}
}

func main() {
	ticks()
}
