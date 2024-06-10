package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	exec_trade "slack-trading/src/cmd/trade/run"
	"slack-trading/src/eventconsumers"
	"slack-trading/src/eventmodels"
	"slack-trading/src/eventproducers"
	"slack-trading/src/eventproducers/accountapi"
	"slack-trading/src/eventproducers/alertapi"
	"slack-trading/src/eventproducers/datafeedapi"
	"slack-trading/src/eventproducers/optionsapi"
	"slack-trading/src/eventproducers/signalapi"
	"slack-trading/src/eventproducers/tradeapi"
	"slack-trading/src/eventpubsub"
	"slack-trading/src/sheets"
	"slack-trading/src/utils"
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
	run()
}

type RouterSetupItem struct {
	Method   string
	URL      string
	Executor eventmodels.RequestExecutor
}

type RouterSetup struct {
	Router    *mux.Router
	Prefix    string
	Executors map[string]eventmodels.RequestExecutor
}

func (r *RouterSetup) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		key := fmt.Sprintf("%v %v", req.Method, req.URL.Path)
		executer, found := r.Executors[key]
		if !found {
			log.Errorf("No handler found for %v", key)
			w.WriteHeader(500)
			return
		}

		eventproducers.ApiRequestHandler3(eventmodels.ReadOptionChainEvent, &eventmodels.ReadOptionChainRequest{}, &eventmodels.ReadOptionChainResponse{}, executer, w, req)
	} else {
		w.WriteHeader(404)
	}
}

func NewRouterSetup(prefix string, router *mux.Router) *RouterSetup {
	r := &RouterSetup{
		Router:    router,
		Prefix:    prefix,
		Executors: make(map[string]eventmodels.RequestExecutor),
	}

	// router.HandleFunc(prefix, r.ServeHTTP)

	return r
}

func (r *RouterSetup) Add(item RouterSetupItem) {
	key := fmt.Sprintf("%v %v%v", item.Method, r.Prefix, item.URL)
	r.Executors[key] = item.Executor
	r.Router.HandleFunc(fmt.Sprintf("%s%s", r.Prefix, item.URL), r.ServeHTTP)
}

type RouterSetupHandler func(r *http.Request, request eventmodels.ApiRequest3) (chan interface{}, chan error)

func FindHighestEV(options []*eventmodels.OptionSpreadContractDTO) (long *eventmodels.OptionSpreadContractDTO, short *eventmodels.OptionSpreadContractDTO) {
	var highestEVLong, highestEVShort float64
	for _, option := range options {
		if option.Stats.ExpectedProfitLong > highestEVLong {
			highestEVLong = option.Stats.ExpectedProfitLong
			long = option
		}
		if option.Stats.ExpectedProfitShort > highestEVShort {
			highestEVShort = option.Stats.ExpectedProfitShort
			short = option
		}
	}

	return
}

func SendHighestEVTradeToMarket(resultCh chan map[string]interface{}, errCh chan error, event eventconsumers.SignalTriggeredEvent, tradierOrderExecuter *TradierOrderExecuter, goEnv string) error {
	select {
	case result := <-resultCh:
		if result != nil {

			options, ok := result["options"].(map[string][]*eventmodels.OptionSpreadContractDTO)
			if !ok {
				return fmt.Errorf("options not found in result")
			}

			if calls, ok := options["calls"]; ok {
				highestEVLongCall, highestEVShortCall := FindHighestEV(calls)
				if highestEVLongCall != nil {
					log.Infof("Ignoring long call: %v", highestEVLongCall)
				} else {
					log.Infof("No Positive EV Long Call found")
				}

				if highestEVShortCall != nil {
					requestedPrc := 0.0
					if highestEVShortCall.CreditReceived != nil {
						requestedPrc = *highestEVShortCall.CreditReceived
					}

					tag := utils.EncodeTag(event.Signal, highestEVShortCall.Stats.ExpectedProfitShort, requestedPrc)

					if err := tradierOrderExecuter.PlaceTradeSpread(event.Symbol, highestEVShortCall.LongOptionSymbol, highestEVShortCall.ShortOptionSymbol, 1, tag, goEnv); err != nil {
						return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Call:: error placing trade: %v", err)
					}
				} else {
					log.Infof("No Positive EV Short Call found")
				}
			} else {
				return fmt.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				highestEVLongPut, highestEVShortPut := FindHighestEV(puts)
				if highestEVLongPut != nil {
					log.Infof("Ignoring long put: %v", highestEVLongPut)
				} else {
					log.Infof("No Positive EV Long Put found")
				}

				if highestEVShortPut != nil {
					requestedPrc := 0.0
					if highestEVShortPut.CreditReceived != nil {
						requestedPrc = *highestEVShortPut.CreditReceived
					}

					tag := utils.EncodeTag(event.Signal, highestEVShortPut.Stats.ExpectedProfitShort, requestedPrc)

					if err := tradierOrderExecuter.PlaceTradeSpread(event.Symbol, highestEVShortPut.LongOptionSymbol, highestEVShortPut.ShortOptionSymbol, 1, tag, goEnv); err != nil {
						return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: error placing trade: %v", err)
					}
				} else {
					log.Infof("No Positive EV Short Put found")
				}
			} else {
				return fmt.Errorf("puts not found in result")
			}
		}

	case err := <-errCh:
		return fmt.Errorf("error: %v", err)
	}

	return nil
}

func run() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		panic("missing PROJECTS_DIR environment variable")
	}

	goEnv := os.Getenv("GO_ENV")
	if goEnv == "" {
		panic("missing GO_ENV environment variable")
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	eventpubsub.Init()

	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	log.Infof("Log level set to %v", log.GetLevel())

	// Get env
	stockQuotesURL := os.Getenv("STOCK_QUOTES_URL")
	calendarURL := os.Getenv("MARKET_CALENDAR_URL")
	optionChainURL := os.Getenv("OPTION_CHAIN_URL")

	brokerBearerToken := os.Getenv("TRADIER_BEARER_TOKEN")
	slackWebhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	// quotesAccountID := os.Getenv("TRADIER_ACCOUNT_ID")
	tradesAccountID := os.Getenv("TRADIER_TRADES_ACCOUNT_ID")
	// tradierOrdersURL := fmt.Sprintf(os.Getenv("TRADIER_ORDERS_URL_TEMPLATE"), quotesAccountID)
	tradierTradesOrderURL := fmt.Sprintf(os.Getenv("TRADIER_TRADES_URL_TEMPLATE"), tradesAccountID)
	tradierTradesBearerToken := os.Getenv("TRADIER_TRADES_BEARER_TOKEN")
	eventStoreDbURL := os.Getenv("EVENTSTOREDB_URL")
	oandaFxQuotesURLBase := os.Getenv("OANDA_FX_QUOTES_URL_BASE")
	oandaBearerToken := os.Getenv("OANDA_BEARER_TOKEN")
	optionsExpirationURL := os.Getenv("OPTION_EXPIRATIONS_URL")
	isDryRun := strings.ToLower(os.Getenv("DRY_RUN")) == "true"

	// Set up google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
		panic(err)
	}

	// Setup router
	port := os.Getenv("PORT")
	if len(port) == 0 {
		log.Fatal("$PORT must be set")
	}

	// Setup dispatcher
	dispatcher := eventmodels.InitializeGlobalDispatcher()
	router := mux.NewRouter()
	tradeapi.SetupHandler(router.PathPrefix("/trades").Subrouter())
	accountapi.SetupHandler(router.PathPrefix("/accounts").Subrouter())
	signalapi.SetupHandler(router.PathPrefix("/signals").Subrouter())
	datafeedapi.SetupHandler(router.PathPrefix("/datafeeds").Subrouter())
	alertapi.SetupHandler(router.PathPrefix("/alerts").Subrouter())

	optionChainRequestExector := &optionsapi.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            brokerBearerToken,
		GoEnv:                  goEnv,
	}

	r := NewRouterSetup("/options", router)
	r.Add(RouterSetupItem{Method: http.MethodGet, URL: "", Executor: optionChainRequestExector})
	r.Add(RouterSetupItem{Method: http.MethodGet, URL: "/spreads", Executor: optionChainRequestExector})

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

	streamParams := []eventmodels.StreamParameter{
		{StreamName: eventmodels.AccountsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionAlertsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	optionContractClient := eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, &eventmodels.OptionContractV1{})
	optionContractClient.Start(ctx)

	// todo: both TrackerV1 and TrackerV2 should be processed
	// todo: stream_version should be stored in eventstoredb UserMetadata field
	// todo: the eventstore metadata field should be queried so that we can process and combine multiple versions of the same stream
	trackersClient := eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, &eventmodels.TrackerV3{})
	trackersClient.Start(ctx)

	// TrackerV3 client for generating option EV signals
	trackersClientV3 := eventconsumers.NewESDBConsumerStream(&wg, eventStoreDbURL, &eventmodels.TrackerV3{})
	trackerV3OptionEVConsumer := eventconsumers.NewTrackerV3Consumer(trackersClientV3)

	// todo: move this, has to be before trackerV3OptionEVConsumer.Start(ctx)
	go func(eventCh <-chan eventconsumers.SignalTriggeredEvent, optionsRequestExecutor *optionsapi.ReadOptionChainRequestExecutor, isDryRun bool) {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Errorf("failed to load location: %v", err)
			return
		}

		startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", "2024-03-01T09:30:00", loc)
		if err != nil {
			log.Errorf("failed to parse startsAt: %v", err)
			return
		}

		endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", "2024-05-31T16:00:00", loc)
		if err != nil {
			log.Errorf("failed to parse endsAt: %v", err)
			return
		}

		tradierOrderExecuter := NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun)

		for event := range eventCh {
			log.Infof("%v triggered for %v", event.Signal, event.Symbol)

			req := &eventmodels.ReadOptionChainRequest{
				Symbol:                    event.Symbol,
				OptionTypes:               []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut},
				ExpirationsInDays:         []int{0, 1},
				MinDistanceBetweenStrikes: 10,
				MaxNoOfStrikes:            4,
				EV: &eventmodels.ReadOptionChainExpectedValue{
					StartsAt: startsAt,
					EndsAt:   endsAt,
				},
			}

			if event.Signal == eventmodels.SuperTrend4h1hStochRsi15mDown {
				resultCh := make(chan map[string]interface{})
				errCh := make(chan error)

				req.EV.Signal = string(eventmodels.SuperTrend4h1hStochRsi15mDown)

				go optionsRequestExecutor.ServeWithParams(req, true, resultCh, errCh)

				if err := SendHighestEVTradeToMarket(resultCh, errCh, event, tradierOrderExecuter, goEnv); err != nil {
					log.Errorf("SuperTrend4h1hStochRsi15mDown: send to market failed: %v", err)
				}
			}

			if event.Signal == eventmodels.SuperTrend4h1hStochRsi15mUp {
				resultCh := make(chan map[string]interface{})
				errCh := make(chan error)

				req.EV.Signal = string(eventmodels.SuperTrend4h1hStochRsi15mUp)

				go optionsRequestExecutor.ServeWithParams(req, true, resultCh, errCh)

				if err := SendHighestEVTradeToMarket(resultCh, errCh, event, tradierOrderExecuter, goEnv); err != nil {
					log.Errorf("SuperTrend4h1hStochRsi15mUp: send to market failed: %v", err)
				}
			}
		}
	}(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, isDryRun)

	trackerV3OptionEVConsumer.Start(ctx)

	eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)
	eventconsumers.NewTradierOrdersMonitoringWorker(&wg, tradierTradesOrderURL, tradierTradesBearerToken).Start(ctx)

	// Start event clients
	eventconsumers.NewOptionChainTickWriterWorker(&wg, stockQuotesURL, optionChainURL, brokerBearerToken, calendarURL).Start(ctx, optionContractClient, trackersClient)

	fxTicksCh := make(chan *eventmodels.FxTick)
	eventconsumers.NewOandaFxTickWriter(&wg, trackersClient, oandaFxQuotesURLBase, oandaBearerToken).Start(ctx, fxTicksCh)

	//eventproducers.NewReportClient(&wg).Start(ctx)
	eventproducers.NewSlackClient(&wg, router).Start(ctx)
	// eventproducers.NewCoinbaseClient(&wg, router).Start(ctx)
	// eventproducers.NewIBClient(&wg, iBServerURL).Start(ctx, "CL")
	//eventconsumers.NewTradeExecutorClient(&wg).Start(ctx)
	//eventconsumers.NewGoogleSheetsClient(ctx, &wg).Start()
	eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)
	//eventconsumers.NewBalanceWorkerClient(&wg).Start(ctx)
	//eventconsumers.NewCandleWorkerClient(&wg).Start(ctx)
	//eventconsumers.NewRsiBotClient(&wg).Start(ctx)
	eventconsumers.NewGlobalDispatcherWorkerClient(&wg, dispatcher).Start(ctx)
	eventconsumers.NewAccountWorkerClient(&wg).Start(ctx)
	// eventproducers.NewTrendSpiderClient(&wg, router).Start(ctx)
	eventproducers.NewESDBProducer(&wg, eventStoreDbURL, streamParams).Start(ctx, fxTicksCh)

	// todo: add back in
	// for _, streamParam := range streamParams {
	// 	eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, []eventmodels.StreamParameter{streamParam}).Start(ctx)
	// }

	eventconsumers.NewOptionAlertWorker(&wg, tradierTradesOrderURL, tradierTradesBearerToken).Start(ctx)

	log.Info("Main: init complete")

	// Block here until program is shut down
	<-stop

	// EntrySignal -> shut down event clients
	cancel()

	// Wait for event clients to shut down
	wg.Wait()

	log.Info("Main: gracefully stopped!")
}

type TradierOrderExecuter struct {
	url         string
	bearerToken string
	dryRun      bool
}

func NewTradierOrderExecuter(url, bearerToken string, dryRun bool) *TradierOrderExecuter {
	return &TradierOrderExecuter{url: url, bearerToken: bearerToken, dryRun: dryRun}
}

func (e *TradierOrderExecuter) PlaceTradeSpread(underlying eventmodels.StockSymbol, buyToOpenSymbol eventmodels.OptionSymbol, sellToOpenSymbol eventmodels.OptionSymbol, quantity int, tag string, goEnv string) error {
	return exec_trade.PlaceTradeSpread(e.url, e.bearerToken, underlying, buyToOpenSymbol, sellToOpenSymbol, quantity, tag, e.dryRun)
}
