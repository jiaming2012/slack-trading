package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/EventStore/EventStore-Client-Go/esdb"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"slack-trading/src/eventconsumers"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventproducers/accountapi"
	"slack-trading/src/eventproducers/alertapi"
	"slack-trading/src/eventproducers/datafeedapi"
	"slack-trading/src/eventproducers/signalapi"
	"slack-trading/src/eventproducers/tradeapi"
	"slack-trading/src/eventpubsub"
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

func loadAccountFixtures2(datafeed *eventmodels.Datafeed) (*eventmodels.Account, error) {
	// todo: fetch from database
	balance := 2000.0
	priceLevels := []*eventmodels.PriceLevel{
		{
			Price:                63.605,
			MaxNoOfTrades:        8,
			AllocationPercent:    0.7,
			StopLoss:             63.0,
			MinimumTradeDistance: 0.5,
		},
		{
			Price:                83.485,
			MaxNoOfTrades:        3,
			AllocationPercent:    0.3,
			StopLoss:             71,
			MinimumTradeDistance: 0.5,
		},
		{
			Price: 93.729,
		},
	}

	account, err := eventmodels.NewAccount("oil playground", balance, datafeed)
	if err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	strategy1, err := eventmodels.NewStrategyDeprecated("elliot wave", "/CL", eventmodels.Up, balance, priceLevels, account)
	if err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	now := time.Now().UTC()
	entrySignal := eventmodels.NewSignalV2("push_up", now)
	resetSignal := eventmodels.NewResetSignal("stall", entrySignal, now)

	strategy1.AddEntryCondition(entrySignal, resetSignal)

	exitSignal1 := eventmodels.NewSignalV2("sqzmom_pivot_down", now)
	exitSignals := []*eventmodels.ExitSignal{
		{
			Signal:      exitSignal1,
			ResetSignal: eventmodels.NewResetSignal("sqzmom_pivot_up", exitSignal1, now),
		},
	}

	exitReentrySignals := []*eventmodels.SignalV2{
		eventmodels.NewSignalV2("sqzmom_pivot_up", now),
	}

	exitConstraint1 := eventmodels.NewExitSignalConstraint("PL(levelIndex) > 0", eventmodels.PriceLevelProfitLossAboveZeroConstraint)
	exitConstraints := []*eventmodels.ExitSignalConstraint{exitConstraint1}

	maxTriggerCount := 3
	strategy1.AddExitCondition("momentum_level_0", 0, exitSignals, exitReentrySignals, exitConstraints, .25, &maxTriggerCount)
	strategy1.AddExitCondition("momentum_level_1", 1, exitSignals, exitReentrySignals, exitConstraints, .25, &maxTriggerCount)

	if err = account.AddStrategy(strategy1); err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	return account, nil
}

func loadAccountFixtures1(datafeed *eventmodels.Datafeed) (*eventmodels.Account, error) {
	// todo: fetch from database
	balance := 2000.0
	priceLevels := []*eventmodels.PriceLevel{
		{
			Price: 23866.0,
		},
		{
			Price:                30327.0,
			MaxNoOfTrades:        8,
			AllocationPercent:    0.7,
			StopLoss:             33681.0,
			MinimumTradeDistance: 15,
		},
		{
			Price:                33681.0,
			MaxNoOfTrades:        3,
			AllocationPercent:    0.3,
			StopLoss:             33681.0,
			MinimumTradeDistance: 50,
		},
	}

	account, err := eventmodels.NewAccount("btc playground", balance, datafeed)
	if err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	strategy1, err := eventmodels.NewStrategyDeprecated("rsi-crossed-over-upper-band", "BTC-USD", eventmodels.Down, balance, priceLevels, account)
	if err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	//entrySignal1 := eventmodels.NewSignalV2("rsi_crossed_under_rsiMA_above_the_overbought_line")
	//resetSignal1 := eventmodels.NewSignalV2("reset_rsiMA_crossed")

	//entrySignal2 := eventmodels.NewSignalV2("rsi_crossed_over_rsiMA_below_the_oversold_line")
	//resetSignal2 := eventmodels.NewSignalV2("reset_rsiMA_crossed")

	now := time.Now().UTC()
	entrySignal3 := eventmodels.NewSignalV2("rsi_crossed_over_upper_band", now)
	resetSignal3 := eventmodels.NewResetSignal("reset_rsi_crossed", entrySignal3, now)

	//entrySignal4 := eventmodels.NewSignalV2("rsi_crossed_under_upper_band")
	//resetSignal4 := eventmodels.NewSignalV2("reset_rsi_crossed")

	//strategy1.AddEntryCondition(entrySignal1, resetSignal1)
	//strategy1.AddEntryCondition(entrySignal2, resetSignal2)
	strategy1.AddEntryCondition(entrySignal3, resetSignal3)
	//strategy1.AddEntryCondition(entrySignal4, resetSignal4)

	exitSignal1 := eventmodels.NewSignalV2("rsi_crossed_over_rsiMA_below_the_oversold_line", now)
	exitSignals := []*eventmodels.ExitSignal{
		{
			Signal:      exitSignal1,
			ResetSignal: eventmodels.NewResetSignal("reset_rsiMA_crossed", exitSignal1, now),
		},
		//{
		//	Signal:      eventmodels.NewSignalV2("m5-bollinger-touch-below"),
		//	ResetSignal: eventmodels.NewSignalV2("m5-bollinger-touch-above"),
		//},
	}

	//exitReentrySignals := []*eventmodels.SignalV2{
	//	eventmodels.NewSignalV2("rsi_crossed_under_rsiMA_above_the_overbought_line", now),
	//}

	exitReentrySignals := []*eventmodels.SignalV2{
		eventmodels.NewSignalV2("reset_rsiMA_crossed", now),
	}

	exitConstraint1 := eventmodels.NewExitSignalConstraint("PL(levelIndex) > 0", eventmodels.PriceLevelProfitLossAboveZeroConstraint)
	exitConstraints := []*eventmodels.ExitSignalConstraint{exitConstraint1}

	maxTriggerCount := 3
	strategy1.AddExitCondition("momentum_level_0", 0, exitSignals, exitReentrySignals, exitConstraints, .5, &maxTriggerCount)
	strategy1.AddExitCondition("momentum_level_1", 1, exitSignals, exitReentrySignals, exitConstraints, .5, &maxTriggerCount)

	if err = account.AddStrategy(strategy1); err != nil {
		return nil, fmt.Errorf("loadAccountFixtures: %w", err)
	}

	return account, nil
}

type TestEvent struct {
	Id            string `json:"id"`
	ImportantData string `json:"importantData"`
}

func eventSourceDBSetup() {
	// settings, err := esdb.ParseConnectionString("esdb://localhost:2113?tls=false")
	settings, err := esdb.ParseConnectionString("esdb+discover://localhost:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000")

	if err != nil {
		panic(err)
	}

	db, err := esdb.NewClient(settings)

	// ---- read event ----
	stream, err := db.ReadStream(context.Background(), "some-stream", esdb.ReadStreamOptions{}, 10)

	if err != nil {
		panic(err)
	}

	defer stream.Close()

	for {
		event, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			panic(err)
		}

		// Doing something productive with the event
		fmt.Println(event.Event.EventNumber)
		fmt.Println(event.Event.EventType)
		fmt.Println(event.Event.EventID)

		var testEvent TestEvent
		err = json.Unmarshal(event.Event.Data, &testEvent)
		if err != nil {
			panic(err)
		}

		fmt.Println(testEvent)
	}

	// ---- write event ----
	// testEvent := TestEvent{
	// 	Id:            uuid.Must(uuid.NewV4()).String(),
	// 	ImportantData: "I wrote my first event!",
	// }

	// data, err := json.Marshal(testEvent)

	// if err != nil {
	// 	panic(err)
	// }

	// eventData := esdb.EventData{
	// 	ContentType: esdb.JsonContentType,
	// 	EventType:   "TestEvent",
	// 	Data:        data,
	// }

	// _, err = db.AppendToStream(context.Background(), "some-stream", esdb.AppendToStreamOptions{}, eventData)

	// if err != nil {
	// 	panic(err)
	// }

}

func run2() {
	eventSourceDBSetup()
}

func main() {
	run()
}

func run() {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup

	// todo: move to environment variables
	// Constants
	// iBServerURL := "wss://localhost:5000/v1/api/ws"
	eventStoreDbURL := "esdb+discover://localhost:2113?tls=false&keepAliveTimeout=10000&keepAliveInterval=10000"

	// Set up logger
	log.SetLevel(log.DebugLevel)

	// Set up event bus
	eventpubsub.Init()

	// Set up google sheets
	//if err := sheets.Init(ctx); err != nil {
	//	panic(err)
	//}

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
	signalapi.SetupHandler(router.PathPrefix("/signals").Subrouter())
	datafeedapi.SetupHandler(router.PathPrefix("/datafeeds").Subrouter())
	alertapi.SetupHandler(router.PathPrefix("/alerts").Subrouter())
	// strategyapi.SetupHandler(router.PathPrefix("/strategies").Subrouter())

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

	// Create datafeeds
	// coinbaseDatafeed := eventmodels.NewDatafeed(eventmodels.CoinbaseDatafeed)
	// ibDatafeed := eventmodels.NewDatafeed(eventmodels.IBDatafeed)
	// manualDatafeed := eventmodels.NewDatafeed(eventmodels.ManualDatafeed)

	// Load account fixtures
	// account1, err := loadAccountFixtures1(coinbaseDatafeed)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// account2, err := loadAccountFixtures2(ibDatafeed)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// accounts := []*eventmodels.Account{account1, account2}
	// accounts := make([]*eventmodels.Account, 0)

	// Start event clients
	//eventproducers.NewReportClient(&wg).Start(ctx)
	eventproducers.NewSlackClient(&wg, router).Start(ctx)
	// eventproducers.NewCoinbaseClient(&wg, router).Start(ctx)
	// eventproducers.NewIBClient(&wg, iBServerURL).Start(ctx, "CL")
	//eventconsumers.NewTradeExecutorClient(&wg).Start(ctx)
	//eventconsumers.NewGoogleSheetsClient(ctx, &wg).Start()
	eventconsumers.NewSlackNotifierClient(&wg).Start(ctx)
	//eventconsumers.NewBalanceWorkerClient(&wg).Start(ctx)
	//eventconsumers.NewCandleWorkerClient(&wg).Start(ctx)
	//eventconsumers.NewRsiBotClient(&wg).Start(ctx)
	eventconsumers.NewGlobalDispatcherWorkerClient(&wg, dispatcher).Start(ctx)
	// eventconsumers.NewAccountWorkerClientFromFixtures(&wg, accounts, coinbaseDatafeed, ibDatafeed, manualDatafeed).Start(ctx)
	// eventproducers.NewTrendSpiderClient(&wg, router).Start(ctx)
	eventproducers.NewEventStoreDBClient(&wg).Start(ctx, eventStoreDbURL)

	eventconsumers.NewOptionAlertWorker(&wg).Start(ctx)

	log.Info("Main: init complete")

	// Block here until program is shut down
	<-stop

	// EntrySignal -> shut down event clients
	cancel()

	// Wait for event clients to shut down
	wg.Wait()

	log.Info("Main: gracefully stopped!")
}
