package data

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/go/backtester-api/models"
)

// equityStore owns the equity plot queries. It holds no in-memory cache.
//
// Locking: the store performs NO locking of its own. Callers (the DatabaseService
// public methods) hold the service-wide mutex where they did before the split.
type equityStore struct {
	db *gorm.DB
}

func newEquityStore(db *gorm.DB) *equityStore {
	return &equityStore{db: db}
}

func (st *equityStore) fetchLiveAccountPlots(liveAccount models.ILiveAccount) ([]models.LiveAccountPlot, error) {
	var items []models.LiveAccountPlot
	if err := st.db.Where("live_account_id = ?", liveAccount.GetId()).Order("timestamp DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to append equity plot records: %w", err)
	}

	return items, nil
}

func (st *equityStore) saveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	rec := &models.EquityPlotRecord{
		PlaygroundID: playgroundId,
		Timestamp:    timestamp,
		Equity:       equity,
	}

	if err := st.db.Create(rec).Error; err != nil {
		return fmt.Errorf("SaveEquityPlotRecord: failed to save equity plot record: %w", err)
	}

	return nil
}

// --- DatabaseService public surface: thin delegations to equityStore ---

// GetEquityPlots spans concerns (playground lookup + equity query), so the
// orchestration stays here and calls into both stores.
func (s *DatabaseService) GetEquityPlots(playgroundId uuid.UUID) ([]models.LiveAccountPlot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	playground, found := s.playgroundStore.playgrounds[playgroundId]
	if !found {
		return nil, fmt.Errorf("failed to find playground: %s", playgroundId.String())
	}

	liveAccount := playground.GetLiveAccount()
	if liveAccount == nil {
		return nil, fmt.Errorf("failed to find live account for playground: %s", playgroundId.String())
	}

	return s.equityStore.fetchLiveAccountPlots(liveAccount)
}

func (s *DatabaseService) SaveEquityPlotRecord(playgroundId uuid.UUID, timestamp time.Time, equity float64) error {
	return s.equityStore.saveEquityPlotRecord(playgroundId, timestamp, equity)
}
