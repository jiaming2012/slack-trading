package data

import (
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

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/dbutils"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

const FetchTradesFromReconciliationOrdersSql = `
SELECT
  t.*
FROM order_reconciles orr
JOIN order_records orec on orr.order_record_id = orec.id
JOIN trade_records t on orec.id = t.order_id
WHERE orr.reconcile_id = $1
`

const FetchReconciliationOrdersSql = `
SELECT orec.*
FROM order_reconciles orr
JOIN order_records orec on orr.order_record_id = orec.id
WHERE orr.reconcile_id = $1
`

type DatabaseService struct {
	mu                   sync.Mutex
	db                   *gorm.DB
	playgrounds          map[uuid.UUID]*models.Playground
	ordersCache          map[uint]*models.OrderRecord
	tradesCache          map[uint]*models.TradeRecord
	liveAccounts         map[models.CreateAccountRequestSource]models.ILiveAccount
	reconcilePlaygrounds map[models.CreateAccountRequestSource]models.IReconcilePlayground
	projectsDir          string
	polygonClient        models.IPolygonClient
	liveRepositories     map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository
	brokerMap            map[models.CreateAccountRequestSource]models.IBroker
	liveAccountsMutex    sync.Mutex
}

func NewDatabaseService(db *gorm.DB, polygonClient models.IPolygonClient) *DatabaseService {
	return &DatabaseService{
		db:                   db,
		playgrounds:          make(map[uuid.UUID]*models.Playground),
		liveAccounts:         make(map[models.CreateAccountRequestSource]models.ILiveAccount),
		reconcilePlaygrounds: make(map[models.CreateAccountRequestSource]models.IReconcilePlayground),
		liveRepositories:     make(map[eventmodels.Instrument]map[time.Duration][]*models.CandleRepository),
		ordersCache:          make(map[uint]*models.OrderRecord),
		tradesCache:          make(map[uint]*models.TradeRecord),
		polygonClient:        polygonClient,
		ordersCache:          make(map[uint]*models.OrderRecord),
		tradesCache:          make(map[uint]*models.TradeRecord),
	}
}

func (s *DatabaseService) FetchReconciliationOrders(reconcileId uint) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord
	if err := s.db.Raw(FetchReconciliationOrdersSql, reconcileId).Scan(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch reconciliation orders: %w", err)
	}

	return orders, nil
}

func (s *DatabaseService) FetchTradesFromReconciliationOrders(reconcileId uint) ([]*models.TradeRecord, error) {
	var trades []*models.TradeRecord
	if err := s.db.Raw(FetchTradesFromReconciliationOrdersSql, reconcileId).Scan(&trades).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch trades from reconciliation orders: %w", err)
	}

	return trades, nil
}

func (s *DatabaseService) FetchReconcilePlayground(source models.CreateAccountRequestSource) (models.IReconcilePlayground, bool, error) {
	reconcilePlayground, found := s.reconcilePlaygrounds[source]
	return reconcilePlayground, found, nil
}

func (s *DatabaseService) FetchReconcilePlaygroundByOrder(order *models.OrderRecord) (models.IReconcilePlayground, bool, error) {
	playground, err := s.FetchPlayground(order.PlaygroundID)
	if err != nil {
		return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: failed to fetch playground: %w", err)
	}

	if playground.ReconcilePlaygroundID == nil {
		return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: reconcile playground id is nil: %v", playground)
	}

	for _, rp := range s.reconcilePlaygrounds {
		if rp.GetId() == *playground.ReconcilePlaygroundID {
			return rp, true, nil
		}
	}

	return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: failed to find reconcile playground: %v", playground.ReconcilePlaygroundID)
}

func (s *DatabaseService) FetchPlayground(playgroundId uuid.UUID) (*models.Playground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if playground, found := s.playgrounds[playgroundId]; found {
		return playground, nil
	}

	return nil, fmt.Errorf("DatabaseService: playground not found: %s", playgroundId.String())
}

func (s *DatabaseService) SavePlaygroundInMemory(p *models.Playground) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.playgrounds[p.GetId()] = p
	return nil
}

func (s *DatabaseService) CreateTransaction(transaction func(tx *gorm.DB) error) error {
	return s.db.Transaction(transaction)
}

func (s *DatabaseService) PopulateLiveAccount(a *models.LiveAccount) error {
	if a.BrokerName != "tradier" {
		return fmt.Errorf("unsupported broker: %s", a.BrokerName)
	}

	if s.brokerMap == nil {
		return fmt.Errorf("must call LoadLiveAccounts before calling PopulateLiveAccount")
	}

	source := a.GetSource()
	broker, found := s.brokerMap[source]

	if !found {
		return fmt.Errorf("loadLiveAccounts: failed to find broker: %v", a.BrokerName)
	}

	a.SetBroker(broker)
	a.SetDatabase(s)

	return nil
}

func (s *DatabaseService) LoadLiveAccounts(brokerMap map[models.CreateAccountRequestSource]models.IBroker) error {
	var liveAccountsRecords []*models.LiveAccount

	s.brokerMap = brokerMap

	if err := s.db.Find(&liveAccountsRecords).Error; err != nil {
		return fmt.Errorf("loadLiveAccounts: failed to load live accounts: %w", err)
	}

	for _, a := range liveAccountsRecords {
		source := a.GetSource()

		broker, found := brokerMap[source]
		if !found {
			return fmt.Errorf("loadLiveAccounts: failed to find broker: %v", a.BrokerName)
		}

		a.SetBroker(broker)
		a.SetDatabase(s)

		s.liveAccounts[source] = a
	}

	for source, broker := range brokerMap {
		if _, found := s.liveAccounts[source]; !found {
			a, err := models.NewLiveAccount(broker, s)
			if err != nil {
				return fmt.Errorf("failed to create live account: %w", err)
			}

			a.SetBroker(broker)
			a.SetDatabase(s)

			if err := s.db.Save(a).Error; err != nil {
				return fmt.Errorf("failed to save live account: %w", err)
			}

			s.liveAccounts[source] = a
		}
	}

	log.Info("loaded all live accounts")

	return nil
}

func (s *DatabaseService) FetchPendingOrders(accountType models.LiveAccountType, seekFromPlayground bool) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord

	if err := s.db.Where("status = ? and account_type = ?", string(models.OrderRecordStatusPending), string(accountType)).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending orders: %w", err)
	}

	if seekFromPlayground {
		var out []*models.OrderRecord

		for _, o := range orders {
			o2, found := s.ordersCache[o.ID]
			if !found {
				return nil, fmt.Errorf("failed to find order in memory: %d", o.ID)
			}

			out = append(out, o2)
		}

		return out, nil
	}

	return orders, nil
}

func (s *DatabaseService) LoadPlaygrounds() error {
	var playgroundsSlice []*models.Playground
	if err := s.db.Preload("Orders").Preload("Orders.Trades").Preload("Orders.ClosedBy").Preload("Orders.Closes").Preload("Orders.Closes.ClosedBy").Preload("Orders.Closes.Trades").Preload("Orders.Reconciles").Preload("Orders.Reconciles.Trades").Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
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
			s.ordersCache[o.ID] = o

			// Store trades in memory
			for _, t := range o.Trades {
				s.tradesCache[t.ID] = t
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

		if err := s.PopulatePlayground(p); err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to populate reconcile playground: %w", err)
		}

		source, err := p.GetSource()
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get source for reconcile playground: %w", err)
		}

		liveAccount := s.liveAccounts[source]

		if liveAccount == nil {
			return fmt.Errorf("loadPlaygrounds: failed to find live account for reconcile playground: %s", p.ID.String())
		}

		reconcilePlayground, err := models.NewReconcilePlayground(p, liveAccount)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to create reconcile playground: %w", err)
		}

		s.reconcilePlaygrounds[source] = reconcilePlayground
	}

	// load other playgrounds
	for _, p := range playgroundsSlice {
		if p.Meta.Environment == models.PlaygroundEnvironmentReconcile {
			continue
		}

		if _, found := s.playgrounds[p.ID]; found {
			log.Warnf("loadPlaygrounds: skipping duplicate playground id: %s", p.ID.String())
			continue
		}

		if err := s.PopulatePlayground(p); err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to populate live playground: %w", err)
		}
	}

	return nil
}

func (s *DatabaseService) FindOrder(playgroundId uuid.UUID, id uint) (*models.Playground, *models.OrderRecord, error) {
	playground, found := s.playgrounds[playgroundId]
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

func (s *DatabaseService) UpdatePlaygroundSession(playgroundSession *models.Playground) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.db.Save(playgroundSession).Error; err != nil {
		return fmt.Errorf("DatabaseService: failed to update playground session: %w", err)
	}

	return nil
}

func (s *DatabaseService) FetchBalances(url string, token string) (eventmodels.FetchTradierBalancesResponseDTO, error) {
	return eventmodels.FetchTradierBalancesResponseDTO{}, nil
}

// func (s *DatabaseService) CreateLiveAccount(broker models.IBroker, accountType models.LiveAccountType) (*models.LiveAccount, error) {
// if balance < 0 {
// 	return nil, fmt.Errorf("balance cannot be negative")
// }

// if err := source.Validate(); err != nil {
// 	return nil, fmt.Errorf("invalid source: %w", err)
// }

// balance check
// if balance > 0 {
// 	balances, err := source.FetchEquity()
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch equity: %w", err)
// 	}

// 	if balances.Equity < balance {
// 		return nil, fmt.Errorf("balance %.2f is greater than equity %.2f", balance, balances.Equity)
// 	}
// }

// 	account, err := models.NewLiveAccount(broker, s)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create live account: %w", err)
// 	}

// 	return account, nil
// }

func (s *DatabaseService) FetchAllLiveRepositories() (repositories []*models.CandleRepository, releaseLockFn func(), err error) {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	repositories = []*models.CandleRepository{}
	for _, symbolRepo := range s.liveRepositories {
		for _, periodRepos := range symbolRepo {
			repositories = append(repositories, periodRepos...)
		}
	}

	return repositories, func() {
		s.liveAccountsMutex.Unlock()
	}, nil
}

func (s *DatabaseService) RemoveLiveRepository(repo *models.CandleRepository) error {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	symbolRepo, ok := s.liveRepositories[repo.GetSymbol()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: symbol %s not found", repo.GetSymbol())
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		return fmt.Errorf("DeleteLiveRepository: period %s not found", repo.GetPeriod())
	}

	foundRepo := false
	for i, r := range periodRepos {
		if r == repo {
			periodRepos = append(periodRepos[:i], periodRepos[i+1:]...)
			foundRepo = true
			break
		}
	}

	if !foundRepo {
		return fmt.Errorf("DeleteLiveRepository: repository not found")
	}

	symbolRepo[repo.GetPeriod()] = periodRepos
	s.liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
}

func (s *DatabaseService) SaveLiveRepository(repo *models.CandleRepository) error {
	s.liveAccountsMutex.Lock()
	defer s.liveAccountsMutex.Unlock()

	symbolRepo, ok := s.liveRepositories[repo.GetSymbol()]
	if !ok {
		symbolRepo = map[time.Duration][]*models.CandleRepository{}
	}

	periodRepos, ok := symbolRepo[repo.GetPeriod()]
	if !ok {
		periodRepos = []*models.CandleRepository{}
	}

	// append the repo to the periodRepos
	periodRepos = append(periodRepos, repo)
	symbolRepo[repo.GetPeriod()] = periodRepos
	s.liveRepositories[repo.GetSymbol()] = symbolRepo

	return nil
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
			// pass
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

	// create playground
	if env == models.PlaygroundEnvironmentReconcile {
		if req.LiveAccount == nil {
			return eventmodels.NewWebError(400, "reconcile playground is missing live account", nil)
		}

		var err error
		now := req.CreatedAt
		err = models.PopulatePlayground(playground, req, nil, now, nil, nil)
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
			s.reconcilePlaygrounds[*req.Account.Source] = reconcilePlayground
		}

		req.ReconcilePlayground = reconcilePlayground

		newTradesQueue := eventmodels.NewFIFOQueue[*models.TradeRecord]("newTradesQueue", 999)
		err = models.PopulatePlayground(playground, req, nil, now, newTradesQueue, repos...)
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
		err = models.PopulatePlayground(playground, req, clock, now, nil, repos...)
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
	startDate := eventmodels.PolygonDate{
		Year:  start.Year,
		Month: start.Month,
		Day:   start.Day,
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

func (s *DatabaseService) GetLiveAccount(source models.CreateAccountRequestSource) (models.ILiveAccount, error) {
	liveAccount, found := s.liveAccounts[source]
	if !found {
		return nil, fmt.Errorf("failed to find live account: %v", source)
	}

	return liveAccount, nil
}

func (s *DatabaseService) PopulatePlayground(p *models.Playground) error {
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
		InitialBalance:    p.StartingBalance,
		Clock:             clockRequest,
		Repositories:      createRepoRequests,
		BackfillOrders:    p.Orders,
		CreatedAt:         p.CreatedAt,
		EquityPlotRecords: plot,
		Tags:              p.Tags,
		LiveAccount:       liveAccount,
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
		} else {
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

func (s *DatabaseService) PlaceOrder(playgroundID uuid.UUID, req *models.CreateOrderRequest) (*models.OrderRecord, error) {
	playground, err := s.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", err)
	}

	if err := req.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid request", err)
	}

	createdOn := playground.GetCurrentTime()

	if req.CloseOrderId != nil {
		if err := s.checkPendingCloses(playground, *req.CloseOrderId); err != nil {
			return nil, eventmodels.NewWebError(400, "pending closes check failed", err)
		}
	}

	var playgroundEnv models.PlaygroundEnvironment
	playgroundMeta := playground.GetMeta()
	playgroundEnv = playgroundMeta.Environment

	liveOrderTempId := uint(0)
	if playgroundEnv == models.PlaygroundEnvironmentLive {
		req.Id = &liveOrderTempId
	}

	order, err := s.makeOrderRecord(playground, req, createdOn)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to place order", err)
	}

	return order, nil
}

func (s *DatabaseService) makeOrderRecord(playground *models.Playground, req *models.CreateOrderRequest, createdOn time.Time) (*models.OrderRecord, error) {
	var orderId uint
	if req.Id != nil {
		orderId = *req.Id
	} else if req.IsAdjustment {
		orderId = 0
	} else {
		orderId = playground.NextOrderID()
	}

	order := models.NewOrderRecord(
		orderId,
		req.ExternalOrderID,
		req.ClientRequestID,
		playground.GetId(),
		req.Class,
		playground.Meta.LiveAccountType,
		createdOn,
		eventmodels.StockSymbol(req.Symbol),
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
	)

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

	for _, change := range changes {
		change.Commit()
	}

	return order, nil
}

func (s *DatabaseService) GetAccountStatsEquity(playgroundID uuid.UUID) ([]*eventmodels.EquityPlot, error) {
	playground, err := s.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	plot := playground.GetEquityPlot()
	return plot, nil
}

func (s *DatabaseService) GetAccountInfo(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []models.OrderRecordStatus, sides []models.TradierOrderSide, symbols []string) (*models.GetAccountResponse, error) {
	playground, err := s.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	positions, err := playground.GetPositions()
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to get positions", nil)
	}

	positionsKV := map[string]*models.Position{}
	for k, v := range positions {
		positionsKV[k.GetTicker()] = v
	}

	response := models.GetAccountResponse{
		Meta:       playground.GetMeta(),
		Balance:    playground.GetBalance(),
		Equity:     playground.GetEquity(positions),
		FreeMargin: playground.GetFreeMarginFromPositionMap(positions),
		Positions:  positionsKV,
	}

	if fetchOrders {
		response.Orders = playground.GetAllOrders()
		filterOrders := from != nil || to != nil || len(status) > 0 || len(sides) > 0 || len(symbols) > 0
		if filterOrders {
			filteredOrders := []*models.OrderRecord{}
			for _, order := range response.Orders {
				if from != nil && order.Timestamp.Before(*from) {
					continue
				}

				if to != nil && order.Timestamp.After(*to) {
					continue
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

func (s *DatabaseService) getOpenOrders(playgroundID uuid.UUID, symbol eventmodels.Instrument) ([]*models.OrderRecord, error) {
	playground, err := s.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", nil)
	}

	// todo: add mutex for playground

	orders := playground.GetOpenOrders(symbol)

	return orders, nil
}

func (s *DatabaseService) fetchOrderIdFromDbByExternalOrderId(playgroundId uuid.UUID, externalOrderID uint) (uint, bool) {
	var orderRecord models.OrderRecord

	if result := s.db.First(&orderRecord, "playground_id = ? AND external_id = ?", playgroundId, externalOrderID); result.Error != nil {
		return 0, false
	}

	return orderRecord.ID, true
}

func (s *DatabaseService) DeletePlaygroundSession(playground *models.Playground) error {
	session := &models.Playground{
		ID: playground.GetId(),
	}

	if err := s.db.Delete(&session).Error; err != nil {
		return fmt.Errorf("deletePlayground: failed to delete playground: %w", err)
	}

	return nil
}

func (s *DatabaseService) SavePlayground(playground *models.Playground) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var txErr error

		if txErr = savePlaygroundTx(tx, playground); txErr != nil {
			return fmt.Errorf("failed to save playground session: %w", txErr)
		}

		playgroundId := playground.GetId()

		if txErr = saveOrderRecordsTx(tx, playground.GetAllOrders(), false); txErr != nil {
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

func (s *DatabaseService) SaveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	rec := &models.EquityPlotRecord{
		PlaygroundID: playgroundId,
		Timestamp:    timestamp,
		Equity:       equity,
	}

	if err := s.db.Create(rec).Error; err != nil {
		return fmt.Errorf("SaveEquityPlotRecord: failed to save equity plot record: %w", err)
	}

	return nil
}

func (s *DatabaseService) SavePlaygroundSession(playground *models.Playground) error {
	return savePlaygroundTx(s.db, playground)
}

func (s *DatabaseService) SaveOrderRecord(order *models.OrderRecord, newBalance *float64, forceNew bool) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var e error
		if e = saveOrderRecordsTx(tx, []*models.OrderRecord{order}, forceNew); e != nil {
			return fmt.Errorf("saveOrderRecord: failed to save order records: %w", e)
		}

		if newBalance != nil {
			if e := saveBalance(tx, order.PlaygroundID, *newBalance); e != nil {
				return fmt.Errorf("saveOrderRecord: failed to save balance: %w", e)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("saveOrderRecord: save order record transaction failed: %w", err)
	}

	// save in cache
	s.ordersCache[order.ID] = order
	for _, t := range order.Trades {
		s.tradesCache[t.ID] = t
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
