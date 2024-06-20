package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdk_trace "go.opentelemetry.io/otel/sdk/trace"
	"gopkg.in/yaml.v3"

	exec_trade "github.com/jiaming2012/slack-trading/src/cmd/trade/run"
	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
	"github.com/jiaming2012/slack-trading/src/eventproducers/accountapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/alertapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/datafeedapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/optionsapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/signalapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/tradeapi"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/sheets"
	"github.com/jiaming2012/slack-trading/src/utils"
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
	Request  eventmodels.ApiRequest3
}

type RouterSetup struct {
	Router *mux.Router
	Prefix string
	Items  map[string]RouterSetupItem
}

func (r *RouterSetup) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		key := fmt.Sprintf("%v %v", req.Method, req.URL.Path)
		routerSetup, found := r.Items[key]
		if !found {
			log.Errorf("No handler found for %v", key)
			w.WriteHeader(500)
			return
		}

		eventproducers.ApiRequestHandler3(routerSetup.Request, routerSetup.Executor, w, req)
	} else {
		w.WriteHeader(404)
	}
}

func NewRouterSetup(prefix string, router *mux.Router) *RouterSetup {
	r := &RouterSetup{
		Router: router,
		Prefix: prefix,
		Items:  make(map[string]RouterSetupItem),
	}

	// router.HandleFunc(prefix, r.ServeHTTP)

	return r
}

func (r *RouterSetup) HandleFunc(path string, f func(http.ResponseWriter, *http.Request)) {
	// handleFunc is a replacement for mux.HandleFunc
	// which enriches the handler's HTTP instrumentation with the pattern as the http.route.
	handleFunc := func(pattern string, handlerFunc func(http.ResponseWriter, *http.Request)) {
		// Configure the "http.route" for the HTTP instrumentation.
		handler := otelhttp.WithRouteTag(pattern, http.HandlerFunc(handlerFunc))
		r.Router.Handle(fmt.Sprintf("%s%s", r.Prefix, path), handler)
	}

	handleFunc(path, f)
}

func (r *RouterSetup) Add(item RouterSetupItem) {
	key := fmt.Sprintf("%v %v%v", item.Method, r.Prefix, item.URL)
	r.Items[key] = item
	r.Router.HandleFunc(fmt.Sprintf("%s%s", r.Prefix, item.URL), r.ServeHTTP)
}

type RouterSetupHandler func(r *http.Request, request eventmodels.ApiRequest3) (chan interface{}, chan error)

func FindHighestEVPerExpiration(options []*eventmodels.OptionSpreadContractDTO) (long []*eventmodels.OptionSpreadContractDTO, short []*eventmodels.OptionSpreadContractDTO) {
	highestEVLongMap := make(map[time.Time]*eventmodels.OptionSpreadContractDTO)
	highestEVShortMap := make(map[time.Time]*eventmodels.OptionSpreadContractDTO)

	for _, option := range options {
		expiration, err := option.GetExpiration()
		if err != nil {
			log.Errorf("FindHighestEV: failed to get expiration: %v", err)
			continue
		}

		highestLongEV, found := highestEVLongMap[expiration]
		if found {
			if option.Stats.ExpectedProfitLong > highestLongEV.Stats.ExpectedProfitLong {
				highestEVLongMap[expiration] = option
			}
		} else {
			highestEVLongMap[expiration] = option
		}

		highestShortEV, found := highestEVShortMap[expiration]
		if found {
			if option.Stats.ExpectedProfitShort > highestShortEV.Stats.ExpectedProfitShort {
				highestEVShortMap[expiration] = option
			}
		} else {
			highestEVShortMap[expiration] = option
		}
	}

	var highestEVLong []*eventmodels.OptionSpreadContractDTO
	var highestEVShort []*eventmodels.OptionSpreadContractDTO

	for _, option := range highestEVLongMap {
		if option.Stats.ExpectedProfitLong > 0 {
			highestEVLong = append(highestEVLong, option)
		}
	}

	for _, option := range highestEVShortMap {
		if option.Stats.ExpectedProfitShort > 0 {
			highestEVShort = append(highestEVShort, option)
		}
	}

	return highestEVLong, highestEVShort
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
				highestEVLongCallSpreads, highestEVShortCallSpreads := FindHighestEVPerExpiration(calls)
				for _, spread := range highestEVLongCallSpreads {
					if spread != nil {
						log.WithField("event", "signal").Infof("Ignoring long call: %v", spread)
					} else {
						log.WithField("event", "signal").Infof("No Positive EV Long Call found")
					}
				}

				for _, spread := range highestEVShortCallSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						if err := tradierOrderExecuter.PlaceTradeSpread(event.Symbol, spread.LongOptionSymbol, spread.ShortOptionSymbol, 1, tag, goEnv); err != nil {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Call:: error placing trade: %v", err)
						}
					} else {
						log.WithField("event", "signal").Infof("No Positive EV Short Call found")
					}
				}
			} else {
				return fmt.Errorf("calls not found in result")
			}

			if puts, ok := options["puts"]; ok {
				highestEVLongPutSpreads, highestEVShortPutSpreads := FindHighestEVPerExpiration(puts)
				for _, spread := range highestEVLongPutSpreads {
					if spread != nil {
						log.WithField("event", "signal").Infof("Ignoring long put: %v", spread)
					} else {
						log.WithField("event", "signal").Infof("No Positive EV Long Put found")
					}
				}

				for _, spread := range highestEVShortPutSpreads {
					if spread != nil {
						requestedPrc := 0.0
						if spread.CreditReceived != nil {
							requestedPrc = *spread.CreditReceived
						}

						tag := utils.EncodeTag(event.Signal, spread.Stats.ExpectedProfitShort, requestedPrc)

						if err := tradierOrderExecuter.PlaceTradeSpread(event.Symbol, spread.LongOptionSymbol, spread.ShortOptionSymbol, 1, tag, goEnv); err != nil {
							return fmt.Errorf("tradierOrderExecuter.PlaceTradeSpread Put:: error placing trade: %v", err)
						}
					} else {
						log.WithField("event", "signal").Infof("No Positive EV Short Put found")
					}
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

type OptionYAML struct {
	Symbol                             string   `yaml:"symbol"`
	StartsAt                           string   `yaml:"startsAt"`
	EndsAt                             string   `yaml:"endsAt"`
	ExpirationsInDays                  []int    `yaml:"expirationsInDays"`
	MinDistanceBetweenStrikes          *float64 `yaml:"minDistanceBetweenStrikes,omitempty"`
	MinStandardDeviationBetweenStrikes *float64 `yaml:"minStandardDeviationBetweenStrikes,omitempty"`
	MaxNoOfStrikes                     int      `yaml:"maxNoOfStrikes"`
}

type OptionsConfigYAML struct {
	Options []OptionYAML `yaml:"options"`
}

func (o *OptionsConfigYAML) GetOption(symbol eventmodels.StockSymbol) (*OptionYAML, error) {
	sym1 := strings.ToLower(string(symbol))
	for _, option := range o.Options {
		sym2 := strings.ToLower(option.Symbol)
		if sym1 == sym2 {
			return &option, nil
		}
	}

	return nil, fmt.Errorf("OptionsConfigYAML: option not found")
}

// setupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func setupOTelSDK(ctx context.Context) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
	if err != nil {
		return nil, err
	}

	res, _ := resource.New(ctx, resource.WithAttributes(attribute.String("service.name", "grodt")))

	tracerProvider := sdk_trace.NewTracerProvider(
		sdk_trace.WithBatcher(traceExporter),
		sdk_trace.WithResource(res),
	)

	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)))
	if err != nil {
		handleErr(err)
		return
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
	if err != nil {
		log.Fatal(err)
	}

	return
}

type EmptyRequest struct {}

func (req *EmptyRequest) ParseHTTPRequest(r *http.Request) error {
	return nil
}

func (req *EmptyRequest) Validate(r *http.Request) error {
	return nil
}

func run() {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		log.Fatalf("missing PROJECTS_DIR environment variable")
	}

	goEnv := os.Getenv("GO_ENV")
	if goEnv == "" {
		log.Fatalf("missing GO_ENV environment variable")
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	eventpubsub.Init()

	// set up logger
	log.SetOutput(os.Stdout)
	log.SetFormatter(&log.JSONFormatter{})

	level, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	log.Infof("Log level set to %v", log.GetLevel())

	log.Infof("Main: you da boss...")

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

	// Set up Telemetry
	log.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	)))

	// Set up OpenTelemetry.
	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		log.Fatalf("failed to setup otel sdk: %v", err)
	}

	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Load config
	optionsConfigInDir := path.Join(projectsDir, "slack-trading", "src", "options-config.yaml")
	data, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		log.Fatalf("failed to read options config: %v", err)
	}

	var optionsConfig OptionsConfigYAML
	if err := yaml.Unmarshal(data, &optionsConfig); err != nil {
		log.Fatalf("failed to unmarshal options config: %v", err)
	}

	// Set up google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
		log.Fatalf("failed to create google sheets client: %v", err)
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
	// signalapi.SetupHandler(router.PathPrefix("/signals").Subrouter())
	datafeedapi.SetupHandler(router.PathPrefix("/datafeeds").Subrouter())
	alertapi.SetupHandler(router.PathPrefix("/alerts").Subrouter())

	optionChainRequestExector := &optionsapi.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            brokerBearerToken,
		GoEnv:                  goEnv,
	}

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

	// signals router
	signalsGetStateExecutor := signalapi.NewGetStateExecutor(trackerV3OptionEVConsumer)
	s := NewRouterSetup("/signals", router)
	s.Add(RouterSetupItem{Method: http.MethodGet, URL: "", Executor: signalsGetStateExecutor, Request: &EmptyRequest{}})

	// options router
	r := NewRouterSetup("/options", router)
	r.Add(RouterSetupItem{Method: http.MethodGet, URL: "", Executor: optionChainRequestExector, Request: &eventmodels.ReadOptionChainRequest{}})
	r.Add(RouterSetupItem{Method: http.MethodGet, URL: "/spreads", Executor: optionChainRequestExector, Request: &eventmodels.ReadOptionChainRequest{}})

	// Setup web server
	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	// Start web server
	go func() {
		log.Infof("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				log.Fatalf("failed to start server: %v", err)
			}
		}
	}()

	// Create channel for shutdown signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	// todo: move this, has to be before trackerV3OptionEVConsumer.Start(ctx)
	go func(eventCh <-chan eventconsumers.SignalTriggeredEvent, optionsRequestExecutor *optionsapi.ReadOptionChainRequestExecutor, config OptionsConfigYAML, isDryRun bool) {
		loc, err := time.LoadLocation("America/New_York")
		if err != nil {
			log.Panicf("failed to load location: %v", err)
		}

		tradierOrderExecuter := NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun)

		for event := range eventCh {
			log.WithField("event", "signal").Infof("tradier executer: %v triggered for %v", event.Signal, event.Symbol)

			optionConfig, err := config.GetOption(event.Symbol)
			if err != nil {
				log.Errorf("tradier executer: failed to get option config: %v", err)
				continue
			}

			startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", optionConfig.StartsAt, loc)
			if err != nil {
				log.Errorf("tradier executer: failed to parse startsAt: %v", err)
				return
			}

			endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", optionConfig.EndsAt, loc)
			if err != nil {
				log.Errorf("tradier executer: failed to parse endsAt: %v", err)
				return
			}

			req := &eventmodels.ReadOptionChainRequest{
				Symbol:                    event.Symbol,
				OptionTypes:               []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut},
				ExpirationsInDays:         optionConfig.ExpirationsInDays,
				MinDistanceBetweenStrikes: optionConfig.MinDistanceBetweenStrikes,
				MaxNoOfStrikes:            optionConfig.MaxNoOfStrikes,
				EV: &eventmodels.ReadOptionChainExpectedValue{
					StartsAt: startsAt,
					EndsAt:   endsAt,
				},
			}

			resultCh := make(chan map[string]interface{})
			errCh := make(chan error)

			req.EV.Signal = event.Signal

			go optionsRequestExecutor.ServeWithParams(req, true, resultCh, errCh)

			if err := SendHighestEVTradeToMarket(resultCh, errCh, event, tradierOrderExecuter, goEnv); err != nil {
				log.Errorf("tradier executer: %v: send to market failed: %v", event.Signal, err)
			}
		}
	}(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, optionsConfig, isDryRun)

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
	log.WithField("event", "signal").Infof("placing trade spread for %v", underlying)
	return exec_trade.PlaceTradeSpread(e.url, e.bearerToken, underlying, buyToOpenSymbol, sellToOpenSymbol, quantity, tag, e.dryRun)
}
