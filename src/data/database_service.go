package data

import (
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
	"github.com/jiaming2012/slack-trading/src/eventmodels"
	"github.com/jiaming2012/slack-trading/src/utils"
)

type DatabaseService struct {
	mu                   sync.Mutex
	db                   *gorm.DB
	playgrounds          map[uuid.UUID]*models.Playground
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
		polygonClient:        polygonClient,
	}
}

func (s *DatabaseService) FetchReconcilePlayground(source models.CreateAccountRequestSource) (models.IReconcilePlayground, bool, error) {
	reconcilePlayground, found := s.reconcilePlaygrounds[source]

	return reconcilePlayground, found, nil

}

func (s *DatabaseService) FetchPlayground(playgroundId uuid.UUID) (*models.Playground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if playground, found := s.playgrounds[playgroundId]; found {
		return playground, nil
	}

	return nil, fmt.Errorf("DatabaseService: playground not found: %s", playgroundId.String())
}

func (s *DatabaseService) SaveInMemoryPlayground(p *models.Playground) error {
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

	log.Info("loaded all live accounts")

	return nil
}

func (s *DatabaseService) FetchPendingOrders(accountType models.LiveAccountType) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord

	if err := s.db.Where("status = ? and account_type = ?", string(models.OrderRecordStatusPending), string(accountType)).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending orders: %w", err)
	}

	return orders, nil
}

func (s *DatabaseService) LoadPlaygrounds() error {
	var playgroundsSlice []*models.Playground
	if err := s.db.Preload("Orders").Preload("Orders.Trades").Preload("Orders.Closes").Preload("Orders.ClosedBy").Preload("Orders.Closes.ClosedBy").Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
		return fmt.Errorf("loadPlaygrounds: failed to load playgrounds: %w", err)
	}

	// load reconcile playgrounds first
	for _, p := range playgroundsSlice {
		if p.Env != string(models.PlaygroundEnvironmentReconcile) {
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
		if p.Env == string(models.PlaygroundEnvironmentReconcile) {
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

	orders := playground.GetOrders()
	for _, order := range orders {
		if *order.ExternalOrderID == id {
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

func (s *DatabaseService) CreatePlayground(playground *models.Playground, req *models.CreatePlaygroundRequest) error {
	env := models.PlaygroundEnvironment(req.Env)

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

		playground.SetLiveAccount(req.LiveAccount)
		
		var err error
		now := req.CreatedAt
		err = models.PopulatePlayground(playground, req.ClientID, req.Account.Balance, req.InitialBalance, nil, req.BackfillOrders, env, now, req.Tags)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create reconcile playground", err)
		}

		if req.SaveToDB {
			if err = s.SavePlaygroundSession(playground); err != nil {
				return fmt.Errorf("failed to save reconcile playground: %v", err)
			}
		}

	} else if env == models.PlaygroundEnvironmentLive {
		if req.LiveAccount == nil {
			return eventmodels.NewWebError(400, "live playground is missing live account", nil)
		}

		playground.SetLiveAccount(req.LiveAccount)

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

			reconcilePlayground, err = s.createNewReconcilePlayground(req.Account.Source, now)
			if err != nil {
				return eventmodels.NewWebError(500, "failed to create new reconcile playground and live account", err)
			}
		}

		playground.SetReconcilePlayground(reconcilePlayground)

		err = models.PopulatePlayground(playground, req.ClientID, req.Account.Balance, req.InitialBalance, nil, req.BackfillOrders, env, now, req.Tags, repos...)
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
		err = models.PopulatePlayground(playground, req.ClientID, req.Account.Balance, req.InitialBalance, clock, req.BackfillOrders, env, now, req.Tags, repos...)
		if err != nil {
			return eventmodels.NewWebError(500, "failed to create playground", err)
		}
	} else {
		return eventmodels.NewWebError(400, "invalid playground environment", nil)
	}

	playground.SetEquityPlot(req.EquityPlotRecords)

	playground.SetOpenOrdersCache()

	if err := s.SaveInMemoryPlayground(playground); err != nil {
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

func (s *DatabaseService) getLiveAccount(source models.CreateAccountRequestSource) (models.ILiveAccount, error) {
	liveAccount, found := s.liveAccounts[source]
	if !found {
		return nil, fmt.Errorf("failed to find live account: %v", source)
	}

	return liveAccount, nil
}

func (s *DatabaseService) createNewReconcilePlayground(source *models.CreateAccountRequestSource, createdAt time.Time) (*models.ReconcilePlayground, error) {
	if source == nil {
		return nil, fmt.Errorf("source is nil")
	}

	liveAccount, err := s.getLiveAccount(*source)
	if err != nil {
		return nil, fmt.Errorf("failed to get broker: %v", err)
	}

	createPlaygroundReq := &models.CreatePlaygroundRequest{
		Env: string(models.PlaygroundEnvironmentReconcile),
		Account: models.CreateAccountRequest{
			Source: source,
		},
		Repositories: nil,
		CreatedAt:    createdAt,
		LiveAccount:  liveAccount,
		SaveToDB:     true,
	}

	var playground *models.Playground
	if err := s.CreatePlayground(playground, createPlaygroundReq); err != nil {
		return nil, fmt.Errorf("failed to create playground: %v", err)
	}

	// liveAccount, err := s.CreateLiveAccount(broker, createPlaygroundReq.Account.Source.AccountType)
	// if err != nil {
	// 	return nil, eventmodels.NewWebError(500, "failed to create live account", err)
	// }

	reconcilePlayground, err := models.NewReconcilePlayground(playground, liveAccount)
	if err != nil {
		return nil, eventmodels.NewWebError(500, "failed to create new reconcile playground", err)
	}

	// reconcilePlaygroundId := reconcilePlayground.GetId()
	// playground.ReconcilePlayground = reconcilePlayground
	// playground.ReconcilePlaygroundID = &reconcilePlaygroundId

	// update playground balance
	// response, err := liveAccount.Source.FetchEquity()
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to fetch equity: %w", err)
	// }
	// balance := response.Equity

	// playground.SetBalance(balance)
	// playground.Meta.InitialBalance = balance
	// playground.Meta.SourceBroker = liveAccount.Source.GetBroker()
	// playground.Meta.SourceAccountId = liveAccount.Source.GetAccountID()
	// playground.Meta.LiveAccountType = liveAccount.Source.GetAccountType()

	// playgroundSession.Balance = balance
	// playgroundSession.StartingBalance = balance
	// playgroundSession.BrokerName = &playground.Meta.SourceBroker
	// playgroundSession.AccountID = &playground.Meta.SourceAccountId

	// liveAccountType := string(liveAccount.Source.GetAccountType())
	// playgroundSession.AccountType = &liveAccountType

	if err := s.UpdatePlaygroundSession(playground); err != nil {
		return nil, fmt.Errorf("failed to update playground session: %v", err)
	}

	return reconcilePlayground, nil
}

func (s *DatabaseService) PopulatePlayground(p *models.Playground) error {
	log.Infof("loading playground: %s", p.ID)

	var source *models.CreateAccountRequestSource
	var clockRequest models.CreateClockRequest
	var liveAccount models.ILiveAccount
	var err error

	if p.Env == "simulator" {
		if p.EndAt == nil {
			return fmt.Errorf("loadPlaygrounds: missing end date for simulator playground")
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
			StopDate:  p.EndAt.Format(time.RFC3339),
		}

	} else if p.Env == "live" {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for live playground")
		}

		liveAccountType := models.LiveAccountType(p.AccountType)
		if err = liveAccountType.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid live account type for live playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:      *p.BrokerName,
			AccountID:   *p.AccountID,
			AccountType: liveAccountType,
		}

		clockRequest = models.CreateClockRequest{
			StartDate: p.StartAt.Format(time.RFC3339),
		}

		liveAccount, err = s.getLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for live playground: %w", err)
		}

	} else if p.Env == "reconcile" {
		if p.BrokerName == nil || p.AccountID == nil {
			return fmt.Errorf("loadPlaygrounds: missing broker, account id, or api key for reconcile playground")
		}

		liveAccountType := models.LiveAccountType(p.AccountType)
		if err = liveAccountType.Validate(); err != nil {
			return fmt.Errorf("loadPlaygrounds: invalid live account type for reconcile playground: %w", err)
		}

		source = &models.CreateAccountRequestSource{
			Broker:      *p.BrokerName,
			AccountID:   *p.AccountID,
			AccountType: liveAccountType,
		}

		liveAccount, err = s.getLiveAccount(*source)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to get live account for reconcile playground: %w", err)
		}

	} else {
		return fmt.Errorf("loadPlaygrounds: unknown environment: %v", p.Env)
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

	err = s.CreatePlayground(p, &models.CreatePlaygroundRequest{
		ID:       &p.ID,
		ClientID: p.ClientID,
		Env:      p.Env,
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

func (s *DatabaseService) PlaceOrder(playgroundID uuid.UUID, req *models.CreateOrderRequest) (*models.OrderRecord, error) {
	playground, err := s.FetchPlayground(playgroundID)
	if err != nil {
		return nil, eventmodels.NewWebError(404, "playground not found", err)
	}

	if err := req.Validate(); err != nil {
		return nil, eventmodels.NewWebError(400, "invalid request", err)
	}

	createdOn := playground.GetCurrentTime()

	var playgroundEnv models.PlaygroundEnvironment
	playgroundMeta := playground.GetMeta()
	if playgroundMeta != nil {
		playgroundEnv = playgroundMeta.Environment
	} else {
		return nil, eventmodels.NewWebError(500, "failed to get playground environment", nil)
	}

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
	} else {
		orderId = playground.NextOrderID()
	}

	order := models.NewOrderRecord(
		orderId,
		req.ExternalOrderID,
		playground.GetId(),
		req.Class,
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

func (s *DatabaseService) GetAccountInfo(playgroundID uuid.UUID, fetchOrders bool, from, to *time.Time, status []models.OrderRecordStatus, sides []models.TradierOrderSide) (*models.GetAccountResponse, error) {
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
		response.Orders = playground.GetOrders()
		filterOrders := from != nil || to != nil || len(status) > 0 || len(sides) > 0
		if filterOrders {
			filteredOrders := []*models.OrderRecord{}
			for _, order := range response.Orders {
				if from != nil && order.Timestamp.Before(*from) {
					continue
				}

				if to != nil && order.Timestamp.After(*to) {
					continue
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
		meta := playground.GetMeta()
		if meta == nil {
			return errors.New("savePlayground: missing playground meta")
		}

		if txErr = saveOrderRecordsTx(tx, playground.GetOrders(), false); txErr != nil {
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
		var oRecs []*models.OrderRecord
		var e error
		if e = saveOrderRecordsTx(tx, []*models.OrderRecord{order}, forceNew); e != nil {
			return fmt.Errorf("saveOrderRecord: failed to save order records: %w", e)
		}

		if len(oRecs) != 1 {
			return fmt.Errorf("saveOrderRecord: expected 1 order record, got %d", len(oRecs))
		}

		if newBalance != nil {
			if err := saveBalance(tx, order.PlaygroundID, *newBalance); err != nil {
				return fmt.Errorf("saveOrderRecord: failed to save balance: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("saveOrderRecord: save order record transaction failed: %w", err)
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
