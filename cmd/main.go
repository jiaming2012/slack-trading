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
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/api"
	"github.com/jiaming2012/slack-trading/src/go/api/accountapi"
	"github.com/jiaming2012/slack-trading/src/go/api/alertapi"
	"github.com/jiaming2012/slack-trading/src/go/api/datafeedapi"
	"github.com/jiaming2012/slack-trading/src/go/api/killswitchapi"
	"github.com/jiaming2012/slack-trading/src/go/api/signalapi"
	"github.com/jiaming2012/slack-trading/src/go/api/slack"
	"github.com/jiaming2012/slack-trading/src/go/api/telemetryapi"
	"github.com/jiaming2012/slack-trading/src/go/api/tradeapi"
	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	backtester_router "github.com/jiaming2012/slack-trading/src/go/backtester/router"
	"github.com/jiaming2012/slack-trading/src/go/backtester/rpc"
	"github.com/jiaming2012/slack-trading/src/go/backtester/safety"
	"github.com/jiaming2012/slack-trading/src/go/backtester/services"
	"github.com/jiaming2012/slack-trading/src/go/data"
	"github.com/jiaming2012/slack-trading/src/go/dbutils"
	"github.com/jiaming2012/slack-trading/src/go/feedhealth"
	"github.com/jiaming2012/slack-trading/src/go/marketdata"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/pubsub"
	"github.com/jiaming2012/slack-trading/src/go/sheets"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
	"github.com/jiaming2012/slack-trading/src/go/utils"
	"github.com/jiaming2012/slack-trading/src/go/workers"
)

// RouterSetupItem defines a single route handler configuration.
type RouterSetupItem struct {
	Method   string
	URL      string
	Executor models.RequestExecutor
	Request  models.ApiRequest3
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

func getTradierBrokers() (map[backtester_models.CreateAccountRequestSource]backtester_models.IBroker, error) {
	brokers := make(map[backtester_models.CreateAccountRequestSource]backtester_models.IBroker)
	brokerName := "tradier"

	for _, accountType := range []backtester_models.AccountRole{backtester_models.AccountRolePaper, backtester_models.AccountRoleMargin} {
		vars := backtester_models.NewLiveAccountVariables(accountType)

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

		brokers[backtester_models.CreateAccountRequestSource{
			AccountRole: accountType,
			Broker:      brokerName,
			AccountID:   accountID,
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
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
		FieldMap: log.FieldMap{
			log.FieldKeyTime:  "ts",
			log.FieldKeyLevel: "level",
			log.FieldKeyMsg:   "msg",
		},
	})
	log.SetOutput(os.Stdout)

	// Initialize the in-process telemetry registry — self-contained, no SDK
	// or network setup required (ADR-0005).
	telemetry.Init()
	// Error-level log entries feed the error-rate alert rule (ADR-0005:
	// this replaces log-pipeline alerting; log storage is a v1 non-goal).
	log.AddHook(telemetry.NewErrorCounterHook())
	log.Info("Telemetry registry initialized")

	log.Infof("Log level set to %v", log.GetLevel())
	log.Info("Main: starting...")

	// Determine live account type from environment
	var liveAccountType backtester_models.AccountRole
	if goEnv == "production" {
		liveAccountType = backtester_models.AccountRoleMargin
	} else {
		liveAccountType = backtester_models.AccountRolePaper
	}

	vars := backtester_models.NewLiveAccountVariables(liveAccountType)

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

	// Setup postgres
	if db, err = dbutils.InitPostgres(postgresHost, postgresPort, postgresUser, postgresPassword, postgresDb); err != nil {
		log.Fatalf("failed to init db: %v", err)
	}

	// Telemetry tables + persistence loops (ADR-0005): snapshot writer
	// flushes the registry, prune bounds retention to 30 days.
	if err := telemetry.Migrate(db); err != nil {
		log.Fatalf("failed to migrate telemetry tables: %v", err)
	}
	go telemetry.StartSnapshotWriter(ctx, db, telemetry.Default, telemetry.SnapshotInterval())
	go telemetry.StartPrune(ctx, db)

	// Alert engine (ADR-0005): evaluates stale-heartbeat and error-rate rules
	// over in-memory telemetry, posts to Slack, re-notifies until acked.
	slackNotifier := workers.NewSlackNotifierClient(&wg, slackWebhookURL)
	alertEngine := telemetry.NewAlertEngine(db, telemetry.Heartbeats, telemetry.Default, slackNotifier)
	go alertEngine.Start(ctx)
	slack.SetAckFunc(alertEngine.Ack)

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

	var optionsConfig models.OptionsConfigYAML
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

	dispatcher := models.InitializeGlobalDispatcher()
	router := mux.NewRouter()
	tradeapi.SetupHandler(router.PathPrefix("/trades").Subrouter())
	accountapi.SetupHandler(router.PathPrefix("/accounts").Subrouter())
	datafeedapi.SetupHandler(router.PathPrefix("/datafeeds").Subrouter())
	alertapi.SetupHandler(router.PathPrefix("/alerts").Subrouter())

	// Hard kill switch: construct the persisted halt controller, install it as
	// the process-wide order gate consulted at the Broker seam, and expose the
	// operator REST surface. The state file defaults to
	// <projectDir>/.safety/halt-state.json (override with KILL_SWITCH_STATE_PATH).
	// It deliberately lives OUTSIDE .cache/ — a wipe-by-convention directory — so
	// clearing caches cannot silently boot a halted server back to clear. If the
	// server was halted before a restart, NewHaltController restores that state
	// so it comes back up halted.
	killSwitchStatePath := os.Getenv("KILL_SWITCH_STATE_PATH")
	if killSwitchStatePath == "" {
		killSwitchStatePath = filepath.Join(projectDir, ".safety", "halt-state.json")
	}
	killSwitchStore := safety.NewFileHaltStore(killSwitchStatePath)
	killSwitchStoreWasEmpty := !killSwitchStore.Exists()
	haltController, err := safety.NewHaltController(killSwitchStore)
	if err != nil {
		log.Fatalf("failed to construct kill-switch halt controller: %v", err)
	}
	backtester_models.SetOrderGate(haltController)

	// Guard/halt observability (ADR-0005, internal telemetry only): the
	// safety_halt_engaged gauge tracks every controller transition (and the
	// startup restore), and the alert engine's auto_halt rule pushes a Slack
	// Alert while an automatic halt is engaged, naming the tripping guard.
	safety.InstallHaltTelemetry(haltController)
	alertEngine.SetHaltStatus(func() (bool, string, string) {
		st := haltController.Status()
		return st.Engaged, string(st.Source), st.Reason
	})

	// Shared-secret token guarding the mutating /kill-switch endpoints.
	// Unset: engage stays available (stopping trading is the safe direction)
	// but release/acknowledge are refused fail-closed.
	killSwitchToken := os.Getenv("KILL_SWITCH_TOKEN")
	if killSwitchToken == "" {
		log.Warnf("KILL_SWITCH_TOKEN is NOT set: /kill-switch/engage is UNAUTHENTICATED and /kill-switch/release + /kill-switch/acknowledge are LOCKED (fail-closed). Set KILL_SWITCH_TOKEN to secure engage and enable release/acknowledge.")
	}
	killswitchapi.SetupHandler(router.PathPrefix("/kill-switch").Subrouter(), haltController, killSwitchToken)
	telemetryapi.SetupHandler(router.PathPrefix("/telemetry").Subrouter(), db, telemetry.Heartbeats, alertEngine)
	if killSwitchStoreWasEmpty {
		log.Warnf("kill-switch halt-state store is EMPTY at %s — booting with NO persisted halt state (source=none). If this file was wiped, any prior halt has been LOST and the server is starting CLEAR; verify this is intended.", killSwitchStatePath)
	}
	if st := haltController.Status(); st.Engaged {
		log.Warnf("kill switch is ENGAGED on startup (source=%s, reason=%q) — order submission is halted until released", st.Source, st.Reason)
	} else {
		log.Infof("kill switch initialized (disengaged); state file: %s", killSwitchStatePath)
	}

	// Anomaly guards (wire-anomaly-guard-feeds): thresholds from GUARD_* env
	// vars with conservative defaults, per-guard "off" sentinel, Fatal on
	// unparseable values. The registry binds every guard to the SAME halt
	// controller the REST surface and order gate use, and is installed via the
	// package hook so the live order pipeline and workers can observe into it
	// without constructor threading (nil hook = guards inert).
	guardEnvCfg, err := safety.LoadGuardEnvConfig()
	if err != nil {
		log.Fatalf("invalid anomaly-guard configuration (fix or set the variable to \"off\"): %v", err)
	}
	guardEnvCfg.LogEffective()

	// Feed-health heartbeat monitor: the live candle-ingestion path observes a
	// wall-clock heartbeat per asset class; the composite signal feeds the
	// feed-staleness guard with the age of the most recent Tick overall.
	feedThresholds := feedhealth.ThresholdConfig{}
	if feedCfgPath, cfgErr := feedhealth.ResolveConfigPath(); cfgErr == nil {
		if loaded, loadErr := feedhealth.LoadThresholdConfig(feedCfgPath); loadErr == nil {
			feedThresholds = loaded
		} else {
			log.Warnf("feedhealth: could not load threshold config (heartbeat monitor still records last-tick ages): %v", loadErr)
		}
	} else {
		log.Warnf("feedhealth: could not resolve config path (heartbeat monitor still records last-tick ages): %v", cfgErr)
	}
	feedHeartbeatMonitor := feedhealth.NewHeartbeatMonitor(feedhealth.RealClock{}, feedThresholds)
	stalenessSignal := safety.NewCompositeStalenessSignal(
		safety.NewFeedHealthStalenessSignal(feedHeartbeatMonitor, "equity", nil),
		safety.NewFeedHealthStalenessSignal(feedHeartbeatMonitor, "option", nil),
	)

	guardRegistry := safety.BuildGuardRegistry(haltController, nil, guardEnvCfg, stalenessSignal)
	safety.SetGuardRegistry(guardRegistry)
	log.Info("anomaly-guard registry constructed and bound to the shared halt controller")

	// Broker-side companion stops (wire-companion-stops): explicit opt-in via
	// COMPANION_STOP_DISTANCE. Unset = feature disabled with a loud warning (a
	// guessed default distance is more dangerous than none); non-positive or
	// unparseable = refuse startup rather than error on every live fill. When
	// armed, every live (Paper/Margin) equity entry fill places a broker-held
	// protective stop through the IBroker seam — below the halt-gated
	// submission path, so an engaged kill switch can never strand an open
	// position without its protective exit. Placement failures surface through
	// the alert engine as "position UNPROTECTED" alerts.
	companionStopCfg, err := safety.LoadCompanionStopEnvConfig()
	if err != nil {
		log.Fatalf("invalid companion-stop configuration (set %s to a positive price distance, or unset it to disable): %v", safety.EnvCompanionStopDistance, err)
	}
	if companionStopCfg == nil {
		log.Warnf("%s is NOT set: broker-side companion stops are DISABLED — live entry fills carry NO broker-held protective exit if this process dies with a position open. Set %s (positive absolute price distance) after the operator-only verification to arm them.", safety.EnvCompanionStopDistance, safety.EnvCompanionStopDistance)
	} else {
		safety.SetCompanionStopper(safety.NewCompanionStopper(*companionStopCfg))
		log.Infof("safety: companion stops ARMED (distance=%v) — every live equity entry fill places a broker-held protective stop, including while the kill switch is engaged", companionStopCfg.StopDistance)
	}
	alertEngine.SetUnprotectedPositions(safety.UnprotectedPositions)

	liveOrdersUpdateQueue := models.NewFIFOQueue[*backtester_models.TradierOrderUpdateEvent]("liveOrdersUpdateQueue", 999)

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

	optionsDataFetcher := marketdata.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey)
	_ = &models.ReadOptionChainRequestExecutor{
		OptionsByExpirationURL: optionsExpirationURL,
		OptionChainURL:         optionChainURL,
		StockURL:               stockQuotesURL,
		BearerToken:            tradierNonTradesBearerToken,
		GoEnv:                  goEnv,
		OptionsDataFetcher:     optionsDataFetcher,
	}

	streamParams := []models.StreamParameter{
		{StreamName: models.AccountsStream, Mutex: &sync.Mutex{}},
		{StreamName: models.OptionAlertsStream, Mutex: &sync.Mutex{}},
		{StreamName: models.OptionChainTickStream, Mutex: &sync.Mutex{}},
		{StreamName: models.StockTickStream, Mutex: &sync.Mutex{}},
	}

	// Setup ESDB producer
	esdbProducer := api.NewESDBProducer(&wg, eventStoreDbURL, streamParams)

	// Start event consumers
	slackNotifier.Start(ctx)
	api.NewSlackClient(&wg, router).Start(ctx)
	workers.NewGlobalDispatcherWorkerClient(&wg, dispatcher).Start(ctx)
	workers.NewAccountWorkerClient(&wg).Start(ctx)

	// Setup signal routes
	processSignalExecutor := signalapi.NewProcessSignalExecutor(esdbProducer)
	s := NewRouterSetup("/signals", router)
	s.Add(RouterSetupItem{Method: http.MethodPost, URL: "", Executor: processSignalExecutor, Request: &models.CreateSignalRequestEventV1DTO{}})

	// Setup data routes
	polygonTickDataMachine := marketdata.NewPolygonClient(polygonApiKey)
	d := NewRouterSetup("/data", router)
	d.Add(RouterSetupItem{Method: http.MethodGet, URL: "/polygon", Executor: polygonTickDataMachine, Request: &models.PolygonDataReadRequestDTO{}})

	// Setup polygon options client with disk cache
	polygonCacheDir := filepath.Join(projectDir, ".cache", "polygon")
	polygonOptionsClient := marketdata.NewPolygonOptionsClient("https://api.polygon.io", polygonApiKey, polygonCacheDir)

	// Setup version route
	appVersion := &marketdata.AppVersion{}
	a := NewRouterSetup("/version", router)
	a.Add(RouterSetupItem{Method: http.MethodGet, URL: "/app", Executor: appVersion, Request: &models.EmptyRequest{}})

	polygonClient := marketdata.NewPolygonClient(polygonApiKey)

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
	pendingMockOrders, err := dbService.FetchPendingOrders([]backtester_models.AccountRole{backtester_models.AccountRoleReconcilation}, false)
	if err != nil {
		log.Fatalf("failed to fetch pending mock orders: %v", err)
	}

	var existingOrders []*backtester_models.PlaceOrderRequest
	for _, order := range pendingMockOrders {
		existingOrders = append(existingOrders, &backtester_models.PlaceOrderRequest{
			OrderID:    order.ExternalOrderID,
			Symbol:     order.Symbol,
			Quantities: []int{int(order.AbsoluteQuantity)},
			Sides:      []backtester_models.TradierOrderSide{order.Side},
			OrderType:  backtester_models.TradierOrderTypeMarket,
			Tag:        order.Tag,
			DryRun:     false,
		})
	}

	brokerMap[backtester_models.CreateAccountRequestSource{
		AccountRole: backtester_models.AccountRoleMock,
		Broker:      "tradier",
		AccountID:   "mock_default",
	}] = backtester_models.NewMockBroker(mockOrderIdStartIndex, existingOrders)

	quotesBearerToken := tradierNonTradesBearerToken
	nowUTC := time.Now().UTC()
	calendar, err := marketdata.FetchMarketCalendar(calendarURL, quotesBearerToken, nowUTC)
	if err != nil {
		log.Fatalf("Failed to fetch market calendar: %v", err)
	}

	// Setup backtester playground router
	if err := backtester_router.SetupHandler(ctx, router.PathPrefix("/playground").Subrouter(), projectDir, polygonApiKey, liveOrdersUpdateQueue, dbService, brokerMap, calendar); err != nil {
		log.Fatalf("failed to setup backtester router: %v", err)
	}

	// Start Tradier API worker (must be after backtester router setup). Its
	// live candle-ingestion path feeds the feed-health heartbeat monitor.
	tradierApiWorker := workers.NewTradierApiWorker(&wg, tradierMarketTimesalesURL, tradierNonTradesBearerToken, polygonClient, liveOrdersUpdateQueue, calendarURL, db, dbService)
	tradierApiWorker.SetFeedHeartbeatMonitor(feedHeartbeatMonitor)
	tradierApiWorker.Start(ctx)

	// Feed-staleness evaluation ticker: evaluates the staleness guard on a
	// fixed interval, suppressed while the market is closed, while no realtime
	// (Paper/Margin) Playground is loaded, and for a grace window equal to the
	// staleness threshold after each market open — so neither a quiet feed
	// outside trading hours nor the overnight last-Tick gap at the opening
	// bell can engage the halt. Calendar failures skip the cycle with a Warn.
	if guardEnvCfg.FeedStalenessEnabled {
		go safety.RunFeedStalenessTicker(ctx, guardEnvCfg.StalenessEvalInterval, guardEnvCfg.Guards.FeedStalenessThreshold, tradierApiWorker.IsMarketOpenErr, func() bool {
			for _, p := range dbService.GetPlaygrounds() {
				if p.GetMeta().Mode.IsRealtime() {
					return true
				}
			}
			return false
		})
	}

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
	var globalSignalRepo backtester_models.ISignalRepository
	if esdbProducer != nil {
		globalSignalRepo = backtester_models.NewESDBSignalRepository(esdbProducer)
	} else {
		globalSignalRepo = backtester_models.NewInMemorySignalRepository()
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
