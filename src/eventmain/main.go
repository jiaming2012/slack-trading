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
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventproducers/accountapi"
	"slack-trading/src/eventproducers/tradeapi"
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
/condition add Trendline break to MrTrendy TrendPursuit with params BTCUSD(transform trendspider symbol COINBASE:^BTCUSD to BTCUSD??) m5 trendline-break bounce up 27000
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

	// Setup dispatcher
	dispatcher := eventmodels.InitializeGlobalDispatcher()
	router := mux.NewRouter()
	tradeapi.SetupHandler(router.PathPrefix("/trades").Subrouter())
	accountapi.SetupHandler(router.PathPrefix("/accounts").Subrouter())

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
	balance := 2000.0
	priceLevels := []*models.PriceLevel{
		{
			Price: 24000.0,
		},
		{
			Price:                25000.0,
			MaxNoOfTrades:        3,
			AllocationPercent:    0.5,
			StopLoss:             26000.0,
			MinimumTradeDistance: 0,
		},
		{
			Price:                27000.0,
			MaxNoOfTrades:        2,
			AllocationPercent:    0.5,
			StopLoss:             28000.0,
			MinimumTradeDistance: 0,
		},
	}

	accountFixture, err := models.NewAccount("playground", balance)
	if err != nil {
		log.Fatal(err)
	}

	accountFixtures := []*models.Account{
		accountFixture,
	}

	trendlineBreakStrategyFixture, err := models.NewStrategy("trendline-break", "BTC-USD", models.Down, balance, priceLevels)
	if err != nil {
		log.Fatal(err)
	}

	signal1 := models.TrendLineBreakSignal{
		Name:      "htf-trendline-break",
		Price:     25884.3, // todo: remove price from signal
		Direction: models.Down,
	}

	signal2 := models.MovingAverageBreakSignal{
		Name:      "small-ma-retrace",
		Price:     27990.4,
		Direction: models.Up,
	}

	trendlineBreakStrategyFixture.AddCondition(signal1)
	trendlineBreakStrategyFixture.AddCondition(signal2)

	if err = accountFixtures[0].AddStrategy(*trendlineBreakStrategyFixture); err != nil {
		log.Fatal(err)
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
	eventconsumers.NewGlobalDispatcherWorkerClient(&wg, dispatcher).Start(ctx)
	//eventconsumers.NewTradingBot(&wg).Start(ctx)
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
