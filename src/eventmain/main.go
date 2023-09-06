package main

import (
	"context"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"slack-trading/src/eventconsumers"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/models"
	"slack-trading/src/sheets"
	"sync"
	"syscall"
)

/* Slack commands
/accounts add MrTrendy 2000 0.5 25966 2 0.5 26024 1 0.5 26073
/accounts update MrTrendy ... ?
/strategy add TrendPursuit to MrTrendy
 - open conditions are part of strategy
 - close conditions are part of strategy
/condition add Trendline break to MrTrendy TrendPursuit with params BTCUSD(transform trendspider symbol COINBASE:^BTCUSD to BTCUSD??) m5 trendline_break bounce up 27000
/condition add BollingerKeltnerConsolidation to TrendPursuit [or should this be part of strategy (for now)]
*/

/* Trendspider Alerts
// --- line cross. E.g. - Moving average cross
// {{"header": {"timeframe": "m5", "signal": "%alert_name%", "symbol": "%alert_symbol%", "price_action_event": "%price_action_event%"}, "data": {"price": "%last_price%"}}

// --- custom alert. E.g. - Heiken Ashi Up
Alert: "Heiken Ashi Up" on COINBASE:^BTCUSD (Heikin Ashi)
Created: Aug 30, 2023 10:19 (Your local time)
Last check: Aug 30, 2023 10:30 (Your local time)
Next check: Aug 30, 2023 10:35 (Your local time)
Active (Multifactor Alert)

All of the following:.................................................... no
  5min Chart(close) (1 candles ago) ≤ 5min Chart(open) (1 candles ago)... yes
  5min Chart(close) > 5min Chart(open)................................... no

// {"header": {"timeframe": "m5", "signal": "%alert_name%", "symbol": "%alert_symbol%", "price_action_event": "%price_action_event%"}, "data": {"price": "%last_price%", "direction": "up"}}


*/

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	// Set up logger
	log.SetLevel(log.DebugLevel)

	// Set up event bus
	eventpubsub.Init()

	// Set up google sheets
	if err := sheets.Init(ctx); err != nil {
		panic(err)
	}

	// Setup router
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "3000"
	}

	router := mux.NewRouter()

	// Setup web server
	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
	}

	// Start web server
	go func() {
		log.Infof("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				panic(err)
			}
		}
	}()

	// Create channel for shutdown signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	// todo: fetch from database
	accountFixtures := []*models.Account{
		{
			Name:              "Playground",
			Balance:           2000,
			MaxLossPercentage: 0.25,
			PriceLevels:       nil,
		},
	}

	// Start event clients
	eventproducers.NewReportClient(&wg).Start(ctx)
	eventproducers.NewSlackClient(&wg, router).Start(ctx)
	eventproducers.NewCoinbaseClient(&wg, router).Start(ctx)
	eventconsumers.NewTradeExecutorClient(&wg).Start(ctx)
	eventconsumers.NewGoogleSheetsClient(ctx, &wg).Start()
	eventconsumers.NewSlackNotifierClient(&wg).Start(ctx)
	eventconsumers.NewBalanceWorkerClient(&wg).Start(ctx)
	eventconsumers.NewCandleWorkerClient(&wg).Start(ctx)
	eventconsumers.NewRsiBotClient(&wg).Start(ctx)
	eventconsumers.NewTradingBot(&wg).Start(ctx)
	//eventconsumers.NewAccountWorkerClient(&wg).Start(ctx)
	eventconsumers.NewAccountWorkerClientFromFixtures(&wg, accountFixtures).Start(ctx)
	eventproducers.NewTrendSpiderClient(&wg, router).Start(ctx)

	log.Info("Main: init complete")

	// Block here until program is shut down
	<-stop

	// Signal -> shut down event clients
	cancel()

	// Wait for event clients to shut down
	wg.Wait()

	log.Info("Main: gracefully stopped!")
}
