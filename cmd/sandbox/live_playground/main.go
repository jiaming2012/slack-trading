package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventservices"
	"github.com/jiaming2012/slack-trading/src/utils"
)

func main() {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Panicf("missing PROJECTS_DIR environment variable")
	}

	if err := utils.InitEnvironmentVariables(projectsDir, "development"); err != nil {
		log.Panicf("failed to init environment variables")
	}

	// polgyon setup
	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	if err != nil {
		log.Fatalf("$POLYGON_API_KEY not set: %v", err)
	}

	polygonClient := eventservices.NewPolygonTickDataMachine(polygonApiKey)
	// newBarsCh := make(chan eventmodels.PolygonAggregateBarV2)

	// go fetchLivePolygonCandles(client, newBarsCh)

	// tradier setup
	ctx := context.Background()
	wg := sync.WaitGroup{}

	// set logger to info
	log.SetLevel(log.InfoLevel)

	tradesAccountID, err := utils.GetEnv("TRADIER_TRADES_ACCOUNT_ID")
	if err != nil {
		log.Fatalf("$TRADIER_TRADES_ACCOUNT_ID not set: %v", err)
	}

	tradierTradesUrlTemplate, err := utils.GetEnv("TRADIER_TRADES_URL_TEMPLATE")
	if err != nil {
		log.Fatalf("$TRADIER_TRADES_URL_TEMPLATE not set: %v", err)
	}

	tradierTradesOrderURL := fmt.Sprintf(tradierTradesUrlTemplate, tradesAccountID)
	tradierMarketTimesalesURL, err := utils.GetEnv("TRADIER_MARKET_TIMESALES_URL")
	if err != nil {
		log.Fatalf("$TRADIER_MARKET_TIMESALES_URL not set: %v", err)
	}

	tradierQuotesBearerToken, err := utils.GetEnv("TRADIER_BEARER_TOKEN")
	if err != nil {
		log.Fatalf("$TRADIER_BEARER_TOKEN not set: %v", err)
	}

	tradierTradesBearerToken, err := utils.GetEnv("TRADIER_TRADES_BEARER_TOKEN")
	if err != nil {
		log.Fatalf("$TRADIER_TRADES_BEARER_TOKEN not set: %v", err)
	}

	liveCandlesUpdateQueue := eventmodels.NewFIFOQueue[*eventmodels.TradierCandleUpdate](1000)
	liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*eventmodels.TradierOrderUpdateEvent](1000)

	panic("pass in a db contection to the worker")

	// tradier engine
	worker := eventconsumers.NewTradierApiWorker(&wg, liveCandlesUpdateQueue, tradierTradesOrderURL, tradierMarketTimesalesURL, tradierQuotesBearerToken, tradierTradesBearerToken, polygonClient, liveOrdersUpdateQueue, nil)

	worker.Start(ctx)

	go func() {
		for {
			candle, ok := liveCandlesUpdateQueue.Dequeue()
			if ok {
				log.Infof("candle: %v", candle)
			} else {
				log.Infof("queue is empty")
			}

			time.Sleep(10 * time.Second)
		}
	}()

	wg.Wait()
	// polygon engine
	// for {
	// 	select {
	// 	case newBar := <-newBarsCh:
	// 		log.Infof("new bar: %v", newBar)
	// 	}
	// }
}

func fetchTradierAccount() {

}

func fetchLivePolygonCandles(client *eventservices.PolygonTickDataMachine, out chan<- eventmodels.PolygonAggregateBarV2) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		log.Fatalf("Error loading location: %v", err)
		return
	}

	var currentBar *eventmodels.PolygonAggregateBarV2
	for {
		timespan := eventmodels.PolygonTimespan{
			Multiplier: 1,
			Unit:       "minute",
		}

		// Get the current date in New York time
		now := time.Now().In(location)
		fromYear, fromMonth, fromDay := now.Date()
		fromPolygonDate := &eventmodels.PolygonDate{
			Year:  fromYear,
			Month: int(fromMonth),
			Day:   fromDay,
		}

		// market close
		now = now.Add(24 * time.Hour)
		toYear, toMonth, toDay := now.Date()
		toPolygonDate := &eventmodels.PolygonDate{
			Year:  toYear,
			Month: int(toMonth),
			Day:   toDay,
		}

		bars, err := client.FetchAggregateBars(eventmodels.StockSymbol("COIN"), timespan, fromPolygonDate, toPolygonDate)
		if err != nil {
			log.Errorf("failed to fetch aggregate bars: %v", err)
			continue
		}

		newBar := bars[len(bars)-1]

		if currentBar == nil {
			currentBar = newBar
		} else {
			if !currentBar.Timestamp.Equal(newBar.Timestamp) {
				currentBar = newBar
				out <- *newBar
			}
		}

		time.Sleep(1 * time.Second)
	}
}
