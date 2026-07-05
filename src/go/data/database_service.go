package data

import (
	"context"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/dbutils"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
	"github.com/jiaming2012/slack-trading/src/go/utils"
)

// DatabaseService is the public facade over four focused stores:
//
//   - playgroundStore:  playground + reconcile-playground caches and queries
//   - orderStore:       order/trade caches and queries
//   - liveAccountStore: live accounts, broker map, live candle repositories
//   - equityStore:      equity plot queries
//
// The public surface (all methods, signatures, and behavior) is unchanged;
// each public method is a thin delegation, and methods that span concerns
// remain here as orchestration. Locking granularity is intentionally
// unchanged: the single service-wide mutex (mu) still guards the in-memory
// caches, and the stores rely on their callers holding it.
type DatabaseService struct {
	mu                   sync.Mutex
	db                   *gorm.DB
	projectsDir          string
	polygonClient        backtester_models.IPolygonClient
	polygonOptionsBroker backtester_models.IOptionsBroker

	playgroundStore  *playgroundStore
	orderStore       *orderStore
	liveAccountStore *liveAccountStore
	equityStore      *equityStore
}

func NewDatabaseService(db *gorm.DB, polygonClient backtester_models.IPolygonClient, optionsBroker backtester_models.IOptionsBroker) *DatabaseService {
	return &DatabaseService{
		db:                   db,
		playgroundStore:      newPlaygroundStore(db),
		orderStore:           newOrderStore(db),
		liveAccountStore:     newLiveAccountStore(db),
		equityStore:          newEquityStore(db),
		polygonClient:        polygonClient,
		polygonOptionsBroker: optionsBroker,
	}
}

func (s *DatabaseService) GetPolygonClient() backtester_models.IPolygonClient {
	return s.polygonClient
}

func (s *DatabaseService) LoadPlaygrounds(calendar *models.MarketCalendar) error {
	var playgroundsSlice []*backtester_models.Playground
	if err := s.db.Preload("Orders", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Trades", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.ReconcileTrades", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.ClosedBy", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Closes", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Closes.ClosedBy", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Closes.Trades", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Reconciles", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("Orders.Reconciles.Trades", func(db *gorm.DB) *gorm.DB {
		return db.Order("id ASC")
	}).Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
		return fmt.Errorf("loadPlaygrounds: failed to load playgrounds: %w", err)
	}

	// if err := s.db.Preload("Orders", func(db *gorm.DB) *gorm.DB {
	//     return db.Order("id ASC") // Fetch orders sorted by OrderID in ascending order
	// }).Preload("Orders.Trades").Preload("Orders.ClosedBy").Preload("Orders.Closes").Preload("Orders.Closes.ClosedBy").Preload("Orders.Closes.Trades").Preload("Orders.Reconciles").Preload("Orders.Reconciles.Trades").Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
	//     return fmt.Errorf("loadPlaygrounds: failed to load playgrounds: %w", err)
	// }

	// Sort orders in each playground by OrderID
	for _, p := range playgroundsSlice {
		sort.Slice(p.Orders, func(i, j int) bool {
			return p.Orders[i].ID < p.Orders[j].ID
		})

		// Store orders in memory
		for _, o := range p.Orders {
			if err := o.Hydrate(); err != nil {
				return fmt.Errorf("loadPlaygrounds: failed to hydrate order: %w", err)
			}

			s.orderStore.ordersCache[o.ID] = o

			// Store trades in memory
			for _, t := range o.Trades {
				s.orderStore.tradesCache[t.ID] = t
			}

			for _, t := range o.ReconcileTrades {
				s.orderStore.tradesCache[t.ID] = t
			}
		}
	}

	// load reconcile playgrounds first
	for _, p := range playgroundsSlice {
		if !p.Meta.IsReconciliation() {
			continue
		}

		if p.BrokerName == nil {
			return fmt.Errorf("loadPlaygrounds: broker name is not set for reconcile playground: %s", p.ID.String())
		}

		if p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: account id is not set for reconcile playground: %s", p.ID.String())
		}

		if err := s.PopulatePlayground(p, calendar); err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to populate reconcile playground: %w", err)
		}

		source, err := p.GetSource()
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get source for reconcile playground: %w", err)
		}

		liveAccount := s.liveAccountStore.liveAccounts[source]

		if liveAccount == nil {
			return fmt.Errorf("loadPlaygrounds: failed to find live account for reconcile playground: %s", p.ID.String())
		}

		reconcilePlayground, err := backtester_models.NewReconcilePlayground(p, liveAccount)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to create reconcile playground: %w", err)
		}

		s.playgroundStore.reconcilePlaygrounds[source] = reconcilePlayground
	}

	// load other playgrounds
	for _, p := range playgroundsSlice {
		if p.Meta.IsReconciliation() {
			continue
		}

		if _, found := s.playgroundStore.playgrounds[p.ID]; found {
			log.Warnf("loadPlaygrounds: skipping duplicate playground id: %s", p.ID.String())
			continue
		}

		if err := s.PopulatePlayground(p, calendar); err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to populate live playground: %w", err)
		}
	}

	return nil
}

func (s *DatabaseService) FindOrder(playgroundId uuid.UUID, id uint) (*backtester_models.Playground, *backtester_models.OrderRecord, error) {
	playground, found := s.playgroundStore.playgrounds[playgroundId]
	if !found {
		return nil, nil, fmt.Errorf("failed to find playground using id %s", playgroundId)
	}

	orders := playground.GetAllOrders()
	for _, order := range orders {
		if order.ExternalOrderID != nil && *order.ExternalOrderID == id {
			return playground, order, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to find Order in playground %s", playground.GetId().String())
}

func (s *DatabaseService) CreateRepos(repoRequests []models.CreateRepositoryRequest, from, to *models.PolygonDate, newCandlesQueue *models.FIFOQueue[*backtester_models.BacktesterCandle]) ([]*backtester_models.CandleRepository, *models.WebError) {
	var feeds []*backtester_models.CandleRepository
	for _, repo := range repoRequests {
		var bars, pastBars []*models.PolygonAggregateBarV2
		var err error

		timespan := models.PolygonTimespan{
			Multiplier: repo.Timespan.Multiplier,
			Unit:       models.PolygonTimespanUnit(repo.Timespan.Unit),
		}

		if repo.Source.Type == models.RepositorySourceTradier {
			// "tradier" marks live playground repos: candles arrive via the live feed,
			// so there are no historical bars to backfill here. The value is persisted
			// in playground records — do not rename or remove without a data migration.
		} else if repo.Source.Type == models.RepositorySourcePolygon {
			bars, err = s.polygonClient.FetchAggregateBars(models.StockSymbol(repo.Symbol), timespan, from, to)
			if err != nil {
				return nil, models.NewWebError(500, "failed to fetch aggregate bars", err)
			}
		} else if repo.Source.Type == models.RepositorySourceCSV {
			if repo.Source.CSVFilename == nil {
				return nil, models.NewWebError(400, "missing CSV filename", nil)
			}

			sourceDir := path.Join(s.projectsDir, "slack-trading", "src", "backtester-api", "data", *repo.Source.CSVFilename)

			bars, err = utils.ImportCandlesFromCsv(sourceDir)
			if err != nil {
				return nil, models.NewWebError(500, "failed to import candles from CSV", err)
			}
		} else {
			return nil, models.NewWebError(400, "invalid repository source", nil)
		}

		if from != nil {
			pastBars, err = s.polygonClient.FetchPastCandles(models.StockSymbol(repo.Symbol), timespan, int(repo.HistoryInDays), from)
			if err != nil {
				return nil, models.NewWebError(500, "failed to fetch past candles", err)
			}
		}

		aggregateBars := append(pastBars, bars...)

		source := models.CandleRepositorySource{
			Type: string(repo.Source.Type),
		}

		startingPosition := len(pastBars)
		repository, err := CreateRepositoryWithPosition(models.StockSymbol(repo.Symbol), timespan, aggregateBars, repo.Indicators, newCandlesQueue, startingPosition, repo.HistoryInDays, source)
		if err != nil {
			log.Errorf("failed to create repository: %v", err)
			return nil, models.NewWebError(500, "failed to create repository", err)
		}

		feeds = append(feeds, repository)
	}

	return feeds, nil
}

func (s *DatabaseService) CreatePlayground(playground *backtester_models.Playground, req *backtester_models.PopulatePlaygroundRequest) error {
	// validations
	if !req.Reconciliation {
		if err := req.Mode.Validate(); err != nil {
			return models.NewWebError(400, "invalid playground mode", err)
		}

		if len(req.Repositories) == 0 {
			return models.NewWebError(400, "missing repositories", nil)
		}
	}

	req.OptionsBroker = s.polygonOptionsBroker

	// create playground
	if req.Reconciliation {
		if req.LiveAccount == nil {
			return models.NewWebError(400, "reconcile playground is missing live account", nil)
		}

		var err error
		now := req.CreatedAt
		err = backtester_models.PopulatePlayground(playground, req, nil, now, nil, nil, nil)
		if err != nil {
			return models.NewWebError(500, "failed to create reconcile playground", err)
		}

		if req.SaveToDB {
			if err = s.SavePlaygroundSession(playground); err != nil {
				return fmt.Errorf("failed to save reconcile playground: %v", err)
			}
		}

	} else if req.Mode.IsRealtime() {
		// todo: hot load live account
		if req.LiveAccount == nil {
			return models.NewWebError(400, "live playground is missing live account", nil)
		}

		// capture all candles up to tomorrow
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		tomorrowStr := tomorrow.Format("2006-01-02")
		from, err := models.NewPolygonDate(tomorrowStr)
		if err != nil {
			return models.NewWebError(400, "failed to parse clock.startDate", err)
		}

		newCandlesQueue := models.NewFIFOQueue[*backtester_models.BacktesterCandle]("newCandlesQueue", 999)

		// fetch or create live repositories
		repos, webErr := s.CreateRepos(req.Repositories, from, nil, newCandlesQueue)
		if webErr != nil {
			return webErr
		}

		playground.SetNewCandlesQueue(newCandlesQueue)

		// save live repositories
		for _, repo := range repos {
			if err := s.SaveLiveRepository(repo); err != nil {
				// fatal as partial save is not allowed
				log.Fatalf("failed to save live repository: %v", err)
			}
		}

		// get reconcile playground
		if req.Account.Source == nil {
			return models.NewWebError(400, "missing account source", nil)
		}

		reconcilePlayground, found, err := s.FetchReconcilePlayground(*req.Account.Source)
		if err != nil {
			return models.NewWebError(500, "failed to fetch live account", err)
		}

		if !found {
			log.Debugf("failed to create live account: %v. Creating a new one ...", err)

			reconcilePlayground, err = dbutils.CreateReconcilePlayground(s, req.Account.Source, now)
			if err != nil {
				return models.NewWebError(500, "failed to create new reconcile playground and live account", err)
			}

			// save reconcile playground
			s.playgroundStore.reconcilePlaygrounds[*req.Account.Source] = reconcilePlayground
		}

		req.ReconcilePlayground = reconcilePlayground

		newTradesQueue := models.NewFIFOQueue[*backtester_models.TradeRecord]("newTradesQueue", 999)
		invalidOrdersQueue := models.NewFIFOQueue[*backtester_models.OrderRecord]("invalidOrdersQueue", 999)
		err = backtester_models.PopulatePlayground(playground, req, nil, now, newTradesQueue, invalidOrdersQueue, req.Calendar, repos...)
		if err != nil {
			return models.NewWebError(500, "failed to create reconcile playground", err)
		}

		// always save live playgrounds if flag is set
		if req.SaveToDB {
			if err = s.SavePlaygroundSession(playground); err != nil {
				return fmt.Errorf("failed to save playground: %v", err)
			}
		}

	} else if req.Mode == backtester_models.ModeSimulation {
		// validations
		from, err := models.NewPolygonDate(req.Clock.StartDate)
		if err != nil {
			return models.NewWebError(400, "failed to parse clock.startDate", err)
		}

		to, err := models.NewPolygonDate(req.Clock.StopDate)
		if err != nil {
			return models.NewWebError(400, "failed to parse clock.stopDate", err)
		}

		// create clock
		clock, err := s.CreateClock(from, to)
		if err != nil {
			return models.NewWebError(500, "failed to create clock", err)
		}

		// create backtester repositories
		repos, webErr := s.CreateRepos(req.Repositories, from, to, nil)
		if webErr != nil {
			return webErr
		}

		// create playground
		now := clock.CurrentTime
		err = backtester_models.PopulatePlayground(playground, req, clock, now, nil, nil, req.Calendar, repos...)
		if err != nil {
			return models.NewWebError(500, "failed to create playground", err)
		}
	} else {
		return models.NewWebError(400, "invalid playground mode", nil)
	}

	playground.SetEquityPlot(req.EquityPlotRecords)

	playground.SetOpenOrdersCache()

	if err := s.SavePlaygroundInMemory(playground); err != nil {
		return fmt.Errorf("failed to save in-memory playground: %w", err)
	}

	playground.CreatedAt = time.Now()

	return nil
}

func (s *DatabaseService) CreateClock(start, stop *models.PolygonDate) (*backtester_models.Clock, error) {
	// Load the location for New York (Eastern Time)
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to load location America/New_York: %w", err)
	}

	// start at stock market open
	fromDate := time.Date(start.Year, time.Month(start.Month), start.Day, 9, 30, 0, 0, loc)

	// end at stock market close
	toDate := time.Date(stop.Year, time.Month(stop.Month), stop.Day, 16, 0, 0, 0, loc)

	// create calendar
	calendarStartDate := fromDate.AddDate(0, 0, -7)
	startDate := models.PolygonDate{
		Year:  calendarStartDate.Year(),
		Month: int(calendarStartDate.Month()),
		Day:   calendarStartDate.Day(),
	}

	endDate := models.PolygonDate{
		Year:  stop.Year,
		Month: stop.Month,
		Day:   stop.Day,
	}

	calendar, err := FetchCalendarMap(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to fetch calendar: %w", err)
	}

	// create clock
	clock := backtester_models.NewClock(fromDate, toDate, calendar)

	return clock, nil
}

func (s *DatabaseService) PopulatePlayground(p *backtester_models.Playground, calendar *models.MarketCalendar) error {
	log.Infof("loading playground: %s", p.ID)

	var source *backtester_models.CreateAccountRequestSource
	var clockRequest backtester_models.CreateClockRequest
	var liveAccount backtester_models.ILiveAccount
	var err error

	if p.Meta.Mode == backtester_models.ModeSimulation {
		if p.EndAt == nil {
			return fmt.Errorf("loadPlaygrounds: missing end date for simulator playground")
		}

		clockRequest = backtester_models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
			StopDate:  p.EndAt.Format(time.RFC3339),
		}

	} else if p.Meta.Mode.IsRealtime() {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for live playground")
		}

		accountRole := p.Meta.Role
		if err = accountRole.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid account role for live playground: %w", err)
		}

		source = &backtester_models.CreateAccountRequestSource{
			Broker:      *p.BrokerName,
			AccountID:   *p.AccountID,
			AccountRole: accountRole,
		}

		clockRequest = backtester_models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
		}

		liveAccount, err = s.GetLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for live playground: %w", err)
		}

	} else if p.Meta.IsReconciliation() {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for reconcile playground")
		}

		accountRole := p.Meta.Role
		if err = accountRole.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid account role for reconcile playground: %w", err)
		}

		source = &backtester_models.CreateAccountRequestSource{
			Broker:      *p.BrokerName,
			AccountID:   *p.AccountID,
			AccountRole: accountRole,
		}

		liveAccount, err = s.GetLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for reconcile playground: %w", err)
		}

	} else {
		return fmt.Errorf("loadPlaygrounds: unknown mode: %q (environment=%q)", p.Meta.Mode, p.Meta.LegacyEnv)
	}

	var createRepoRequests []models.CreateRepositoryRequest
	for _, r := range p.Repositories {
		req, err := r.ToCreateRepositoryRequest()
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to convert repository: %w", err)
		}

		createRepoRequests = append(createRepoRequests, req)
	}

	var plot []*models.EquityPlot
	for _, r := range p.EquityPlotRecords {
		plot = append(plot, &models.EquityPlot{
			Timestamp: r.Timestamp,
			Value:     r.Equity,
		})
	}

	err = s.CreatePlayground(p, &backtester_models.PopulatePlaygroundRequest{
		ID:             &p.ID,
		ClientID:       p.ClientID,
		Mode:           p.Meta.Mode,
		Reconciliation: p.Meta.IsReconciliation(),
		Account: backtester_models.CreateAccountRequest{
			Balance: p.Balance,
			Source:  source,
		},
		InitialBalance:    p.Meta.InitialBalance,
		Clock:             clockRequest,
		Repositories:      createRepoRequests,
		BackfillOrders:    p.Orders,
		CreatedAt:         p.CreatedAt,
		EquityPlotRecords: plot,
		Tags:              p.Tags,
		LiveAccount:       liveAccount,
		Calendar:          calendar,
		SaveToDB:          false,
	})

	if err != nil {
		return fmt.Errorf("loadPlaygrounds: failed to create playground: %w", err)
	}

	return nil
}

func (s *DatabaseService) checkPendingCloses(playground *backtester_models.Playground, closeOrderID uint) error {
	orders := playground.GetAllOrders()
	var orderToClose *backtester_models.OrderRecord
	pendingCloseQuantity := 0.0
	for _, order := range orders {
		if order.IsFilled() {
			if order.ID == closeOrderID {
				orderToClose = order
			}
		} else if order.Status == backtester_models.OrderRecordStatusPending {
			if order.CloseOrderId != nil && *order.CloseOrderId == closeOrderID {
				pendingCloseQuantity += order.AbsoluteQuantity
			}
		}
	}

	if orderToClose == nil {
		return fmt.Errorf("failed to find order to close: %d", closeOrderID)
	}

	remainingOpenQty, err := orderToClose.GetRemainingOpenQuantity()
	if err != nil {
		return fmt.Errorf("failed to get remaining open quantity: %w", err)
	}

	if pendingCloseQuantity > math.Abs(remainingOpenQty) {
		return fmt.Errorf("pending close quantity %.2f is greater than remaining open quantity %.2f", pendingCloseQuantity, remainingOpenQty)
	}

	return nil
}

func (s *DatabaseService) PlaceOrder(playgroundID uuid.UUID, requests *backtester_models.CreateOrderRequest) (*backtester_models.OrderRecord, error) {
	orders, err := s.PlaceOrders(playgroundID, []*backtester_models.CreateOrderRequest{requests})
	if err != nil {
		return nil, fmt.Errorf("PlaceOrder: %w", err)
	}

	return orders[0], nil
}

func (s *DatabaseService) PlaceOrders(playgroundID uuid.UUID, requests []*backtester_models.CreateOrderRequest) ([]*backtester_models.OrderRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(playgroundID)
	if err != nil {
		return nil, models.NewWebError(404, "playground not found", err)
	}

	playground.GetPlaceOrderLock().Lock()
	defer playground.GetPlaceOrderLock().Unlock()

	for _, req := range requests {
		if err := req.Validate(); err != nil {
			return nil, models.NewWebError(400, "invalid request", err)
		}

		if req.CloseOrderId != nil {
			if err := s.checkPendingCloses(playground, *req.CloseOrderId); err != nil {
				return nil, models.NewWebError(400, "pending closes check failed", err)
			}
		}
	}

	var orders []*backtester_models.OrderRecord
	createdOn := playground.GetCurrentTime()
	for _, req := range requests {
		order, err := s.commitOrderRecord(playground, req, createdOn)
		if err != nil {
			return nil, models.NewWebError(500, "failed to place order", err)
		}

		if playground.Meta.Mode != backtester_models.ModeSimulation {
			if err := s.orderStore.waitForOrderRecord(order.ID); err != nil {
				return nil, models.NewWebError(500, "failed to wait for order record", err)
			}
		}

		if telemetry.ShouldEmitOrderTelemetry(playground.Meta.LegacyEnv) {
			log.WithFields(log.Fields{
				"event":         "order_placed",
				"playground_id": playground.GetId().String(),
				"order_id":      order.ID,
				"symbol":        req.Symbol,
				"side":          string(req.Side),
				"quantity":      req.Quantity,
				"order_type":    string(req.OrderType),
				"environment":   playground.Meta.LegacyEnv,
				"account_type":  string(playground.Meta.Role),
				"client_id":     telemetry.ClientIDOrEmpty(playground.GetClientId()),
			}).Info("order placed")

			if telemetry.OrdersPlaced != nil {
				telemetry.OrdersPlaced.Add(context.Background(), 1, telemetry.PlaygroundAttrs(playground.Meta.LegacyEnv, string(playground.Meta.Role), telemetry.ClientIDOrEmpty(playground.GetClientId())))
			}
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func (s *DatabaseService) commitOrderRecord(playground *backtester_models.Playground, req *backtester_models.CreateOrderRequest, createdOn time.Time) (*backtester_models.OrderRecord, error) {
	order := &backtester_models.OrderRecord{}
	if req.Id != nil {
		order.ID = *req.Id
	} else {
		// use database to generate a new ID
		if playground.Meta.Mode != backtester_models.ModeSimulation {
			order.PlaygroundID = playground.GetId()
			if err := s.db.Create(&order).Error; err != nil {
				return nil, fmt.Errorf("makeOrderRecord: failed to create order record: %w", err)
			}
		}
	}

	if playground.Meta.Mode == backtester_models.ModeSimulation {
		externalId := playground.NextOrderID()
		order.ID = externalId
		req.ExternalOrderID = &externalId
	}

	backtester_models.PopulateOrderRecord(
		order,
		req.ExternalOrderID,
		req.ClientRequestID,
		playground.GetId(),
		req.Symbol,
		req.Class,
		playground.Meta.Role,
		createdOn,
		req.Side,
		req.Quantity,
		req.OrderType,
		req.Duration,
		req.RequestedPrice,
		req.Price,
		req.StopPrice,
		backtester_models.OrderRecordStatusPending,
		req.Tag,
		req.CloseOrderId,
		req.IsSystemOrder,
		req.Attributes,
		req.PreviousBalance,
	)

	order.SignalID = req.SignalID

	if req.IsAdjustment {
		if !playground.Meta.IsReconciliation() {
			return nil, fmt.Errorf("makeOrderRecord: only reconcile playgrounds can place adjustment orders")
		}

		order.IsAdjustment = true
		order.AccountRole = backtester_models.AccountRoleReconcilation
		log.Infof("placing adjustment order: %v", order)
	}

	changes, err := playground.PlaceOrder(order)
	if err != nil {
		return nil, fmt.Errorf("placeOrder: failed to place order: %w", err)
	}

	// Apply the in-memory commits in order while staging the order-save
	// intents, then persist the whole batch inside one privately-owned
	// transaction (all-or-nothing, same as the former CreateTransaction).
	var saveIntents []backtester_models.OrderSaveIntent
	for _, change := range changes {
		if change.Commit != nil {
			if e := change.Commit(); e != nil {
				return nil, fmt.Errorf("placeOrder: failed to commit order change: %w", e)
			}
		}

		saveIntents = append(saveIntents, change.SaveIntents...)

		log.Infof("done committing order change: %s", change.Info)
	}

	if err := s.orderStore.saveOrderRecordIntents(saveIntents); err != nil {
		return nil, fmt.Errorf("placeOrder: failed to commit order changes: %w", err)
	}

	return order, nil
}

func (s *DatabaseService) GetAccountStatsEquity(playgroundID uuid.UUID) ([]*models.EquityPlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(playgroundID)
	if err != nil {
		return nil, models.NewWebError(404, "playground not found", nil)
	}

	plot := playground.GetEquityPlot()
	return plot, nil
}

func (s *DatabaseService) GetAccount(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []backtester_models.OrderRecordStatus, sides []backtester_models.TradierOrderSide, symbols []string) (*backtester_models.GetAccountResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	internalPlayground := s.playgroundStore.getPlayground(playgroundID)
	if internalPlayground == nil {
		return nil, models.NewWebError(404, "playground not found internally", nil)
	}

	var orders []*backtester_models.OrderRecord
	if internalPlayground.GetMode() == backtester_models.ModeSimulation {
		orders = internalPlayground.GetAllOrders()
	} else {
		playground, err := s.playgroundStore.fetchPlaygroundFromDB(playgroundID)
		if err != nil {
			return nil, models.NewWebError(404, "playground not found", nil)
		}

		orders = playground.Orders
	}

	// cache := internalPlayground.GetPositionCache()

	// playground.SetPositionCache(cache)

	positionCache, err := internalPlayground.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, models.NewWebError(500, "failed to get positions", nil)
	}

	positionsKV := positionCache.Iter()

	meta := internalPlayground.GetMeta()
	meta.CurrentTime = internalPlayground.GetCurrentTime()

	response := backtester_models.GetAccountResponse{
		Meta:       meta,
		Balance:    internalPlayground.GetBalance(),
		Equity:     internalPlayground.GetEquity(positionCache),
		FreeMargin: internalPlayground.GetFreeMarginFromPositionMap(positionCache),
		Positions:  positionsKV,
		Events:     internalPlayground.Events,
	}

	if fetchOrders {
		response.Orders = orders
		filterOrders := from != nil || to != nil || len(status) > 0 || len(sides) > 0 || len(symbols) > 0
		if filterOrders {
			filteredOrders := []*backtester_models.OrderRecord{}
			for _, order := range response.Orders {
				var closeTimestampMax *time.Time
				for _, closeOrder := range order.ClosedBy {
					if closeTimestampMax != nil {
						if closeOrder.Timestamp.After(*closeTimestampMax) {
							closeTimestampMax = &closeOrder.Timestamp
						}
					} else {
						closeTimestampMax = &closeOrder.Timestamp
					}
				}

				if closeTimestampMax != nil {
					if from != nil && order.Timestamp.Before(*from) && closeTimestampMax.Before(*from) {
						continue
					}

					if to != nil && order.Timestamp.After(*to) {
						continue
					}
				}

				if len(symbols) > 0 {
					found := false
					for _, s := range symbols {
						if strings.EqualFold(order.Symbol, s) {
							found = true
							break
						}
					}

					if !found {
						continue
					}
				}

				if len(status) > 0 {
					found := false
					for _, s := range status {
						if order.Status == s {
							found = true
							break
						}
					}

					if !found {
						continue
					}
				}

				if len(sides) > 0 {
					found := false
					for _, s := range sides {
						if order.Side == s {
							found = true
							break
						}
					}

					if !found {
						continue
					}
				}

				filteredOrders = append(filteredOrders, order)
			}

			response.Orders = filteredOrders
		}
	}

	return &response, nil
}

// func (s *DatabaseService) getOpenOrders(playgroundID uuid.UUID, symbol models.Instrument) ([]*backtester_models.OrderRecord, error) {
// 	playground, err := s.FetchPlayground(playgroundID)
// 	if err != nil {
// 		return nil, models.NewWebError(404, "playground not found", nil)
// 	}

// 	// todo: add mutex for playground

// 	orders := playground.GetOpenOrders(symbol)

// 	return orders, nil
// }

// func (s *DatabaseService) fetchOrderIdFromDbByExternalOrderId(playgroundId uuid.UUID, externalOrderID uint) (uint, bool) {
// 	var orderRecord backtester_models.OrderRecord

// 	if result := s.db.First(&orderRecord, "playground_id = ? AND external_id = ?", playgroundId, externalOrderID); result.Error != nil {
// 		return 0, false
// 	}

// 	return orderRecord.ID, true
// }

// SavePlayground spans concerns (playground session + order records + equity
// plots), so it stays here as orchestration over the shared tx helpers.
func (s *DatabaseService) SavePlayground(playground *backtester_models.Playground) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Simulator playgrounds use in-memory nonce IDs that would collide
		// with existing GORM auto-increment IDs. Remap them to fresh IDs.
		//
		// Note: backtester_models.RemapAndSavePlayground takes a raw *gorm.DB — a known
		// gorm.DB leak into the models package, out of scope for this refactor.
		if playground.GetMeta().Mode == backtester_models.ModeSimulation {
			return backtester_models.RemapAndSavePlayground(tx, playground)
		}

		// Live/reconcile path — IDs are already GORM-assigned
		var txErr error

		if txErr = savePlaygroundTx(tx, playground); txErr != nil {
			return fmt.Errorf("failed to save playground session: %w", txErr)
		}

		playgroundId := playground.GetId()

		orders := playground.GetAllOrders()
		if txErr = saveOrderRecordsTx(tx, orders, false); txErr != nil {
			return fmt.Errorf("failed to save order records: %w", txErr)
		}

		if txErr = saveEquityPlotRecords(tx, playgroundId, playground.GetEquityPlot()); txErr != nil {
			return fmt.Errorf("failed to save equity plot records: %w", txErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("savePlayground: failed to save playground: %w", err)
	}

	return nil
}

func CreateRepository(symbol models.StockSymbol, timespan models.PolygonTimespan, bars []*models.PolygonAggregateBarV2, indicators []string, newCandlesQueue *models.FIFOQueue[*backtester_models.BacktesterCandle], historyInDays uint32, source models.CandleRepositorySource) (*backtester_models.CandleRepository, error) {
	return CreateRepositoryWithPosition(symbol, timespan, bars, indicators, newCandlesQueue, 0, historyInDays, source)
}

func CreateRepositoryWithPosition(symbol models.StockSymbol, timespan models.PolygonTimespan, bars []*models.PolygonAggregateBarV2, indicators []string, newCandlesQueue *models.FIFOQueue[*backtester_models.BacktesterCandle], startingPosition int, historyInDays uint32, source models.CandleRepositorySource) (*backtester_models.CandleRepository, error) {
	period := timespan.ToDuration()
	repo, err := backtester_models.NewCandleRepository(symbol, period, bars, indicators, newCandlesQueue, historyInDays, source)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return repo, nil
}
