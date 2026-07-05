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

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/go/dbutils"
	eventmodels "github.com/jiaming2012/slack-trading/src/go/models"
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
	polygonClient        models.IPolygonClient
	polygonOptionsBroker models.IOptionsBroker

	playgroundStore  *playgroundStore
	orderStore       *orderStore
	liveAccountStore *liveAccountStore
	equityStore      *equityStore
}

func NewDatabaseService(db *gorm.DB, polygonClient models.IPolygonClient, optionsBroker models.IOptionsBroker) *DatabaseService {
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

func (s *DatabaseService) GetPolygonClient() models.IPolygonClient {
	return s.polygonClient
}

// CreateTransaction exposes a raw *gorm.DB transaction to callers. This is a
// known gorm.DB leak through the IDatabaseService interface into the models
// package (used by models/playground.go). Closing it is out of scope for the
// store split — see database_service_interface.go.
func (s *DatabaseService) CreateTransaction(transaction func(tx *gorm.DB) error) error {
	return s.db.Transaction(transaction)
}

func (s *DatabaseService) LoadPlaygrounds(calendar *eventmodels.MarketCalendar) error {
	var playgroundsSlice []*models.Playground
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
		if p.Meta.Environment != models.PlaygroundEnvironmentReconcile {
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

		reconcilePlayground, err := models.NewReconcilePlayground(p, liveAccount)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to create reconcile playground: %w", err)
		}

		s.playgroundStore.reconcilePlaygrounds[source] = reconcilePlayground
	}

	// load other playgrounds
	for _, p := range playgroundsSlice {
		if p.Meta.Environment == models.PlaygroundEnvironmentReconcile {
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

func (s *DatabaseService) FindOrder(playgroundId uuid.UUID, id uint) (*models.Playground, *models.OrderRecord, error) {
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

func (s *DatabaseService) CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle]) ([]*models.CandleRepository, *eventmodels.WebError) {
	var feeds []*models.CandleRepository
	for _, repo := range repoRequests {
		var bars, pastBars []*eventmodels.PolygonAggregateBarV2
		var err error

		timespan := eventmodels.PolygonTimespan{
			Multiplier: repo.Timespan.Multiplier,
			Unit:       eventmodels.PolygonTimespanUnit(repo.Timespan.Unit),
		}

		if repo.Source.Type == eventmodels.RepositorySourceTradier {
			// "tradier" marks live playground repos: candles arrive via the live feed,
			// so there are no historical bars to backfill here. The value is persisted
			// in playground records — do not rename or remove without a data migration.
		} else if repo.Source.Type == eventmodels.RepositorySourcePolygon {
			bars, err = s.polygonClient.FetchAggregateBars(eventmodels.StockSymbol(repo.Symbol), timespan, from, to)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch aggregate bars", err)
			}
		} else if repo.Source.Type == eventmodels.RepositorySourceCSV {
			if repo.Source.CSVFilename == nil {
				return nil, eventmodels.NewWebError(400, "missing CSV filename", nil)
			}

			sourceDir := path.Join(s.projectsDir, "slack-trading", "src", "backtester-api", "data", *repo.Source.CSVFilename)

			bars, err = utils.ImportCandlesFromCsv(sourceDir)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to import candles from CSV", err)
			}
		} else {
			return nil, eventmodels.NewWebError(400, "invalid repository source", nil)
		}

		if from != nil {
			pastBars, err = s.polygonClient.FetchPastCandles(eventmodels.StockSymbol(repo.Symbol), timespan, int(repo.HistoryInDays), from)
			if err != nil {
				return nil, eventmodels.NewWebError(500, "failed to fetch past candles", err)
			}
		}

		aggregateBars := append(pastBars, bars...)

		source := eventmodels.CandleRepositorySource{
			Type: string(repo.Source.Type),
		}

		startingPosition := len(pastBars)
		repository, err := CreateRepositoryWithPosition(eventmodels.StockSymbol(repo.Symbol), timespan, aggregateBars, repo.Indicators, newCandlesQueue, startingPosition, repo.HistoryInDays, source)
		if err != nil {
			log.Errorf("failed to create repository: %v", err)
			return nil, eventmodels.NewWebError(500, "failed to create repository", err)
		}

		feeds = append(feeds, repository)
	}

	return feeds, nil
}

func (s *DatabaseService) CreatePlayground(playground *models.Playground, req *models.PopulatePlaygroundRequest) error {
	env := req.Env

	// validations
	if err := env.Validate(); err != nil {
		return eventmodels.NewWebError(400, "invalid playground environment", err)
	}

	if env != models.PlaygroundEnvironmentReconcile {
		if len(req.Repositories) == 0 {
			return eventmodels.NewWebError(400, "missing repositories", nil)
		}
	}

	req.OptionsBroker = s.polygonOptionsBroker

	// create playground
	if env == models.PlaygroundEnvironmentReconcile {
		if req.LiveAccount == nil {
			return eventmodels.NewWebError(400, "reconcile playground is missing live account", nil)
		}

		var err error
		now := req.CreatedAt
		err = models.PopulatePlayground(playground, req, nil, now, nil, nil, nil)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create reconcile playground", err)
		}

		if req.SaveToDB {
			if err = s.SavePlaygroundSession(playground); err != nil {
				return fmt.Errorf("failed to save reconcile playground: %v", err)
			}
		}

	} else if env == models.PlaygroundEnvironmentLive {
		// todo: hot load live account
		if req.LiveAccount == nil {
			return eventmodels.NewWebError(400, "live playground is missing live account", nil)
		}

		// capture all candles up to tomorrow
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		tomorrowStr := tomorrow.Format("2006-01-02")
		from, err := eventmodels.NewPolygonDate(tomorrowStr)
		if err != nil {
			return eventmodels.NewWebError(400, "failed to parse clock.startDate", err)
		}

		newCandlesQueue := eventmodels.NewFIFOQueue[*models.BacktesterCandle]("newCandlesQueue", 999)

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
			return eventmodels.NewWebError(400, "missing account source", nil)
		}

		reconcilePlayground, found, err := s.FetchReconcilePlayground(*req.Account.Source)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to fetch live account", err)
		}

		if !found {
			log.Debugf("failed to create live account: %v. Creating a new one ...", err)

			reconcilePlayground, err = dbutils.CreateReconcilePlayground(s, req.Account.Source, now)
			if err != nil {
				return eventmodels.NewWebError(500, "failed to create new reconcile playground and live account", err)
			}

			// save reconcile playground
			s.playgroundStore.reconcilePlaygrounds[*req.Account.Source] = reconcilePlayground
		}

		req.ReconcilePlayground = reconcilePlayground

		newTradesQueue := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesQueue", 999)
		invalidOrdersQueue := eventmodels.NewFIFOQueue[*models.OrderRecord]("invalidOrdersQueue", 999)
		err = models.PopulatePlayground(playground, req, nil, now, newTradesQueue, invalidOrdersQueue, req.Calendar, repos...)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create reconcile playground", err)
		}

		// always save live playgrounds if flag is set
		if req.SaveToDB {
			if err = s.SavePlaygroundSession(playground); err != nil {
				return fmt.Errorf("failed to save playground: %v", err)
			}
		}

	} else if env == models.PlaygroundEnvironmentSimulator {
		// validations
		from, err := eventmodels.NewPolygonDate(req.Clock.StartDate)
		if err != nil {
			return eventmodels.NewWebError(400, "failed to parse clock.startDate", err)
		}

		to, err := eventmodels.NewPolygonDate(req.Clock.StopDate)
		if err != nil {
			return eventmodels.NewWebError(400, "failed to parse clock.stopDate", err)
		}

		// create clock
		clock, err := s.CreateClock(from, to)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create clock", err)
		}

		// create backtester repositories
		repos, webErr := s.CreateRepos(req.Repositories, from, to, nil)
		if webErr != nil {
			return webErr
		}

		// create playground
		now := clock.CurrentTime
		err = models.PopulatePlayground(playground, req, clock, now, nil, nil, req.Calendar, repos...)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create playground", err)
		}
	} else {
		return eventmodels.NewWebError(400, "invalid playground environment", nil)
	}

	playground.SetEquityPlot(req.EquityPlotRecords)

	playground.SetOpenOrdersCache()

	if err := s.SavePlaygroundInMemory(playground); err != nil {
		return fmt.Errorf("failed to save in-memory playground: %w", err)
	}

	playground.CreatedAt = time.Now()

	return nil
}

func (s *DatabaseService) CreateClock(start, stop *eventmodels.PolygonDate) (*models.Clock, error) {
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
	startDate := eventmodels.PolygonDate{
		Year:  calendarStartDate.Year(),
		Month: int(calendarStartDate.Month()),
		Day:   calendarStartDate.Day(),
	}

	endDate := eventmodels.PolygonDate{
		Year:  stop.Year,
		Month: stop.Month,
		Day:   stop.Day,
	}

	calendar, err := FetchCalendarMap(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("createClock: failed to fetch calendar: %w", err)
	}

	// create clock
	clock := models.NewClock(fromDate, toDate, calendar)

	return clock, nil
}

func (s *DatabaseService) PopulatePlayground(p *models.Playground, calendar *eventmodels.MarketCalendar) error {
	log.Infof("loading playground: %s", p.ID)

	var source *models.CreateAccountRequestSource
	var clockRequest models.CreateClockRequest
	var liveAccount models.ILiveAccount
	var err error

	if p.Meta.Environment == models.PlaygroundEnvironmentSimulator {
		if p.EndAt == nil {
			return fmt.Errorf("loadPlaygrounds: missing end date for simulator playground")
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
			StopDate:  p.EndAt.Format(time.RFC3339),
		}

	} else if p.Meta.Environment == models.PlaygroundEnvironmentLive {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for live playground")
		}

		liveAccountType := p.Meta.LiveAccountType
		if err = liveAccountType.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid live account type for live playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:          *p.BrokerName,
			AccountID:       *p.AccountID,
			LiveAccountType: liveAccountType,
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
		}

		liveAccount, err = s.GetLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for live playground: %w", err)
		}

	} else if p.Meta.Environment == models.PlaygroundEnvironmentReconcile {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for reconcile playground")
		}

		liveAccountType := p.Meta.LiveAccountType
		if err = liveAccountType.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid live account type for reconcile playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:          *p.BrokerName,
			AccountID:       *p.AccountID,
			LiveAccountType: liveAccountType,
		}

		liveAccount, err = s.GetLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for reconcile playground: %w", err)
		}

	} else {
		return fmt.Errorf("loadPlaygrounds: unknown environment: %v", p.Meta.Environment)
	}

	var createRepoRequests []eventmodels.CreateRepositoryRequest
	for _, r := range p.Repositories {
		req, err := r.ToCreateRepositoryRequest()
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to convert repository: %w", err)
		}

		createRepoRequests = append(createRepoRequests, req)
	}

	var plot []*eventmodels.EquityPlot
	for _, r := range p.EquityPlotRecords {
		plot = append(plot, &eventmodels.EquityPlot{
			Timestamp: r.Timestamp,
			Value:     r.Equity,
		})
	}

	err = s.CreatePlayground(p, &models.PopulatePlaygroundRequest{
		ID:       &p.ID,
		ClientID: p.ClientID,
		Env:      p.Meta.Environment,
		Account: models.CreateAccountRequest{
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

func (s *DatabaseService) checkPendingCloses(playground *models.Playground, closeOrderID uint) error {
	orders := playground.GetAllOrders()
	var orderToClose *models.OrderRecord
	pendingCloseQuantity := 0.0
	for _, order := range orders {
		if order.IsFilled() {
			if order.ID == closeOrderID {
				orderToClose = order
			}
		} else if order.Status == models.OrderRecordStatusPending {
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

func (s *DatabaseService) PlaceOrder(playgroundID uuid.UUID, requests *models.CreateOrderRequest) (*models.OrderRecord, error) {
	orders, err := s.PlaceOrders(playgroundID, []*models.CreateOrderRequest{requests})
	if err != nil {
		return nil, fmt.Errorf("PlaceOrder: %w", err)
	}

	return orders[0], nil
}

func (s *DatabaseService) PlaceOrders(playgroundID uuid.UUID, requests []*models.CreateOrderRequest) ([]*models.OrderRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", err)
	}

	playground.GetPlaceOrderLock().Lock()
	defer playground.GetPlaceOrderLock().Unlock()

	for _, req := range requests {
		if err := req.Validate(); err != nil {
			return nil, eventmodels.NewWebError(400, "invalid request", err)
		}

		if req.CloseOrderId != nil {
			if err := s.checkPendingCloses(playground, *req.CloseOrderId); err != nil {
				return nil, eventmodels.NewWebError(400, "pending closes check failed", err)
			}
		}
	}

	var orders []*models.OrderRecord
	createdOn := playground.GetCurrentTime()
	for _, req := range requests {
		order, err := s.commitOrderRecord(playground, req, createdOn)
		if err != nil {
			return nil, eventmodels.NewWebError(500, "failed to place order", err)
		}

		if playground.Meta.Environment != models.PlaygroundEnvironmentSimulator {
			if err := s.orderStore.waitForOrderRecord(order.ID); err != nil {
				return nil, eventmodels.NewWebError(500, "failed to wait for order record", err)
			}
		}

		if telemetry.ShouldEmitOrderTelemetry(string(playground.Meta.Environment)) {
			log.WithFields(log.Fields{
				"event":         "order_placed",
				"playground_id": playground.GetId().String(),
				"order_id":      order.ID,
				"symbol":        req.Symbol,
				"side":          string(req.Side),
				"quantity":      req.Quantity,
				"order_type":    string(req.OrderType),
				"environment":   string(playground.Meta.Environment),
				"account_type":  string(playground.Meta.LiveAccountType),
				"client_id":     telemetry.ClientIDOrEmpty(playground.GetClientId()),
			}).Info("order placed")

			if telemetry.OrdersPlaced != nil {
				telemetry.OrdersPlaced.Add(context.Background(), 1, telemetry.PlaygroundAttrs(string(playground.Meta.Environment), string(playground.Meta.LiveAccountType), telemetry.ClientIDOrEmpty(playground.GetClientId())))
			}
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func (s *DatabaseService) commitOrderRecord(playground *models.Playground, req *models.CreateOrderRequest, createdOn time.Time) (*models.OrderRecord, error) {
	order := &models.OrderRecord{}
	if req.Id != nil {
		order.ID = *req.Id
	} else {
		// use database to generate a new ID
		if playground.Meta.Environment != models.PlaygroundEnvironmentSimulator {
			order.PlaygroundID = playground.GetId()
			if err := s.db.Create(&order).Error; err != nil {
				return nil, fmt.Errorf("makeOrderRecord: failed to create order record: %w", err)
			}
		}
	}

	if playground.Meta.Environment == models.PlaygroundEnvironmentSimulator {
		externalId := playground.NextOrderID()
		order.ID = externalId
		req.ExternalOrderID = &externalId
	}

	models.PopulateOrderRecord(
		order,
		req.ExternalOrderID,
		req.ClientRequestID,
		playground.GetId(),
		req.Symbol,
		req.Class,
		playground.Meta.LiveAccountType,
		createdOn,
		req.Side,
		req.Quantity,
		req.OrderType,
		req.Duration,
		req.RequestedPrice,
		req.Price,
		req.StopPrice,
		models.OrderRecordStatusPending,
		req.Tag,
		req.CloseOrderId,
		req.IsSystemOrder,
		req.Attributes,
		req.PreviousBalance,
	)

	order.SignalID = req.SignalID

	if req.IsAdjustment {
		if playground.Meta.Environment != models.PlaygroundEnvironmentReconcile {
			return nil, fmt.Errorf("makeOrderRecord: only reconcile playgrounds can place adjustment orders")
		}

		order.IsAdjustment = true
		order.LiveAccountType = models.LiveAccountTypeReconcilation
		log.Infof("placing adjustment order: %v", order)
	}

	changes, err := playground.PlaceOrder(order)
	if err != nil {
		return nil, fmt.Errorf("placeOrder: failed to place order: %w", err)
	}

	err = s.CreateTransaction(func(tx *gorm.DB) error {
		for _, change := range changes {
			if e := change.Commit(tx); e != nil {
				return fmt.Errorf("placeOrder: failed to commit order change: %w", e)
			}

			log.Infof("done committing order change: %s", change.Info)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("placeOrder: failed to commit order changes: %w", err)
	}

	return order, nil
}

func (s *DatabaseService) GetAccountStatsEquity(playgroundID uuid.UUID) ([]*eventmodels.EquityPlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, err := s.playgroundStore.fetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	plot := playground.GetEquityPlot()
	return plot, nil
}

func (s *DatabaseService) GetAccount(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []models.OrderRecordStatus, sides []models.TradierOrderSide, symbols []string) (*models.GetAccountResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	internalPlayground := s.playgroundStore.getPlayground(playgroundID)
	if internalPlayground == nil {
		return nil, eventmodels.NewWebError(404, "playground not found internally", nil)
	}

	var orders []*models.OrderRecord
	if internalPlayground.GetEnvironment() == models.PlaygroundEnvironmentSimulator {
		orders = internalPlayground.GetAllOrders()
	} else {
		playground, err := s.playgroundStore.fetchPlaygroundFromDB(playgroundID)
		if err != nil {
			return nil, eventmodels.NewWebError(404, "playground not found", nil)
		}

		orders = playground.Orders
	}

	// cache := internalPlayground.GetPositionCache()

	// playground.SetPositionCache(cache)

	positionCache, err := internalPlayground.UpdatePricesAndGetPositionCache()
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to get positions", nil)
	}

	positionsKV := positionCache.Iter()

	meta := internalPlayground.GetMeta()
	meta.CurrentTime = internalPlayground.GetCurrentTime()

	response := models.GetAccountResponse{
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
			filteredOrders := []*models.OrderRecord{}
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

// func (s *DatabaseService) getOpenOrders(playgroundID uuid.UUID, symbol eventmodels.Instrument) ([]*models.OrderRecord, error) {
// 	playground, err := s.FetchPlayground(playgroundID)
// 	if err != nil {
// 		return nil, eventmodels.NewWebError(404, "playground not found", nil)
// 	}

// 	// todo: add mutex for playground

// 	orders := playground.GetOpenOrders(symbol)

// 	return orders, nil
// }

// func (s *DatabaseService) fetchOrderIdFromDbByExternalOrderId(playgroundId uuid.UUID, externalOrderID uint) (uint, bool) {
// 	var orderRecord models.OrderRecord

// 	if result := s.db.First(&orderRecord, "playground_id = ? AND external_id = ?", playgroundId, externalOrderID); result.Error != nil {
// 		return 0, false
// 	}

// 	return orderRecord.ID, true
// }

// SavePlayground spans concerns (playground session + order records + equity
// plots), so it stays here as orchestration over the shared tx helpers.
func (s *DatabaseService) SavePlayground(playground *models.Playground) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Simulator playgrounds use in-memory nonce IDs that would collide
		// with existing GORM auto-increment IDs. Remap them to fresh IDs.
		//
		// Note: models.RemapAndSavePlayground takes a raw *gorm.DB — a known
		// gorm.DB leak into the models package, out of scope for this refactor.
		if playground.GetMeta().Environment == models.PlaygroundEnvironmentSimulator {
			return models.RemapAndSavePlayground(tx, playground)
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

func CreateRepository(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, bars []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle], historyInDays uint32, source eventmodels.CandleRepositorySource) (*models.CandleRepository, error) {
	return CreateRepositoryWithPosition(symbol, timespan, bars, indicators, newCandlesQueue, 0, historyInDays, source)
}

func CreateRepositoryWithPosition(symbol eventmodels.StockSymbol, timespan eventmodels.PolygonTimespan, bars []*eventmodels.PolygonAggregateBarV2, indicators []string, newCandlesQueue *eventmodels.FIFOQueue[*models.BacktesterCandle], startingPosition int, historyInDays uint32, source eventmodels.CandleRepositorySource) (*models.CandleRepository, error) {
	period := timespan.ToDuration()
	repo, err := models.NewCandleRepository(symbol, period, bars, indicators, newCandlesQueue, historyInDays, source)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return repo, nil
}
