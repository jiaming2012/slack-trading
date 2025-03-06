package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
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
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	backtester_router "github.com/jiaming2012/slack-trading/src/backtester-api/router"
	"github.com/jiaming2012/slack-trading/src/backtester-api/rpc"
	"github.com/jiaming2012/slack-trading/src/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/data"
	"github.com/jiaming2012/slack-trading/src/dbutils"
	"github.com/jiaming2012/slack-trading/src/eventconsumers"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/eventproducers"
	"github.com/jiaming2012/slack-trading/src/eventproducers/accountapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/alertapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/datafeedapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/signalapi"
	"github.com/jiaming2012/slack-trading/src/eventproducers/tradeapi"
	"github.com/jiaming2012/slack-trading/src/eventpubsub"
	"github.com/jiaming2012/slack-trading/src/eventservices"
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
	key := fmt.Sprintf("%v %v", req.Method, req.URL.Path)
	routerSetup, found := r.Items[key]
	if !found {
		log.Errorf("No handler found for %v", key)
		w.WriteHeader(404)
		return
	}

	eventproducers.ApiRequestHandler3(req.Context(), routerSetup.Request, routerSetup.Executor, w, req)
}

func NewRouterSetup(prefix string, router *mux.Router) *RouterSetup {
	r := &RouterSetup{
		Router: router,
		Prefix: prefix,
		Items:  make(map[string]RouterSetupItem),
	}

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
		log.Fatalf("runtime.Start: %v", err)
	}

	return
}

func processSignalTriggeredEvent(event eventmodels.SignalTriggeredEvent, tradierOrderExecuter *eventmodels.TradierOrderExecuter, optionsRequestExecutor *eventmodels.ReadOptionChainRequestExecutor, projectsDir string, config eventmodels.OptionsConfigYAML, riskProfileConstraint *eventmodels.RiskProfileConstraint, loc *time.Location, goEnv string) error {
	tracer := otel.GetTracerProvider().Tracer("main:signal")
	ctx, span := tracer.Start(event.Ctx, "<- SignalTriggeredEvent")
	defer span.End()

	logger := log.WithContext(ctx)

	logger.WithField("event", "signal").Infof("tradier executer: %v triggered for %v", event.Signal, event.Symbol)

	optionConfig, err := config.GetOption(event.Symbol)
	if err != nil {
		return fmt.Errorf("tradier executer: failed to get option config: %v", err)
	}

	startsAt, err := time.ParseInLocation("2006-01-02T15:04:05", optionConfig.StartsAt, loc)
	if err != nil {
		return fmt.Errorf("tradier executer: failed to parse startsAt: %v", err)
	}

	endsAt, err := time.ParseInLocation("2006-01-02T15:04:05", optionConfig.EndsAt, loc)
	if err != nil {
		return fmt.Errorf("tradier executer: failed to parse endsAt: %v", err)
	}

	span.SetAttributes(attribute.String("symbol", string(event.Symbol)), attribute.String("startsAt", startsAt.String()), attribute.String("endsAt", endsAt.String()))

	req := &eventmodels.ReadOptionChainRequest{
		Symbol:                    event.Symbol,
		OptionTypes:               []eventmodels.OptionType{eventmodels.OptionTypeCall, eventmodels.OptionTypePut},
		ExpirationsInDays:         optionConfig.ExpirationsInDays,
		MinDistanceBetweenStrikes: optionConfig.MinDistanceBetweenStrikes,
		MaxNoOfStrikes:            optionConfig.MaxNoOfStrikes,
		EV: &eventmodels.ReadOptionChainExpectedValue{
			StartsAt: startsAt,
			EndsAt:   endsAt,
			Signal:   event.Signal,
		},
		IsHistorical: false,
	}

	resultCh := make(chan map[string]interface{})
	errCh := make(chan error)

	req.EV.Signal = event.Signal

	// instead of finding the next friday, we can just use the expiration date from the config yaml
	nextOptionExpDate := utils.DeriveNextFriday(event.Timestamp)
	// nextOptionExpDate := utils.DeriveNextExpiration(event.Timestamp, optionConfig.ExpirationsInDays)

	data, err := optionsRequestExecutor.OptionsDataFetcher.FetchOptionChainDataInput(req.Symbol, req.IsHistorical, event.Timestamp, event.Timestamp, nextOptionExpDate, req.MaxNoOfStrikes, *req.MinDistanceBetweenStrikes, req.ExpirationsInDays)
	if err != nil {
		return fmt.Errorf("tradier executer: %v: failed to collect data: %v", event.Signal, err)
	}

	if data == nil {
		return fmt.Errorf("tradier executer: %v: failed to collect data", event.Signal)
	}

	if len(data.OptionContracts) == 0 {
		return fmt.Errorf("tradier executer: %v: no option chain data", event.Signal)
	}

	go optionsRequestExecutor.ServeWithParams(ctx, req, *data, true, projectsDir, time.Now(), resultCh, errCh)

	if err := eventconsumers.SendHighestEVTradeToMarket(ctx, resultCh, errCh, event, tradierOrderExecuter, riskProfileConstraint, optionConfig.MaxNoOfPositions, goEnv); err != nil {
		log.Errorf("tradier executer: %v: send to market failed: %v", event.Signal, err)
	}

	return nil
}

func getTradierBrokers() (map[models.CreateAccountRequestSource]models.IBroker, error) {
	brokers := make(map[models.CreateAccountRequestSource]models.IBroker)
	brokerName := "tradier"

	for _, accountType := range []models.LiveAccountType{models.LiveAccountTypePaper, models.LiveAccountTypeMargin} {
		vars := models.NewLiveAccountVariables(accountType)

		// tradierBalancesUrlTemplate, err := vars.GetTradierBalancesUrlTemplate()
		// if err != nil {
		// 	return nil, fmt.Errorf("failed to get tradier balances url template: %w", err)
		// }

		accountID, err := vars.GetTradierTradesAccountID()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier account id: %w", err)
		}

		tradierTradesBearerToken, err := vars.GetTradierTradesBearerToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier trades bearer token: %w", err)
		}

		// balancesUrl := fmt.Sprintf(tradierBalancesUrlTemplate, accountID)

		// accountSource := models.LiveAccountSource{
		// 	Broker:       brokerName,
		// 	AccountID:    accountID,
		// 	AccountType:  accountType,
		// 	BalancesUrl:  balancesUrl,
		// 	TradesApiKey: tradierTradesBearerToken,
		// }

		tradierTradesUrlTemplate, err := vars.GetTradierTradesUrlTemplate()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier trades url template: %w", err)
		}

		tradesUrl := fmt.Sprintf(tradierTradesUrlTemplate, accountID)

		stockQuotesURL, err := utils.GetEnv("TRADIER_STOCK_QUOTES_URL")
		if err != nil {
			return nil, fmt.Errorf("$TRADIER_STOCK_QUOTES_URL not set: %v", err)
		}

		tradierNonTradesBearerToken, err := vars.GetTradierNonTradesBearerToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier non trades bearer token: %w", err)
		}

		source := services.NewLiveAccountSource(brokerName, accountID, tradesUrl, tradierTradesBearerToken, accountType)

		broker := services.NewTradierBroker(tradesUrl, stockQuotesURL, tradierNonTradesBearerToken, tradierTradesBearerToken, &source)

		brokers[models.CreateAccountRequestSource{
			AccountType: accountType,
			Broker:      brokerName,
			AccountID:   accountID,
		}] = broker
	}

	return brokers, nil
}

var db *gorm.DB

func run() {
	projectsDir, err := utils.GetEnv("PROJECTS_DIR")
	if err != nil {
		log.Fatalf("PROJECTS_DIR not set: %v", err)
	}

	goEnv, err := utils.GetEnv("GO_ENV")
	if goEnv == "" {
		log.Fatalf("GO_ENV not set: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	if err := utils.InitEnvironmentVariables(projectsDir, goEnv); err != nil {
		log.Panic(err)
	}

	eventpubsub.Init()

	// set up logger
	// lokiClient, err := lokiclient.NewClientFromEnv()

	log.SetOutput(os.Stdout)

	log.Infof("Log level set to %v", log.GetLevel())

	log.Infof("Main: you da boss...")

	// Get env
	var liveAccountType models.LiveAccountType
	if goEnv == "production" {
		liveAccountType = models.LiveAccountTypeMargin
	} else {
		liveAccountType = models.LiveAccountTypePaper
	}

	// todo: this needs to be updated to be dynamic: a live trade can be paper or margin
	vars := models.NewLiveAccountVariables(liveAccountType)

	stockQuotesURL, err := utils.GetEnv("TRADIER_STOCK_QUOTES_URL")
	if err != nil {
		log.Fatalf("$TRADIER_STOCK_QUOTES_URL not set: %v", err)
	}

	calendarURL, err := utils.GetEnv("TRADIER_MARKET_CALENDAR_URL")
	if err != nil {
		log.Fatalf("$TRADIER_MARKET_CALENDAR_URL not set: %v", err)
	}

	optionChainURL, err := utils.GetEnv("TRADIER_OPTION_CHAIN_URL")
	if err != nil {
		log.Fatalf("$TRADIER_OPTION_CHAIN_URL not set: %v", err)
	}

	slackWebhookURL, err := utils.GetEnv("SLACK_OPTION_ALERTS_WEBHOOK_URL")
	if err != nil {
		log.Fatalf("$SLACK_OPTION_ALERTS_WEBHOOK_URL not set: %v", err)
	}

	polygonApiKey, err := utils.GetEnv("POLYGON_API_KEY")
	if err != nil {
		log.Fatalf("$POLYGON_API_KEY not set: %v", err)
	}

	tradierTradesOrderURL, err := vars.GetTradierTradesOrderURL()
	if err != nil {
		log.Fatalf("tradierTradesOrderURL could not be set: %v", err)
	}
	_ = tradierTradesOrderURL

	tradierPositionsUrlTemplate, err := vars.GetTradierPositionsUrlTemplate()
	if err != nil {
		log.Fatalf("$TRADIER_POSITIONS_URL_TEMPLATE not set: %v", err)
	}

	tradesAccountID, err := vars.GetTradierTradesAccountID()
	if err != nil {
		log.Fatalf("$TRADIER_TRADES_ACCOUNT_ID not set: %v", err)
	}

	tradierPositionsURL := fmt.Sprintf(tradierPositionsUrlTemplate, tradesAccountID)
	_ = tradierPositionsURL

	tradierTradesBearerToken, err := vars.GetTradierTradesBearerToken()
	if err != nil {
		log.Fatalf("$TRADIER_TRADES_BEARER_TOKEN not set: %v", err)
	}
	_ = tradierTradesBearerToken

	tradierNonTradesBearerToken, err := vars.GetTradierNonTradesBearerToken()
	if err != nil {
		log.Fatalf("$TRADIER_NON_TRADES_BEARER_TOKEN not set: %v", err)
	}

	tradierMarketTimesalesURL, err := utils.GetEnv("TRADIER_MARKET_TIMESALES_URL")
	if err != nil {
		log.Fatalf("$TRADIER_MARKET_TIMESALES_URL not set: %v", err)
	}

	eventStoreDbURL, err := utils.GetEnv("EVENTSTOREDB_URL")
	if err != nil {
		log.Fatalf("$EVENTSTOREDB_URL not set: %v", err)
	}

	// oandaFxQuotesURLBase, err := utils.GetEnv("OANDA_FX_QUOTES_URL_BASE")
	// if err != nil {
	// 	log.Fatalf("$OANDA_FX_QUOTES_URL_BASE not set: %v", err)
	// }

	// oandaBearerToken, err := utils.GetEnv("OANDA_BEARER_TOKEN")
	// if err != nil {
	// 	log.Fatalf("$OANDA_BEARER_TOKEN not set: %v", err)
	// }

	optionsExpirationURL, err := utils.GetEnv("TRADIER_OPTION_EXPIRATIONS_URL")
	if err != nil {
		log.Fatalf("$TRADIER_OPTION_EXPIRATIONS_URL not set: %v", err)
	}

	optionsConfigFile, err := utils.GetEnv("OPTIONS_CONFIG_FILE")
	if err != nil {
		log.Fatalf("$OPTIONS_CONFIG_FILE not set: %v", err)
	}

	postgresHost, err := utils.GetEnv("POSTGRES_HOST")
	if err != nil {
		log.Fatalf("$POSTGRES_HOST not set: %v", err)
	}

	postgresPort, err := utils.GetEnv("POSTGRES_PORT")
	if err != nil {
		log.Fatalf("$POSTGRES_PORT not set: %v", err)
	}

	postgresUser, err := utils.GetEnv("POSTGRES_USER")
	if err != nil {
		log.Fatalf("$POSTGRES_USER not set: %v", err)
	}

	postgresPassword, err := utils.GetEnv("POSTGRES_PASSWORD")
	if err != nil {
		log.Fatalf("$POSTGRES_PASSWORD not set: %v", err)
	}

	postgresDb, err := utils.GetEnv("POSTGRES_DB")
	if err != nil {
		log.Fatalf("$POSTGRES_DB not set: %v", err)
	}

	isDryRunEnv, err := utils.GetEnv("DRY_RUN")
	if err != nil {
		log.Fatalf("$DRY_RUN not set: %v", err)
	}

	isDryRun := strings.ToLower(isDryRunEnv) == "true"
	_ = isDryRun

	// Set up Telemetry
	log.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	)))

	// Set up OpenTelemetry.
	// otelShutdown, err := setupOTelSDK(ctx)
	// if err != nil {
	// 	log.Fatalf("failed to setup otel sdk: %v", err)
	// }

	// Handle shutdown properly so nothing leaks.
	// defer func() {
	// 	err = errors.Join(err, otelShutdown(context.Background()))
	// }()

	// Setup postgres
	if db, err = dbutils.InitPostgres(postgresHost, postgresPort, postgresUser, postgresPassword, postgresDb); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}

	// Load config
	optionsConfigInDir := path.Join(projectsDir, "slack-trading", "src", optionsConfigFile)
	config, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		log.Fatalf("failed to read options config: %v", err)
	}

	var optionsConfig eventmodels.OptionsConfigYAML
	if err := yaml.Unmarshal(config, &optionsConfig); err != nil {
		log.Fatalf("failed to unmarshal options config: %v", err)
	}

	// todo: move to config
	riskProfileConstraint := eventmodels.NewRiskProfileConstraint()
	riskProfileConstraint.AddItem(0.2, 1000)
	riskProfileConstraint.AddItem(0.8, 1800)

	// Set up google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
		log.Fatalf("failed to create google sheets client: %v", err)
	}

	// Setup router
	port, err := utils.GetEnv("PORT")
	if err != nil {
		log.Fatalf("$PORT not set: %v", err)
	}

	// Setup dispatcher
	dispatcher := eventmodels.InitializeGlobalDispatcher()
	router := mux.NewRouter()
	tradeapi.SetupHandler(router.PathPrefix("/trades").Subrouter())
	accountapi.SetupHandler(router.PathPrefix("/accounts").Subrouter())
	// signalapi.SetupHandler(router.PathPrefix("/signals").Subrouter())
	datafeedapi.SetupHandler(router.PathPrefix("/datafeeds").Subrouter())
	alertapi.SetupHandler(router.PathPrefix("/alerts").Subrouter())

	liveOrdersUpdateQueue := eventmodels.NewFIFOQueue[*models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 999)

	// Register pprof handlers
	pprofRouter := router.PathPrefix("/debug/pprof").Subrouter()
	pprofRouter.HandleFunc("/", http.HandlerFunc(pprof.Index))
	pprofRouter.HandleFunc("/cmdline", http.HandlerFunc(pprof.Cmdline))
	pprofRouter.HandleFunc("/profile", http.HandlerFunc(pprof.Profile))
	pprofRouter.HandleFunc("/symbol", http.HandlerFunc(pprof.Symbol))
	pprofRouter.HandleFunc("/trace", http.HandlerFunc(pprof.Trace))
	pprofRouter.Handle("/allocs", pprof.Handler("allocs"))
	pprofRouter.Handle("/block", pprof.Handler("block"))
	pprofRouter.Handle("/goroutine", pprof.Handler("goroutine"))
	pprofRouter.Handle("/heap", pprof.Handler("heap"))
	pprofRouter.Handle("/mutex", pprof.Handler("mutex"))
	pprofRouter.Handle("/threadcreate", pprof.Handler("threadcreate"))

	optionsDataFetcher := eventservices.NewPolygonOptionsDataFetcher("https://api.polygon.io", polygonApiKey)

	optionChainRequestExector := &eventmodels.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            tradierNonTradesBearerToken,
		GoEnv:                  goEnv,
		OptionsDataFetcher:     optionsDataFetcher,
	}
	_ = optionChainRequestExector

	streamParams := []eventmodels.StreamParameter{
		{StreamName: eventmodels.AccountsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionAlertsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	// optionContractClient := eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, &eventmodels.OptionContractV1{})
	// optionContractClient.Start(ctx)

	// todo: both TrackerV1 and TrackerV2 should be processed
	// todo: stream_version should be stored in eventstoredb UserMetadata field
	// todo: the eventstore metadata field should be queried so that we can process and combine multiple versions of the same stream
	// trackersClient := eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, &eventmodels.TrackerV3{})
	// trackersClient.Start(ctx)

	// TrackerV3 client for generating option EV signals
	// trackersClientV3 := eventconsumers.NewESDBConsumerStream(&wg, eventStoreDbURL, &eventmodels.TrackerV3{})
	// trackerV3OptionEVConsumer := eventconsumers.NewTrackerConsumerV3(trackersClientV3)

	// Setup ESDB producer
	esdbProducer := eventproducers.NewESDBProducer(&wg, eventStoreDbURL, streamParams)

	// todo: move this, has to be before trackerV3OptionEVConsumer.Start(ctx)
	// go func(eventCh <-chan eventmodels.SignalTriggeredEvent, optionsRequestExecutor *eventmodels.ReadOptionChainRequestExecutor, config eventmodels.OptionsConfigYAML, isDryRun bool) {
	// 	loc, err := time.LoadLocation("America/New_York")
	// 	if err != nil {
	// 		log.Panicf("failed to load location: %v", err)
	// 	}

	// 	tradierOrderExecuter := eventmodels.NewTradierOrderExecuter(tradierTradesOrderURL, tradierTradesBearerToken, isDryRun, func() ([]eventmodels.TradierPositionDTO, error) {
	// 		return eventservices.FetchTradierPositions(tradierPositionsURL, tradierTradesBearerToken)
	// 	})

	// 	for event := range eventCh {
	// 		if err := processSignalTriggeredEvent(event, tradierOrderExecuter, optionsRequestExecutor, projectsDir, config, riskProfileConstraint, loc, goEnv); err != nil {
	// 			log.Errorf("failed to process signal triggered event: %v", err)
	// 		}
	// 	}
	// }(trackerV3OptionEVConsumer.GetSignalTriggeredCh(), optionChainRequestExector, optionsConfig, isDryRun)

	// trackerV3OptionEVConsumer.Start(ctx, false)

	eventconsumers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)

	// Start event clients
	// eventconsumers.NewOptionChainTickWriterWorker(&wg, stockQuotesURL, optionChainURL, brokerBearerToken, calendarURL).Start(ctx, optionContractClient, trackersClient)

	// fxTicksCh := make(chan *eventmodels.FxTick)
	// eventconsumers.NewOandaFxTickWriter(&wg, trackersClient, oandaFxQuotesURLBase, oandaBearerToken).Start(ctx, fxTicksCh)

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

	// signals router
	// signalsGetStateExecutor := signalapi.NewGetStateExecutor(trackerV3OptionEVConsumer)
	processSignalExecutor := signalapi.NewProcessSignalExecutor(esdbProducer)
	s := NewRouterSetup("/signals", router)
	// s.Add(RouterSetupItem{Method: http.MethodGet, URL: "", Executor: signalsGetStateExecutor, Request: &eventmodels.EmptyRequest{}})
	s.Add(RouterSetupItem{Method: http.MethodPost, URL: "", Executor: processSignalExecutor, Request: &eventmodels.CreateSignalRequestEventV1DTO{}})

	// Setup polygon tick data machine
	polygonTickDataMachine := eventservices.NewPolygonTickDataMachine(polygonApiKey)
	d := NewRouterSetup("/data", router)
	d.Add(RouterSetupItem{Method: http.MethodGet, URL: "/polygon", Executor: polygonTickDataMachine, Request: &eventmodels.PolygonDataReadRequestDTO{}})

	// Setup app version
	appVersion := &eventservices.AppVersion{}
	a := NewRouterSetup("/version", router)
	a.Add(RouterSetupItem{Method: http.MethodGet, URL: "/app", Executor: appVersion, Request: &eventmodels.EmptyRequest{}})

	// Setup database service
	dbService := data.NewDatabaseService(db)

	// Setup brokers
	brokerMap, err := getTradierBrokers()
	if err != nil {
		log.Fatalf("failed to get tradier brokers: %v", err)
	}

	// Setup backtester router playground
	if err := backtester_router.SetupHandler(ctx, router.PathPrefix("/playground").Subrouter(), projectsDir, polygonApiKey, liveOrdersUpdateQueue, dbService, brokerMap); err != nil {
		log.Fatalf("failed to setup backtester router: %v", err)
	}

	polygonClient := eventservices.NewPolygonTickDataMachine(polygonApiKey)

	// this must be after the backtester router setup
	eventconsumers.NewTradierApiWorker(&wg, tradierMarketTimesalesURL, tradierNonTradesBearerToken, polygonClient, liveOrdersUpdateQueue, calendarURL, db, dbService).Start(ctx)

	// options router
	// r := NewRouterSetup("/options", router)
	// r.Add(RouterSetupItem{Method: http.MethodGet, URL: "", Executor: optionChainRequestExector, Request: &eventmodels.ReadOptionChainRequest{}})
	// r.Add(RouterSetupItem{Method: http.MethodGet, URL: "/spreads", Executor: optionChainRequestExector, Request: &eventmodels.ReadOptionChainRequest{}})

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

	// start the twirp server
	go func() {
		rpc.SetupTwirpServer(dbService)
	}()

	// Create channel for shutdown signals.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	signal.Notify(stop, syscall.SIGTERM)

	// todo: add back in
	// for _, streamParam := range streamParams {
	// 	eventconsumers.NewESDBConsumer(&wg, eventStoreDbURL, []eventmodels.StreamParameter{streamParam}).Start(ctx)
	// }

	// eventconsumers.NewOptionAlertWorker(&wg, tradierTradesOrderURL, tradierTradesBearerToken).Start(ctx)

	log.Info("Main: init complete")

	// Block here until program is shut down
	<-stop

	// EntrySignal -> shut down event clients
	cancel()

	// Wait for event clients to shut down
	wg.Wait()

	log.Info("Main: gracefully stopped!")
}
