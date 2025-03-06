package models

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockDatabase struct {
	orderRecords         map[uuid.UUID][]*OrderRecord
	playgrounds          map[uuid.UUID]*Playground
	reconcilePlaygrounds map[uuid.UUID]IReconcilePlayground
}

func (m *MockDatabase) SetReconcilePlayground(playgroundId uuid.UUID, reconcilePlayground IReconcilePlayground) {
	m.reconcilePlaygrounds[playgroundId] = reconcilePlayground
}

func (m *MockDatabase) SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error {
	playgroundId := order.Playground.ID

	if _, found := m.orderRecords[playgroundId]; !found {
		return fmt.Errorf("MockDatabase: playground not found in order records")
	}

	playground, found := m.playgrounds[playgroundId]
	if !found {
		return fmt.Errorf("MockDatabase: playground not found")
	}

	liveAccount, found := m.reconcilePlaygrounds[playgroundId]
	if !found {
		return fmt.Errorf("MockDatabase: live account not found")
	}

	order.Playground = &Playground{
		ID:                  playground.GetId(),
		AccountType:         string(playground.GetLiveAccountType()),
		ReconcilePlayground: liveAccount,
	}

	bFoundOrderRecord := false
	for idx, o := range m.orderRecords[playgroundId] {
		if o.ExternalOrderID == order.ExternalOrderID {
			m.orderRecords[playgroundId][idx] = order
			bFoundOrderRecord = true
			break
		}
	}

	if !bFoundOrderRecord {
		m.orderRecords[playgroundId] = append(m.orderRecords[playgroundId], order)
	}

	return nil
}

func (m *MockDatabase) LoadPlaygrounds() error {
	return nil
}

func (m *MockDatabase) SavePlaygroundSession(playground *Playground) error {
	m.playgrounds[playground.GetId()] = playground
	m.orderRecords[playground.GetId()] = make([]*OrderRecord, 0)
	return nil
}

func (m *MockDatabase) SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error {
	return nil
}

func (m *MockDatabase) UpdatePlaygroundSession(playgroundSession *Playground) error {
	return nil
}

func (m *MockDatabase) FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error) {
	return nil, false, nil
}

func (m *MockDatabase) FetchPlayground(playgroundId uuid.UUID) (*Playground, error) {
	return nil, nil
}

func (m *MockDatabase) GetPlaygrounds() []*Playground {
	return nil
}

func (m *MockDatabase) GetPlaygroundByClientId(clientId string) *Playground {
	return nil
}

func (m *MockDatabase) GetPlayground(playgroundID uuid.UUID) (*Playground, error) {
	return nil, nil
}

func (m *MockDatabase) DeletePlayground(playgroundID uuid.UUID) error {
	return nil
}

func (m *MockDatabase) SaveInMemoryPlayground(p *Playground) error {
	return nil
}

func (m *MockDatabase) FindOrder(playgroundId uuid.UUID, id uint) (*Playground, *OrderRecord, error) {
	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, nil, fmt.Errorf("MockDatabase: playground not found")
	}

	orders := m.orderRecords[playgroundId]
	for _, order := range orders {
		if order.ID == id {
			return playground, order, nil
		}
	}

	return nil, nil, fmt.Errorf("MockDatabase: order not found")
}

func (m *MockDatabase) FetchReconcilePlayground(source CreateAccountRequestSource) (IReconcilePlayground, bool, error) {
	return nil, false, nil
}

func (m *MockDatabase) FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error) {
	var orders []*OrderRecord

	for pId := range m.playgrounds {
		orderRecords := m.orderRecords[pId]
		for _, order := range orderRecords {
			if order.Status == OrderRecordStatusPending && order.AccountType == string(accountType) {
				orders = append(orders, order)
			}
		}
	}

	return orders, nil
}

func (m *MockDatabase) CreateRepos(repoRequests []eventmodels.CreateRepositoryRequest, from, to *eventmodels.PolygonDate, newCandlesQueue *eventmodels.FIFOQueue[*BacktesterCandle]) ([]*CandleRepository, *eventmodels.WebError) {
	return nil, nil
}

func (m *MockDatabase) RemoveLiveRepository(repo *CandleRepository) error {
	return nil
}

func (m *MockDatabase) SaveLiveRepository(repo *CandleRepository) error {
	return nil
}

func (m *MockDatabase) PopulatePlayground(p *Playground) error {
	return nil
}

func (m *MockDatabase) CreateLiveAccount(broker IBroker, accountType LiveAccountType) (*LiveAccount, error) {
	return nil, nil
}

func (m *MockDatabase) CreateTransaction(transaction func(tx *gorm.DB) error) error {
	return nil
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		orderRecords:         make(map[uuid.UUID][]*OrderRecord),
		playgrounds:          make(map[uuid.UUID]*Playground),
		reconcilePlaygrounds: make(map[uuid.UUID]IReconcilePlayground),
	}
}
