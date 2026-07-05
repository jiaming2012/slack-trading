package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	backtester_router "github.com/jiaming2012/slack-trading/src/go/backtester-api/router"
	"github.com/jiaming2012/slack-trading/src/go/backtester-api/rpc"
	"github.com/jiaming2012/slack-trading/src/go/backtester-api/services"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/dbutils"
	"github.com/jiaming2012/slack-trading/src/go/workers"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/api"
	"github.com/jiaming2012/slack-trading/src/go/api/accountapi"
	"github.com/jiaming2012/slack-trading/src/go/api/alertapi"
	"github.com/jiaming2012/slack-trading/src/go/api/datafeedapi"
	"github.com/jiaming2012/slack-trading/src/go/api/signalapi"
	"github.com/jiaming2012/slack-trading/src/go/api/tradeapi"
	"github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/eventservices"
	"github.com/jiaming2012/slack-trading/src/go/sheets"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// RouterSetupItem defines a single route handler configuration.
type RouterSetupItem struct {
	Method   string
	URL      string
	Executor eventmodels.RequestExecutor
	Request  eventmodels.ApiRequest3
}

// RouterSetup manages a group of routes under a common prefix.
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

	api.ApiRequestHandler3(req.Context(), routerSetup.Request, routerSetup.Executor, w, req)
}

func NewRouterSetup(prefix string, router *mux.Router) *RouterSetup {
	return &RouterSetup{
		Router: router,
		Prefix: prefix,
		Items:  make(map[string]RouterSetupItem),
	}
}

func (r *RouterSetup) Add(item RouterSetupItem) {
	key := fmt.Sprintf("%v %v%v", item.Method, r.Prefix, item.URL)
	r.Items[key] = item
	r.Router.HandleFunc(fmt.Sprintf("%s%s", r.Prefix, item.URL), r.ServeHTTP)
}

func getTradierBrokers() (map[models.CreateAccountRequestSource]models.IBroker, error) {
	brokers := make(map[models.CreateAccountRequestSource]models.IBroker)
	brokerName := "tradier"

	for _, accountType := range []models.LiveAccountType{models.LiveAccountTypePaper, models.LiveAccountTypeMargin} {
		vars := models.NewLiveAccountVariables(accountType)

		tradierBalancesUrlTemplate, err := vars.GetTradierBalancesUrlTemplate()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier balances url template: %w", err)
		}

		accountID, err := vars.GetTradierTradesAccountID()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier account id: %w", err)
		}

		tradierTradesBearerToken, err := vars.GetTradierTradesBearerToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier trades bearer token: %w", err)
		}

		tradierTradesUrlTemplate, err := vars.GetTradierTradesUrlTemplate()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier trades url template: %w", err)
		}

		tradesUrl := fmt.Sprintf(tradierTradesUrlTemplate, accountID)

		tradierPositionsUrlTemplate, err := vars.GetTradierPositionsUrlTemplate()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier positions url template: %w", err)
		}

		tradierPositionsURL := fmt.Sprintf(tradierPositionsUrlTemplate, accountID)

		stockQuotesURL, err := utils.GetEnv("TRADIER_STOCK_QUOTES_URL")
		if err != nil {
			return nil, fmt.Errorf("$TRADIER_STOCK_QUOTES_URL not set: %v", err)
		}

		tradierNonTradesBearerToken, err := vars.GetTradierNonTradesBearerToken()
		if err != nil {
			return nil, fmt.Errorf("failed to get tradier non trades bearer token: %w", err)
		}

		balancesUrl := fmt.Sprintf(tradierBalancesUrlTemplate, accountID)

		source := services.NewLiveAccountSource(brokerName, accountID, balancesUrl, tradierTradesBearerToken, accountType)

		broker := services.NewTradierBroker(tradesUrl, stockQuotesURL, tradierPositionsURL, tradierNonTradesBearerToken, tradierTradesBearerToken, &source)

		brokers[models.CreateAccountRequestSource{
			LiveAccountType: accountType,
			Broker:          brokerName,
			AccountID:       accountID,
		}] = broker
	}

	return brokers, nil
}

var db *gorm.DB

func main() {
	projectDir, err := utils.GetEnv("TRADING_PROJECT_DIR")
	if err != nil {
		log.Fatalf("TRADING_PROJECT_DIR not set: %v", err)
	}

	goEnv, err := utils.GetEnv("GO_ENV")
	if goEnv == "" {
		log.Fatalf("GO_ENV not set: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}

	if err := utils.InitEnvironmentVariables(projectDir, goEnv); err != nil {
		log.Panic(err)
	}

	pubsub.Init()

	// Configure logrus logfmt formatter (D-04, D-05)
	log.SetFormatter(&log.TextFormatter{
		DisableColors:  true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		FieldMap: log.FieldMap{
			log.FieldKeyTime:  "ts",
			log.FieldKeyLevel: "level",
			log.FieldKeyMsg:   "msg",
		},
	})
	log.SetOutput(os.Stdout)

	// Initialize OpenTelemetry SDK (OTEL-01, OTEL-02)
	otelShutdown, otelErr := utils.SetupOTelSDK(ctx, "grodt", "1.0.0")
	if otelErr != nil {
		log.Warnf("Failed to initialize OTel SDK: %v (continuing without telemetry)", otelErr)
	} else {
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()
			if err := otelShutdown(shutdownCtx); err != nil {
				log.Errorf("OTel shutdown error: %v", err)
			}
		}()
		log.Info("OTel SDK initialized successfully")

		if err := telemetry.Init(); err != nil {
			log.Fatalf("Failed to initialize telemetry metrics: %v", err)
		}
		log.Info("Telemetry metrics initialized successfully")
	}

	log.Infof("Log level set to %v", log.GetLevel())
	log.Info("Main: starting...")

	// Determine live account type from environment
	var liveAccountType models.LiveAccountType
	if goEnv == "production" {
		liveAccountType = models.LiveAccountTypeMargin
	} else {
		liveAccountType = models.LiveAccountTypePaper
	}

	vars := models.NewLiveAccountVariables(liveAccountType)

	// Load required environment variables
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

	// Set up telemetry hooks
	// otellogrus adds log entries as span events (visible in Tempo traces)
	log.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	)))

	// OTel Log SDK bridge exports logs via OTLP to Loki (standalone, no span required)
	log.AddHook(utils.NewOTelLogrusHook(
		log.PanicLevel,
		log.FatalLevel,
		log.ErrorLevel,
		log.WarnLevel,
		log.InfoLevel,
	))

	// Setup postgres
	if db, err = dbutils.InitPostgres(postgresHost, postgresPort, postgresUser, postgresPassword, postgresDb); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}

	// Load options config
	// OPTIONS_CONFIG_PATH, if set, overrides the default path (useful for worktrees)
	optionsConfigInDir := os.Getenv("OPTIONS_CONFIG_PATH")
	if optionsConfigInDir == "" {
		optionsConfigInDir = path.Join(projectDir, "src", "go", optionsConfigFile)
	}
	configBytes, err := os.ReadFile(optionsConfigInDir)
	if err != nil {
		log.Fatalf("failed to read options config: %v", err)
	}

	var optionsConfig eventmodels.OptionsConfigYAML
	if err := yaml.Unmarshal(configBytes, &optionsConfig); err != nil {
		log.Fatalf("failed to unmarshal options config: %v", err)
	}

	_ = optionsConfig

	// Set up google sheets
	if _, _, err := sheets.NewClientFromEnv(ctx); err != nil {
		log.Fatalf("failed to create google sheets client: %v", err)
	}

	// Setup HTTP router
	port, err := utils.GetEnv("PORT")
	if err != nil {
		log.Fatalf("$PORT not set: %v", err)
	}

	dispatcher := eventmodels.InitializeGlobalDispatcher()
	router := mux.NewRouter()
	tradeapi.SetupHandler(router.PathPrefix("/trades").Subrouter())
	accountapi.SetupHandler(router.PathPrefix("/accounts").Subrouter())
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

	optionsDataFetcher := eventservices.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey)
	_ = &eventmodels.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            tradierNonTradesBearerToken,
		GoEnv:                  goEnv,
		OptionsDataFetcher:     optionsDataFetcher,
	}

	streamParams := []eventmodels.StreamParameter{
		{StreamName: eventmodels.AccountsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionAlertsStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: eventmodels.StockTickStream, Mutex: &sync.Mutex{}},
	}

	// Setup ESDB producer
	esdbProducer := api.NewESDBProducer(&wg, eventStoreDbURL, streamParams)

	// Start event consumers
	workers.NewSlackNotifierClient(&wg, slackWebhookURL).Start(ctx)
	api.NewSlackClient(&wg, router).Start(ctx)
	workers.NewGlobalDispatcherWorkerClient(&wg, dispatcher).Start(ctx)
	workers.NewAccountWorkerClient(&wg).Start(ctx)

	// Setup signal routes
	processSignalExecutor := signalapi.NewProcessSignalExecutor(esdbProducer)
	s := NewRouterSetup("/signals", router)
	s.Add(RouterSetupItem{Method: http.MethodPost, URL: "", Executor: processSignalExecutor, Request: &eventmodels.CreateSignalRequestEventV1DTO{}})

	// Setup data routes
	polygonTickDataMachine := eventservices.NewPolygonClient(polygonApiKey)
	d := NewRouterSetup("/data", router)
	d.Add(RouterSetupItem{Method: http.MethodGet, URL: "/polygon", Executor: polygonTickDataMachine, Request: &eventmodels.PolygonDataReadRequestDTO{}})

	// Setup polygon options client with disk cache
	polygonCacheDir := filepath.Join(projectDir, ".cache", "polygon")
	polygonOptionsClient := eventservices.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey, polygonCacheDir)

	// Setup version route
	appVersion := &eventservices.AppVersion{}
	a := NewRouterSetup("/version", router)
	a.Add(RouterSetupItem{Method: http.MethodGet, URL: "/app", Executor: appVersion, Request: &eventmodels.EmptyRequest{}})

	polygonClient := eventservices.NewPolygonClient(polygonApiKey)

	// Setup database service
	dbService := data.NewDatabaseService(db, polygonClient, polygonOptionsClient)

	// Setup brokers
	brokerMap, err := getTradierBrokers()
	if err != nil {
		log.Fatalf("failed to get tradier brokers: %v", err)
	}

	mockOrderIdStartIndex, err := dbService.GetMockOrderIdStartIndex()
	if err != nil {
		log.Fatalf("failed to get mock order id start index: %v", err)
	}

	if err := dbService.DeleteMockOrders(); err != nil {
		log.Fatalf("failed to delete mock orders: %v", err)
	}

	// Add mock broker for reconciliation testing
	pendingMockOrders, err := dbService.FetchPendingOrders([]models.LiveAccountType{models.LiveAccountTypeReconcilation}, false)
	if err != nil {
		log.Fatalf("failed to fetch pending mock orders: %v", err)
	}

	var existingOrders []*models.PlaceOrderRequest
	for _, order := range pendingMockOrders {
		existingOrders = append(existingOrders, &models.PlaceOrderRequest{
			OrderID:    order.ExternalOrderID,
			Symbol:     order.Symbol,
			Quantities: []int{int(order.AbsoluteQuantity)},
			Sides:      []models.TradierOrderSide{order.Side},
			OrderType:  models.TradierOrderTypeMarket,
			Tag:        order.Tag,
			DryRun:     false,
		})
	}

	brokerMap[models.CreateAccountRequestSource{
		LiveAccountType: models.LiveAccountTypeMock,
		Broker:          "tradier",
		AccountID:       "mock_default",
	}] = models.NewMockBroker(mockOrderIdStartIndex, existingOrders)

	quotesBearerToken := tradierNonTradesBearerToken
	nowUTC := time.Now().UTC()
	calendar, err := eventservices.FetchMarketCalendar(calendarURL, quotesBearerToken, nowUTC)
	if err != nil {
		log.Fatalf("Failed to fetch market calendar: %v", err)
	}

	// Setup backtester playground router
	if err := backtester_router.SetupHandler(ctx, router.PathPrefix("/playground").Subrouter(), projectDir, polygonApiKey, liveOrdersUpdateQueue, dbService, brokerMap, calendar); err != nil {
		log.Fatalf("failed to setup backtester router: %v", err)
	}

	// Start Tradier API worker (must be after backtester router setup)
	workers.NewTradierApiWorker(&wg, tradierMarketTimesalesURL, tradierNonTradesBearerToken, polygonClient, liveOrdersUpdateQueue, calendarURL, db, dbService).Start(ctx)

	// Start heartbeat goroutine (per D-04: every 30 seconds)
	go telemetry.StartHeartbeat(ctx, dbService.GetHeartbeatStats, time.Now())

	// Start HTTP server
	srv := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%s", port),
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		log.Infof("listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil {
			if err.Error() != "http: Server closed" {
				log.Errorf("failed to start HTTP server on %s: %v", srv.Addr, err)
			}
		}
	}()

	// Create global signal repository: ESDB for live mode, in-memory for dev/sim
	var globalSignalRepo models.ISignalRepository
	if esdbProducer != nil {
		globalSignalRepo = models.NewESDBSignalRepository(esdbProducer)
	} else {
		globalSignalRepo = models.NewInMemorySignalRepository()
	}

	// Start Twirp server
	go func() {
		rpc.SetupTwirpServer(polygonOptionsClient, dbService, esdbProducer, globalSignalRepo)
	}()

	// Wait for shutdown signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	log.Info("Main: init complete")

	<-stop

	cancel()
	wg.Wait()

	log.Info("Main: gracefully stopped!")
}
