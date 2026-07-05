package data

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	backtester_models "github.com/jiaming2012/slack-trading/src/go/backtester/models"
	"github.com/jiaming2012/slack-trading/src/go/models"
	"github.com/jiaming2012/slack-trading/src/go/telemetry"
)

// playgroundStore owns the in-memory playground and reconcile-playground caches
// plus the playground-centric GORM queries.
//
// Locking: the store performs NO locking of its own. Callers (the DatabaseService
// public methods) hold the service-wide mutex exactly as they did before the
// split — locking granularity was intentionally left unchanged. Do not add
// per-store locking without auditing every call site.
type playgroundStore struct {
	db                   *gorm.DB
	playgrounds          map[uuid.UUID]*backtester_models.Playground
	reconcilePlaygrounds map[backtester_models.CreateAccountRequestSource]backtester_models.IReconcilePlayground
}

func newPlaygroundStore(db *gorm.DB) *playgroundStore {
	return &playgroundStore{
		db:                   db,
		playgrounds:          make(map[uuid.UUID]*backtester_models.Playground),
		reconcilePlaygrounds: make(map[backtester_models.CreateAccountRequestSource]backtester_models.IReconcilePlayground),
	}
}

func (st *playgroundStore) fetchPlayground(playgroundId uuid.UUID) (*backtester_models.Playground, error) {
	if playground, found := st.playgrounds[playgroundId]; found {
		return playground, nil
	}

	return nil, fmt.Errorf("DatabaseService: playground not found: %s", playgroundId.String())
}

func (st *playgroundStore) fetchPlaygroundFromDB(playgroundId uuid.UUID) (*backtester_models.Playground, error) {
	var playground *backtester_models.Playground

	if err := st.db.Preload("Orders", func(db *gorm.DB) *gorm.DB {
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
	}).Preload("EquityPlotRecords").Where("id = ?", playgroundId).First(&playground).Error; err != nil {
		return nil, fmt.Errorf("loadPlaygrounds: failed to load orders in playgrounds: %w", err)
	}

	if playground == nil {
		return nil, fmt.Errorf("failed to find playground in DB: %s", playgroundId.String())
	}

	return playground, nil
}

func (st *playgroundStore) getPlayground(playgroundID uuid.UUID) *backtester_models.Playground {
	playground, ok := st.playgrounds[playgroundID]
	if !ok {
		return nil
	}

	return playground
}

func (st *playgroundStore) getPlaygroundByClientId(clientId string) *backtester_models.Playground {
	for _, playground := range st.playgrounds {
		cId := playground.GetClientId()
		if cId != nil && *cId == clientId {
			return playground
		}
	}

	return nil
}

func (st *playgroundStore) getPlaygrounds() []*backtester_models.Playground {
	var slice []*backtester_models.Playground
	for _, playground := range st.playgrounds {
		slice = append(slice, playground)
	}

	return slice
}

func (st *playgroundStore) deletePlayground(playgroundID uuid.UUID) error {
	_, ok := st.playgrounds[playgroundID]
	if !ok {
		return models.NewWebError(404, "playground not found", nil)
	}

	delete(st.playgrounds, playgroundID)

	return nil
}

func (st *playgroundStore) savePlaygroundInMemory(p *backtester_models.Playground) error {
	st.playgrounds[p.GetId()] = p
	return nil
}

func (st *playgroundStore) getPlaygroundsByReconcileId(reconcileId uuid.UUID) ([]*backtester_models.Playground, error) {
	var playgrounds []*backtester_models.Playground
	for _, p := range st.playgrounds {
		if p.ReconcilePlaygroundID != nil && *p.ReconcilePlaygroundID == reconcileId {
			if p.Meta.Environment == backtester_models.PlaygroundEnvironmentLive {
				playgrounds = append(playgrounds, p)
			}
		}
	}

	return playgrounds, nil
}

func (st *playgroundStore) updatePlaygroundSession(playgroundSession *backtester_models.Playground) error {
	if err := st.db.Save(playgroundSession).Error; err != nil {
		return fmt.Errorf("DatabaseService: failed to update playground session: %w", err)
	}

	return nil
}

func (st *playgroundStore) deletePlaygroundSession(playground *backtester_models.Playground) error {
	session := &backtester_models.Playground{
		ID: playground.GetId(),
	}

	if err := st.db.Delete(&session).Error; err != nil {
		return fmt.Errorf("deletePlayground: failed to delete playground: %w", err)
	}

	return nil
}

func (st *playgroundStore) savePlaygroundSession(playground *backtester_models.Playground) error {
	return savePlaygroundTx(st.db, playground)
}

func (st *playgroundStore) fetchReconcilePlayground(source backtester_models.CreateAccountRequestSource) (backtester_models.IReconcilePlayground, bool, error) {
	reconcilePlayground, found := st.reconcilePlaygrounds[source]
	return reconcilePlayground, found, nil
}

func (st *playgroundStore) fetchReconcilePlaygroundByOrder(order *backtester_models.OrderRecord) (backtester_models.IReconcilePlayground, bool, error) {
	playground, err := st.fetchPlayground(order.PlaygroundID)
	if err != nil {
		return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: failed to fetch playground: %w", err)
	}

	if playground.ReconcilePlaygroundID == nil {
		return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: reconcile playground id is nil: %v", playground)
	}

	for _, rp := range st.reconcilePlaygrounds {
		if rp.GetId() == *playground.ReconcilePlaygroundID {
			return rp, true, nil
		}
	}

	return nil, false, fmt.Errorf("FetchReconcilePlaygroundByOrder: failed to find reconcile playground: %v", playground.ReconcilePlaygroundID)
}

func (st *playgroundStore) heartbeatStats() telemetry.HeartbeatStats {
	stats := telemetry.HeartbeatStats{}
	var latestTick time.Time

	for _, p := range st.playgrounds {
		switch p.Meta.Environment {
		case backtester_models.PlaygroundEnvironmentLive:
			stats.LiveCount++
		case backtester_models.PlaygroundEnvironmentReconcile:
			stats.ReconcileCount++
		case backtester_models.PlaygroundEnvironmentSimulator:
			stats.SimulatorCount++
		}

		// Count open orders for live/reconcile only
		if telemetry.ShouldEmitOrderTelemetry(string(p.Meta.Environment)) {
			for _, order := range p.GetAllOrders() {
				if order.Status == backtester_models.OrderRecordStatusNew ||
					order.Status == backtester_models.OrderRecordStatusPending ||
					order.Status == backtester_models.OrderRecordStatusPartiallyFilled {
					stats.OpenOrderCount++
				}
			}
		}

		// Track latest tick time across all live playgrounds
		if p.Meta.Environment == backtester_models.PlaygroundEnvironmentLive {
			currentTime := p.Meta.CurrentTime
			if currentTime.After(latestTick) {
				latestTick = currentTime
			}
		}
	}

	stats.LastTickTime = latestTick
	return stats
}

// --- DatabaseService public surface: thin delegations to playgroundStore ---

func (s *DatabaseService) GetHeartbeatStats() telemetry.HeartbeatStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.heartbeatStats()
}

func (s *DatabaseService) FetchPlayground(playgroundId uuid.UUID) (*backtester_models.Playground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// return s.playgroundStore.fetchPlaygroundFromDB(playgroundId)
	return s.playgroundStore.fetchPlayground(playgroundId)
}

func (s *DatabaseService) GetPlayground(playgroundID uuid.UUID) (*backtester_models.Playground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground := s.playgroundStore.getPlayground(playgroundID)
	if playground == nil {
		return nil, models.NewWebError(404, "playground not found", nil)
	}

	return playground, nil
}

func (s *DatabaseService) GetPlaygroundByClientId(clientId string) *backtester_models.Playground {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.getPlaygroundByClientId(clientId)
}

func (s *DatabaseService) GetPlaygrounds() []*backtester_models.Playground {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.getPlaygrounds()
}

func (s *DatabaseService) DeletePlayground(playgroundID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.deletePlayground(playgroundID)
}

func (s *DatabaseService) SavePlaygroundInMemory(p *backtester_models.Playground) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.savePlaygroundInMemory(p)
}

func (s *DatabaseService) GetPlaygroundsByReconcileId(reconcileId uuid.UUID) ([]*backtester_models.Playground, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.getPlaygroundsByReconcileId(reconcileId)
}

func (s *DatabaseService) UpdatePlaygroundSession(playgroundSession *backtester_models.Playground) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.updatePlaygroundSession(playgroundSession)
}

func (s *DatabaseService) DeletePlaygroundSession(playground *backtester_models.Playground) error {
	return s.playgroundStore.deletePlaygroundSession(playground)
}

func (s *DatabaseService) SavePlaygroundSession(playground *backtester_models.Playground) error {
	return s.playgroundStore.savePlaygroundSession(playground)
}

func (s *DatabaseService) FetchReconcilePlayground(source backtester_models.CreateAccountRequestSource) (backtester_models.IReconcilePlayground, bool, error) {
	return s.playgroundStore.fetchReconcilePlayground(source)
}

func (s *DatabaseService) FetchReconcilePlaygroundByOrder(order *backtester_models.OrderRecord) (backtester_models.IReconcilePlayground, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.playgroundStore.fetchReconcilePlaygroundByOrder(order)
}
