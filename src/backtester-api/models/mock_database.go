package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/jiaming2012/slack-trading/src/eventmodels"
)

type MockDatabase struct {
	orderRecords         map[uuid.UUID][]*OrderRecord
	playgrounds          map[uuid.UUID]*Playground
	reconcilePlaygrounds map[CreateAccountRequestSource]IReconcilePlayground
	liveAccounts         map[CreateAccountRequestSource]ILiveAccount
}

func (m *MockDatabase) CreatePlayground(playground *Playground, req *PopulatePlaygroundRequest) error {
	period := time.Minute
	source := eventmodels.CandleRepositorySource{
		Type: "test",
	}

	var symbol eventmodels.StockSymbol
	if len(req.Repositories) == 0 {
		symbol = eventmodels.NewStockSymbol("AAPL")
	} else if len(req.Repositories) == 1 {
		symbol = eventmodels.NewStockSymbol(req.Repositories[0].Symbol)
	} else {
		return fmt.Errorf("only one repository is supported in mock environment")
	}

	startDate, err := time.Parse("2006-01-02", req.Clock.StartDate)
	if err != nil {
		return fmt.Errorf("failed to parse start date: %v", err)
	}

	stopDate, err := time.Parse("2006-01-02", req.Clock.StopDate)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %v", err)
	}

	clock := NewClock(startDate, stopDate, nil)

	feed := []*eventmodels.PolygonAggregateBarV2{
		{
			Timestamp: startDate,
			Open:      100.0,
			High:      101.0,
			Low:       99.0,
			Close:     100.5,
			Volume:    1000,
		},
	}

	repo, err := NewCandleRepository(symbol, period, feed, []string{}, nil, 0, source)
	if err != nil {
		return fmt.Errorf("failed to create mock candle repository: %v", err)
	}

	return PopulatePlayground(playground, req, clock, clock.CurrentTime, repo)
}

func (m *MockDatabase) SetReconcilePlayground(source CreateAccountRequestSource, reconcilePlayground IReconcilePlayground) {
	m.reconcilePlaygrounds[source] = reconcilePlayground
}

func (m *MockDatabase) SaveOrderRecord(order *OrderRecord, newBalance *float64, forceNew bool) error {
	playgroundId := order.PlaygroundID

	if _, found := m.orderRecords[playgroundId]; !found {
		return fmt.Errorf("MockDatabase: playground not found in order records")
	}

	// playground, found := m.playgrounds[playgroundId]
	// if !found {
	// 	return fmt.Errorf("MockDatabase: playground not found")
	// }

	// liveAccount, found := m.reconcilePlaygrounds[playgroundId]
	// if !found {
	// 	return fmt.Errorf("MockDatabase: live account not found")
	// }

	// order.Playground = &Playground{
	// 	ID:                  playground.GetId(),
	// 	AccountType:         string(playground.GetLiveAccountType()),
	// 	ReconcilePlayground: liveAccount,
	// }

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

func (m *MockDatabase) GetLiveAccount(source CreateAccountRequestSource) (ILiveAccount, error) {
	return m.liveAccounts[source], nil
}

func (m *MockDatabase) SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error {
	m.liveAccounts[*source] = liveAccount
	return nil
}

func (m *MockDatabase) UpdatePlaygroundSession(playgroundSession *Playground) error {
	return nil
}

func (m *MockDatabase) FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error) {
	return nil, false, nil
}

func (m *MockDatabase) FetchPlayground(playgroundId uuid.UUID) (*Playground, error) {
	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, fmt.Errorf("MockDatabase: playground not found")
	}

	return playground, nil
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
	p, found := m.reconcilePlaygrounds[source]
	return p, found, nil
}

func (m *MockDatabase) FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error) {
	var orders []*OrderRecord

	for pId := range m.playgrounds {
		orderRecords := m.orderRecords[pId]
		for _, order := range orderRecords {
			if order.Status == OrderRecordStatusPending && order.LiveAccountType == accountType {
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

func (m *MockDatabase) PopulateLiveAccount(l *LiveAccount) error {
	return nil
}

func (m *MockDatabase) LoadLiveAccounts(brokerMap map[CreateAccountRequestSource]IBroker) error {
	return nil
}

func (m *MockDatabase) CreateTransaction(transaction func(tx *gorm.DB) error) error {
	return nil
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		orderRecords:         make(map[uuid.UUID][]*OrderRecord),
		playgrounds:          make(map[uuid.UUID]*Playground),
		reconcilePlaygrounds: make(map[CreateAccountRequestSource]IReconcilePlayground),
	}
}
