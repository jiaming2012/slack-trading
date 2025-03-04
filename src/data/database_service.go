package data

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/backtester-api/models"
)

type DatabaseService struct {
	mu           sync.Mutex
	playgrounds  map[uuid.UUID]models.IPlayground
	liveAccounts map[models.CreateAccountRequestSource]models.ILiveAccount
}

var (
	db *gorm.DB
)

func NewDatabaseService(_db *gorm.DB) *DatabaseService {
	db = _db

	return &DatabaseService{
		playgrounds:  make(map[uuid.UUID]models.IPlayground),
		liveAccounts: make(map[models.CreateAccountRequestSource]models.ILiveAccount),
	}
}

func (s *DatabaseService) FetchPlayground(playgroundId uuid.UUID) (models.IPlayground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if playground, found := s.playgrounds[playgroundId]; found {
		return playground, nil
	}

	return nil, fmt.Errorf("DatabaseService: playground not found: %s", playgroundId.String())
}

func (s *DatabaseService) SaveInMemoryPlayground(p models.IPlayground) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.playgrounds[p.GetId()] = p
	return nil
}

func (s *DatabaseService) LoadLiveAccounts(apiService models.IBacktesterApiService) error {
	var liveAccountsRecords []models.LiveAccount
	var err error

	if err = db.Preload("ReconcilePlaygroundSession").Preload("ReconcilePlaygroundSession.Orders").Preload("ReconcilePlaygroundSession.Orders.Trades").Preload("ReconcilePlaygroundSession.Orders.Closes").Preload("ReconcilePlaygroundSession.Orders.ClosedBy").Preload("ReconcilePlaygroundSession.Orders.Closes.ClosedBy").Preload("ReconcilePlaygroundSession.EquityPlotRecords").Find(&liveAccountsRecords).Error; err != nil {
		return fmt.Errorf("loadLiveAccounts: failed to load live accounts: %w", err)
	}

	for _, a := range liveAccountsRecords {
		_playground, found := s.playgrounds[a.ReconcilePlaygroundID]
		if !found {
			_playground, err = apiService.PopulatePlayground(a.ReconcilePlaygroundSession)
			if err != nil {
				return fmt.Errorf("loadLiveAccounts: failed to populate playground: %w", err)
			}

			s.playgrounds[a.ReconcilePlaygroundID] = _playground
		}

		playground, ok := _playground.(*models.Playground)
		if !ok {
			return fmt.Errorf("loadLiveAccounts: failed to cast playground to playground: %w", err)
		}

		reconcilePlayground, err := models.NewReconcilePlayground(playground)
		if err != nil {
			return fmt.Errorf("loadLiveAccounts: failed to create reconcile playground: %w", err)
		}

		acc, err := apiService.CreateLiveAccount(a.BrokerName, a.AccountType, reconcilePlayground)
		if err != nil {
			return fmt.Errorf("loadLiveAccounts: failed to create live account: %w", err)
		}

		source := models.CreateAccountRequestSource{
			Broker:      a.BrokerName,
			AccountID:   a.AccountId,
			AccountType: a.AccountType,
		}

		if _, found := s.liveAccounts[source]; found {
			return fmt.Errorf("loadLiveAccounts: duplicate live account source: %v", source)
		}

		s.liveAccounts[source] = acc
	}

	log.Info("loaded all live accounts")

	return nil
}

func (s *DatabaseService) FetchPendingOrders(accountType models.LiveAccountType) ([]*models.OrderRecord, error) {
	var orders []*models.OrderRecord

	if err := db.Preload("Playground").Preload("Playground.LiveAccount").Where("status = ? and account_type = ?", string(models.BacktesterOrderStatusPending), string(accountType)).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch pending orders: %w", err)
	}

	for i, o := range orders {
		if o == nil {
			return nil, fmt.Errorf("fetch pending orders query failed with status %s and account type %s", string(models.BacktesterOrderStatusPending), string(accountType))
		}

		iLiveAccount, found, err := s.FetchLiveAccount(&models.CreateAccountRequestSource{
			Broker:      *o.Playground.Broker,
			AccountID:   *o.Playground.AccountID,
			AccountType: models.LiveAccountType(*o.Playground.LiveAccountType),
		})

		if err != nil {
			return nil, fmt.Errorf("failed to fetch live account: %w", err)
		}

		if !found {
			return nil, fmt.Errorf("failed to find live account: %v", o.Playground.LiveAccount)
		}

		liveAccount, ok := iLiveAccount.(*models.LiveAccount)
		if !ok {
			return nil, fmt.Errorf("failed to cast live account to live account: %v", iLiveAccount)
		}

		orders[i].Playground.LiveAccount = liveAccount
	}

	return orders, nil
}

func (s *DatabaseService) LoadPlaygrounds(apiService models.IBacktesterApiService) error {
	var playgroundsSlice []models.PlaygroundSession
	if err := db.Preload("Orders").Preload("Orders.Trades").Preload("Orders.Closes").Preload("Orders.ClosedBy").Preload("Orders.Closes.ClosedBy").Preload("EquityPlotRecords").Find(&playgroundsSlice).Error; err != nil {
		return fmt.Errorf("loadPlaygrounds: failed to load playgrounds: %w", err)
	}

	for _, p := range playgroundsSlice {
		if _, found := s.playgrounds[p.ID]; found {
			log.Debugf("loadPlaygrounds: skipping duplicate playground id: %s", p.ID.String())
			continue
		}

		playground, err := apiService.PopulatePlayground(p)
		if err != nil {
			return fmt.Errorf("loadPlaygrounds: failed to populate playground: %w", err)
		}

		s.playgrounds[playground.GetId()] = playground
	}

	return nil
}

func (s *DatabaseService) FindOrder(playgroundId uuid.UUID, id uint) (models.IPlayground, *models.BacktesterOrder, error) {
	playground, found := s.playgrounds[playgroundId]
	if !found {
		return nil, nil, fmt.Errorf("failed to find playground using id %s", playgroundId)
	}

	orders := playground.GetOrders()
	for _, order := range orders {
		if order.ID == id {
			return playground, order, nil
		}
	}

	return nil, nil, fmt.Errorf("failed to find Order in playground %s", playground.GetId().String())
}

func (s *DatabaseService) UpdatePlaygroundSession(playgroundSession *models.PlaygroundSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := db.Save(playgroundSession).Error; err != nil {
		return fmt.Errorf("DatabaseService: failed to update playground session: %w", err)
	}

	return nil
}
