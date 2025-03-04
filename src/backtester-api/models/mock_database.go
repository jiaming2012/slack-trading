package models

import (
	"fmt"

	"github.com/google/uuid"
)

type MockDatabase struct {
	orders       map[uuid.UUID][]*BacktesterOrder
	orderRecords map[uuid.UUID][]*OrderRecord
	playgrounds  map[uuid.UUID]IPlayground
	liveAccounts map[uuid.UUID]*LiveAccount
}

func (m *MockDatabase) SetLiveAccount(playgroundId uuid.UUID, liveAccount *LiveAccount) {
	m.liveAccounts[playgroundId] = liveAccount
}

func (m *MockDatabase) SaveOrderRecord(playgroundId uuid.UUID, order *BacktesterOrder, newBalance *float64, liveAccountType LiveAccountType) (*OrderRecord, error) {
	if _, found := m.orderRecords[playgroundId]; !found {
		return nil, fmt.Errorf("MockDatabase: playground not found in order records")
	}

	if _, found := m.orders[playgroundId]; !found {
		return nil, fmt.Errorf("MockDatabase: playground not found in orders")
	}

	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, fmt.Errorf("MockDatabase: playground not found")
	}

	liveAccount, found := m.liveAccounts[playgroundId]
	if !found {
		return nil, fmt.Errorf("MockDatabase: live account not found")
	}

	typ := string(liveAccountType)
	orderRec := order.ToOrderRecord(playgroundId, liveAccountType)
	orderRec.Playground = PlaygroundSession{
		ID:              playground.GetId(),
		LiveAccountType: &typ,
		LiveAccount:     liveAccount,
	}

	bFoundOrderRecord := false
	for idx, o := range m.orderRecords[playgroundId] {
		if o.ExternalOrderID == order.ID {
			m.orderRecords[playgroundId][idx] = orderRec
			bFoundOrderRecord = true
			break
		}
	}

	if !bFoundOrderRecord {
		m.orderRecords[playgroundId] = append(m.orderRecords[playgroundId], orderRec)
	}

	bFoundOrder := false
	for idx, o := range m.orders[playgroundId] {
		if o.ID == order.ID {
			m.orders[playgroundId][idx] = order
			bFoundOrder = true
			break
		}
	}

	if !bFoundOrder {
		m.orders[playgroundId] = append(m.orders[playgroundId], order)
	}

	return orderRec, nil
}

func (m *MockDatabase) LoadPlaygrounds(apiService IBacktesterApiService) error {
	return nil
}

func (m *MockDatabase) SavePlaygroundSession(playground IPlayground) (*PlaygroundSession, error) {
	m.playgrounds[playground.GetId()] = playground
	m.orderRecords[playground.GetId()] = make([]*OrderRecord, 0)
	m.orders[playground.GetId()] = make([]*BacktesterOrder, 0)
	return nil, nil
}

func (m *MockDatabase) SaveLiveAccount(source *CreateAccountRequestSource, liveAccount ILiveAccount) error {
	return nil
}

func (m *MockDatabase) UpdatePlaygroundSession(playgroundSession *PlaygroundSession) error {
	return nil
}

func (m *MockDatabase) FetchLiveAccount(source *CreateAccountRequestSource) (ILiveAccount, bool, error) {
	return nil, false, nil
}

func (m *MockDatabase) FetchPlayground(playgroundId uuid.UUID) (IPlayground, error) {
	return nil, nil
}

func (m *MockDatabase) GetPlaygrounds() []IPlayground {
	return nil
}

func (m *MockDatabase) GetPlaygroundByClientId(clientId string) IPlayground {
	return nil
}

func (m *MockDatabase) GetPlayground(playgroundID uuid.UUID) (IPlayground, error) {
	return nil, nil
}

func (m *MockDatabase) DeletePlayground(playgroundID uuid.UUID) error {
	return nil
}

func (m *MockDatabase) SaveInMemoryPlayground(p IPlayground) error {
	return nil
}

func (m *MockDatabase) FindOrder(playgroundId uuid.UUID, id uint) (IPlayground, *BacktesterOrder, error) {
	playground, found := m.playgrounds[playgroundId]
	if !found {
		return nil, nil, fmt.Errorf("MockDatabase: playground not found")
	}

	orders := m.orders[playgroundId]
	for _, order := range orders {
		if order.ID == id {
			return playground, order, nil
		}
	}

	return nil, nil, fmt.Errorf("MockDatabase: order not found")
}

func (m *MockDatabase) FetchPendingOrders(accountType LiveAccountType) ([]*OrderRecord, error) {
	var orders []*OrderRecord

	for pId := range m.playgrounds {
		orderRecords := m.orderRecords[pId]
		for _, order := range orderRecords {
			if order.Status == string(BacktesterOrderStatusPending) && order.AccountType == string(accountType) {
				orders = append(orders, order)
			}
		}
	}

	return orders, nil
}

func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		orders:       make(map[uuid.UUID][]*BacktesterOrder),
		orderRecords: make(map[uuid.UUID][]*OrderRecord),
		playgrounds:  make(map[uuid.UUID]IPlayground),
		liveAccounts: make(map[uuid.UUID]*LiveAccount),
	}
}
